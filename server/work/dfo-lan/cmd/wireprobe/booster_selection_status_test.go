package main

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
)

// 实机开箱走 openBoosterItem（main.go:2332），不是 useBooster。源选择箱
// 590015875 的分类 [0 0] 声明 [booster equipment upgrade]=12 / [separate]=8，
// 开箱落出的武器必须带 +12 强化 / +8 锻造；这条回归锁住 openBoosterItem 的盖章。
func TestOpenBoosterItemSelectionBoxStatus(t *testing.T) {
	const srcSum = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	boxes, err := catalog.NewSelectionBoxes(catalog.SelectionBoxes{
		Model:  catalog.SelectionBoxModel,
		Source: pvf.ArchiveSnapshot{Checksum: srcSum},
		Boxes: map[string]catalog.SelectionBox{
			"590015875": {
				Template: 590015875,
				Path:     "stackable/590015001/590015875.stk",
				SHA256:   "bd72f736326aaa03e9b48a773bb0adb623530aea4eacee14881c3d734aa34000",
				Categories: []catalog.SelectionCategory{{
					Category:  [2]byte{0, 0},
					Grade:     5,
					Reinforce: 12,
					Refine:    8,
					Items:     []catalog.SelectionItem{{Template: 101040829, Count: 1}},
					Sections:  []string{"[equipment]"},
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if r, f, ok := boxes.CategoryStatus(590015875, [2]byte{0, 0}); !ok || r != 12 || f != 8 {
		t.Fatalf("crafted status wrong: %d/%d ok=%v", r, f, ok)
	}

	prof, e := catalog.LoadCharacters("../../configs/characters.next25.json")
	if e != nil {
		t.Fatal(e)
	}
	gear, e := inventory.LoadEquipmentCatalog("../../configs/equipment.current37.json", prof.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	// configs/equipment-full.* 导出在本树已退休（bootstrap 只在 -pvf 原生装备下装配
	// Full）。101040829 不在 equipment.current37 的 JSON 索引里，改用同一源身份
	// （7ef2db59…）的 inventory testdata 全量目录，自选盒装备分支才有可发放的 kind。
	full, e := inventory.OpenFullEquipmentCatalog("../../internal/inventory/testdata/equipment-flow", srcSum)
	if e != nil {
		t.Fatal(e)
	}
	defer full.Close()
	gear.Full = full
	rules := inventory.BagRules{Source: prof.Source.Checksum, EquipmentSlots: [2]uint16{9, 64}, Slots: map[string][2]uint16{"[booster]": {65, 104}}, MissingStackLimit: 1000}
	wear := &workflow.WearService{WearService: inventory.WearService{Catalog: gear, Professions: prof, BagRules: rules}}
	lootSvc := &loot.Service{Equipment: gear, BagRules: rules, Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}}

	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 590015875, Amount: 1}}}
	state, _ := inventory.SaveBag(json.RawMessage(`{"level":115}`), bag)
	char := database.Character{ID: 10, AccountID: 1, State: state}
	store := newMockBoosterStore(char)

	req := make([]byte, 8+4)
	binary.LittleEndian.PutUint16(req[0:2], 65)         // slot
	binary.LittleEndian.PutUint32(req[2:6], 1)          // amount
	binary.LittleEndian.PutUint16(req[6:8], 0)          // category [0 0]
	binary.LittleEndian.PutUint32(req[8:12], 101040829) // selection

	w := &worldSession{role: char, loot: lootSvc, selectionBoxes: boxes}
	if _, err := w.openBoosterItem(context.Background(), store, wear, lootSvc, nil, odysseyWeaponChoices{}, req, req); err != nil {
		t.Fatal(err)
	}
	res, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Equipment) != 1 {
		t.Fatalf("expected 1 landed equipment, got %d", len(res.Equipment))
	}
	row := inventory.EquipmentRow(res.Equipment[0])
	t.Logf("landed: offset10&0x1f=%d Refine=%d", row[10]&0x1f, res.Equipment[0].Refine)
	if row[10]&0x1f != 12 {
		t.Fatalf("强化低五位=%d 期望12（开箱落装未盖章的实机缺陷）", row[10]&0x1f)
	}
	if res.Equipment[0].Refine != 8 {
		t.Fatalf("锻造=%d 期望8", res.Equipment[0].Refine)
	}
}

// 第八关的誓言水晶礼盒 590015953 源里写 `[equipment] 100401592 11`（tooltip
// 「Contains 11 Dim Oath Crystals」），开一次要落 11 颗。openBoosterItem 过去
// 一律按礼盒数 req.Amount 发放，实机 2026-10-03 只落 1 颗；这条回归钉住
// 「件数取源声明值」，并且要求 ACK160 的计数与背包一致。
func TestOpenBoosterItemSelectionBoxUsesSourceCount(t *testing.T) {
	boxes, err := catalog.NewSelectionBoxes(catalog.SelectionBoxes{
		Model:  catalog.SelectionBoxModel,
		Source: pvf.ArchiveSnapshot{Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"},
		Boxes: map[string]catalog.SelectionBox{
			"590015953": {
				Template: 590015953,
				Path:     "stackable/590015001/590015953.stk",
				SHA256:   "bd72f736326aaa03e9b48a773bb0adb623530aea4eacee14881c3d734aa34000",
				Categories: []catalog.SelectionCategory{{
					Category: [2]byte{0, 0},
					Grade:    5,
					Items:    []catalog.SelectionItem{{Template: 100401592, Count: 11}},
					Sections: []string{"[equipment]"},
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	rules := inventory.BagRules{
		Source:         "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80",
		EquipmentSlots: [2]uint16{9, 64},
		Slots:          map[string][2]uint16{"[booster]": {65, 104}}, MissingStackLimit: 1000,
	}
	lootSvc := &loot.Service{
		BagRules: rules,
		Catalog: catalog.LootCatalog{Items: map[uint32]catalog.LootItem{
			100401592: {ID: 100401592, Kind: "equipment"},
		}},
	}

	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 590015953, Amount: 1}}}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":115}`), bag)
	if e != nil {
		t.Fatal(e)
	}
	char := database.Character{ID: 11, AccountID: 1, State: state}
	store := newMockBoosterStore(char)

	req := make([]byte, 12)
	binary.LittleEndian.PutUint16(req[0:2], 65)         // box slot
	binary.LittleEndian.PutUint32(req[2:6], 1)          // box amount
	binary.LittleEndian.PutUint16(req[6:8], 0)          // category [0 0]
	binary.LittleEndian.PutUint32(req[8:12], 100401592) // pick

	w := &worldSession{role: char, loot: lootSvc, selectionBoxes: boxes}
	plan, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, nil, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal(err)
	}
	res, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	landed := 0
	for _, eq := range res.Equipment {
		if eq.Template == 100401592 {
			landed++
		}
	}
	if landed != 11 {
		t.Fatalf("背包里 %d 颗誓言水晶，期望 11（源声明 [equipment] 100401592 11）", landed)
	}

	// ACK160 结果体：1 成功 + 2 错误 + 4 礼盒 + 2 礼盒槽 + 4 辅助 + 2 条数，
	// 每条 15 字节（模板 u32、数量 u32、7 个 0）。数量必须与背包一致。
	var ack []byte
	for _, p := range plan {
		if p.Name == "booster_open_ack" {
			ack = p.Payload
		}
	}
	if ack == nil {
		t.Fatal("missing booster_open_ack")
	}
	if n := binary.LittleEndian.Uint16(ack[13:15]); n != 1 {
		t.Fatalf("ACK 结果条数=%d 期望 1", n)
	}
	if tpl := binary.LittleEndian.Uint32(ack[15:19]); tpl != 100401592 {
		t.Fatalf("ACK 模板=%d 期望 100401592", tpl)
	}
	if cnt := binary.LittleEndian.Uint32(ack[19:23]); cnt != 11 {
		t.Fatalf("ACK 数量=%d 期望 11", cnt)
	}
}
