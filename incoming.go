package main

import (
	"errors"
	"time"

	"github.com/jeffbean/zkpacket/proto"

	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type opTime struct {
	time   time.Time
	opCode proto.OpType
	watch  bool
}

func (o *opTime) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddString("opName", o.opCode.String())
	kv.AddBool("watch", o.watch)
	kv.AddString("time", o.time.String())
	return nil
}

var errBufferTooShort = errors.New("buffer too short")

func processIncomingOperation(client *client, header *proto.RequestHeader, buf []byte) (*opTime, error) {
	// This section is breaking up how to process different request types all based on the header operation
	// We have a few special cases where we want to see metrics for watchs and multi operations
	ot := &opTime{opCode: header.Opcode, watch: false}
	l := logger.With(zap.Object("header", header))

	var res interface{}
	var err error

	switch header.Opcode {
	case proto.OpPing:
	case proto.OpNotify:
		if header.Xid == 0 {
			res = &proto.ConnectRequest{}
			if _, err := zk.DecodePacket(buf, res); err != nil {
				return ot, err
			}
			return ot, nil
		}
		res, err = processOperation(proto.OpNotify, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return ot, err
		}
	case proto.OpMulti:
		res, err = processMultiOperation(buf[8:])
		if err != nil {
			return ot, err
		}
	case proto.OpGetData:
		res := &proto.GetDataRequest{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return ot, err
		}
		ot.watch = res.Watch
	case proto.OpGetChildren2:
		res := &proto.GetChildren2Request{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return nil, err
		}
		ot.watch = res.Watch
	case proto.OpExists:
		res := &proto.ExistsRequest{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return nil, err
		}
		ot.watch = res.Watch
	default:
		if len(buf) < 8 {
			return nil, errBufferTooShort
		}
		res, err = processOperation(header.Opcode, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return nil, err
		}
	}
	l.Debug("--> processed incoming result", zap.Any("result", res))

	return ot, nil
}
