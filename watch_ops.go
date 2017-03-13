package main

import "go.uber.org/zap/zapcore"

// watchOpType is the type of ZK operation but has registred a watch. Used to track operation metrics
//go:generate stringer -type=watchOpType
type watchOpType OpType

const (
	opGetDataW watchOpType = iota
	opExistsW
	opGetChildren2W
)

// MarshalLogObject renders the logging structure for the OpType
func (o watchOpType) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt32("code", int32(o))
	kv.AddString("name", o.String())
	return nil
}
