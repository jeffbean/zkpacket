package main

import (
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Create a layer type, should be unique and high, so it doesn't conflict,
// giving it a name and a decoder to use.
var ZookeeperLayerType = gopacket.RegisterLayerType(1200, gopacket.LayerTypeMetadata{"ZookeeperLayer", gopacket.DecodeFunc(decodeZookeeperLayer)})

// Implement my layer
type ZookeeperLayer struct {
	DataSize []byte
	payload []byte
}
func (m ZookeeperLayer) LayerType() gopacket.LayerType { return ZookeeperLayerType }
func (m ZookeeperLayer) LayerContents() []byte { return m.DataSize }
func (m ZookeeperLayer) LayerPayload() []byte { return m.payload }

// Now implement a decoder... this one strips off the first 4 bytes of the
// packet.
func decodeZookeeperLayer(data []byte, p gopacket.PacketBuilder) error {
	// Create my layer
	p.AddLayer(&ZookeeperLayer{data[:4], data[4:]})
	// Determine how to handle the rest of the packet
	return p.NextDecoder(layers.LayerTypeEthernet)
}
