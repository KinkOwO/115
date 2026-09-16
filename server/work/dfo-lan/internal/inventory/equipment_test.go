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
	body, e := protocol.InventoryRestore([][protocol.CurrentItemRecordSize]byte{EquipmentRow(BagEquipment{9, 20002, 25})})
	if e != nil || hex.EncodeToString(body) != r.Payload {
		t.Fatal("native equipment record mismatch", e)
	}
}
