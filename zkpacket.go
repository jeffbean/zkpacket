package main

import (
	"fmt"

	"github.com/samuel/go-zookeeper/zk"
	"github.com/uber-go/zap"
)

type requestHeader struct {
	Xid    int32
	Opcode int32
}

type responseHeader struct {
	Xid  int32
	Zxid int64
	Err  zk.ErrCode
}

type connectRequest struct {
	ProtocolVersion int32
	LastZxidSeen    int64
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type connectResponse struct {
	ProtocolVersion int32
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type watcherEvent struct {
	Type  zk.EventType
	State zk.State
	Path  string
}

func (r *responseHeader) MarshalLog(kv zap.KeyValue) error {
	kv.AddInt("xid", int(r.Xid))
	kv.AddInt64("Zxid", r.Zxid)
	kv.AddInt("errorCode", int(r.Err))
	kv.AddString("errorMsg", errCodeToMessage(r.Err))
	return nil
}

func (r *responseHeader) String() string {
	return fmt.Sprintf("%v", errCodeToMessage(r.Err))
}
