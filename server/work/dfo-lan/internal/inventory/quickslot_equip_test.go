package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// 快捷栏装备：2026-09-29 实机 CMD19 把护身符（charm，装备区槽 29/30）拖进快捷栏
// （槽 3/8）被服务端拒掉，客户端弹通用 "The target inventory is full"。抓包报文
// 与堆叠物同型：Count=0 时 SourceSlot 是目标快捷槽、DestinationSlot 是物品当前槽。
// 根因是 MoveOrdinary 的 find() 把列表 0 只当成装备区（9..64），快捷槽 2..8 全部
// "slot outside equipment bag"；快捷栏行同样存 b.Equipment，swap 语义天然兼容。
func TestQuickSlotEquipmentMoveRequest(t *testing.T) {
	const sum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const charm = uint32(100360010) // equipment/character/common/charm/100360010.equ
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		{ID: charm, Path: "equipment/character/common/charm/100360010.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[charm]"}},
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
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 64}, QuickSlots: [2]uint16{0, 8}},
		Rules:       WearRules{Source: sum, Slots: map[string]uint16{"[charm]": 9}},
	}
	role := func(bag Bag) storage.Character {
		state, e := SaveBag(json.RawMessage(`{"level":5,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		return storage.Character{Profession: 0, ConfigVersion: sum, State: state}
	}

	// Case 1: 护身符从装备区槽 29 拖进快捷槽 8（实机 01:51:30 / 02:41:37 的报文形状）。
	raw, e := svc.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 29, Template: charm}}}),
		protocol.ItemMoveRequest{SourceSlot: 8, SourceItem: 0, Count: 0,
			DestinationSlot: 29, DestinationItem: charm, Selection: 0xffffffff})
	if e != nil {
		t.Fatalf("charm into quick slot 8 refused: %v", e)
	}
	moved, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(moved.Equipment) != 1 || moved.Equipment[0].Slot != 8 || moved.Equipment[0].Template != charm {
		t.Fatalf("charm not stored at quick slot 8: %+v", moved.Equipment)
	}

	// Case 2: 从快捷槽 8 拖回装备区槽 29（反向报文：SourceSlot=29, DestinationSlot=8）。
	raw, e = svc.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 8, Template: charm}}}),
		protocol.ItemMoveRequest{SourceSlot: 29, SourceItem: 0, Count: 0,
			DestinationSlot: 8, DestinationItem: charm, Selection: 0xffffffff})
	if e != nil {
		t.Fatalf("charm out of quick slot 8 refused: %v", e)
	}
	back, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(back.Equipment) != 1 || back.Equipment[0].Slot != 29 || back.Equipment[0].Template != charm {
		t.Fatalf("charm not back at bag slot 29: %+v", back.Equipment)
	}

	// Case 3: 快捷槽已被堆叠物占用时，装备落不进去（与堆叠路径互斥）。
	bagStack := Bag{Version: "ordinary-bag-v1",
		Items:     []BagItem{{Slot: 8, Template: 1106, Amount: 30}},
		Equipment: []BagEquipment{{Slot: 29, Template: charm}}}
	if _, e = svc.MoveOrdinary(role(bagStack),
		protocol.ItemMoveRequest{SourceSlot: 8, SourceItem: 0, Count: 0,
			DestinationSlot: 29, DestinationItem: charm, Selection: 0xffffffff}); e == nil {
		t.Fatal("charm landed on a quick slot occupied by a stack")
	}

	// Case 4: 快捷槽 0/1 是金币/点券格，装备不许进。
	if _, e = svc.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 29, Template: charm}}}),
		protocol.ItemMoveRequest{SourceSlot: 0, SourceItem: 0, Count: 0,
			DestinationSlot: 29, DestinationItem: charm, Selection: 0xffffffff}); e == nil {
		t.Fatal("charm entered gold/coin slot 0")
	}

	// Case 5: 常规装备区内部移动不受影响（回归：9..64 内的 swap 照旧）。
	other := uint32(100220171)
	cat2 := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		{ID: charm, Path: "equipment/character/common/charm/100360010.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[charm]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
		}},
		{ID: other, Path: "equipment/character/common/belt/larmor/100220171.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[belt]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
		}},
	}}
	b2, e := json.Marshal(cat2)
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(path, b2, 0o600); e != nil {
		t.Fatal(e)
	}
	eq2, e := LoadEquipmentCatalog(path, sum)
	if e != nil {
		t.Fatal(e)
	}
	svc2 := &WearService{Catalog: eq2, Professions: svc.Professions, BagRules: svc.BagRules, Rules: svc.Rules}
	raw, e = svc2.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 10, Template: charm}, {Slot: 12, Template: other}}}),
		protocol.ItemMoveRequest{SourceSlot: 10, SourceItem: charm, Count: 1,
			DestinationSlot: 12, DestinationItem: other, Selection: 0xffffffff})
	if e != nil {
		t.Fatalf("ordinary equipment bag swap refused: %v", e)
	}
	swapped, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	bySlot := map[uint16]uint32{}
	for _, it := range swapped.Equipment {
		bySlot[it.Slot] = it.Template
	}
	if bySlot[10] != other || bySlot[12] != charm {
		t.Fatalf("bag swap wrong: %+v", bySlot)
	}
}

// 快捷栏装备约束（2026-09-29 用户要求"只能装备一个"）：快捷栏同一时间只能放一件
// 装备。往空快捷槽新增第二件被拒；拖到已被装备占用的快捷槽（换装）与快捷槽之间
// 互拖（调位）仍放行，数量保持一件。
func TestQuickSlotEquipmentOneOnly(t *testing.T) {
	const sum = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	const charmA = uint32(100360010)
	const charmB = uint32(100360012)
	cat := EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: sum}, Rows: []EquipmentDefinition{
		{ID: charmA, Path: "equipment/character/common/charm/100360010.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[charm]"}},
			"[usable job]":     {{Type: 6, Text: "[all]"}},
		}},
		{ID: charmB, Path: "equipment/character/common/charm/100360012.equ", SHA256: sum, Fields: map[string][]pvf.Token{
			"[equipment type]": {{Type: 6, Text: "[charm]"}},
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
		BagRules:    BagRules{EquipmentSlots: [2]uint16{9, 64}, QuickSlots: [2]uint16{0, 8}},
		Rules:       WearRules{Source: sum, Slots: map[string]uint16{"[charm]": 9}},
	}
	role := func(bag Bag) storage.Character {
		state, e := SaveBag(json.RawMessage(`{"level":5,"advancement":0}`), bag)
		if e != nil {
			t.Fatal(e)
		}
		return storage.Character{Profession: 0, ConfigVersion: sum, State: state}
	}
	dragIn := func(bag Bag, targetSlot, bagSlot uint16, id uint32) error {
		_, e := svc.MoveOrdinary(role(bag),
			protocol.ItemMoveRequest{SourceSlot: targetSlot, SourceItem: 0, Count: 0,
				DestinationSlot: bagSlot, DestinationItem: id, Selection: 0xffffffff})
		return e
	}

	// 快捷栏已有一件（槽 8），再从背包拖第二件到空快捷槽 5 → 拒绝。
	if e := dragIn(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 8, Template: charmA}, {Slot: 29, Template: charmB}}}, 5, 29, charmB); e == nil {
		t.Fatal("second charm entered an empty quick slot")
	}

	// 拖到已被装备占用的快捷槽（换装）→ 放行，快捷栏仍只有一件。
	raw, e := svc.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 8, Template: charmA}, {Slot: 29, Template: charmB}}}),
		protocol.ItemMoveRequest{SourceSlot: 8, SourceItem: 0, Count: 0,
			DestinationSlot: 29, DestinationItem: charmB, Selection: 0xffffffff})
	if e != nil {
		t.Fatalf("swap onto occupied quick slot refused: %v", e)
	}
	swapped, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	bySlot := map[uint16]uint32{}
	for _, it := range swapped.Equipment {
		bySlot[it.Slot] = it.Template
	}
	if bySlot[8] != charmB || bySlot[29] != charmA {
		t.Fatalf("swap result wrong: %+v", bySlot)
	}

	// 快捷槽之间互拖（调位 8→5）→ 放行。
	raw, e = svc.MoveOrdinary(role(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 8, Template: charmA}}}),
		protocol.ItemMoveRequest{SourceSlot: 5, SourceItem: 0, Count: 0,
			DestinationSlot: 8, DestinationItem: charmA, Selection: 0xffffffff})
	if e != nil {
		t.Fatalf("quick-slot rearrange refused: %v", e)
	}
	moved, e := ReadBag(raw)
	if e != nil {
		t.Fatal(e)
	}
	if len(moved.Equipment) != 1 || moved.Equipment[0].Slot != 5 || moved.Equipment[0].Template != charmA {
		t.Fatalf("rearranged charm wrong: %+v", moved.Equipment)
	}

	// 第一件放进空快捷栏 → 放行。
	if e := dragIn(Bag{Version: "ordinary-bag-v1",
		Equipment: []BagEquipment{{Slot: 29, Template: charmA}}}, 8, 29, charmA); e != nil {
		t.Fatalf("first charm into empty quick bar refused: %v", e)
	}
}
