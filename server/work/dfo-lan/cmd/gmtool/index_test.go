package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// TestStackableClassification 锁定 stack_type -> 客户端页签分类的映射。
//
// 依据：docs/背包物品分类迁移实现报告.md §3「权威槽位区间表」与 §5.1
// 「扩展背包规则」的分类→槽位映射表。文档未提及的 stack_type 一律按
// §5.1/§3 的兜底规则（默认落入消耗品栏 65~120）归入消耗品。
func TestStackableClassification(t *testing.T) {
	cases := []struct {
		stackType string
		wantKey   string
		why       string
	}{
		// §3：消耗品栏 65~120
		{"[waste]", typeConsumable, "药水/消耗品，§3 列入消耗品栏"},
		{"[throw]", typeConsumable, "投掷物，§3 列入消耗品栏"},
		{"[booster]", typeConsumable, "礼盒，§3 列入消耗品栏"},
		{"[booster selection]", typeConsumable, "§3 列入消耗品栏"},
		{"[booster random]", typeConsumable, "§3 列入消耗品栏"},
		{"[cera package]", typeConsumable, "福袋/券包，§3 列入消耗品栏"},
		{"[usable cera package]", typeConsumable, "§3 列入消耗品栏"},
		{"[etc]", typeConsumable, "§3 列入消耗品栏"},
		{"[contract]", typeConsumable, "§3 列入消耗品栏"},
		{"[recipe]", typeConsumable, "§3 列入消耗品栏"},
		// §3：材料栏 121~176
		{"[material]", typeMaterial, "§3 列入材料栏"},
		{"[upgrade limit cube]", typeMaterial, "§3 列入材料栏"},
		// §3：任务栏 177~232
		{"[quest]", typeQuest, "§3 列入任务栏"},
		{"[quest receive]", typeQuest, "§3 列入任务栏"},
		// §3：副职业栏 233~288
		{"[material expert job]", typeProfession, "§3 列入副职业栏"},
		// §3：徽章栏 289~344
		{"[avatar emblem]", typeEmblem, "§3 列入徽章栏"},
		// 文档未提及 -> §5.1 兜底为消耗品
		{"[rune]", typeConsumable, "文档未提及，按 §5.1 兜底消耗品"},
		{"[enchant waste]", typeConsumable, "文档未提及，按 §5.1 兜底消耗品"},
		{"[legacy]", typeConsumable, "文档未提及，按 §5.1 兜底消耗品"},
		{"[made up type]", typeConsumable, "完全未知的 stack_type 也必须兜底，不能归入其它"},
	}
	for _, c := range cases {
		key, label := classifyStackable(c.stackType)
		if key != c.wantKey {
			t.Errorf("classifyStackable(%q) key = %q, 期望 %q（%s）", c.stackType, key, c.wantKey, c.why)
		}
		if label == "" {
			t.Errorf("classifyStackable(%q) 返回了空标签", c.stackType)
		}
	}
}

