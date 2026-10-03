package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// A character must be able to wear any structurally valid piece it meets the
// level/job/grow requirement for, regardless of the drop-pool rules ([free]
// attach and a rarity ceiling). Those rules decide what a monster may DROP,
// not what a character may WEAR. Before the fix the equip path called Basic and
// refused a [trade delete] bound shoe (e.g. the Explorer-collection 100261068)
// and every rarity-2 uncommon the archer otherwise qualified for - the user's
// "my own gear cannot be equipped" report. This locks the wear gate to the
// structural Reward check plus slot fit plus WearableBy.
func TestWearAcceptsBoundAndUncommonGear(t *testing.T) {
	const sum = "1111111111111111111111111111111111111111111111111111111111111111"
	shoe := func(id uint32, attach string, rarity int32) EquipmentDefinition {
		return EquipmentDefinition{ID: id, Path: "equipment/x.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[attach type]":    {{Type: 6, Text: attach}},
			"[rarity]":         {{Type: 0, Value: rarity}},
			"[equipment type]": {{Type: 6, Text: "[shoes]"}},
			"[minimum level]":  {{Type: 0, Value: 1}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
			"[durability]":     {{Type: 0, Value: 40}},
			"[grade]":          {{Type: 0, Value: 1}},
		}}
	}
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		shoe(5001, "[free]", 0),         // control: drop-pool eligible
		shoe(5002, "[trade delete]", 0), // bound: Reward yes, Basic no
		shoe(5003, "[free]", 2),         // uncommon: Reward yes, Basic no
	}}
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.json")
	b, e := json.Marshal(cat)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog(path, sum)
	if e != nil {
		t.Fatal(e)
	}
	// Bound gear stays out of the drop pool. An uncommon (rarity 2) piece is
	// droppable: the source rarity roll names rarity 0/1/2 and everything the
	// catalog carries above grade 20 is rarity>=1, so excluding 2 left every
	// dungeon above level 21 with no gear to drop at all.
	for _, d := range eq.DropPool() {
		if d.ID == 5002 {
			t.Fatalf("drop pool leaked bound gear %d", d.ID)
		}
	}
	uncommonDroppable := false
	for _, d := range eq.DropPool() {
		if d.ID == 5003 {
			uncommonDroppable = true
		}
	}
	if !uncommonDroppable {
		t.Fatal("rarity 2 gear is no longer droppable; high-level dungeons lose their whole gear pool")
	}
	svc := &WearService{
		Catalog:     eq,
		Professions: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: sum}, Professions: map[byte]catalog.Profession{16: {Job: "[archer]"}}},
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 44}},
		Rules:       WearRules{Source: sum, Slots: map[string]uint16{"[shoes]": 19}},
	}
	for _, id := range []uint32{5001, 5002, 5003} {
		bag := Bag{Version: "ordinary-bag-v1", Equipment: []BagEquipment{{Slot: 9, Template: id}}}
		state, e := SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		role := Role{Profession: 16, ConfigVersion: savecontract.Identity(), State: state}
		r := protocol.ItemMoveRequest{SourceSlot: 9, SourceItem: id, DestinationList: 3, DestinationSlot: 19, Count: 1, Selection: 0xffffffff}
		raw, e := svc.MoveOrdinary(role, r)
		if e != nil {
			t.Fatalf("archer could not equip template %d: %v", id, e)
		}
		worn, e := ReadBag(raw)
		if e != nil || len(worn.Worn) != 1 || worn.Worn[0].Template != id || worn.Worn[0].Slot != 19 {
			t.Fatalf("template %d not worn: %+v %v", id, worn, e)
		}
	}
}

