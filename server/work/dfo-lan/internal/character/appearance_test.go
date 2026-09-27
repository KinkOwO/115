package character

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"testing"

	"dfolan/internal/inventory"
	"dfolan/internal/storage"
)

// 宠物幻化栏（穿戴槽 32）里的宠物顶替槽 26 的模板，名字仍是槽 26 宠物的名字：
// 幻化只换外观。实机 2026-09-26 玩家报「宠物可以放进幻化栏了 但是宠物外观没有变」，
// 取证见 wornCreature 的注释。
func TestWornCreatureSkinOverridesModel(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[` +
		`{"slot":26,"template":63008},{"slot":32,"template":63003}]}}`)
	id, name := wornCreature(raw)
	if id != 63003 || name != "Botis" {
		t.Fatalf("expected skin 63003 with equipped name Botis, got %d %q", id, name)
	}
}

// 幻化栏是外观覆盖，不是第二只宠物：没有穿戴宠物时槽 32 不单独生效。
func TestWornCreatureSkinAloneIsNotACreature(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[` +
		`{"slot":32,"template":63003}]}}`)
	if id, name := wornCreature(raw); id != 0 || name != "" {
		t.Fatalf("expected no creature, got %d %q", id, name)
	}
}

// The skin template must reach the client through the mode-0 creature segment,
// which is the only downlink that carries a creature's item id.
func TestMode0ProbesCarrySkinCreatureSegment(t *testing.T) {
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn: []inventory.BagEquipment{
			{Slot: 26, Template: 63008},
			{Slot: 32, Template: 63003},
		},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	role := storage.Character{WireID: 1, Name: "LanTest01", Profession: 0, State: state}
	s := &Service{}
	want := append(binary.LittleEndian.AppendUint32(nil, 63003), 5, 0, 0, 0)
	want = append(want, []byte("Botis")...)
	want = append(want, 1)
	for name, probe := range map[string]func() ([]byte, error){
		"entry":      func() ([]byte, error) { return s.EntryBasicProbe(role, [2]byte{}) },
		"appearance": func() ([]byte, error) { return s.AppearanceProbe(role, [2]byte{}) },
	} {
		got, e := probe()
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		if !bytes.Contains(got, want) {
			t.Fatalf("%s: mode-0 packet lacks skin creature segment {63003, Botis, present=1}", name)
		}
	}
}

func TestWornCreatureExtraction(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":26,"template":63000}]}}`)
	id, name := wornCreature(raw)
	if id != 63000 || name != "Faras" {
		t.Fatalf("expected 63000 Faras, got %d %q", id, name)
	}
}

// The mode-0 userinfo creature segment is the town-follower display path
// (docs/宠物显示实现-G0198 §2.1): u32 slot-26 template + dstr name + u8
// present. The present byte must be 1 whenever an item id is set - 0 keeps
// the created companion hidden. Both mode-0 producers (entry and the
// post-move AppearanceProbe refresh) must carry the segment.
func TestMode0ProbesCarryWornCreatureSegment(t *testing.T) {
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Worn:    []inventory.BagEquipment{{Slot: 26, Template: 63000}},
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	role := storage.Character{WireID: 1, Name: "LanTest01", Profession: 0, State: state}
	s := &Service{}
	want := append(binary.LittleEndian.AppendUint32(nil, 63000), 5, 0, 0, 0)
	want = append(want, []byte("Faras")...)
	want = append(want, 1)
	for name, probe := range map[string]func() ([]byte, error){
		"entry":      func() ([]byte, error) { return s.EntryBasicProbe(role, [2]byte{}) },
		"appearance": func() ([]byte, error) { return s.AppearanceProbe(role, [2]byte{}) },
	} {
		got, e := probe()
		if e != nil {
			t.Fatalf("%s: %v", name, e)
		}
		if !bytes.Contains(got, want) {
			t.Fatalf("%s: mode-0 packet lacks creature segment {63000, Faras, present=1}", name)
		}
	}
}

// Without a worn creature the segment is all zero, including the present
// byte (official no-pet sample shape).
func TestMode0ProbeWithoutCreatureHasZeroSegment(t *testing.T) {
	state := json.RawMessage(`{"level":1,"advancement":0}`)
	role := storage.Character{WireID: 1, Name: "LanTest01", Profession: 0, State: state}
	s := &Service{}
	got, e := s.EntryBasicProbe(role, [2]byte{})
	if e != nil {
		t.Fatal(e)
	}
	if bytes.Contains(got, append(binary.LittleEndian.AppendUint32(nil, 63000), 1)) {
		t.Fatal("creature segment leaked into a creature-less actor packet")
	}
}