// TestEquipmentSlotClassification 锁定 [equipment type] cell -> 装备位 的映射。
//
// 这里把 configs/equipment.current37.json 全部 19,955 行里实测出现过的 22 种
// cell 取值一个不漏地列出来（行数见 index.go 的 equipmentSlotCells 注释）。
// 前 14 种对应服务端现役装备位（configs/equipment-wear.current35.json 的 slots，
// 槽位号 12..25），其余 8 种没有对应装备位，必须稳定落入「其它装备」。
func TestEquipmentSlotClassification(t *testing.T) {
	cases := []struct {
		cell     string
		wantSlot string
		wantWear int
		why      string
	}{
		// 服务端现役 14 个装备位（equipment-wear.current35.json: slots）
		{"[weapon]", "武器", 12, "slots [weapon]=12"},
		{"[title name]", "称号", 13, "slots [title name]=13"},
		{"[coat]", "上衣", 14, "slots [coat]=14"},
		{"[shoulder]", "头肩", 15, "slots [shoulder]=15"},
		{"[pants]", "下装", 16, "slots [pants]=16"},
		{"[shoes]", "鞋", 17, "slots [shoes]=17"},
		{"[waist]", "腰带", 18, "slots [waist]=18"},
		{"[amulet]", "项链", 19, "slots [amulet]=19"},
		{"[wrist]", "手镯", 20, "slots [wrist]=20"},
		{"[ring]", "戒指", 21, "slots [ring]=21"},
		{"[support]", "辅助装备", 22, "slots [support]=22"},
		{"[magic stone]", "魔法石", 23, "slots [magic stone]=23"},
		{"[support weapon]", "辅助武器", 24, "slots [support weapon]=24（equipment-full.json 里 27 行）"},
		{"[earring]", "耳环", 25, "slots [earring]=25"},
		// 目录里出现但不对应装备位 -> 其它装备
		{"[talisman]", slotOther, 0, "护石：不在 slots 表里"},
		{"[amalgamation stone]", slotOther, 0, "融合石：不在 slots 表里"},
		{"[oath]", slotOther, 0, "誓约：不在 slots 表里"},
		{"[primer]", slotOther, 0, "刻印：不在 slots 表里"},
		{"[artifact red]", slotOther, 0, "不在 slots 表里"},
		{"[artifact blue]", slotOther, 0, "不在 slots 表里"},
		{"[artifact green]", slotOther, 0, "不在 slots 表里"},
		{"[flag]", slotOther, 0, "旗帜：不在 slots 表里"},
		{"[creature]", slotOther, 0, "宠物：不在 slots 表里"},
		// equipment-full.json 里额外的取值
		{"[charm]", slotOther, 0, "护符：不在 slots 表里"},
		// 装扮类（cell 自带 " avatar" 后缀）单独一桶，避免把 5 万件时装混进装备位
		{"[hat avatar]", slotAvatar, 0, "装扮：cell 带 avatar 后缀"},
		{"[coat avatar]", slotAvatar, 0, "装扮：cell 带 avatar 后缀"},
		{"[weapon avatar]", slotAvatar, 0, "装扮：cell 带 avatar 后缀"},
		{"[aurora avatar]", slotAvatar, 0, "装扮：cell 带 avatar 后缀"},
		// 完全没有见过 / 空的 cell：不能崩，也不能丢掉条目
		{"", slotOther, 0, "空 cell 归其它装备"},
		{"[unknown future cell]", slotOther, 0, "未知 cell 归其它装备，不猜测"},
	}
	for _, c := range cases {
		slot, group, wear := slotOfEquipmentCell(c.cell)
		if slot != c.wantSlot || wear != c.wantWear {
			t.Errorf("slotOfEquipmentCell(%q) = (%q, %q, %d), 期望 (%q, _, %d)（%s）",
				c.cell, slot, group, wear, c.wantSlot, c.wantWear, c.why)
		}
		if slot == "" || group == "" {
			t.Errorf("slotOfEquipmentCell(%q) 返回了空标签", c.cell)
		}
		if slot != slotOther && slot != slotAvatar && slotGroupOf(slot) == groupOther {
			t.Errorf("装备位 %q 的大类不该是「其它」", slot)
		}
	}
}

