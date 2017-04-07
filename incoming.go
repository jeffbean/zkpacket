package main

import (
	"errors"
	"time"

	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type opTime struct {
	time   time.Time
	opCode OpType
	watch  bool
}

func (o *opTime) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddString("opName", o.opCode.String())
	kv.AddBool("watch", o.watch)
	kv.AddString("time", o.time.String())
	return nil
}

var errBufferTooShort = errors.New("buffer too short")

func processIncomingOperation(client *client, header *requestHeader, buf []byte) (*opTime, error) {
	// This section is breaking up how to process different request types all based on the header operation
	// We have a few special cases where we want to see metrics for watchs and multi operations
	ot := &opTime{opCode: header.Opcode, watch: false}
	l := logger.With(zap.Object("header", header))

	var res interface{}
	var err error

	switch header.Opcode {
	case OpPing:
	case OpNotify:
		if header.Xid == 0 {
			res = &connectRequest{}
			if _, err := zk.DecodePacket(buf, res); err != nil {
				return ot, err
			}
			return ot, nil
		}
		res, err = processOperation(OpNotify, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return ot, err
		}
	case OpMulti:
		res, err = processMultiOperation(buf[8:])
		if err != nil {
			return ot, err
		}
	case OpGetData:
		res := &getDataRequest{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return ot, err
		}
		ot.watch = res.Watch
	case OpGetChildren2:
		res := &getChildren2Request{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return nil, err
		}
		ot.watch = res.Watch
	case OpExists:
		res := &existsRequest{}
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
