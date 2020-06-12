package main

import (
	"net"
	"testing"

	"github.com/jeffbean/zkpacket/proto"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestPassingTest(t *testing.T) {
	assert.Equal(t, true, true)
}

// func TestProcessIncomingOperation(t *testing.T) {
// 	logger = zap.NewNop()
// 	fakeClient := &client{host: net.ParseIP("127.0.0.5"), port: 10, xid: 10}
// 	fakeRequestHeader := &requestHeader{
// 		Xid:    10,
// 		Opcode: OpCreate,
// 	}
// 	fakeCreateRequest := &zk.CreateRequest{
// 		Path:  "/foo",
// 		Data:  []byte("foo"),
// 		Acl:   []zk.ACL{zk.ACL{Perms: 0, Scheme: "bar", ID: "bean"}},
// 		Flags: 0,
// 	}
// 	fakeBuffer := []byte(fakeCreateRequest.Path)
// 	fakeBuffer = append(fakeBuffer, []byte(fakeCreateRequest.Data)...)
// 	for _, acl := range fakeCreateRequest.Acl {
// 		fakeBuffer = append(fakeBuffer, []byte(acl.Perms)...)
// 		fakeBuffer = append(fakeBuffer, []byte(acl.Scheme)...)
// 		fakeBuffer = append(fakeBuffer, []byte(acl.ID)...)

// 	}
// 	fakeBuffer = append(fakeBuffer, []byte(fakeCreateRequest.Flags)...)

// 	opTimeThing, err := processIncomingOperation(fakeClient, fakeRequestHeader, fakeBuffer)
// 	assert.NoError(t, err)
// 	wantOpTime := &opTime{opCode: OpCreate, watch: false}
// 	assert.Equal(t, wantOpTime, opTimeThing)
// }

func TestProcessIncomingOperationNoPayload(t *testing.T) {
	logger = zap.NewNop()
	fakeClient := &client{host: net.ParseIP("127.0.0.5"), port: 10, xid: 10}
	fakeRequestHeader := &proto.RequestHeader{
		Xid:    10,
		Opcode: proto.OpCreate,
	}
	fakeBuffer := []byte{}
	_, err := processIncomingOperation(fakeClient, fakeRequestHeader, fakeBuffer)
	assert.Error(t, err, errBufferTooShort)
}