// TestEquipmentTypeCellsInRealCatalog 用真实的 configs/equipment.current37.json
// 复算一遍 [equipment type] 的取值集合：必须与 index.go 注释里声明的 22 种完全一致，
// 并且每一种都能被分类（要么命中现役装备位，要么稳定落入「其它装备」）。
//
// 这条测试是"枚举全部 19,955 行，一个不漏"这个要求本身的守门人：
// 目录一旦更新出新 cell，这里会直接失败，逼着人回去补映射而不是猜。
func TestEquipmentTypeCellsInRealCatalog(t *testing.T) {
	if os.Getenv("DFO_EXTENDED_CATALOG_INTEGRATION") != "1" {
		t.Skip("requires externally generated 19955-row equipment catalog; set DFO_EXTENDED_CATALOG_INTEGRATION=1")
	}
	const p = "../../configs/equipment.current37.json"
	if _, err := os.Stat(p); err != nil {
		t.Skipf("装备目录不在（%s 不可读：%v），跳过对真实目录的复算", p, err)
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Rows []catalogRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Rows) != 19955 {
		t.Fatalf("装备目录行数 %d，期望 19955（变了就要重新核对 cell 枚举）", len(doc.Rows))
	}
	got := map[string]int{}
	for _, row := range doc.Rows {
		cell := cellText(row.Fields["[equipment type]"])
		if cell == "" {
			t.Errorf("ID %d 没有 [equipment type] cell 文本", row.ID)
			continue
		}
		got[cell]++
		if slot, group, _ := slotOfEquipmentCell(cell); slot == "" || group == "" {
			t.Errorf("ID %d 的 cell %q 没有分类", row.ID, cell)
		}
	}
	// index.go 注释里逐条列出的 22 种取值及行数。
	want := map[string]int{
		"[weapon]": 6890, "[coat]": 1751, "[pants]": 1632, "[shoes]": 1590,
		"[shoulder]": 1513, "[waist]": 1505, "[talisman]": 910,
		"[amalgamation stone]": 750, "[wrist]": 502, "[amulet]": 496,
		"[ring]": 492, "[magic stone]": 479, "[support]": 444, "[earring]": 433,
		"[title name]": 311, "[oath]": 113, "[primer]": 76,
		"[artifact red]": 20, "[artifact blue]": 20, "[artifact green]": 20,
		"[flag]": 6, "[creature]": 2,
	}
	if len(got) != len(want) {
		keys := make([]string, 0, len(got))
		for k := range got {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		t.Fatalf("[equipment type] 取值 %d 种，期望 %d 种；实际：%v", len(got), len(want), keys)
	}
	for cell, n := range want {
		if got[cell] != n {
			t.Errorf("[equipment type] %q 行数 %d，期望 %d", cell, got[cell], n)
		}
	}
}

// TestRarityLabelsAndIndex 锁定品级中文名与 rarity= 参数的解析。
//
// 依据：客户端内层 PVF 的 string/ui.uv.str 文本表原文
// （common_rarity_0..8 = 普通/高级/稀有/神器/史诗/勇者/传说/神话/太初）。
// 注意 names.zh.json 里同一批键是「纪事/原始的」，与客户端不一致，
// 所以默认兜底必须用客户端那一份。
func TestRarityLabelsAndIndex(t *testing.T) {
	ix := &ItemIndex{namesClient: NameTable{}, namesZH: NameTable{}, namesEN: NameTable{}}
	want := []string{"普通", "高级", "稀有", "神器", "史诗", "勇者", "传说", "神话", "太初"}
	if len(defaultRarityLabels) != len(want) {
		t.Fatalf("内置品级名 %d 条，期望 %d 条", len(defaultRarityLabels), len(want))
	}
	for i, w := range want {
		if got := ix.rarityLabel(int32(i)); got != w {
			t.Errorf("rarityLabel(%d) = %q，期望 %q（客户端 string/ui.uv.str）", i, got, w)
		}
		if n, ok := ix.RarityIndex(w); !ok || n != i {
			t.Errorf("RarityIndex(%q) = (%d,%v)，期望 (%d,true)", w, n, ok, i)
		}
		if n, ok := ix.RarityIndex(string(rune('0' + i))); !ok || n != i {
			t.Errorf("RarityIndex(%d) = (%d,%v)，期望 (%d,true)", i, n, ok, i)
		}
	}
	// 客户端名字表优先于汉化表
	ix.namesClient["common_rarity_8"] = "太初"
	ix.namesZH["common_rarity_8"] = "原始的"
	ix.namesZH["common_rarity_5"] = "纪事"
	if got := ix.rarityLabel(8); got != "太初" {
		t.Errorf("客户端名字表应当优先，得到 %q", got)
	}
	// 汉化流水线那套名字作为别名仍然可以当过滤条件（显示名不变）
	if n, ok := ix.RarityIndex("原始的"); !ok || n != 8 {
		t.Errorf("别名「原始的」应解析为 8，得到 (%d,%v)", n, ok)
	}
	if n, ok := ix.RarityIndex("纪事"); !ok || n != 5 {
		t.Errorf("别名「纪事」应解析为 5，得到 (%d,%v)", n, ok)
	}
	if got := ix.rarityLabel(8); got != "太初" {
		t.Errorf("别名不该改变显示名，得到 %q", got)
	}
	// 非法值必须被拒（接口据此返回 400），而不是静默给 0 条结果
	for _, bad := range []string{"", "9", "-1", "999", "传说级", "abc"} {
		if _, ok := ix.RarityIndex(bad); ok {
			t.Errorf("RarityIndex(%q) 不该被接受", bad)
		}
	}
}

// TestClientNamesWinOverPipelineNames 验证物品名的优先级：
// 客户端 PVF 文本表 > 汉化流水线 names.zh.json > 物品库自带的英文名。
//
// 真实例子：id 101001153 在客户端是「太初之星 - 短剑」，
// 在 names.zh.json 里是「原始的星辰 - 短剑」。
func TestClientNamesWinOverPipelineNames(t *testing.T) {
	ix := &ItemIndex{
		byID:        map[uint32]int{},
		namesClient: NameTable{"name_101001153": "太初之星 - 短剑"},
		namesZH:     NameTable{"name_101001153": "原始的星辰 - 短剑", "name_2": "流水线名"},
		namesEN:     NameTable{"name_101001153": "Primordial Star - Short Sword"},
	}
	ix.push(IndexItem{ID: 101001153, Name: "Primordial Star - Short Sword", Kind: "equipment"})
	ix.push(IndexItem{ID: 2, Name: "Fallback EN", Kind: "stackable"})
	ix.push(IndexItem{ID: 3, Name: "Only EN", Kind: "stackable"})
	ix.finish()
	// finish 会按 ID 重排，所以按 ID 取而不是按下标。
	cases := map[uint32]struct{ name, nameEN string }{
		101001153: {"太初之星 - 短剑", "Primordial Star - Short Sword"},
		2:         {"流水线名", "Fallback EN"},
		3:         {"Only EN", "Only EN"},
	}
	for id, w := range cases {
		it, ok := ix.Get(id)
		if !ok {
			t.Fatalf("ID %d 不在索引里", id)
		}
		if it.Name != w.name {
			t.Errorf("ID %d 名字 %q，期望 %q", id, it.Name, w.name)
		}
		if it.NameEN != w.nameEN {
			t.Errorf("ID %d 英文名 %q，期望 %q", id, it.NameEN, w.nameEN)
		}
	}
}

// TestRefineEquipmentBySlotMap 验证部位表（equipment.slots.json）给装备
// 补上部位与最低等级，且装备库文件里没写等级的条目也补上。
func TestRefineEquipmentBySlotMap(t *testing.T) {
	ix := &ItemIndex{
		byID:        map[uint32]int{},
		namesClient: NameTable{},
		namesZH:     NameTable{},
		namesEN:     NameTable{},
		slotMap: map[string]slotMapEntry{
			"101001153": {Cell: "[weapon]", Level: 115},
			"10018":     {Cell: "[coat]", Level: 20},
			"500001":    {Cell: "[hat avatar]", Level: 1},
			"900001":    {Cell: "[talisman]", Level: 100},
		},
	}
	for _, it := range []IndexItem{
		{ID: 101001153, Name: "sword", Kind: "equipment"},
		{ID: 10018, Name: "coat", Kind: "equipment"},
		{ID: 500001, Name: "hat", Kind: "equipment"},
		{ID: 900001, Name: "talisman", Kind: "equipment"},
		{ID: 7, Name: "potion", Kind: "stackable"},
	} {
		ix.push(it)
	}
	ix.refineEquipment(nil)
	ix.finish()

	want := map[uint32]struct {
		slot  string
		group string
		level int32
	}{
		101001153: {"武器", "武器", 115},
		10018:     {"上衣", "防具", 20},
		500001:    {"时装", "时装", 1},
		900001:    {"其它装备", "其它", 100},
		7:         {"", "", 0},
	}
	for id, w := range want {
		it, ok := ix.Get(id)
		if !ok {
			t.Fatalf("ID %d 不在索引里", id)
		}
		if it.Slot != w.slot || it.Level != w.level {
			t.Errorf("ID %d = (slot %q, level %d)，期望 (%q, %d)", id, it.Slot, it.Level, w.slot, w.level)
		}
		if w.group != "" && it.Group != w.group {
			t.Errorf("ID %d 大类 %q，期望 %q", id, it.Group, w.group)
		}
	}
}

// fixtureIndex 造一个覆盖"装备位 + 堆叠物 + 品级 + 等级"的小索引。
func fixtureIndex() *ItemIndex {
	ix := &ItemIndex{
		byID:        map[uint32]int{},
		namesClient: NameTable{},
		namesZH:     NameTable{},
		namesEN:     NameTable{},
		slotMap:     map[string]slotMapEntry{},
	}
	ix.items = []ItemEntry{
		{ID: 1, Kind: "stackable", Name: "红色药水", NameEN: "Red Potion", Grade: 1, Rarity: 0, RarityLabel: "普通", TypeKey: typeConsumable, TypeLabel: "消耗品"},
		{ID: 2, Kind: "stackable", Name: "蓝色药水", NameEN: "Blue Potion", Grade: 1, Rarity: 0, RarityLabel: "普通", TypeKey: typeConsumable, TypeLabel: "消耗品"},
		{ID: 3, Kind: "stackable", Name: "铁矿石", NameEN: "Iron Ore", Grade: 1, Rarity: 1, RarityLabel: "高级", TypeKey: typeMaterial, TypeLabel: "材料"},
		{ID: 10000, Kind: "equipment", Name: "布甲上衣", NameEN: "Cloth Coat", Level: 30, Grade: 1, Rarity: 5, RarityLabel: "勇者", TypeKey: "上衣", TypeLabel: "上衣", Slot: "上衣", Group: "防具"},
		{ID: 10001, Kind: "equipment", Name: "木剑", NameEN: "Wooden Sword", Level: 115, Grade: 1, Rarity: 8, RarityLabel: "太初", TypeKey: "武器", TypeLabel: "武器", Slot: "武器", Group: "武器"},
		{ID: 10002, Kind: "equipment", Name: "圣剑", NameEN: "Holy Sword", Level: 115, Grade: 1, Rarity: 4, RarityLabel: "史诗", TypeKey: "武器", TypeLabel: "武器", Slot: "武器", Group: "武器"},
		{ID: 10003, Kind: "equipment", Name: "未知项链", NameEN: "Unknown Necklace", Level: 0, Rarity: 2, RarityLabel: "稀有", TypeKey: "项链", TypeLabel: "项链", Slot: "项链", Group: "首饰"},
		{ID: 10004, Kind: "equipment", Name: "布帽", NameEN: "Cloth Hat", Level: 10, Grade: 1, Rarity: 0, RarityLabel: "普通", TypeKey: "头肩", TypeLabel: "头肩", Slot: "头肩", Group: "防具"},
	}
	ix.finish()
	return ix
}

// TestSearchTypeFilterAndQuery 验证 type= 过滤与 q= 搜索可叠加，且空 q 按类型列出。
func TestSearchTypeFilterAndQuery(t *testing.T) {
	ix := fixtureIndex()

	// 空条件 + type：按类型列出（不套默认低等级门槛）
	got := ix.Search(ItemFilter{Type: typeConsumable, Limit: 60})
	if len(got) != 2 {
		t.Fatalf("空 q 按消耗品列出得到 %d 件，期望 2 件", len(got))
	}
	for _, it := range got {
		if it.TypeKey != typeConsumable {
			t.Errorf("消耗品过滤混入了 %s（ID %d）", it.TypeKey, it.ID)
		}
	}

	// q + type 叠加：只有同时命中两者才返回
	if got = ix.Search(ItemFilter{Q: "药水", Type: typeMaterial, Limit: 60}); len(got) != 0 {
		t.Fatalf("q=药水 type=材料 期望 0 件，得到 %d 件", len(got))
	}
	if got = ix.Search(ItemFilter{Q: "药水", Type: typeConsumable, Limit: 60}); len(got) != 2 {
		t.Fatalf("q=药水 type=消耗品 期望 2 件，得到 %d 件", len(got))
	}

	// 顶层「装备」是聚合桶
	if got = ix.Search(ItemFilter{Type: typeEquipment, Limit: 60}); len(got) != 5 {
		t.Fatalf("type=equipment 期望 5 件装备，得到 %d 件", len(got))
	}
	// 装备位可直接当 type= 用
	if got = ix.Search(ItemFilter{Type: "防具", Limit: 60}); len(got) != 2 {
		t.Fatalf("type=防具（大类）期望 2 件，得到 %+v", got)
	}
	if got = ix.Search(ItemFilter{Q: "木剑", Type: "武器", Limit: 60}); len(got) != 1 || got[0].ID != 10001 {
		t.Fatalf("q=木剑 type=武器 期望命中 10001，得到 %+v", got)
	}
	if got = ix.Search(ItemFilter{Q: "木剑", Type: "防具", Limit: 60}); len(got) != 0 {
		t.Fatalf("q=木剑 type=防具 期望 0 件，得到 %d 件", len(got))
	}

	// 纯数字按 ID 前缀，并且也受 type= 约束
	if got = ix.Search(ItemFilter{Q: "1000", Limit: 60}); len(got) != 5 {
		t.Fatalf("ID 前缀搜索 1000 期望命中 5 件，得到 %d 件", len(got))
	}
	if got = ix.Search(ItemFilter{Q: "1000", Type: typeConsumable, Limit: 60}); len(got) != 0 {
		t.Fatalf("ID 前缀 + 消耗品过滤期望 0 件，得到 %d 件", len(got))
	}
	if got = ix.Search(ItemFilter{Q: "1000", Type: typeEquipment, Limit: 60}); len(got) != 5 {
		t.Fatalf("ID 前缀 + 装备过滤期望 5 件，得到 %d 件", len(got))
	}
}

// TestSearchSlotLevelRarityFilters 验证三个新筛选（部位/等级/稀有度）单独与叠加都正确。
func TestSearchSlotLevelRarityFilters(t *testing.T) {
	ix := fixtureIndex()

	// 部位：接受中文标签，也接受原始 [cell] 文本
	got := ix.Search(ItemFilter{Slot: "武器", Limit: 60})
	if len(got) != 2 {
		t.Fatalf("slot=武器 期望 2 件，得到 %d 件：%+v", len(got), got)
	}
	if got = ix.Search(ItemFilter{Slot: "[weapon]", Limit: 60}); len(got) != 2 {
		t.Fatalf("slot=[weapon] 期望 2 件，得到 %d 件", len(got))
	}
	// 部位筛选不能把非装备放进来（堆叠物没有部位）
	if got = ix.Search(ItemFilter{Slot: "消耗品", Limit: 60}); len(got) != 0 {
		t.Fatalf("slot 只对装备有意义，得到 %d 件", len(got))
	}

	// 等级区间：等级未知（0）的装备在给了等级条件时必须被排除
	got = ix.Search(ItemFilter{LevelMin: 111, LevelMax: 115, Limit: 60})
	if len(got) != 2 {
		t.Fatalf("level 111-115 期望 2 件（木剑/圣剑），得到 %d 件：%+v", len(got), got)
	}
	for _, it := range got {
		if it.Level < 111 || it.Level > 115 {
			t.Errorf("等级过滤混入了 Lv%d（ID %d）", it.Level, it.ID)
		}
	}
	if got = ix.Search(ItemFilter{LevelMin: 21, LevelMax: 50, Limit: 60}); len(got) != 1 || got[0].ID != 10000 {
		t.Fatalf("level 21-50 期望只有 10000，得到 %+v", got)
	}
	// 只有下限 / 只有上限
	if got = ix.Search(ItemFilter{LevelMin: 111, Limit: 60}); len(got) != 2 {
		t.Fatalf("level_min=111 期望 2 件，得到 %d 件", len(got))
	}
	if got = ix.Search(ItemFilter{LevelMax: 10, Limit: 60}); len(got) != 1 || got[0].ID != 10004 {
		t.Fatalf("level_max=10 期望只有 10004，得到 %+v", got)
	}

	// 稀有度：数字与中文名等价；普通(0) 不能被当成"不限"
	if got = ix.Search(ItemFilter{Rarity: 0, RaritySet: true, Limit: 60}); len(got) != 3 {
		t.Fatalf("rarity=普通 期望 3 件，得到 %d 件：%+v", len(got), got)
	}
	if got = ix.Search(ItemFilter{Rarity: 8, RaritySet: true, Limit: 60}); len(got) != 1 || got[0].ID != 10001 {
		t.Fatalf("rarity=太初 期望只有 10001，得到 %+v", got)
	}
	if got = ix.Search(ItemFilter{Limit: 60}); len(got) == 0 {
		t.Fatal("不筛品级时不该被当成「只看普通」")
	}

	// 三个条件叠加：部位 + 等级 + 稀有度
	got = ix.Search(ItemFilter{Slot: "武器", LevelMin: 110, Rarity: 4, RaritySet: true, Limit: 60})
	if len(got) != 1 || got[0].ID != 10002 {
		t.Fatalf("武器 + Lv>=110 + 史诗 期望只有圣剑(10002)，得到 %+v", got)
	}
	// 叠加后无匹配
	if got = ix.Search(ItemFilter{Slot: "上衣", LevelMin: 110, Limit: 60}); len(got) != 0 {
		t.Fatalf("上衣 + Lv>=110 期望 0 件，得到 %d 件", len(got))
	}

	// type= 与 slot= 叠加：两者都满足才返回
	if got = ix.Search(ItemFilter{Type: "防具", Slot: "头肩", Limit: 60}); len(got) != 1 || got[0].ID != 10004 {
		t.Fatalf("type=防具 + slot=头肩 期望只有 10004，得到 %+v", got)
	}
	if got = ix.Search(ItemFilter{Type: "武器", Slot: "头肩", Limit: 60}); len(got) != 0 {
		t.Fatalf("type=武器 + slot=头肩 期望 0 件，得到 %d 件", len(got))
	}

	// limit 生效
	if got = ix.Search(ItemFilter{Type: typeEquipment, Limit: 2}); len(got) != 2 {
		t.Fatalf("limit=2 期望 2 件，得到 %d 件", len(got))
	}
}

// TestFiltersCounts 验证 /api/filters 的数量统计与顺序。
func TestFiltersCounts(t *testing.T) {
	ix := fixtureIndex()
	f := ix.Filters()

	if f.Equipment != 5 || f.SlotCovered != 5 || f.SlotUnknown != 0 {
		t.Errorf("装备/已归类/未归类 = %d/%d/%d，期望 5/5/0", f.Equipment, f.SlotCovered, f.SlotUnknown)
	}
	if f.LevelKnown != 4 || f.LevelUnknown != 1 {
		t.Errorf("有等级/无等级 = %d/%d，期望 4/1", f.LevelKnown, f.LevelUnknown)
	}
	// 部位数量：以 equipmentSlotOrder 为准，未出现的部位数量为 0 但必须存在
	if len(f.Slots) != len(equipmentSlotOrder)+2 {
		t.Fatalf("部位选项 %d 个，期望 %d（14 个现役装备位 + 时装 + 其它装备）",
			len(f.Slots), len(equipmentSlotOrder)+2)
	}
	for i, label := range equipmentSlotOrder {
		if f.Slots[i].Key != label || f.Slots[i].Label != label {
			t.Errorf("第 %d 个部位是 %q，期望 %q（顺序必须是装备位顺序）", i, f.Slots[i].Key, label)
		}
		if f.Slots[i].Cell == "" || f.Slots[i].WearSlot < 12 || f.Slots[i].WearSlot > 25 {
			t.Errorf("部位 %s 缺少 cell 或装备位号：%+v", label, f.Slots[i])
		}
	}
	if f.Slots[len(equipmentSlotOrder)].Key != slotAvatar || f.Slots[len(equipmentSlotOrder)+1].Key != slotOther {
		t.Errorf("最后两桶必须是 时装 / 其它装备：%+v", f.Slots[len(equipmentSlotOrder):])
	}
	counts := map[string]int{}
	for _, s := range f.Slots {
		counts[s.Key] = s.Count
	}
	if counts["武器"] != 2 || counts["上衣"] != 1 || counts["头肩"] != 1 || counts["项链"] != 1 {
		t.Errorf("部位数量不对：%+v", counts)
	}

	// 等级档：固定 7 档，数量按等级归属，合计等于有等级的装备数
	if len(f.Levels) != len(levelBands) {
		t.Fatalf("等级档 %d 个，期望 %d 个", len(f.Levels), len(levelBands))
	}
	sum := 0
	for _, l := range f.Levels {
		sum += l.Count
	}
	if sum != f.LevelKnown {
		t.Errorf("等级档合计 %d，期望等于有等级的装备数 %d", sum, f.LevelKnown)
	}
	byKey := map[string]int{}
	for _, l := range f.Levels {
		byKey[l.Key] = l.Count
	}
	if byKey["~20"] != 1 || byKey["21-50"] != 1 || byKey["111-115"] != 2 || byKey["116+"] != 0 {
		t.Errorf("等级档数量不对：%+v", byKey)
	}

	// 品级：0..8 全部给出，中文名与客户端一致
	if len(f.Rarities) != len(defaultRarityLabels) {
		t.Fatalf("品级选项 %d 个，期望 %d 个", len(f.Rarities), len(defaultRarityLabels))
	}
	for i, r := range f.Rarities {
		if r.Key != string(rune('0'+i)) {
			t.Errorf("第 %d 个品级的 key 是 %q", i, r.Key)
		}
		if r.Label != defaultRarityLabels[i] {
			t.Errorf("品级 %d 中文名 %q，期望 %q", i, r.Label, defaultRarityLabels[i])
		}
	}
	if f.Rarities[8].Label != "太初" || f.Rarities[8].Count != 1 {
		t.Errorf("品级 8 期望 太初 x1，得到 %+v", f.Rarities[8])
	}
	if f.RaritySource == "" {
		t.Error("必须说明品级中文名取自哪里")
	}
}

// TestTypesCounts 验证 /api/types 的数量统计、固定顺序与装备子分类（部位）。
func TestTypesCounts(t *testing.T) {
	ix := fixtureIndex()
	types := ix.Types()
	if len(types) != len(orderedTypes) {
		t.Fatalf("分类数 %d，期望 %d", len(types), len(orderedTypes))
	}
	want := map[string]int{
		typeConsumable: 2, typeMaterial: 1, typeEquipment: 5,
		typeQuest: 0, typeProfession: 0, typeEmblem: 0, typeOther: 0,
	}
	total := 0
	for i, ty := range types {
		if ty.Key != orderedTypes[i].Key {
			t.Errorf("第 %d 个分类是 %q，期望 %q（顺序必须稳定）", i, ty.Key, orderedTypes[i].Key)
		}
		if ty.Count != want[ty.Key] {
			t.Errorf("分类 %s 数量 %d，期望 %d", ty.Key, ty.Count, want[ty.Key])
		}
		if ty.Label == "" {
			t.Errorf("分类 %s 缺少中文标签", ty.Key)
		}
		total += ty.Count
		if ty.Key != typeEquipment && len(ty.Children) != 0 {
			t.Errorf("只有装备类该有 children，%s 却有 %d 个", ty.Key, len(ty.Children))
		}
	}
	if total != len(ix.items) {
		t.Errorf("分类数量合计 %d，期望等于物品总数 %d", total, len(ix.items))
	}

	var equip ItemType
	for _, ty := range types {
		if ty.Key == typeEquipment {
			equip = ty
		}
	}
	// 装备 children 是部位，按装备位顺序，且合计等于装备总数
	wantSub := []string{"武器", "上衣", "头肩", "项链"}
	if len(equip.Children) != len(wantSub) {
		t.Fatalf("装备 children 期望 %d 个，得到 %d 个：%+v", len(wantSub), len(equip.Children), equip.Children)
	}
	sub := 0
	for i, ch := range equip.Children {
		if ch.Key != wantSub[i] || ch.Label != wantSub[i] {
			t.Errorf("第 %d 个部位是 %q，期望 %q", i, ch.Key, wantSub[i])
		}
		sub += ch.Count
	}
	if sub != equip.Count {
		t.Errorf("装备部位合计 %d，期望等于装备总数 %d", sub, equip.Count)
	}
}

// TestLoadNamesKeepsOnlyUsedPrefixes 验证 66MB 名字表加载后只保留用得到的键，
// 避免全量常驻；三类前缀都要留下。
func TestLoadNamesKeepsOnlyUsedPrefixes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "names.client.json")
	raw := `{
	  "name_10300000": "普拉塔尼的黄金石碎片",
	  "growtype_name_0": "格斗家",
	  "common_rarity_8": "太初",
	  "skill_explain_123": "不该保留",
	  "quest_title_456": "不该保留",
	  "name_101001153": "太初之星 - 短剑"
	}`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	kept := loadNames(path)
	if len(kept) != 4 {
		t.Fatalf("裁剪后剩 %d 条，期望 4 条：%v", len(kept), kept)
	}
	if kept["name_10300000"] != "普拉塔尼的黄金石碎片" {
		t.Errorf("中文名没有保留：%q", kept["name_10300000"])
	}
	if kept["name_101001153"] != "太初之星 - 短剑" {
		t.Errorf("装备中文名没有保留：%q", kept["name_101001153"])
	}
	if kept["growtype_name_0"] != "格斗家" {
		t.Errorf("职业名没有保留（classOf 依赖它）：%q", kept["growtype_name_0"])
	}
	if kept["common_rarity_8"] != "太初" {
		t.Errorf("品级名没有保留（稀有度筛选依赖它）：%q", kept["common_rarity_8"])
	}
	if _, bad := kept["skill_explain_123"]; bad {
		t.Error("无关键应当被裁掉")
	}
	// 文件不存在时返回空表而不是 panic
	if got := loadNames(filepath.Join(dir, "missing.json")); len(got) != 0 {
		t.Errorf("缺失文件应返回空表，得到 %d 条", len(got))
	}
}

