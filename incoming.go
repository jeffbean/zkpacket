package main

import (
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

func processIncomingOperation(client *client, header *requestHeader, buf []byte) (*opTime, error) {
	// This section is breaking up how to process different request types all based on the header operation
	// We have a few special cases where we want to see metrics for watchs and multi operations
	ot := &opTime{opCode: header.Opcode, watch: false}
	l := logger.With(zap.Object("header", header))
	switch header.Opcode {
	case OpPing:
	case OpNotify:
		if header.Xid == 0 {
			res := &connectRequest{}
			if _, err := zk.DecodePacket(buf, res); err != nil {
				return ot, err
			}
			l.Info("---> client connect", zap.Any("res", res))
			return ot, nil
		}
		res, err := processOperation(OpNotify, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return ot, err
		}
		l.Debug("--> client notify result", zap.Any("result", res))
	case OpMulti:
		res, err := processMultiOperation(buf[8:])
		if err != nil {
			return ot, err
		}
		l.Debug("--> client multi request", zap.Any("res", res))
	case OpGetData:
		res := &getDataRequest{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return ot, err
		}
		ot.watch = res.Watch
		l.Debug("--> client getData request", zap.Any("result", res))
		return ot, nil
	case OpGetChildren2:
		res := &getChildren2Request{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return nil, err
		}
		ot.watch = res.Watch

		l.Debug("--> client getChildren2Request request", zap.Any("result", res))
	case OpExists:
		res := &existsRequest{}
		if _, err := zk.DecodePacket(buf[8:], res); err != nil {
			return nil, err
		}
		ot.watch = res.Watch
		l.Debug("--> client getExist request", zap.Any("result", res))
	default:
		res, err := processOperation(header.Opcode, buf[8:], zk.RequestStructForOp)
		if err != nil {
			return nil, err
		}
		l.Debug("--> client request result", zap.Any("result", res))
	}
	return ot, nil
}
