package main

// Use tcpdump to create a test file
// tcpdump -w test.pcap
// or use the example above for writing pcap files

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/jeffbean/go-zookeeper/zk"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

const zkDefaultPort = 2181

var (
	device = flag.String("interface", "eth0", "interface to listen on")

	// metrics
	addr = flag.String("listen-address", ":8085", "The address to listen on for HTTP requests.")

	// output is how we communicate with the user the main content
	output io.Writer = os.Stdout
	// logger to show any messages to the user
	logger *zap.Logger
	dl     = zap.NewAtomicLevel()
	// device is the listening interface to listen on
	snapshotLen int32 = 1024
	timeout           = -1 * time.Second

	tcp *layers.TCP
	ip  *layers.IPv4
)

type client struct {
	host net.IP
	port layers.TCPPort
	xid  int32
}

type opTime struct {
	time   time.Time
	opCode OpType
	watch  bool
}

func (o *opTime) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddString("opName", o.opCode.String())
	kv.AddBool("watch", o.watch)
	return nil
}

func (c *client) String() string {
	return fmt.Sprintf("%v:%v:%v", c.host, c.port, c.xid)
}

type clientResquestMap map[string]*opTime

func main() {
	flag.Parse()
	loggerConfig := zap.NewDevelopmentConfig()
	loggerConfig.EncoderConfig = zapcore.EncoderConfig{
		LevelKey:      "L",
		TimeKey:       "",
		MessageKey:    "M",
		NameKey:       "N",
		CallerKey:     "",
		StacktraceKey: "S",
		EncodeLevel:   zapcore.CapitalColorLevelEncoder,
	}
	logger, _ = loggerConfig.Build()
	// TODO: make this a flag for cmdline
	loggerConfig.Level.SetLevel(zap.DebugLevel)

	http.Handle("/metrics", promhttp.Handler())
	go http.ListenAndServe(*addr, nil)

	handle, err := pcap.OpenLive(*device, snapshotLen, false /* promiscuous */, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// Set filter for capture
	var filter = fmt.Sprintf("tcp and port %v", zkDefaultPort)
	if err := handle.SetBPFFilter(filter); err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(output, "Filter: %v\n", filter)
	rMap := clientResquestMap{}

	// Loop through packets in file
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		processZookeeperPackets(packet, rMap)
	}
}

func castLayers(packet gopacket.Packet) (*layers.TCP, *layers.IPv4, error) {
	// Need TCP to use the source and destination ports to see the driection of the packets
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	// Need Network info to track and inspect the IP info of the client and servers.
	ipLayer := packet.LayerClass(layers.LayerClassIPNetwork)

	if tcpLayer == nil || ipLayer == nil {
		// FIXME: check if its v4 before casting it based on returned Layer from LayerClass call
		return nil, nil, errors.New("required layers not found")
	}
	// Cast the layer to the struct
	tcp, _ = tcpLayer.(*layers.TCP)
	ip, _ = ipLayer.(*layers.IPv4)

	if tcp == nil || ip == nil {
		return nil, nil, errors.New("failed to cast required layers TCP or IP")
	}

	return tcp, ip, nil
}

func processZookeeperPackets(packet gopacket.Packet, rMap clientResquestMap) {
	// In this hot path we want to return as soon as we know anything is not going through

	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		logger.Error("error layer found in packet", zap.Error(err.Error()))
		return
	}

	tcp, ip, err := castLayers(packet)
	if err != nil {
		return
	}

	applicationLayer := packet.ApplicationLayer()
	if applicationLayer == nil {
		return
	}
	appPayload := applicationLayer.Payload()

	// For Zookeeper the first 4 bytes is the payload size. We ignore it for now.
	if tcp.SrcPort == zkDefaultPort {
		if err := handleResponce(ip, tcp, appPayload[4:], rMap, packet.Metadata()); err != nil {
			logger.Error("error processing packet", zap.Error(err))
			return
		}
	}
	if tcp.DstPort == zkDefaultPort {
		if err := handleClient(ip, tcp, appPayload[4:], rMap, packet.Metadata()); err != nil {
			logger.Error("error processing packet", zap.Error(err))
			return
		}
	}
}

