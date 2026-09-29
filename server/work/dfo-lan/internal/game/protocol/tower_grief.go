package protocol

import (
	"encoding/binary"
	"fmt"
)

// TowerGriefClearReward is NOTI1255 (TOG_CLEAR_REWARD). The 115 client
// handler sub_144ED79A0 reads u32, u16 floor, u8 count, then item-template
// and amount u32 pairs. Rows must describe items already committed to the bag.
type TowerRewardItem struct {
	Template uint32
	Amount   uint32
}

func TowerGriefClearReward(floor uint16, rows ...TowerRewardItem) ([]byte, error) {
	if floor == 0 || floor > 100 {
		return nil, fmt.Errorf("invalid Tower of Grief floor %d", floor)
	}
	if len(rows) > 255 {
		return nil, fmt.Errorf("too many Tower of Grief reward rows")
	}
	data := make([]byte, 7+8*len(rows))
	binary.LittleEndian.PutUint16(data[4:], floor)
	data[6] = byte(len(rows))
	for i, row := range rows {
		if row.Template == 0 || row.Amount == 0 {
			return nil, fmt.Errorf("invalid Tower of Grief reward row")
		}
		offset := 7 + i*8
		binary.LittleEndian.PutUint32(data[offset:], row.Template)
		binary.LittleEndian.PutUint32(data[offset+4:], row.Amount)
	}
	return data, nil
}
