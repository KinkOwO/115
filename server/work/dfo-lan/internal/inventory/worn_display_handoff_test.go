package inventory

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestHandoffWornDisplayUpdate(t *testing.T) {
	raw, e := SaveBag(json.RawMessage(`{"level":6}`), Bag{Version: "ordinary-bag-v1", Gold: 345, Worn: []BagEquipment{{Slot: 19, Template: 20002}}})
	if e != nil {
		t.Fatal(e)
	}
	original := append([]byte(nil), raw...)
	p, e := WornSpaceUpdate(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(p) != 3+protocol.CurrentItemRecordSize || p[0] != 3 || binary.LittleEndian.Uint16(p[1:]) != 1 {
		t.Fatal("wrong NOTI14 worn layout")
	}
	row := EquipmentRow(BagEquipment{Slot: 19, Template: 20002})
	if !bytes.Equal(p[3:], row[:]) {
		t.Fatal("equipment row changed")
	}
	if !bytes.Equal(original, raw) {
		t.Fatal("display refresh changed state")
	}
	empty, e := WornSpaceUpdate(json.RawMessage(`{}`))
	if e != nil || len(empty) != 0 {
		t.Fatal("empty equipment should not trigger rebuild")
	}
}