// TestNameKeyForMatchesL10nConvention 验证 name_<id> 与部位表 id 键的拼法。
func TestNameKeyForMatchesL10nConvention(t *testing.T) {
	ix := &ItemIndex{}
	cases := map[uint32]string{
		10300000:  "name_10300000",
		101001153: "name_101001153",
		0:         "name_0",
	}
	for id, want := range cases {
		if got := ix.nameKeyFor(id); got != want {
			t.Errorf("nameKeyFor(%d) = %q, 期望 %q", id, got, want)
		}
		if want2 := want[len(namePrefix):]; idKey(id) != want2 {
			t.Errorf("idKey(%d) = %q, 期望 %q（部位表的键）", id, idKey(id), want2)
		}
	}
}

// TestSlotMapFileRoundTrip 验证 equipment.slots.json 的读写格式，
// 并顺带覆盖"文件缺失/损坏时返回空表而不是崩"。
func TestSlotMapFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "equipment.slots.json")
	doc := slotMapFile{
		Source:  "test",
		BuiltAt: "2026-09-18T00:00:00Z",
		Slots: map[string]slotMapEntry{
			"101001153": {Cell: "[weapon]", Level: 115},
			"10018":     {Cell: "[coat]", Level: 20},
		},
	}
	b, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	got := loadSlotMap(path)
	if len(got) != 2 || got["101001153"].Cell != "[weapon]" || got["101001153"].Level != 115 {
		t.Fatalf("部位表读取结果不对：%+v", got)
	}
	if n := len(loadSlotMap(filepath.Join(dir, "missing.json"))); n != 0 {
		t.Errorf("缺失文件应返回空表，得到 %d 条", n)
	}
	bad := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(bad, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if n := len(loadSlotMap(bad)); n != 0 {
		t.Errorf("损坏文件应返回空表，得到 %d 条", n)
	}
}
