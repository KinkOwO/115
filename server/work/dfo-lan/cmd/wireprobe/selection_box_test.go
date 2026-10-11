package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"testing"
)

// 2026-09-22 玩家报告：右键装备自选盒弹「库存已满」。
// 自选盒（[booster select category]）不在 booster-catalog.json 里，落进通用随机池
// 分支后服务端回 "no reward pool defined"，客户端把它显示成"库存已满"。
// 现在这类盒子先走源范围校验：合法选择照常发放，越界被拒，没有选择时回一个空成功
// （客户端据此弹选择界面），而不是通用失败码。
//
// 装备目录用自洽 fixture：运行时那份 equipment.current35.json 只有 1536 行、不含
// 自选盒里的装备，而这里要验的是"选择 → 校验 → 发放"这条链路本身。
func loadSelectionBoxesForTest(t *testing.T) *catalog.SelectionBoxes {
	t.Helper()
	boxes, err := catalog.LoadSelectionBoxes("../../internal/catalog/testdata/selection-flow.json")
	if err != nil {
		t.Fatal(err)
	}
	return boxes
}

func equipmentFixtureFor(t *testing.T, picked uint32) *workflow.WearService {
	t.Helper()
	body := fmt.Sprintf(`{
	  "source": {"format":"test","path":"test","size":1,"checksum":"test","file_count":1,"group_count":1},
	  "rows": [{"ID":%d,"Path":"equipment/character/common/jacket/cloth/%d.equ",
	    "SHA256":"0000000000000000000000000000000000000000000000000000000000000000",
	    "Fields":{"[rarity]":[{"type":0,"value":3}],
	      "[equipment type]":[{"type":6,"value":166625925,"text":"[coat]"}],
	      "[durability]":[{"type":0,"value":60}]}}]
	}`, picked, picked)
	catPath := filepath.Join(t.TempDir(), "equipment.json")
	if err := os.WriteFile(catPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog(catPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	rules := inventory.BagRules{
		Source:            "test",
		Slots:             map[string][2]uint16{"[throw]": {65, 120}, "[material]": {121, 176}},
		MissingStackLimit: 1000,
		EquipmentSlots:    [2]uint16{9, 64},
	}
	return &workflow.WearService{WearService: inventory.WearService{Catalog: gear, BagRules: rules}}
}

func selectionBoxRequest(slot uint16, category uint16, picks ...uint32) []byte {
	p := make([]byte, 16+4*len(picks))
	binary.LittleEndian.PutUint16(p[0:2], slot)
	binary.LittleEndian.PutUint32(p[2:6], 1)
	binary.LittleEndian.PutUint16(p[6:8], category)
	for i, pick := range picks {
		binary.LittleEndian.PutUint32(p[8+i*4:12+i*4], pick)
	}
	return p
}

// firstSelectionBox 取一个"非奥德赛武器盒、且类别里带装备"的真实自选盒，返回
// (盒模板, 类别[u16], 装备模板)，避免把某个盒 id 写死进用例。
func firstSelectionBox(t *testing.T, boxes *catalog.SelectionBoxes) (uint32, uint16, uint32) {
	t.Helper()
	ids := make([]uint32, 0, len(boxes.Boxes))
	for key := range boxes.Boxes {
		n, err := strconv.ParseUint(key, 10, 32)
		if err == nil {
			ids = append(ids, uint32(n))
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if id == 10417789 {
			continue
		}
		box, _ := boxes.ByTemplate(id)
		for _, cat := range box.Categories {
			for _, it := range cat.Items {
				if it.Count == 1 && it.Template > 100000000 {
					return id, uint16(cat.Category[0]) | uint16(cat.Category[1])<<8, it.Template
				}
			}
		}
	}
	t.Fatal("no equipment-carrying selection box in the catalog")
	return 0, 0, 0
}

func selectionBoxSession(t *testing.T, boxes *catalog.SelectionBoxes, boxTemplate uint32) (*worldSession, *mockBoosterStore) {
	t.Helper()
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items:   []inventory.BagItem{{Slot: 65, Template: boxTemplate, Amount: 1}},
	}
	state, _ := inventory.SaveBag(json.RawMessage(`{}`), bag)
	char := database.Character{ID: 11, AccountID: 1, ConfigVersion: boxes.Source.SaveIdentity(), State: state}
	w := &worldSession{role: char, selectionBoxes: boxes}
	return w, newMockBoosterStore(char)
}

func TestSelectionBoxGrantsThePickedEquipment(t *testing.T) {
	boxes := loadSelectionBoxesForTest(t)
	boxTemplate, category, picked := firstSelectionBox(t, boxes)
	wear := equipmentFixtureFor(t, picked)
	w, store := selectionBoxSession(t, boxes, boxTemplate)

	req := selectionBoxRequest(65, category, picked)
	plan, err := w.openBoosterItem(context.Background(), store, wear, nil, nil, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal("open selection box failed:", err)
	}
	if len(plan) == 0 {
		t.Fatal("no packets")
	}
	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	granted := false
	for _, eq := range resBag.Equipment {
		if eq.Template == picked {
			granted = true
		}
	}
	if !granted {
		t.Fatalf("picked item %d not granted: equipment=%+v items=%+v", picked, resBag.Equipment, resBag.Items)
	}
	for _, it := range resBag.Items {
		if it.Slot == 65 {
			t.Fatalf("the box was not consumed: %+v", resBag.Items)
		}
	}
}

// 导出源来自 client-build/Script.inner.pvf，而客户端加载自己的 Script.pvf（两份不是
// 同一个构建），客户端 UI 给出的选择可能不在导出列表里，所以范围外的选择只能记录、
// 不能拒绝——否则客户端合法给出的选择会被服务端拒掉。这条用例锁住"照发不误"。
func TestSelectionBoxGrantsEvenWhenTheClientPVFDiffers(t *testing.T) {
	boxes := loadSelectionBoxesForTest(t)
	boxTemplate, category, _ := firstSelectionBox(t, boxes)
	wear := equipmentFixtureFor(t, 999999)
	w, store := selectionBoxSession(t, boxes, boxTemplate)

	req := selectionBoxRequest(65, category, 999999)
	plan, err := w.openBoosterItem(context.Background(), store, wear, nil, nil, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal("a pick the exported source does not list must still be granted:", err)
	}
	if len(plan) == 0 {
		t.Fatal("no packets")
	}
	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, eq := range resBag.Equipment {
		if eq.Template == 999999 {
			found = true
		}
	}
	if !found {
		t.Fatalf("pick %d not granted: %+v", 999999, resBag.Equipment)
	}
}

// 打开界面那一次请求（没有选择）不能报错、也不能发放：它只负责让客户端弹列表。
func TestSelectionBoxWithoutAPickGrantsNothing(t *testing.T) {
	boxes := loadSelectionBoxesForTest(t)
	boxTemplate, category, _ := firstSelectionBox(t, boxes)
	wear := equipmentFixtureFor(t, 100000001)
	w, store := selectionBoxSession(t, boxes, boxTemplate)

	req := selectionBoxRequest(65, category)
	plan, err := w.openBoosterItem(context.Background(), store, wear, nil, nil, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].Name != "selection_box_awaiting_pick" {
		t.Fatalf("expected an awaiting-pick answer: %+v", plan)
	}
	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Equipment) != 0 || len(resBag.Items) != 1 || resBag.Items[0].Template != boxTemplate {
		t.Fatalf("an unanswered box must not change the bag: %+v", resBag)
	}
}

// ★ 2026-10-11：Royal Pass 每日登录礼包 590722589 的**真实形态** —— 分类块里写的
// 是 `[stackable]`（不是 [equipment]），三选一与件数全在那里：
//
//	[booster select category] 0 0
//	[stackable] 10327284 90 10327285 45 10362432 50 [/stackable]
//
// 解析器原本把 `[stackable]` 当"未建模段"跳过 ⇒ `Resolve` 的件数表为空 ⇒ 发放回落
// 到**礼盒自身数量**（源 `[daily reward list]` 写的是 1）⇒ 实机表现是
// 「tooltip 写 x45，进背包只有 1 个」。
//
// 断言落在**下发帧**上：客户端背包里的数量取自 `booster_open_ack` 的 count 字段，
// 只看存档会漏掉"存档对了但帧里还是 1"这一类。
func TestSelectionBoxGrantsTheSourceStackableCount(t *testing.T) {
	boxes, err := catalog.NewSelectionBoxes(catalog.SelectionBoxes{
		Model:  catalog.SelectionBoxModel,
		Source: pvf.ArchiveSnapshot{Checksum: "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"},
		Boxes: map[string]catalog.SelectionBox{
			"590722589": {
				Template: 590722589,
				Path:     "stackable/cash/590722589.stk",
				SHA256:   "0000000000000000000000000000000000000000000000000000000000000001",
				Categories: []catalog.SelectionCategory{{
					Category: [2]byte{0, 0},
					Items: []catalog.SelectionItem{
						{Template: 10327284, Count: 90},
						{Template: 10327285, Count: 45},
						{Template: 10362432, Count: 50},
					},
					Sections: []string{"[stackable]"},
				}},
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	// 件数表非空 = 这条路径才有"按源发放"的能力；为空就是又回落到礼盒自身数量。
	if items, _, checked := boxes.Resolve(590722589, [2]byte{0, 0}, []uint32{10327285}); !checked ||
		len(items) != 1 || items[0].Count != 45 {
		t.Fatalf("resolve = %+v checked=%v", items, checked)
	}

	rules := inventory.BagRules{
		Source:            "test",
		EquipmentSlots:    [2]uint16{9, 64},
		Slots:             map[string][2]uint16{"[booster]": {65, 120}, "[material]": {121, 176}},
		MissingStackLimit: 1000,
	}
	lootSvc := &loot.Service{
		BagRules: rules,
		Catalog: catalog.LootCatalog{
			Source: pvf.ArchiveSnapshot{Checksum: "test"},
			Items: map[uint32]catalog.LootItem{
				10327285: {ID: 10327285, Kind: "stackable", StackableType: "[material]", StackLimit: 1000},
			},
		},
	}
	w, store := selectionBoxSession(t, boxes, 590722589)

	req := selectionBoxRequest(65, 0, 10327285)
	plan, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, nil, odysseyWeaponChoices{}, req, req)
	if err != nil {
		t.Fatal("open the daily reward box failed:", err)
	}
	var ack []byte
	for _, p := range plan {
		if p.Name == "booster_open_ack" {
			ack = p.Payload
		}
	}
	if ack == nil {
		t.Fatalf("no booster_open_ack in %+v", plan)
	}
	// BoosterOpenSuccess 布局：u8 1 | u16 0 | u32 盒 | u16 槽 | u32 0 | u16 条数
	// | 每条 u32 模板 + u32 数量 + 7B。
	if len(ack) < 15+15 {
		t.Fatalf("ack 太短：%d", len(ack))
	}
	if n := binary.LittleEndian.Uint16(ack[13:15]); n != 1 {
		t.Fatalf("granted rows = %d", n)
	}
	if tmpl := binary.LittleEndian.Uint32(ack[15:19]); tmpl != 10327285 {
		t.Fatalf("granted template = %d", tmpl)
	}
	if got := binary.LittleEndian.Uint32(ack[19:23]); got != 45 {
		t.Fatalf("下发数量 = %d，应为 45（源 `10327285 45`）", got)
	}
	// 存档次之：材料确实以 45 落包。
	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	for _, it := range resBag.Items {
		if it.Template == 10327285 {
			if it.Amount != 45 {
				t.Fatalf("背包数量 = %d，应为 45", it.Amount)
			}
			return
		}
	}
	t.Fatalf("材料没进包：%+v", resBag.Items)
}
