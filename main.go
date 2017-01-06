package main

// Use tcpdump to create a test file
// tcpdump -w test.pcap
// or use the example above for writing pcap files

import (
	"fmt"
	"log"
	"net"
	"time"
	"errors"

	"github.com/google/gopacket/pcap"
	"github.com/google/gopacket/layers"
	"github.com/google/gopacket"
	"github.com/samuel/go-zookeeper/zk"
)

const zkDefaultPort = 2181

var (
	pcapFile    = "testing_zk.pcap"
	device      = "lo0"
	snapshotLen int32 = 1024
	promiscuous = false
	err         error
	timeout     = 30 * time.Second
	handle      *pcap.Handle

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
	handle, err = pcap.OpenOffline(pcapFile)

	 //handle, err = pcap.OpenLive(device, snapshotLen, promiscuous, timeout)
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
	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer != nil {
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
	header := &zk.RequestHeader{}
	_, err := zk.DecodePacket(buf[:8], header)
	if err != nil {
		return err
	}

	client := &client{host: ip.SrcIP, port: tcp.SrcPort, xid: header.Xid}

	rMap[client.String()] = header.Opcode
	if header.Xid == 0 && header.Opcode == 0 {
		res := &zk.ConnectRequest{}
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
	// blen := int(binary.BigEndian.Uint32(buf[:4]))
	// fmt.Printf("%#v\n", buf[16:])
	if ip == nil || tcp == nil {
		return errors.New("ip or tcp layer not detected")
	}
	header := &zk.ResponseHeader{}
	_, err = zk.DecodePacket(buf[:16], header)
	if err != nil {
		return err
	}
	client := &client{host: ip.SrcIP, port: tcp.DstPort, xid: header.Xid}
	// fmt.Printf("Tracked Client: %v\n", client)
	operation, found := rMap[client.String()]
	if found && operation != 0 {
		// fmt.Printf("Tracked Op: %v\n", operation)
		rStruct := zk.RequestStructForOp(operation)
		_, err = zk.DecodePacket(buf[16:], rStruct)
		if err != nil {
			return err
		}
		fmt.Printf("<= Server: %#v", rStruct)
		return nil
	}

	if header.Xid == 0 {
		res := &zk.ConnectResponse{}
		_, err = zk.DecodePacket(buf, res)
		if err != nil {
			return err
		}
		fmt.Printf("<xxx Server connect: %#v", res)
		return nil
	}
	if header.Xid == -1 {
		res := &zk.WatcherEvent{}
		_, err := zk.DecodePacket(buf[16:], res)
		if err != nil {
			return err
		}
	}
	fmt.Println("WHAA")

	return nil
}
