package proto

import "go.uber.org/zap/zapcore"

// RequestHeader is the first bytes for all request packets
type RequestHeader struct {
	Xid    int32
	Opcode OpType
}

// ConnectRequest is the packet bytes struct for a connection request
type ConnectRequest struct {
	ProtocolVersion int32
	LastZxidSeen    int64
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type GetDataRequest pathWatchRequest

type GetChildren2Request pathWatchRequest

type ExistsRequest pathWatchRequest

type multiRequestOp struct {
	Header multiHeader
	Op     interface{}
}
type multiRequest struct {
	Ops        []multiRequestOp
	DoneHeader multiHeader
}

// MarshalLogObject renders the log message for the header
func (r *RequestHeader) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt("xid", int(r.Xid))
	kv.AddInt32("opcode", int32(r.Opcode))
	kv.AddString("opName", r.Opcode.String())
	return nil
}
