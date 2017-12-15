package proto

import "github.com/jeffbean/go-zookeeper/zk"

// ResponseHeader is the first bytes for all ZK response packets
type ResponseHeader struct {
	Xid  int32
	Zxid int64
	Err  zk.ErrCode
}

// ConnectResponse is the packet from ZK server connection request
type ConnectResponse struct {
	ProtocolVersion int32
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}
