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
	"github.com/samuel/go-zookeeper/zk"
	"github.com/samuel/go-zookeeper/zk/proto"
)

var (
	pcapFile    = "zk-get.pcap"
	device      = "lo0"
	snapshotLen = 1024
	promiscuous = false
	err         error
	timeout     = 30 * time.Second
	handle      *pcap.Handle
	// Will reuse these for each packet
	ethLayer layers.Ethernet
	ipLayer  layers.IPv4
	tcpLayer layers.TCP
)
var logger log.Logger

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
	handle, err = pcap.OpenOffline(pcapFile)

	// handle, err = pcap.OpenLive(device, snapshot_len, promiscuous, timeout)
	if err != nil {
		log.Fatal(err)
	}
	defer handle.Close()

	// Set filter
	var filter = fmt.Sprintf("tcp and port %v", DefaultPort)
	err = handle.SetBPFFilter(filter)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Only capturing TCP port %v packet\n", DefaultPort)

	// Loop through packets in file
	packetSource := gopacket.NewPacketSource(handle, handle.LinkType())
	for packet := range packetSource.Packets() {

		printPacketInfo(packet, rMap)
	}
}

func printPacketInfo(packet gopacket.Packet, rMap clientResquestMap) error {
	var tcp *layers.TCP
	var ip *layers.IPv4
	// Let's see if the packet is TCP
	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer != nil {
		// fmt.Println("TCP layer detected.")
		tcp, _ = tcpLayer.(*layers.TCP)
	}
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
		ip, _ = ipLayer.(*layers.IPv4)
	}

	applicationLayer := packet.ApplicationLayer()
	if applicationLayer != nil {
		appPayload := applicationLayer.Payload()
		if tcpLayer != nil && tcp != nil {
			if tcp.SrcPort == DefaultPort {
				handleResponce(ip, tcp, appPayload[4:], rMap)
			}
			if tcp.DstPort == DefaultPort {
				handleClient(ip, tcp, appPayload[4:], rMap)
			}
		}

		// fmt.Printf("length of slice: %v\n", len(applicationLayer.Payload()))

		// fmt.Printf("%v\n", applicationLayer.Payload())
		// Search for a string inside the payload
		fmt.Println()
	}

	// // Check for errors
	if err := packet.ErrorLayer(); err != nil {
		fmt.Println("Error decoding some part of the packet:", err)
	}
	return nil
}

func handleClient(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {

	header := &proto.RequestHeader{}
	_, err := proto.DecodePacket(buf[:8], header)
	if err != nil {
		return err
	}
	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}

	rMap[client.String()] = header.Opcode
	if header.Xid == 0 && header.Opcode == 0 {
		res := &proto.ConnectRequest{}
		_, err = proto.DecodePacket(buf, res)
		if err != nil {
			return err
		}
		fmt.Printf("xxx> Connect Client: %#v", res)
		return nil
	}
	rStruct := proto.RequestStructForOp(header.Opcode)
	_, err = proto.DecodePacket(buf[8:], rStruct)
	if err != nil {
		return err
	}
	fmt.Printf("=> Client: %#v", rStruct)
	return nil
}

func handleResponce(ip *layers.IPv4, tcp *layers.TCP, buf []byte, rMap clientResquestMap) error {
	// blen := int(binary.BigEndian.Uint32(buf[:4]))
	// fmt.Printf("%#v\n", buf[16:])

	header := &proto.ResponseHeader{}
	_, err = proto.DecodePacket(buf[:16], header)
	if err != nil {
		return err
	}
	client := &client{host: ip.SrcIP, port: tcp.DstPort, xid: header.Xid}
	// fmt.Printf("Tracked Client: %v\n", client)
	operation, found := rMap[client.String()]
	if found && operation != 0 {
		// fmt.Printf("Tracked Op: %v\n", operation)
		rStruct := proto.RequestStructForOp(operation)
		_, err = proto.DecodePacket(buf[16:], rStruct)
		if err != nil {
			return err
		}
		fmt.Printf("<= Server: %#v", rStruct)
		return nil
	}

	if header.Xid == 0 {
		res := &proto.ConnectResponse{}
		_, err = proto.DecodePacket(buf, res)
		if err != nil {
			return err
		}
		fmt.Printf("<xxx Server connect: %#v", res)
		return nil
	}
	if header.Xid == proto.WatchXID {
		res := &proto.WatcherEvent{}
		_, err := proto.DecodePacket(buf[16:], res)
		if err != nil {
			return err
		}
	}
	fmt.Printf("WHAA")

	return nil
}
