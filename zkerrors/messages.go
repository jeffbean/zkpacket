package zkerrors

import "github.com/jeffbean/go-zookeeper/zk"

const (
	// ErrOk The OK Error code from ZK packets
	ErrOk = 0
	// System and server-side errors
	errSystemError          = -1
	errRuntimeInconsistency = -2
	errDataInconsistency    = -3
	errConnectionLoss       = -4
	errMarshallingError     = -5
	errUnimplemented        = -6
	errOperationTimeout     = -7
	errBadArguments         = -8
	errInvalidState         = -9

	// API errors
	errAPIError                zk.ErrCode = -100
	errNoNode                  zk.ErrCode = -101 // *
	errNoAuth                  zk.ErrCode = -102
	errBadVersion              zk.ErrCode = -103 // *
	errNoChildrenForEphemerals zk.ErrCode = -108
	errNodeExists              zk.ErrCode = -110 // *
	errNotEmpty                zk.ErrCode = -111
	errSessionExpired          zk.ErrCode = -112
	errInvalidCallback         zk.ErrCode = -113
	errInvalidACL              zk.ErrCode = -114
	errAuthFailed              zk.ErrCode = -115
	errClosing                 zk.ErrCode = -116
	errNothing                 zk.ErrCode = -117
	errSessionMoved            zk.ErrCode = -118
)

var errCodeToString = map[zk.ErrCode]string{
	ErrOk:                      "",
	errAPIError:                "api error",
	errNoNode:                  "node does not exist",
	errNoAuth:                  "not authenticated",
	errBadVersion:              "version conflict",
	errNoChildrenForEphemerals: "ephemeral nodes may not have children",
	errNodeExists:              "node already exists",
	errNotEmpty:                "node has children",
	errSessionExpired:          "session has been expired by the server",
	errInvalidACL:              "invalid ACL specified",
	errAuthFailed:              "client authentication failed",
	errClosing:                 "zookeeper is closing",
	errNothing:                 "no server responsees to process",
	errSessionMoved:            "session moved to another server, so operation is ignored",
}

// ZKErrCodeToMessage converts the ZK error code to a message
func ZKErrCodeToMessage(ec zk.ErrCode) string {
	if errString, ok := errCodeToString[ec]; ok {
		return errString
	}
	return "unknown error"
}
