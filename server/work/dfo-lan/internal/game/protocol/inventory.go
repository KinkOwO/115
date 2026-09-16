package protocol

import (
	"encoding/binary"
	"fmt"
)

// OrdinaryItem is the current 181-byte base row. Special equipment branches
// require extra source/type validation and are deliberately separate.
func OrdinaryItem(slot uint16, template, amount uint32) [CurrentItemRecordSize]byte {
	var p [CurrentItemRecordSize]byte
	binary.LittleEndian.PutUint16(p[:], slot)
	binary.LittleEndian.PutUint32(p[2:], template)
	binary.LittleEndian.PutUint32(p[6:], amount)
	return p
}

func itemRows(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if len(rows) > 65535 {
		return nil, fmt.Errorf("too many inventory rows")
	}
	p := add16(nil, uint16(len(rows)))
	seen := map[uint16]bool{}
	for _, r := range rows {
		slot := binary.LittleEndian.Uint16(r[:])
		if seen[slot] {
			return nil, fmt.Errorf("duplicate inventory slot")
		}
		seen[slot] = true
		p = append(p, r[:]...)
	}
	return p, nil
}

// NOTI13 list0 first reads a u16 count of locked slots, then item count.
func InventoryRestore(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{0, 0, 0}, p...), nil
}
func InventoryUpdate(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{0}, p...), nil
}

func InventorySpaceUpdate(space byte, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if space != 0 && space != 3 {
		return nil, fmt.Errorf("unsupported equipment update space")
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append([]byte{space}, p...), nil
}

// Current NOTI13 list3 omits list0's lock count and adds a u32 period after
// every181-byte row (1452d6a0b). Period0 represents ordinary permanent gear.
func WornRestore(rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if _, e := itemRows(rows); e != nil {
		return nil, e
	}
	p := add16([]byte{3}, uint16(len(rows)))
	for _, r := range rows {
		p = append(p, r[:]...)
		p = add32(p, 0)
	}
	return p, nil
}

type PickupRequest struct {
	Object                       uint32
	ActorX, ActorY, DropX, DropY uint16
	Context, Flag                byte // Opaque client observations; never item IDs or amounts.
}

func DecodePickup(p []byte) (PickupRequest, error) {
	var r PickupRequest
	if len(p) != 21 && len(p) != 24 && len(p) != 32 {
		return r, fmt.Errorf("pickup requires21 bytes with cipher block padding")
	}
	for _, b := range p[21:] {
		if b != 0 {
			return r, fmt.Errorf("pickup padding")
		}
	}
	if p[4] != 0 || p[20] > 1 {
		return r, fmt.Errorf("unsupported pickup option")
	}
	r = PickupRequest{Object: binary.LittleEndian.Uint32(p), ActorX: binary.LittleEndian.Uint16(p[6:]), ActorY: binary.LittleEndian.Uint16(p[8:]), DropX: binary.LittleEndian.Uint16(p[12:]), DropY: binary.LittleEndian.Uint16(p[14:]), Context: p[5], Flag: p[20]}
	if r.Object == 0 {
		return r, fmt.Errorf("zero scene object")
	}
	return r, nil
}

// NOTI39 gold consumes eight (u8 flag,u32 amount) rows and has no ordinary
// tail. Neutral flags avoid applying an unverified currency delta; NOTI14
// restores the authoritative gold balance before this scene removal.
func PickupConfirmed(object uint32, actor, slot uint16, gold bool) ([]byte, error) {
	if object == 0 || actor == 0 || actor == 65535 {
		return nil, fmt.Errorf("invalid pickup identity")
	}
	p := add16(add32(nil, object), actor)
	if gold {
		return append(p, make([]byte, 40)...), nil
	}
	p = append(p, make([]byte, 8)...)
	p = add16(add16(p, actor), slot)
	return append(p, 0), nil
}

// Current solo party0 gold row includes a flag, amount and an extra-count
// byte. This invokes both the local currency delta and overhead number;
// the remaining seven party rows are absent (flag0, amount0).
func GoldPickupConfirmed(object uint32, actor uint16, amount uint32) ([]byte, error) {
	if object == 0 || actor == 0 || actor == 65535 || amount == 0 {
		return nil, fmt.Errorf("invalid gold pickup")
	}
	p := add16(add32(nil, object), actor)
	p = add32(append(p, 1), amount)
	return append(p, make([]byte, 36)...), nil
}

// Failure consumes the scene object after the dispatcher's u16 error code.
// A generic Refusal alone truncates145244e5d and leaves pickup in flight.
func PickupRefused(object uint32) ([]byte, error) {
	if object == 0 {
		return nil, fmt.Errorf("zero pickup refusal object")
	}
	return add32(Refusal(4), object), nil
}

func MonsterDeathDrops(entity uint16, drops []SceneDrop) ([]byte, error) {
	if entity == 0 || entity == 65535 || len(drops) > 65535 {
		return nil, fmt.Errorf("invalid death/drop count")
	}
	p := add16(add16(nil, entity), uint16(len(drops)))
	seen := map[uint32]bool{}
	for _, d := range drops {
		if seen[d.Object] {
			return nil, fmt.Errorf("duplicate scene drop")
		}
		seen[d.Object] = true
		v, e := OrdinarySceneDropRecord(d)
		if e != nil {
			return nil, e
		}
		p = append(p, v...)
	}
	return append(p, 0, 0, 255, 0), nil
}
