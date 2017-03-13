package main

import "go.uber.org/zap/zapcore"

// Based on ZK 3.5 https://github.com/apache/zookeeper/blob/branch-3.5/src/java/main/org/apache/zookeeper/ZooDefs.java

// OpType is the type of ZK operation. Used to track operation metrics
//go:generate stringer -type=OpType
type OpType int32

const (
	// OpNotify is for watch notifications
	OpNotify OpType = iota
	// OpCreate is zk connection Create()
	OpCreate
	// OpDelete is zk connection Delete()
	OpDelete

	OpExists
	OpGetData
	OpSetData // 5

	OpGetACL
	OpSetACL
	OpGetChildren
	OpSync // 9

	OpPing OpType = iota + 1 // 11
	OpGetChildren2
	OpCheck
	OpMulti

	OpCreate2 // 15
	OpReconfig
	OpCheckWatches
	OpRemoveWatches
	OpCreateContainer

	OpDeleteContainer // 20
	OpCreateTTL

	OpCreateSession OpType = -12
	OpClose         OpType = -11

	OpSetAuth    OpType = 100
	OpSetWatches OpType = 101
	OpSasl       OpType = 102
	// OpError is for specifying errors
	OpError OpType = -1
)

// MarshalLogObject renders the logging structure for the OpType
func (o OpType) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt32("code", int32(o))
	kv.AddString("name", o.String())
	return nil
}
