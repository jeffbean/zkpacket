package proto

import (
	"fmt"

	"github.com/jeffbean/zkpacket/zkerrors"

	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap/zapcore"
)

// ResponseHeader is the first bytes for all ZK response packets
type responseHeader struct {
	Xid  int32
	Zxid int64
	Err  zk.ErrCode
}

type connectResponse struct {
	ProtocolVersion int32
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

// MarshalLogObject renders the  header for logging
func (r *responseHeader) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt("xid", int(r.Xid))
	kv.AddInt64("zxid", r.Zxid)
	kv.AddInt("errorCode", int(r.Err))
	kv.AddString("errorMsg", zkerrors.ZKErrCodeToMessage(r.Err))
	return nil
}

func (r *responseHeader) String() string {
	return fmt.Sprintf("XID: %v, ZXID: %v, Err: %v", r.Xid, r.Zxid, zkerrors.ZKErrCodeToMessage(r.Err))
}
