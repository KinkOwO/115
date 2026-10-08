package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"

	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"
)

func TestSourceAttributesMatchNativeLoader(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile("testdata/native_source_stats.json")
	if e != nil {
		t.Fatal(e)
	}
	var rows []struct {
		Profession byte
		SHA        string `json:"source_sha256"`
		Wire       string `json:"wire_hex"`
	}
	if e = json.Unmarshal(raw, &rows); e != nil {
		t.Fatal(e)
	}
	for _, r := range rows {
		p := c.Professions[r.Profession]
		if p.RawSHA256 != r.SHA {
			t.Fatal("fixture source changed")
		}
		state, _ := json.Marshal(State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourcePath: p.Path, SourceSHA256: p.RawSHA256})
		got, e := (&Service{Catalog: c}).EntryAddition(Character{WireID: 3, Profession: r.Profession, State: state})
		if e != nil {
			t.Fatal(e)
		}
		want, _ := hex.DecodeString(r.Wire)
		if !bytes.Equal(got[269:360], want) {
			t.Fatalf("profession %d differs from native PVF loader\ngot %x\nwant%x", r.Profession, got[269:360], want)
		}
	}
}

// 幻化槽（穿戴槽 32）必须进入 mode-1 的穿戴块，否则小退重登后客户端手上没有
// 槽 32 的物品，F6 幻化框拿不到模板，宠物图标空白（实机 2026-09-27）。
// 槽 32 是 [creature] 物品，行布局与槽 26 同源，一行固定 132 字节。
func TestEntryAdditionProjectsCreatureSkinWornSlot(t *testing.T) {
	c, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	p := c.Professions[0]
	raw, e := json.Marshal(State{Level: 1, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills, SourcePath: p.Path, SourceSHA256: p.RawSHA256})
	if e != nil {
		t.Fatal(e)
	}
	s := &Service{Catalog: c, DetailedWornCandidate: true}
	role := func(bag inventory.Bag) Character {
		state, e := inventory.SaveBag(raw, bag)
		if e != nil {
			t.Fatal(e)
		}
		return Character{WireID: 3, Profession: 0, State: state}
	}
	empty, e := s.EntryAddition(role(inventory.Bag{Version: "ordinary-bag-v1"}))
	if e != nil {
		t.Fatal(e)
	}
	if empty[361] != 0 {
		t.Fatalf("空穿戴的 mode-1 行数=%d，期望 0", empty[361])
	}
	got, e := s.EntryAddition(role(inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{{Slot: 32, Template: 63008, Durability: 3}}}))
	if e != nil {
		t.Fatal(e)
	}
	if got[361] != 1 {
		t.Fatalf("幻化槽行数=%d，期望 1", got[361])
	}
	if got[362] != 32 || binary.LittleEndian.Uint32(got[363:]) != 63008 || binary.LittleEndian.Uint16(got[372:]) != 3 {
		t.Fatalf("幻化槽行头: %x", got[361:375])
	}
	if len(got) != len(empty)+132 {
		t.Fatalf("幻化槽使 mode-1 载荷增长 %d 字节，期望 132", len(got)-len(empty))
	}
}
