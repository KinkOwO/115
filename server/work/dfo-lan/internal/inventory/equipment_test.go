package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestBasicEquipmentNativeRow(t *testing.T) {
	f, e := os.ReadFile("../game/protocol/testdata/native_equipment_record.json")
	if e != nil {
		t.Fatal(e)
	}
	var r struct {
		Payload string `json:"payload_hex"`
	}
	if e = json.Unmarshal(f, &r); e != nil {
		t.Fatal(e)
	}
	body, e := protocol.InventoryRestore([][protocol.CurrentItemRecordSize]byte{EquipmentRow(BagEquipment{Slot: 9, Template: 20002, Durability: 25})})
	if e != nil || hex.EncodeToString(body) != r.Payload {
		t.Fatal("native equipment record mismatch", e)
	}
}
