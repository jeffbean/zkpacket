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
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/uber-go/tally"
	promreporter "github.com/uber-go/tally/prometheus"
	"github.com/uber-go/zap"
)

const zkDefaultPort = 2181

var flagNoColor = flag.Bool("no-color", false, "Disable color output")

var (
	// output is how we communicate with the user the main content
	output io.Writer = os.Stdout
	// clientOutput is the clientside communication colored to be easy to read
	clientOutput = color.New(color.FgYellow)
	// serverOutput
	serverOutput = color.New(color.FgBlue)
	// logger to show any
	logger = zap.New(zap.NewTextEncoder())
	// device is the listening interface to listen on
	device            = "lo0"
	snapshotLen int32 = 1024
	timeout           = 5 * time.Second

	// Will reuse these for each packet
	ethLayer layers.Ethernet
	ipLayer  layers.IPv4
	tcpLayer layers.TCP
)

type client struct {
	host net.IP
	port layers.TCPPort
	xid  int32
}

func (c *client) String() string {
	return fmt.Sprintf("%v:%v:%v", c.host, c.port, c.xid)
}

type clientResquestMap map[string]int32

func main() {

	if *flagNoColor {
		color.NoColor = true // disables colorized output
	}

	r := promreporter.NewReporter(promreporter.Options{})
	// Note: `promreporter.DefaultSeparator` is "_".
	// Prometheus doesnt like metrics with "." or "-" in them.
	scope, closer := tally.NewCachedRootScope("zkpacket", map[string]string{}, r, 1*time.Second, promreporter.DefaultSeparator)
	defer closer.Close()

	counter := scope.Tagged(map[string]string{
		"cluster": "dev",
	}).Counter("teesting_counter")

	http.Handle("/metrics", r.HTTPHandler())
	fmt.Printf("Serving :8080/metrics\n")
	go http.ListenAndServe("localhost:8080", nil)

	handle, err := pcap.OpenLive(device, snapshotLen, false /* promiscuous */, timeout)
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
		if err := printPacketInfo(packet, rMap, counter); err != nil {
			fmt.Fprintf(output, "error %v\n", err)
		}
	}
}

func printPacketInfo(packet gopacket.Packet, rMap clientResquestMap, counter tally.Counter) error {
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
			counter.Inc(1)
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
		logger.Debug("responce error", zap.Marshaler("header", header))
		color.New(color.FgRed).Fprintf(output, "<= Server: %v\n", header)
		return nil
	}

	client := &client{host: ip.SrcIP, port: tcp.DstPort, xid: header.Xid}
	logger.Debug("decoded respnce", zap.Marshaler("header", header), zap.Object("client", client))

	operation, found := rMap[client.String()]
	if found && operation != 0 {
		rStruct := zk.ResponceStructForOp(operation)
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
