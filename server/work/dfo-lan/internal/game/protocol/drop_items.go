package protocol

import (
	"encoding/binary"
	"fmt"
)

const CurrentItemRecordSize = 181

type SceneDrop struct {
	Object          uint32
	Item            [CurrentItemRecordSize]byte
	Auxiliary       uint32
	Sentinel, Owner uint16
}

// OrdinarySceneDropRecord is only the current ordinary-item wire record. The
// drop owner must resolve PVF eligibility and any special-item extra branch.
// It does not roll drops, fabricate item definitions, or grant inventory.
func OrdinarySceneDropRecord(d SceneDrop) ([]byte, error) {
	if d.Object == 0 || d.Owner == 0 || d.Owner == 65535 {
		return nil, fmt.Errorf("invalid scene drop owner")
	}
	if binary.LittleEndian.Uint32(d.Item[2:]) == 0 && binary.LittleEndian.Uint32(d.Item[6:]) == 0 {
		return nil, fmt.Errorf("zero gold drop")
	}
	p := add32(nil, d.Object)
	p = append(p, d.Item[:]...)
	p = add32(p, d.Auxiliary)
	return add16(add16(p, d.Sentinel), d.Owner), nil
}
