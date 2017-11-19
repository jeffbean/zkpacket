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
	"strconv"
	"time"

	"github.com/jeffbean/zkpacket/proto"

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

func processZookeeperPackets(packet gopacket.Packet, rMap clientResquestMap) {
	// In this hot path we want to return as soon as we know anything is not going through

	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		logger.Error("error layer found in packet", zap.Error(err.Error()))
		return
	}

	tcp, ip, err := castLayers(packet)
	if err != nil {
		logger.Error("failed casting required packet layers", zap.Error(err))
		return
	}

	applicationLayer := packet.ApplicationLayer()
	if applicationLayer == nil {
		// We dont log here since this can be a multitide of packets
		return
	}
	appPayload := applicationLayer.Payload()

	// For Zookeeper the first 4 bytes is the payload size. We ignore it for now.
	// TODO: convert byte slice into float for metrics
	// zkPayloadSize := appPayload[4:]
	// packetSizeHistogram.Observe(zkPayloadSize)

	// TODO: add the ablity to swap this logic if you want to sniff on a client
	// if the source port is ZK port, we treat everything as a server request
	if tcp.SrcPort == zkDefaultPort {
		if err := handleOutgoing(ip, tcp, appPayload[4:], rMap, packet.Metadata()); err != nil {
			logger.Error("error processing packet", zap.Error(err))
			return
		}
	}
	// If we detect the destination is the ZK port we treat this as an incoming client call.
	if tcp.DstPort == zkDefaultPort {
		if err := handleIncoming(ip, tcp, appPayload[4:], rMap, packet.Metadata()); err != nil {
			logger.Error("error processing packet", zap.Error(err))
			return
		}
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

func handleIncoming(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap, metaData *gopacket.PacketMetadata) error {
	// The incoming packets all have headers. the only relaible part that we can then determine how to decode the packet payload
	header := &proto.RequestHeader{}
	if _, err := zk.DecodePacket(buf[:8], header); err != nil {
		logger.Error("--> failed to decode header", zap.Error(err), zap.Binary("first-eight-bytes", buf[:8]))
		return err
	}

	// TODO: Add metric for even pings?
	// This is the pingRequest. lets ignore for now
	if header.Opcode == proto.OpPing {
		return nil
	}
	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}

	ot, err := processIncomingOperation(client, header, buf)
	if err != nil {
		logger.Error("failed to process incoming operation", zap.Error(err))
	}
	ot.time = metaData.Timestamp
	operationCounter.With(
		prometheus.Labels{
			"operation": header.Opcode.String(),
			"direction": "incoming",
			"watch":     strconv.FormatBool(ot.watch),
		},
	).Inc()

	rMap[client.String()] = ot
	// logger.Debug("--> incoming tracking operation", zap.Object("trackingOperation", ot))
	return nil
}

func handleOutgoing(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap, packetTime *gopacket.PacketMetadata) error {
	header := &proto.ResponseHeader{}
	if _, err := zk.DecodePacket(buf[:16], header); err != nil {
		return err
	}
	l := logger.With(zap.Object("h", header))
	// Thoery: This means the rest of the packet is blank
	// Have not proven it with tests just yet
	if header.Err < 0 {
		l.Warn("<-- responce error")
		return nil
	}

	// Dont track the ping reponces
	if header.Xid == -2 {
		return nil
	}
	switch header.Xid {
	case 0:
		res := &proto.ConnectResponse{}
		if _, err := zk.DecodePacket(buf, res); err != nil {
			return err
		}
		l.Debug("<-- connect", zap.Stringer("src", ip.SrcIP), zap.Any("responce", res))
		// serverOutput.Fprintf(output, "<xxx Server connect: %#v\n", res)
		return nil
	case -1:
		// Watch event
		// TODO: Impliment watch tracking
		/// i dont think its possible without much more work on tracking paths as well as xids.
		// {"h": {"xid": -1, "zxid": -1, "errorCode": 0, "errorMsg": ""}, "res": {"type": 3, "path": "/node-299352457"}}
		res := &proto.WatcherEvent{}
		_, err := zk.DecodePacket(buf[16:], res)
		if err != nil {
			return err
		}
		l.Info("<-- watcher event notification", zap.Any("res", res))
		operationCounter.With(prometheus.Labels{
			"operation": "watch_notification",
			"direction": "outgoing",
			"watch":     "false",
		}).Inc()
		return nil
	}

	client := &client{host: ip.DstIP, port: tcp.DstPort, xid: header.Xid}
	// see if we have a client request for this server reply
	operation, found := rMap[client.String()]

	if found && operation.opCode != 0 {
		l.Debug("<-- outgoing operation found",
			zap.Stringer("src", ip.SrcIP),
			zap.Stringer("client", client),
		)
		opSeconds := packetTime.Timestamp.Sub(operation.time).Seconds()
		operationCounter.With(
			prometheus.Labels{
				"operation": operation.opCode.String(),
				"direction": "outgoing",
				"watch":     strconv.FormatBool(operation.watch),
			},
		).Inc()
		operationHistogram.With(
			prometheus.Labels{"operation": operation.opCode.String()},
		).Observe(opSeconds)

		switch operation.opCode {
		case proto.OpMulti:
			res, err := processMultiOperation(buf[16:])
			if err != nil {
				return err
			}
			l.Debug("<-- multi responce", zap.Reflect("res", res), zap.Object("op", operation.opCode))
		default:
			res, err := processOperation(operation.opCode, buf[16:], zk.ResponseStructForOp)
			if err != nil {
				return err
			}
			l.Debug("<-- outgoing responce", zap.Any("struct", res))
		}
		delete(rMap, client.String())
		return nil
	}
	l.Warn("detected server packet with no tracked request, unable to decode.")
	return nil
}

func processOperation(op proto.OpType, buf []byte, cb func(int32) interface{}) (interface{}, error) {
	rStruct := cb(int32(op))
	// logger.Debug("found struct for operation", zap.Object("op", op), zap.Reflect("struct", rStruct))

	if _, err := zk.DecodePacket(buf, rStruct); err != nil {
		logger.Error("failed to decode struct", zap.Error(err), zap.Object("op", op), zap.Binary("payload", buf))
		return rStruct, err
	}
	return rStruct, nil
}

func processMultiOperation(buf []byte) (*proto.MultiResponse, error) {
	mHeader := &proto.MultiResponse{}

	_, err := mHeader.Decode(buf)
	if err != nil {
		return nil, err
	}
	// logger.Debug("process multi operation", zap.Int("offset", offset), zap.Any("multiResponse", mHeader))
	return mHeader, nil
}
