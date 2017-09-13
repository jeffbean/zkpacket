package proto

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
	// OpExists is zk connection Exists()
	OpExists
	// OpGetData is zk connection Get()
	OpGetData
	// OpSetData is zk connection Set()
	OpSetData

	// OpGetACL is zk connection GetACL()
	OpGetACL
	OpSetACL
	OpGetChildren
	OpSync // 9

	// OpPing is the zk client connection ping request
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
	// private ops to represent watch operations
	opGetDataW      OpType = 200
	opExistsW       OpType = 201
	opGetChildren2W OpType = 202

	// OpError is for specifying errors
	OpError OpType = -1
)

// MarshalLogObject renders the logging structure for the OpType
func (o OpType) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt32("code", int32(o))
	kv.AddString("name", o.String())
	return nil
}
