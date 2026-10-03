package main

import (
	"encoding/json"
	"os"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
)

// 沉月湖翻牌的**固定产出**必须只来自源里声明过的模板，并且能结算 —— 否则 C2284 会在
// ValidateMoonRewards 处被拒（实机 2026-10-02：`Moon stack destination missing`，
// 客户端表现为点 Start 没反应）。
//
// 期望值全部来自源，不是发明的：
//
//	2f_100004137.dgn: [normal group index] 2 21251 3 1 21291，[minimum required level] 115
//	etc/dungeondroptablebygroup 组 21291: 10362429 深渊门票、10362432 迷雾工商协会银币
//	list/stackable.lst 索引: 两者都是 [material]（有明确槽位带，可结算）
//
// 数量（门票 60 / 银币 15）与"装备 1..3 件"是服务端口径：玩家 2026-10-02 的实测产出是
// 银币 ×14~15、门票 ×60、另加若干件装备，而 PVF 里没有"这次给几张"这种表。
func TestMoonRewardPoolFromSource(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for the Moon reward pool source check")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: path, MaxBytes: 1024 * 1024 * 1024}, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = a.Close() }()
	dungeons, err := catalog.ImportDungeons(a, []uint32{100004136, 100004137})
	if err != nil {
		t.Fatal(err)
	}
	second, ok := dungeons.Dungeons[100004137]
	if !ok {
		t.Fatal("100004137 不在直读副本目录里")
	}
	if second.MinimumLevel != 115 {
		t.Fatalf("月湖第二层的 [minimum required level] = %d，期望 115（装备掷骰用的等级）", second.MinimumLevel)
	}
	items, err := catalog.ImportLoot(a, 150)
	if err != nil {
		t.Fatal(err)
	}
	// 直读启动路径在构造掉落服务前会用**同源直读**的物品索引补齐 stackable
	// （main.go -> pvfCoreCatalogs.supplementStackables -> SupplementItemIndex）。
	// 门票/银币只在这份索引里：少了这一步 Catalog 只有 1023 个掉落候选。
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		t.Fatal(err)
	}
	if err := items.SupplementItemIndex(index); err != nil {
		t.Fatal(err)
	}
	bag, err := inventory.LoadBagRules("../../configs/inventory.current37.json", dungeons.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := loot.Parse(items)
	if err != nil {
		t.Fatal(err)
	}
	rules, err := loot.LoadRules("../../configs/drop.compat90.json")
	if err != nil {
		t.Fatal(err)
	}
	// 装备源：configs/equipment.current37.json 是上一轮构建的导出、checksum 与当前内层
	// 归档不同（真源的那份由生产启动的 equipment-selection 直读准备，它还需要角色目录，
	// 而角色导出同样是旧构建）。这里把 source 对齐当前真源后加载它，只用于让装备掷骰
	// 有池可用 —— 装备定义本身（部位、最低等级、耐久）在两次构建间取自同一套客户端资源。
	raw, err := os.ReadFile("../../configs/equipment.current37.json")
	if err != nil {
		t.Fatal(err)
	}
	var equipmentJSON inventory.EquipmentCatalog
	if err = json.Unmarshal(raw, &equipmentJSON); err != nil {
		t.Fatal(err)
	}
	equipmentJSON.Source.Checksum = dungeons.Source.Checksum
	gear, err := inventory.NewEquipmentCatalog(equipmentJSON, dungeons.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	svc := &loot.Service{Catalog: items, BagRules: bag, Equipment: gear, Tables: tables, Rules: rules}
	if g, ok := items.DropGroupByID(21291); !ok || len(g.Explicit) == 0 {
		t.Fatalf("月湖门票/银币组 21291 不可读: ok=%v %+v", ok, g)
	}
	fixed, pool, err := moonRewardChoicesFromDungeon(second, svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("Moon 池来源: %s", pool)
	want := []loot.MoonRewardChoice{
		{Template: 10362429, Count: 60, Group: 21291},
		{Template: 10362432, Count: 15, Group: 21291},
	}
	if len(fixed) != len(want) {
		t.Fatalf("固定产出 = %+v，期望 %+v", fixed, want)
	}
	for i := range want {
		if fixed[i] != want[i] {
			t.Fatalf("固定产出[%d] = %+v，期望 %+v", i, fixed[i], want[i])
		}
	}
	// 端到端：进本前的这道校验必须放过这套策略（这正是先前的卡点）。
	policy := loot.MoonRewardPolicy{
		Source:  items.Source.SaveIdentity(),
		Draws:   second.RewardCard,
		Choices: fixed,
		Equipment: loot.MoonEquipmentPolicy{
			Min: moonSoloEquipmentMin, Max: moonSoloEquipmentMax,
			Weights: moonSoloEquipmentWeights,
			Level:   byte(second.MinimumLevel), Rank: moonSoloEquipmentRank,
		},
	}
	if err := svc.ValidateMoonRewards(policy); err != nil {
		t.Fatal("直读策略未通过 ValidateMoonRewards:", err)
	}
	// 回归钉：先前面板图标是 [etc]（无槽位带），必须一直被拒；装备罐是
	// [upgradable legacy] 代理，两条发放路径都不认，也不能被直接发出来。
	for _, bad := range []uint32{10403102, 10403103, 10403104} {
		if _, ok := svc.MoonSettleableStack(bad); ok {
			t.Fatalf("模板 %d 不应被判为可结算堆叠物", bad)
		}
	}
	if _, ok := svc.MoonSettleableStack(10404337); ok {
		t.Fatal("装备罐 10404337 是 [upgradable legacy] 代理，不能被判为可结算堆叠物")
	}
}

// 装备侧判据：普通装备可结算；另开容器的部位（时装/宠物/护石）与查不到定义的模板一律不发。
//
// 判据与发奖同源（Service.moonEquipmentDestination）：要能查到定义、部位不是另开容器的
// 那类、且装备源给得出耐久。
func TestMoonSettleableEquipmentBoundary(t *testing.T) {
	const checksum = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	row := func(id uint32, kind string, level int32) inventory.EquipmentDefinition {
		return inventory.EquipmentDefinition{
			ID: id, Path: "equipment/test", SHA256: checksum[0:64],
			Fields: map[string][]pvf.Token{
				"[equipment type]": {{Type: 6, Text: kind}, {Type: 0, Value: 14}},
				"[minimum level]":  {{Type: 0, Value: level}},
				"[durability]":     {{Type: 0, Value: 40}},
				"[rarity]":         {{Type: 0, Value: 2}},
			},
		}
	}
	gear, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: checksum},
		Rows: []inventory.EquipmentDefinition{
			row(1001, "[coat]", 115), // 月湖会出的那类装备
			row(1003, "[aurora avatar]", 115),
		},
	}, checksum)
	if err != nil {
		t.Fatal(err)
	}
	svc := &loot.Service{Equipment: gear}
	if _, ok := svc.MoonSettleableEquipment(1001, 115); !ok {
		t.Fatal("115 级普通装备应可结算")
	}
	if _, ok := svc.MoonSettleableEquipment(1003, 115); ok {
		t.Fatal("时装部位不属于普通装备，不能结算")
	}
	if _, ok := svc.MoonSettleableEquipment(9999, 115); ok {
		t.Fatal("目录里没有的模板不能结算")
	}
}
