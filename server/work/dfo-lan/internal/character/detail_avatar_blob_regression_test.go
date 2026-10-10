package character

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// 实机缺陷回归（2026-10-07）：存档里宠物装备行（穿戴槽 27/28/29）曾被写入
// 30 字节全零的 avatar_options（邮件补孔缺陷），登录追加包因此整包被
// protocol.DetailedEquipment 拒绝（"avatar blob on non-avatar detailed row"），
// 角色卡在选人界面。槽 26~29 / 32 的行在原生 reader sub_1452C1540 里没有头像格，
// 所以投影必须丢掉这种块 —— 一份坏数据不能让角色登不进去。
func TestEntryAdditionDropsAvatarBlobOnCreatureGearRow(t *testing.T) {
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
	role := func(row inventory.BagEquipment) Character {
		state, e := inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{row}})
		if e != nil {
			t.Fatal(e)
		}
		return Character{WireID: 3, Profession: 0, State: state}
	}
	clean := inventory.BagEquipment{Slot: 27, Template: 100950255, Durability: 4}
	dirty := clean
	dirty.AvatarOptions = make([]byte, 30) // 缺陷写入的全零块（长度非零即触发拒绝）
	dirty.AvatarSockets = []byte{}

	got, e := s.EntryAddition(role(dirty))
	if e != nil {
		t.Fatalf("带脏头像块的宠物装备行仍拒绝了登录包：%v", e)
	}
	want, e := s.EntryAddition(role(clean))
	if e != nil {
		t.Fatal(e)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("脏块改变了下发视图：\ngot  %x\nwant %x", got, want)
	}
	if got[361] != 1 {
		t.Fatalf("宠物装备行数=%d，期望 1", got[361])
	}
}

// 时装槽（≤ 11）的头像块是**真数据**，投影必须原样保留（上面的丢弃规则不能越界）。
func TestEntryAdditionKeepsAvatarBlobOnAvatarRow(t *testing.T) {
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
	row := inventory.BagEquipment{Slot: 3, Template: 40601, Durability: 9, AvatarOptions: []byte{4, 0, 0, 0, 0, 0}, AvatarSockets: []byte{1, 0, 0, 0}}
	state, e := inventory.SaveBag(raw, inventory.Bag{Version: "ordinary-bag-v1", Worn: []inventory.BagEquipment{row}})
	if e != nil {
		t.Fatal(e)
	}
	got, e := s.EntryAddition(Character{WireID: 3, Profession: 0, State: state})
	if e != nil {
		t.Fatal(e)
	}
	// 行头 40 字节后是 u32 长度 + 孔块；6 字节块必须还在。
	body := got[362:]
	if len(body) < 40+4+6 {
		t.Fatalf("时装行太短：%d", len(body))
	}
	if binary.LittleEndian.Uint32(body[40:]) != 6 || !bytes.Equal(body[44:50], []byte{4, 0, 0, 0, 0, 0}) {
		t.Fatalf("时装头像块没有原样下发：%x", body[40:60])
	}
}