// 宠物幻化栏（穿戴槽 32）。2026-09-26 实机五次 CMD19 都是
// `list7 槽8 item=63003/63008 -> list3 槽32`，五次全被
// "equipment does not fit destination slot" 拒掉：配置里 [creature] 只映射到 26，
// 而 115 客户端拖进幻化栏的是宠物本体，不是 [creature skin]。玩家看到的
// 「The target inventory is full ... Can't move the item.」就是这次拒绝。
//
// 放行条件之一是扩展券已开启这一栏（USERINFO1 解锁字节 bit5）：客户端 UI 的挂锁
// 读同一位，所以未开启时仍应拒绝。
func TestWearAcceptsCreatureIntoUnlockedSkinSlot(t *testing.T) {
	const sum = "2222222222222222222222222222222222222222222222222222222222222222"
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		{ID: 63003, Path: "equipment/creature/63003.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[creature]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
		}},
	}}
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.json")
	b, e := json.Marshal(cat)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog(path, sum)
	if e != nil {
		t.Fatal(e)
	}
	svc := &WearService{
		Catalog:     eq,
		Professions: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: sum}, Professions: map[byte]catalog.Profession{0: {Job: "[all]"}}},
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 44}},
		Rules:       WearRules{Source: sum, Special: true, Slots: map[string]uint16{"[creature]": 26, "[creature skin]": 32}},
	}
	move := func(flags byte) (json.RawMessage, error) {
		bag := Bag{Version: "ordinary-bag-v1", ExpandEquipFlags: flags,
			Special: map[byte][]BagEquipment{7: {{Slot: 8, Template: 63003}}}}
		state, e := SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		return svc.MoveOrdinary(Role{Profession: 0, ConfigVersion: savecontract.Identity(), State: state},
			protocol.ItemMoveRequest{SourceList: 7, SourceSlot: 8, SourceItem: 63003,
				DestinationList: 3, DestinationSlot: 32, Count: 1, Selection: 0xffffffff})
	}
	if _, e := move(0); e == nil {
		t.Fatal("creature entered the skin slot while the expansion bit was clear")
	}
	raw, e := move(ExpandCreatureSkin)
	if e != nil {
		t.Fatalf("creature refused by the unlocked skin slot: %v", e)
	}
	worn, e := ReadBag(raw)
	if e != nil || len(worn.Worn) != 1 || worn.Worn[0].Slot != 32 || worn.Worn[0].Template != 63003 {
		t.Fatalf("creature not worn in the skin slot: %+v %v", worn, e)
	}
}

// 光环幻化栏（穿戴槽 11）。与宠物侧完全同型：规则表里 [aurora avatar] 只映射到 9
// （光环本体槽），而 115 客户端拖进幻化栏做幻化的正是**光环本体**，目标槽 11。实机
// 四次 "equipment does not fit destination slot" 就是这次拒绝，客户端弹的是
// 「The target inventory is full ... Can't move the item.」。
//
// 放行条件之一是扩展券已开启这一栏（USERINFO1 解锁字节 bit3）：客户端 UI 的挂锁读同
// 一位，未开栏时服务端不该放行；而**开错位**（只开了宠物那一位）同样不能放行。
func TestWearAcceptsAuraIntoUnlockedSkinSlot(t *testing.T) {
	const sum = "3333333333333333333333333333333333333333333333333333333333333333"
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		{ID: 101009001, Path: "equipment/avatar/101009001.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[aurora avatar]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
		}},
	}}
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.json")
	b, e := json.Marshal(cat)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b, 0o600); e != nil {
		t.Fatal(e)
	}
	eq, e := LoadEquipmentCatalog(path, sum)
	if e != nil {
		t.Fatal(e)
	}
	svc := &WearService{
		Catalog:     eq,
		Professions: catalog.Characters{Source: pvf.ArchiveSnapshot{Checksum: sum}, Professions: map[byte]catalog.Profession{0: {Job: "[all]"}}},
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 44}},
		Rules:       WearRules{Source: sum, Special: true, Slots: map[string]uint16{"[aurora avatar]": 9, "[aura skin avatar]": 11}},
	}
	move := func(flags byte) (json.RawMessage, error) {
		bag := Bag{Version: "ordinary-bag-v1", ExpandEquipFlags: flags,
			Special: map[byte][]BagEquipment{1: {{Slot: 0, Template: 101009001}}}}
		state, e := SaveBag(json.RawMessage(`{"level":1,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		return svc.MoveOrdinary(Role{Profession: 0, ConfigVersion: savecontract.Identity(), State: state},
			protocol.ItemMoveRequest{SourceList: 1, SourceSlot: 0, SourceItem: 101009001,
				DestinationList: 3, DestinationSlot: 11, Count: 1, Selection: 0xffffffff})
	}
	if _, e := move(0); e == nil {
		t.Fatal("aura entered the skin slot while the expansion bit was clear")
	}
	if _, e := move(ExpandCreatureSkin); e == nil {
		t.Fatal("the creature bit opened the aura skin slot")
	}
	raw, e := move(ExpandAuraSkin)
	if e != nil {
		t.Fatalf("aura refused by the unlocked skin slot: %v", e)
	}
	worn, e := ReadBag(raw)
	if e != nil || len(worn.Worn) != 1 || worn.Worn[0].Slot != 11 || worn.Worn[0].Template != 101009001 {
		t.Fatalf("aura not worn in the skin slot: %+v %v", worn, e)
	}
}
