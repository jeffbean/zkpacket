package main

// Use tcpdump to create a test file
// tcpdump -w test.pcap
// or use the example above for writing pcap files

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket/pcap"
	"github.com/pkg/errors"
	"github.com/samuel/go-zookeeper/zk"
	"github.com/uber-go/zap"
)

const zkDefaultPort = 2181

var (
	device            = "lo0"
	snapshotLen int32 = 1024
	promiscuous       = false
	err         error
	timeout     = 30 * time.Second
	handle      *pcap.Handle
	logger      = zap.New(zap.NewTextEncoder())
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
	rMap := clientResquestMap{}
	// Open file instead of device
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// defer handle.Close()
	// // Open device
	// handle, err = pcap.OpenOffline(pcapFile)

	handle, err = pcap.OpenLive(device, snapshotLen, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// Set filter
	var filter = fmt.Sprintf("tcp and port %v", zkDefaultPort)
	err = handle.SetBPFFilter(filter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Only capturing TCP port %v packet\n", zkDefaultPort)

	// Loop through packets in file
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {
		if err := printPacketInfo(packet, rMap); err != nil {
			fmt.Printf("error %v\n", err)
		}
	}
}

func printPacketInfo(packet gopacket.Packet, rMap clientResquestMap) error {
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
		fmt.Println()
	}

	// // Check for errors
	if err := packet.ErrorLayer(); err != nil {
		fmt.Println("Error decoding some part of the packet:", err)
	}
	return nil
}

func handleClient(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {
	if ip == nil || tcp == nil {
		return errors.New("ip or tcp layer not detected")
	}
	header := &requestHeader{}
	_, err := zk.DecodePacket(buf[:8], header)
	if err != nil {
		return err
	}

	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}

	rMap[client.String()] = header.Opcode
	if header.Xid == 0 && header.Opcode == 0 {
		res := &connectRequest{}
		_, err = zk.DecodePacket(buf, res)
		if err != nil {
			return err
		}
		fmt.Printf("xxx> Connect Client: %#v", res)
		return nil
	}
	rStruct := zk.RequestStructForOp(header.Opcode)
	_, err = zk.DecodePacket(buf[8:], rStruct)
	if err != nil {
		return err
	}
	fmt.Printf("=> Client: %#v", rStruct)
	return nil
}

func handleResponce(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {
	if ip == nil || tcp == nil {
		return errors.New("ip or tcp layer not detected")
	}
	header := &responseHeader{}
	_, err = zk.DecodePacket(buf[:16], header)
	if err != nil {
		return err
	}
	// Thoery: This means the rest of the packet is blank
	if header.Err < 0 {
		logger.Debug("responce error", zap.Marshaler("header", header))
		fmt.Printf("<= Server: %#v", header)
		return nil
	}

	client := &client{host: ip.SrcIP, port: tcp.DstPort, xid: header.Xid}
	logger.Debug("decoded respnce", zap.Marshaler("header", header), zap.Object("client", client))

	// fmt.Printf("Tracked Client: %v\n", client)
	operation, found := rMap[client.String()]
	if found && operation != 0 {
		// fmt.Printf("Tracked Operation: %v\n", operation)
		rStruct := zk.ResponceStructForOp(operation)
		_, err = zk.DecodePacket(buf[16:], rStruct)
		if err != nil {
			return errors.Wrapf(err, "responce struct attempt: %#v", buf[16:])
		}
		fmt.Printf("<= Server: %#v", rStruct)
		return nil
	}

	if header.Xid == 0 {
		res := &connectResponse{}
		_, err = zk.DecodePacket(buf, res)
		if err != nil {
			return err
		}
		fmt.Printf("<xxx Server connect: %#v", res)
		return nil
	}

	if header.Xid == -1 {
		res := &watcherEvent{}
		_, err := zk.DecodePacket(buf[16:], res)
		if err != nil {
			return err
		}
	}
	return nil
}
