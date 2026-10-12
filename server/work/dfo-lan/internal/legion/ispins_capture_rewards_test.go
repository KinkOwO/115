package legion

import (
	"encoding/binary"
	"fmt"
)

// Historical capture values are never used for production awards.
type IspinsRewardRecord struct {
	Item  uint32
	Count byte
}

// Official per-stage additional reward tables (next78 §2.4). Item ids are the
// 0x009Dxxxx material family plus the 0x009E0E20 finale item.
var IspinsAdditionalRewards = [4][]IspinsRewardRecord{
	{{0x9DB1BC, 28}, {0x9DB1BD, 12}, {0x9D94F3, 50}, {0x9D94F6, 25}, {0x9D94F1, 100}},
	{{0x9DB1BC, 28}, {0x9DB1BD, 12}, {0x9D94F3, 50}, {0x9D94F1, 100}},
	{{0x9DB1BC, 28}, {0x9DB1BD, 12}, {0x9D94F3, 50}, {0x9D94F1, 100}},
	{{0x9DB1BC, 28}, {0x9DB1BD, 12}, {0x9D94F3, 50}, {0x9D94F1, 100}, {0x9E0E20, 2}},
}

// IspinsBasicClearReward builds the native 7772B N2252 payload:
// zero buffer, the constant 20x 0x009DB1BC entry at 1600, and the stage
// token at 7760 that must equal the N31 head (next78 §2.3).
// Local client/DFO.exe handler 1424FDC30 reads 0x1e5c bytes directly through
// 146EA0BE0. Unlike the newer official capture it does not inflate zlib;
// a compressed body trips the reader's deliberate null write at 146EA0C30.
func IspinsBasicClearReward(stage int) ([]byte, error) {
	if stage < 0 || stage > 3 {
		return nil, fmt.Errorf("ispins reward stage %d out of range", stage)
	}
	raw := make([]byte, 7772)
	binary.LittleEndian.PutUint32(raw[1600:], 0x009DB1BC)
	binary.LittleEndian.PutUint32(raw[1604:], 20)
	copy(raw[7760:], IspinsStageTokens[stage][:])
	return raw, nil
}

// IspinsAdditionalClearReward builds the native fixed 2405B N2253
// payload: 40B records, first flagged 01, the rest 00, byte 9 = 03.
// Local handler 1424FDB60 likewise reads 0x965 bytes without inflation.
func IspinsAdditionalClearReward(stage int) ([]byte, error) {
	if stage < 0 || stage > 3 {
		return nil, fmt.Errorf("ispins reward stage %d out of range", stage)
	}
	raw := make([]byte, 2405)
	for i, rec := range IspinsAdditionalRewards[stage] {
		off := i * 40
		if i == 0 {
			raw[off] = 1
		}
		binary.LittleEndian.PutUint32(raw[off+1:], rec.Item)
		raw[off+5] = rec.Count
		raw[off+9] = 3
	}
	return raw, nil
}
