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

	"github.com/fatih/color"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/grafana/grafana/pkg/cmd/grafana-cli/logger"
	"github.com/jeffbean/go-zookeeper/zk"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

const zkDefaultPort = 2181

var (
	flagNoColor = flag.Bool("no-color", false, "Disable color output")
	device      = flag.String("interface", "eth0", "interface to listen on")

	// metrics
	addr = flag.String("listen-address", ":8085", "The address to listen on for HTTP requests.")

	// output is how we communicate with the user the main content
	output io.Writer = os.Stdout
	// clientOutput is the clientside communication colored to be easy to read
	clientOutput = color.New(color.FgYellow)
	// serverOutput
	serverOutput = color.New(color.FgBlue)
	// logger to show any messages to the user
	sugar *zap.SugaredLogger
	dl    = zap.NewAtomicLevel()
	// device is the listening interface to listen on
	snapshotLen int32 = 1024
	timeout           = -1 * time.Second
)

type client struct {
	host net.IP
	port layers.TCPPort
	xid  int32
	time time.Time
}

func (c *client) String() string {
	return fmt.Sprintf("%v:%v:%v", c.host, c.port, c.xid)
}

type clientResquestMap map[string]int32

func main() {
	flag.Parse()
	if *flagNoColor {
		color.NoColor = true // disables colorized output
	}
	loggerConfig := zap.NewDevelopmentConfig()
	logger, _ := loggerConfig.Build()
	sugar = logger.Sugar()

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
		if err := printPacketInfo(packet, rMap); err != nil {
			fmt.Fprintf(output, "error %v\n", err)
		}
	}
}

func printPacketInfo(packet gopacket.Packet, rMap clientResquestMap) error {
	// Check for errors
	if err := packet.ErrorLayer(); err != nil {
		fmt.Fprintf(output, "Error decoding some part of the packet: %v", err)
		return nil
	}
	var tcp *layers.TCP
	var ip *layers.IPv4
	// Let's see if the packet is TCP
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		tcp, _ = tcpLayer.(*layers.TCP)
	}
	ipLayer := packet.LayerClass(layers.LayerClassIPNetwork)
	if ipLayer != nil {
		// TODO: check if its v4 before casting it based on returned Layer from LayerClass call
		ip, _ = ipLayer.(*layers.IPv4)
	}
	applicationLayer := packet.ApplicationLayer()
	if applicationLayer != nil {
		appPayload := applicationLayer.Payload()
		if tcpLayer != nil && tcp != nil {
			if tcp.SrcPort == zkDefaultPort {
				if err := handleResponce(ip, tcp, appPayload[4:], rMap); err != nil {
					return err
				}
			}
			if tcp.DstPort == zkDefaultPort {
				if err := handleClient(ip, tcp, appPayload[4:], rMap); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func handleClient(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {
	if ip == nil || tcp == nil {
		return errors.New("ip or tcp layer not detected")
	}
	header := &requestHeader{}
	if _, err := zk.DecodePacket(buf[:8], header); err != nil {
		return err
	}
	operationCounter.With(prometheus.Labels{"operation": fmt.Sprintf("%v", header.Opcode)}).Inc()

	if header.Opcode == 11 {
		return nil
	}
	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}

	rMap[client.String()] = header.Opcode
	if header.Xid == 0 && header.Opcode == 0 {
		res := &connectRequest{}
		if _, err := zk.DecodePacket(buf, res); err != nil {
			return err
		}
		clientOutput.Fprintf(output, "xxx> Connect Client: %#v\n", res)
		return nil
	}
	rStruct := zk.RequestStructForOp(header.Opcode)
	if _, err := zk.DecodePacket(buf[8:], rStruct); err != nil {
		return err
	}
	clientOutput.Fprintf(output, "=> Client: %#v - %#v\n", header, rStruct)
	return nil
}

func handleResponce(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {
	if ip == nil || tcp == nil {
		return errors.New("ip or tcp layer not detected")
	}
	header := &responseHeader{}
	if _, err := zk.DecodePacket(buf[:16], header); err != nil {
		return err
	}
	// Thoery: This means the rest of the packet is blank
	// Have not proven it with tests just yet
	if header.Err < 0 {
		logger.Debug("responce error", zap.Object("header", header))
		color.New(color.FgRed).Fprintf(output, "<= Server: %v\n", header)
		return nil
	}
	if header.Xid == -2 {
		return nil
	}
	client := &client{host: ip.SrcIP, port: tcp.DstPort, xid: header.Xid}
	sugar.Debug("decoded responce", "header", header, "client", client)

	operation, found := rMap[client.String()]
	if found && operation != 0 {
		rStruct := zk.ResponseStructForOp(operation)
		if _, err := zk.DecodePacket(buf[16:], rStruct); err != nil {
			return errors.Wrapf(err, "responce struct attempt: %#v", buf)
		}
		serverOutput.Fprintf(output, "<= Server: %#v\n", rStruct)
		return nil
	}

	if header.Xid == 0 {
		res := &connectResponse{}
		if _, err := zk.DecodePacket(buf, res); err != nil {
			return err
		}
		serverOutput.Fprintf(output, "<xxx Server connect: %#v\n", res)
		return nil
	}

	if header.Xid == -1 {
		res := &watcherEvent{}
		if _, err := zk.DecodePacket(buf[16:], res); err != nil {
			return err
		}
	}
	return nil
}
