package inventory

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"
)

func TestMergedWornInlayWithoutWardrobePreservesUnknownFields(t *testing.T) {
	checksum := strings.Repeat("a", 64)
	eq, e := NewEquipmentCatalog(EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: checksum}, Rows: []EquipmentDefinition{{ID: 40601, Path: "coat.equ", SHA256: checksum, Fields: map[string][]pvf.Token{
		"[equipment type]": {{Type: 6, Text: "[coat avatar]"}}, "[emblem socket default]": {{Type: 3, Text: "[C socket]"}},
	}}}}, checksum)
	if e != nil {
		t.Fatal(e)
	}
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: checksum}, Items: map[uint32]catalog.LootItem{20001: {ID: 20001, Kind: "stackable", StackableType: "[avatar emblem]", StackLimit: 100}}}
	s := &ItemService{Catalog: c, Equipment: eq, EmblemInlay: &EmblemInlayRules{Source: checksum, Masks: map[uint32]uint16{20001: 4}}}
	record := protocol.OrdinaryItem(3, 40601, 1)
	raw, e := SaveBag(json.RawMessage(`{"level":115,"unrelated":{"keep":7}}`), Bag{Version: "ordinary-bag-v1", Gold: 456, Worn: []BagEquipment{{Slot: 3, Template: 40601, Record: record[:], Period: 12345}}, Items: []BagItem{{Slot: 289, Template: 20001, Amount: 2}}})
	if e != nil {
		t.Fatal(e)
	}
	// No Special[1] exists. Newer inventory metadata must survive this selective save.
	var top map[string]json.RawMessage
	json.Unmarshal(raw, &top)
	var inv map[string]json.RawMessage
	json.Unmarshal(top["inventory"], &inv)
	inv["future_capacity"] = json.RawMessage(`{"keep":99}`)
	top["inventory"], _ = json.Marshal(inv)
	raw, _ = json.Marshal(top)
	before := append([]byte(nil), raw...)
	req := protocol.UseEmblemRequest{Space: 3, AvatarSlot: 3, Template: 40601, Inputs: []protocol.UseEmblemInput{{Slot: 289, Template: 20001, Socket: 0}}}
	updated, receipt, e := s.PrepareEmblemInlay(Role{State: raw}, req, 0)
	if e != nil {
		t.Fatal(e)
	}
	b, e := ReadBag(updated)
	if e != nil {
		t.Fatal(e)
	}
	if b.Gold != 456 || len(b.Items) != 1 || b.Items[0].Amount != 1 || b.EmblemInlaySeq != 1 || len(b.Worn) != 1 || b.Worn[0].Period != 12345 || !bytes.Equal(b.Worn[0].Record, record[:]) {
		t.Fatalf("instance/currency lost: %+v", b)
	}
	if binary.LittleEndian.Uint16(b.Worn[0].AvatarOptions) != 4 || binary.LittleEndian.Uint32(b.Worn[0].AvatarOptions[2:]) != 20001 {
		t.Fatal("inlay not saved")
	}
	if !bytes.Contains(updated, []byte(`"future_capacity":{"keep":99}`)) || !bytes.Contains(updated, []byte(`"unrelated":{"keep":7}`)) || !bytes.Equal(raw, before) {
		t.Fatal("unknown fields/input lost")
	}
	var r EmblemInlayReceipt
	if e = json.Unmarshal(receipt, &r); e != nil {
		t.Fatal(e)
	}
	if e = s.PrepareEmblemInlayReceipt(&r, updated); e != nil {
		t.Fatal(e)
	}
	if len(r.Avatar) == 0 || r.Avatar[0] != 3 {
		t.Fatal("worn refresh used wrong container")
	}
	req.Template++
	if _, _, e = s.PrepareEmblemInlay(Role{State: raw}, req, 0); e == nil {
		t.Fatal("stale target accepted")
	}
	if !bytes.Equal(raw, before) {
		t.Fatal("rejection mutated input")
	}
}
