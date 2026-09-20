package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestAccountMaterialSlotMapMatchesClientTables(t *testing.T) {
	// Verified against client/DFO.exe.i64 sub_145ACA5E0 / sub_145AD74A0 /
	// sub_145AD6B70 (tables at 0x14A9288E0..0x14A928924).
	want := map[uint32]uint16{
		3033: 363, 3034: 364, 3035: 365, 3036: 366, 3037: 367, 3262: 368,
		10100115: 369, 10100116: 370, 10099773: 371, 10099774: 372, 10099775: 373, 10158124: 374,
		10361512: 375, 10361513: 376, 10361514: 377, 10361515: 378, 10361516: 379,
	}
	if len(accountMaterialSlotByTemplate) != 17 || len(accountMaterialTemplateBySlot) != 17 {
		t.Fatalf("expected 17 fixed templates, got %d/%d", len(accountMaterialSlotByTemplate), len(accountMaterialTemplateBySlot))
	}
	for template, slot := range want {
		got, ok := AccountMaterialSlot(template)
		if !ok || got != slot {
			t.Fatalf("template %d slot = %d,%v want %d", template, got, ok, slot)
		}
		back, ok := StorageRowTemplate(slot)
		if !ok || back != template {
			t.Fatalf("slot %d template = %d,%v want %d", slot, back, ok, template)
		}
	}
	if _, ok := AccountMaterialSlot(999999); ok {
		t.Fatalf("unrelated template must not map into the storage")
	}
}

func TestAccountMaterialsAddAndRows(t *testing.T) {
	m := NewAccountMaterials()
	m, slot, e := m.Add(3037, 20)
	if e != nil || slot != 367 {
		t.Fatalf("add slot=%d err=%v", slot, e)
	}
	m, _, e = m.Add(3037, 5)
	if e != nil {
		t.Fatal(e)
	}
	if m.Count(3037) != 25 || m.Count(3033) != 0 {
		t.Fatalf("counts %d/%d", m.Count(3037), m.Count(3033))
	}
	rows := m.Rows()
	if len(rows) != 1 {
		t.Fatalf("zero-count slots must be omitted, got %d rows", len(rows))
	}
	if got := binary.LittleEndian.Uint16(rows[0][:]); got != 367 {
		t.Fatalf("row slot %d", got)
	}
	if got := binary.LittleEndian.Uint32(rows[0][2:]); got != 3037 {
		t.Fatalf("row template %d", got)
	}
	if got := binary.LittleEndian.Uint32(rows[0][6:]); got != 25 {
		t.Fatalf("row amount %d", got)
	}
	if _, _, e = m.Add(42, 1); e == nil {
		t.Fatalf("foreign template must be rejected")
	}
}

func TestAccountMaterialsRoundTrip(t *testing.T) {
	m := NewAccountMaterials()
	m, _, _ = m.Add(10158124, 7)
	raw, e := m.Save()
	if e != nil {
		t.Fatal(e)
	}
	back, e := ReadAccountMaterials(raw)
	if e != nil {
		t.Fatal(e)
	}
	if back.Count(10158124) != 7 {
		t.Fatalf("round trip count %d", back.Count(10158124))
	}
	empty, e := ReadAccountMaterials(nil)
	if e != nil || len(empty.Counts) != 0 {
		t.Fatalf("empty document: %v %v", empty, e)
	}
	if _, e = ReadAccountMaterials(json.RawMessage(`{"version":"other"}`)); e == nil {
		t.Fatalf("foreign version must be rejected")
	}
	if _, e = ReadAccountMaterials(json.RawMessage(`{"version":"account-materials-v1","counts":{"100":5}}`)); e == nil {
		t.Fatalf("foreign slot must be rejected")
	}
}

func TestSweepAccountMaterials(t *testing.T) {
	b := Bag{Version: "ordinary-bag-v1", Items: []BagItem{
		{Slot: 121, Template: 3037, Amount: 20},
		{Slot: 122, Template: 3037, Amount: 4},
		{Slot: 123, Template: 6001, Amount: 1},
		{Slot: 124, Template: 10361512, Amount: 9},
	}}
	swept, deltas, e := SweepAccountMaterials(b)
	if e != nil {
		t.Fatal(e)
	}
	if len(swept.Items) != 1 || swept.Items[0].Template != 6001 {
		t.Fatalf("kept %+v", swept.Items)
	}
	if len(deltas) != 2 {
		t.Fatalf("deltas %+v", deltas)
	}
	if deltas[0].Slot != 367 || deltas[0].Amount != 24 {
		t.Fatalf("cube delta %+v", deltas[0])
	}
	if deltas[1].Slot != 375 || deltas[1].Amount != 9 {
		t.Fatalf("old soul delta %+v", deltas[1])
	}
	m := NewAccountMaterials()
	m, e = m.ApplyDeltas(deltas)
	if e != nil {
		t.Fatal(e)
	}
	if m.Count(3037) != 24 || m.Count(10361512) != 9 {
		t.Fatalf("counts %d/%d", m.Count(3037), m.Count(10361512))
	}
	// Sweeping an already-clean bag is a no-op (retry safety).
	again, deltas, e := SweepAccountMaterials(swept)
	if e != nil || len(deltas) != 0 || len(again.Items) != 1 {
		t.Fatalf("resweep %+v %+v %v", again.Items, deltas, e)
	}
	if _, _, e = SweepAccountMaterials(Bag{Version: "ordinary-bag-v1", Items: []BagItem{{Slot: 130, Template: 3037, Amount: ^uint32(0)}, {Slot: 131, Template: 3037, Amount: 1}}}); e == nil {
		t.Fatalf("overflow must be rejected")
	}
}

func TestInventoryRestoreSpace35Layout(t *testing.T) {
	m := NewAccountMaterials()
	m, _, _ = m.Add(3262, 3)
	body, e := protocol.InventoryRestoreSpace(AccountMaterialSpace, m.Rows())
	if e != nil {
		t.Fatal(e)
	}
	// {space} + u16 count + row; unlike list0 there is no extra u16 lock
	// count (sub_1452D5A80 only reads it for lists 0/1).
	if len(body) != 1+2+protocol.CurrentItemRecordSize {
		t.Fatalf("length %d", len(body))
	}
	if body[0] != 35 || binary.LittleEndian.Uint16(body[1:]) != 1 {
		t.Fatalf("prefix %v", body[:3])
	}
	if got := binary.LittleEndian.Uint16(body[3:]); got != 368 {
		t.Fatalf("row slot %d", got)
	}
	// Space 0 keeps the historical three-byte prefix.
	list0, e := protocol.InventoryRestoreSpace(0, m.Rows())
	if e != nil {
		t.Fatal(e)
	}
	if list0[0] != 0 || list0[1] != 0 || list0[2] != 0 {
		t.Fatalf("list0 prefix %v", list0[:3])
	}
}
