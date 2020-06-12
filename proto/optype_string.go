// Code generated by "stringer -type=OpType"; DO NOT EDIT

package proto

import "fmt"

const (
	_OpType_name_0 = "OpCreateSessionOpClose"
	_OpType_name_1 = "OpErrorOpNotifyOpCreateOpDeleteOpExistsOpGetDataOpSetDataOpGetACLOpSetACLOpGetChildrenOpSync"
	_OpType_name_2 = "OpPingOpGetChildren2OpCheckOpMultiOpCreate2OpReconfigOpCheckWatchesOpRemoveWatchesOpCreateContainerOpDeleteContainerOpCreateTTL"
	_OpType_name_3 = "OpSetAuthOpSetWatchesOpSasl"
	_OpType_name_4 = "opGetDataWopExistsWopGetChildren2W"
)

var (
	_OpType_index_0 = [...]uint8{0, 15, 22}
	_OpType_index_1 = [...]uint8{0, 7, 15, 23, 31, 39, 48, 57, 65, 73, 86, 92}
	_OpType_index_2 = [...]uint8{0, 6, 20, 27, 34, 43, 53, 67, 82, 99, 116, 127}
	_OpType_index_3 = [...]uint8{0, 9, 21, 27}
	_OpType_index_4 = [...]uint8{0, 10, 19, 34}
)

func (i OpType) String() string {
	switch {
	case -12 <= i && i <= -11:
		i -= -12
		return _OpType_name_0[_OpType_index_0[i]:_OpType_index_0[i+1]]
	case -1 <= i && i <= 9:
		i -= -1
		return _OpType_name_1[_OpType_index_1[i]:_OpType_index_1[i+1]]
	case 11 <= i && i <= 21:
		i -= 11
		return _OpType_name_2[_OpType_index_2[i]:_OpType_index_2[i+1]]
	case 100 <= i && i <= 102:
		i -= 100
		return _OpType_name_3[_OpType_index_3[i]:_OpType_index_3[i+1]]
	case 200 <= i && i <= 202:
		i -= 200
		return _OpType_name_4[_OpType_index_4[i]:_OpType_index_4[i+1]]
	default:
		return fmt.Sprintf("OpType(%d)", i)
	}
}
