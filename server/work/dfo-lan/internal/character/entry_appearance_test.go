package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"

	"encoding/binary"
	"encoding/json"
	"testing"
)

// AppearanceProbe is the post-move look refresh: the mode0 userinfo with the
// equipped-appearance block bound to the current worn set. The row value is
// the piece's own template ID (the live-failed alternative was the
// [equipment type] class cell, which made the client drop the weapon with
// "no weapon equipped"), and every worn slot contributes a row.
//
// 更正（2026-09-19）：本测试原先断言槽 14 被排除在外，依据是把 0x145a8a780
// 读作「装备槽准入集合」。实机 client_trace 否证了该前提（入场时客户端逐条
// 记录 equip : 12/14/15/16/17/18/24，且写出该日志的循环 0x145640b00 遍历
// 48 个槽、非空即打印），0x145a8a780 的键集实为装扮层表。断言已反转为
// 「每个穿戴槽一行」。
func TestAppearanceProbeBindsTemplateIDPerWornSlot(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	// A katana in the weapon slot and a coat in the coat slot: both are worn
	// slots and both carry their own template ID.
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: 101010438},
			{Slot: 14, Template: 400070177},
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(Character{
		WireID: 1, Name: "LanTest01", Profession: 0,
		State: append(state[:len(state)-1], []byte(`,"advancement":0}`)...),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	// 1 + 1 + 2 context + 160 + 2 actor id + 4 name length + name + 5
	// profession/advancement/level/pad/pad.
	const at = 176 + len("LanTest01")
	if count := int(got[at]); count != 2 {
		t.Fatalf("appearance count=%d, want 2 (one row per worn slot)", count)
	}
	// Each entry is 35 bytes of base (slot + placeholder + len + one-byte flags
	// + weapon tail + attach/reserved cells) plus the 4-byte model payload, so
	// a row with a bound model is 39 bytes and the model sits nine bytes in.
	want := []struct {
		slot  byte
		model uint32
	}{{12, 101010438}, {14, 400070177}}
	pos := at + 1
	for i, w := range want {
		if n := int(binary.LittleEndian.Uint32(got[pos+5:])); n != 4 {
			t.Fatalf("row %d declares len=%d, want 4", i, n)
		}
		if slot := got[pos]; slot != w.slot {
			t.Fatalf("row %d slot=%d, want %d", i, slot, w.slot)
		}
		if model := binary.LittleEndian.Uint32(got[pos+9:]); model != w.model {
			t.Fatalf("row %d binding=%d, want template ID %d", i, model, w.model)
		}
		pos += 39
	}
}

// The entry-time packet carries the worn rows through the same list-row
// projection as the roster rows. 20260918 user report: an empty entry block
// leaves every 0x405 binding at zero and the client reports "no weapon
// equipped".
func TestEntryBasicProbeCarriesWornAppearance(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: 101010438},
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.EntryBasicProbe(Character{
		WireID: 1, Name: "LanTest01", Profession: 0,
		State: append(state[:len(state)-1], []byte(`,"advancement":0}`)...),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	const at = 176 + len("LanTest01")
	if count := int(got[at]); count != 1 {
		t.Fatalf("entry appearance count=%d, want 1", count)
	}
	if slot := int(got[at+1]); slot != 12 {
		t.Fatalf("entry appearance slot=%d, want 12", slot)
	}
}

// A knight wears a [support weapon] shield on worn slot 24 next to the weapon
// on slot 12. Both rows must reach the client: the live trace of a knight
// entering town records the client accepting the shield row verbatim
// ("equip : 24 - 骑士之盾(113370002)"), so the row is neither filtered nor
// allowed to displace the weapon row.
//
// Client-side provenance (DFO.exe 2.38.2.34 US):
//
//	0x1470cb31e .. the [equipment type] enum table:
//	              weapon=12, coat=14, shoulder=15, pants=16, shoes=17,
//	              waist=18, support weapon=24 - the same numbers the client
//	              prints as slot IDs in its own entry trace.
//	0x145640b00 .. the loop that prints "equip : %d - %s(%d)" over 0x30 slots.
func TestEntryBasicProbeCarriesKnightShieldRow(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: 101010438},
			{Slot: 24, Template: 113370002},
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.EntryBasicProbe(Character{
		WireID: 1, Name: "LanTest01", Profession: 9,
		State: append(state[:len(state)-1], []byte(`,"advancement":0}`)...),
	}, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	const at = 176 + len("LanTest01")
	if count := int(got[at]); count != 2 {
		t.Fatalf("entry appearance count=%d, want 2 (weapon and shield rows)", count)
	}
	// The entry projection carries the piece's item ID in the placeholder cell
	// and declares a zero-length payload, i.e. "this row names the piece, it
	// does not bind a model index". One such row is 35 bytes.
	want := []struct {
		slot byte
		item uint32
	}{{12, 101010438}, {24, 113370002}}
	pos := at + 1
	for i, w := range want {
		if slot := int(got[pos]); slot != int(w.slot) {
			t.Fatalf("row %d slot=%d, want %d", i, slot, w.slot)
		}
		if item := binary.LittleEndian.Uint32(got[pos+1:]); item != w.item {
			t.Fatalf("row %d item=%d, want %d", i, item, w.item)
		}
		if n := int(binary.LittleEndian.Uint32(got[pos+5:])); n != 0 {
			t.Fatalf("row %d declares len=%d, want 0 on the entry projection", i, n)
		}
		pos += 35
	}
}

// A creature rides worn slot 26, which sits beyond the client's
// equipped-appearance table (slot 25 top). The appearance refresh must skip
// it rather than fail the whole block: live 2026-09-21 every CMD19 equip on a
// creature-wearing character was refused with "equipped appearance slot 26
// exceeds the client table".
func TestAppearanceProbeSkipsCreatureSlot(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 12, Template: 101010438},
			{Slot: 26, Template: 63019},
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(Character{
		WireID: 1, Name: "LanTest02", Profession: 0,
		State: append(state[:len(state)-1], []byte(`,"advancement":0}`)...),
	}, [2]byte{})
	if e != nil {
		t.Fatalf("creature-wearing role must still refresh appearance: %v", e)
	}
	const at = 176 + len("LanTest02")
	if count := int(got[at]); count != 1 {
		t.Fatalf("appearance count=%d, want 1 (creature slot 26 skipped)", count)
	}
	if slot := got[at+1]; slot != 12 {
		t.Fatalf("row slot=%d, want 12", slot)
	}
}

func TestAppearanceProbeCarriesCloneAndLookAvatars(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	// Slot 1 has both a clone avatar (Group 0) and an appearance avatar (Group 1)
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 1, Template: 517560000, Group: 0}, // clone hair
			{Slot: 1, Template: 517562678, Group: 1}, // appearance hair
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: professions}
	got, e := s.AppearanceProbe(Character{
		WireID: 1, Name: "LanTest03", Profession: 0,
		State: append(state[:len(state)-1], []byte(`,"advancement":0}`)...),
	}, [2]byte{})
	if e != nil {
		t.Fatalf("AppearanceProbe failed: %v", e)
	}
	const at = 176 + len("LanTest03")
	if count := int(got[at]); count != 1 {
		t.Fatalf("appearance count=%d, want 1 (merged slot 1)", count)
	}
	pos := at + 1
	if slot := got[pos]; slot != 1 {
		t.Fatalf("slot=%d, want 1", slot)
	}
	if model := binary.LittleEndian.Uint32(got[pos+9:]); model != 517562678 {
		t.Fatalf("model=%d, want look avatar 517562678", model)
	}
	// Live 2026-09-22: nonzero attach cells killed the client (0xC0000005).
	if attachA := binary.LittleEndian.Uint32(got[pos+26:]); attachA != 0 {
		t.Fatalf("attachA=%d, want 0 (baseline encoding)", attachA)
	}
	if attachB := binary.LittleEndian.Uint32(got[pos+30:]); attachB != 0 {
		t.Fatalf("attachB=%d, want 0 (baseline encoding)", attachB)
	}
}
