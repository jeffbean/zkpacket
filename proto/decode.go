package proto

import (
	"encoding/binary"
	"errors"
	"reflect"

	"github.com/jeffbean/zkpacket/zkerrors"

	"github.com/jeffbean/go-zookeeper/zk"
	"go.uber.org/zap/zapcore"
)

type pathWatchRequest struct {
	Path  string
	Watch bool
}

// We special case these so we track if these are watching actions
type existsRequest pathWatchRequest
type getChildren2Request pathWatchRequest

type multiHeader struct {
	Type OpType
	Done bool
	Err  zk.ErrCode
}

type multiResponse struct {
	Ops        []multiResponseOp
	DoneHeader multiHeader
}

type multiResponseOp struct {
	Header multiHeader
	String string
	Stat   *zk.Stat
	Err    zk.ErrCode
}

type watcherEvent struct {
	Type  zk.EventType
	State zk.State
	Path  string
}

type watchType int

const (
	watchTypeData = iota
	watchTypeExist
	watchTypeChild
)

type watchPathType struct {
	path  string
	wType watchType
}

type decoder interface {
	Decode(buf []byte) (int, error)
}

func (r *multiResponse) Decode(buf []byte) (int, error) {
	var multiErr error

	r.Ops = make([]multiResponseOp, 0)
	r.DoneHeader = multiHeader{-1, true, -1}
	total := 0
	for {
		header := &multiHeader{}
		n, err := decodePacketValue(buf[total:], reflect.ValueOf(header))
		if err != nil {
			return total, err
		}
		total += n
		if header.Done {
			r.DoneHeader = *header
			break
		}

		res := multiResponseOp{Header: *header}
		var w reflect.Value
		switch header.Type {
		default:
			return total, zk.ErrAPIError
		case OpError:
			w = reflect.ValueOf(&res.Err)
		case OpCreate:
			w = reflect.ValueOf(&res.String)
		case OpSetData:
			res.Stat = new(zk.Stat)
			w = reflect.ValueOf(res.Stat)
		case OpCheck, OpDelete:
		}
		if w.IsValid() {
			n, err := decodePacketValue(buf[total:], w)
			if err != nil {
				return total, err
			}
			total += n
		}
		r.Ops = append(r.Ops, res)
		if multiErr == nil && res.Err != zkerrors.ErrOk {
			// Use the first error as the error returned from Multi().
			multiErr = errors.New(zkerrors.ZKErrCodeToMessage(res.Err))
		}
	}
	return total, multiErr
}

func decodePacketValue(buf []byte, v reflect.Value) (int, error) {
	rv := v
	kind := v.Kind()
	if kind == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
		kind = v.Kind()
	}

	n := 0
	switch kind {
	default:
		return n, zk.ErrUnhandledFieldType
	case reflect.Struct:
		if de, ok := rv.Interface().(decoder); ok {
			return de.Decode(buf)
		} else if de, ok := v.Interface().(decoder); ok {
			return de.Decode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := decodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		v.SetBool(buf[n] != 0)
		n++
	case reflect.Int32:
		v.SetInt(int64(binary.BigEndian.Uint32(buf[n : n+4])))
		n += 4
	case reflect.Int64:
		v.SetInt(int64(binary.BigEndian.Uint64(buf[n : n+8])))
		n += 8
	case reflect.String:
		ln := int(binary.BigEndian.Uint32(buf[n : n+4]))
		v.SetString(string(buf[n+4 : n+4+ln]))
		n += 4 + ln
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := int(binary.BigEndian.Uint32(buf[n : n+4]))
			n += 4
			values := reflect.MakeSlice(v.Type(), count, count)
			v.Set(values)
			for i := 0; i < count; i++ {
				n2, err := decodePacketValue(buf[n:], values.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
		case reflect.Uint8:
			ln := int(int32(binary.BigEndian.Uint32(buf[n : n+4])))
			if ln < 0 {
				n += 4
				v.SetBytes(nil)
			} else {
				bytes := make([]byte, ln)
				copy(bytes, buf[n+4:n+4+ln])
				v.SetBytes(bytes)
				n += 4 + ln
			}
		}
	}
	return n, nil
}

func (r *multiHeader) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddBool("done", r.Done)
	kv.AddInt32("opcode", int32(r.Type))
	kv.AddString("opName", r.Type.String())
	kv.AddInt("errorCode", int(r.Err))
	kv.AddString("errorMsg", zkerrors.ZKErrCodeToMessage(r.Err))
	return nil
}

func (r *watcherEvent) MarshalLogObject(kv zapcore.ObjectEncoder) error {
	kv.AddInt32("type", int32(r.Type))
	kv.AddString("path", r.Path)
	return nil
}