func handleClient(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap, metaData *gopacket.PacketMetadata) error {
	header := &requestHeader{}
	if _, err := zk.DecodePacket(buf[:8], header); err != nil {
		logger.Error("--> failed to decode header", zap.Error(err), zap.Binary("first-eight-bytes", buf[:8]))
		return err
	}
	// TODO: Add metric for even pings?
	// This is the pingRequest. lets ignore for now
	if header.Opcode == OpPing {
		return nil
	}

	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}
	trackingOperation := &opTime{opCode: header.Opcode, time: metaData.Timestamp}
	// This section is breaking up how to process different request types all based on the header operation
	// We have a few special cases where we want to see metrics for watchs and multi operations
	switch header.Opcode {
	case OpPing:
	case OpNotify:
		if header.Xid == 0 {
			res := &connectRequest{}
			if _, err := zk.DecodePacket(buf, res); err != nil {
				return err
			}
			logger.Info("---> client connect", zap.Reflect("res", res), zap.Object("header", header))
			return nil
		}
		res, err := processOperation(OpNotify, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return err
		}
		logger.Debug("--> client notify result", zap.Object("header", header), zap.Any("result", res))
	case OpMulti:
		res, err := processMultiOperation(buf[8:])
		if err != nil {
			return err
		}
		logger.Debug("--> client multi request", zap.Reflect("res", res), zap.Object("header", header))
	case OpGetData:
		getData := &getDataRequest{}
		_, err := zk.DecodePacket(buf[8:], getData)
		if err != nil {
			return err
		}
		if getData.Watch {
			logger.Debug("I made it to a watch notification")
			trackingOperation = &opTime{opCode: OpType(opGetDataW), time: metaData.Timestamp, watch: getData.Watch}
		}
		logger.Debug("--> client getData request", zap.Object("header", header), zap.Any("result", getData), zap.Object("trackingOp", trackingOperation))
	case OpGetChildren2:
		getData := &getChildren2Request{}
		_, err := zk.DecodePacket(buf[8:], getData)
		if err != nil {
			return err
		}
		if getData.Watch {
			trackingOperation.opCode = OpType(opGetChildren2W)
		}
		logger.Debug("--> client getChildren2Request request", zap.Object("header", header), zap.Any("result", getData))
	case OpExists:
		req := &existsRequest{}
		_, err := zk.DecodePacket(buf[8:], req)
		if err != nil {
			return err
		}
		if req.Watch {
			trackingOperation.opCode = OpType(opExistsW)
		}
		logger.Debug("--> client getExist request", zap.Object("header", header), zap.Any("result", req))
	default:
		res, err := processOperation(header.Opcode, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return err
		}
		logger.Debug("--> client request result", zap.Object("header", header), zap.Any("result", res))
	}
	rMap[client.String()] = trackingOperation
	operationCounter.With(prometheus.Labels{"operation": trackingOperation.opCode.String()}).Inc()

	return nil
}

func handleResponce(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap, packetTime *gopacket.PacketMetadata) error {
	header := &responseHeader{}
	if _, err := zk.DecodePacket(buf[:16], header); err != nil {
		return err
	}
	// Thoery: This means the rest of the packet is blank
	// Have not proven it with tests just yet
	if header.Err < 0 {
		logger.Warn("<-- responce error", zap.Object("header", header))
		return nil
	}

	// Dont track the ping reponces
	if header.Xid == -2 {
		return nil
	}
	switch header.Xid {
	case 0:
		res := &connectResponse{}
		if _, err := zk.DecodePacket(buf, res); err != nil {
			return err
		}
		logger.Debug("<-- connect", zap.Stringer("src", ip.SrcIP), zap.Any("responce", res))
		// serverOutput.Fprintf(output, "<xxx Server connect: %#v\n", res)
		return nil
	case -1:
		// Watch event
		// TODO: Impliment watch tracking
		// logger.Warn("This code is condition ground zero")
		res := &watcherEvent{}
		_, err := zk.DecodePacket(buf[16:], res)
		if err != nil {
			return err
		}
		logger.Info("<-- watcher event notification", zap.Any("res", res))
	}

	client := &client{host: ip.DstIP, port: tcp.DstPort, xid: header.Xid}
	// see if we have a client request for this server reply
	operation, found := rMap[client.String()]
	logger.Debug("<-- server client operation found",
		zap.Object("header", header),
		zap.Stringer("src", ip.SrcIP),
		zap.Stringer("client", client),
	)
	if found && operation.opCode != 0 {
		opSeconds := packetTime.Timestamp.Sub(operation.time).Seconds()

		operationHistogram.With(
			prometheus.Labels{"operation": operation.opCode.String()},
		).Observe(opSeconds)
		switch operation.opCode {
		case OpMulti:
			res, err := processMultiOperation(buf[16:])
			if err != nil {
				return err
			}
			logger.Debug("<-- multi responce", zap.Reflect("res", res), zap.Object("op", operation.opCode))
		default:
			res, err := processOperation(operation.opCode, buf[16:], zk.ResponseStructForOp)
			if err != nil {
				return err
			}
			logger.Debug("<-- server responce", zap.Any("struct", res), zap.Object("header", header))
		}
		delete(rMap, client.String())
		return nil
	}
	return nil
}

func processOperation(op OpType, buf []byte, cb func(int32) interface{}) (interface{}, error) {
	rStruct := cb(int32(op))
	// logger.Debug("found struct for operation", zap.Object("op", op), zap.Reflect("struct", rStruct))

	if _, err := zk.DecodePacket(buf, rStruct); err != nil {
		logger.Error("failed to decode struct", zap.Error(err), zap.Object("op", op), zap.Binary("payload", buf))
		return rStruct, err
	}
	return rStruct, nil
}

func processMultiOperation(buf []byte) (*multiResponse, error) {
	mHeader := &multiResponse{}

	_, err := mHeader.Decode(buf)
	if err != nil {
		return nil, err
	}
	// logger.Debug("process multi operation", zap.Int("offset", offset), zap.Any("multiResponse", mHeader))
	return mHeader, nil
}
