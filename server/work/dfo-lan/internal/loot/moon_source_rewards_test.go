package loot

import (
	"math"
	"strings"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
)

// 源驱动翻牌（moon_source_rewards.go）的单测。
//
// 这里**不碰真实 PVF**：策略是纯数据（catalog.DungeonRewardBlock），掷骰用的组与物品
// 都可以在 fixture 里给出来。端到端（直读内层 PVF 的那一路）在
// cmd/wireprobe/moon_rewards_native_test.go 那一族里跑。

func moonSourceFixture() *Service {
	h := strings.Repeat("a", 64)
	return &Service{
		Catalog: catalog.LootCatalog{
			Source: pvf.ArchiveSnapshot{Checksum: h},
			Items: map[uint32]catalog.LootItem{
				101: {ID: 101, Kind: "stackable", StackableType: "[material]", StackLimit: 10},
				102: {ID: 102, Kind: "stackable", StackableType: "[material]", StackLimit: 10},
				103: {ID: 103, Kind: "stackable", StackableType: "[etc]", StackLimit: 10},
				104: {ID: 104, Kind: "stackable", StackableType: "[upgradable legacy]", StackLimit: 10},
			},
			DropGroups: []catalog.DropGroup{
				{ID: 7, Explicit: []catalog.DropWeight{{Template: 101, Weight: 1}}},
				{ID: 8, Explicit: []catalog.DropWeight{{Template: 102, Weight: 1}}},
				{ID: 9, Explicit: []catalog.DropWeight{{Template: 103, Weight: 1}}},
				{ID: 10, Explicit: []catalog.DropWeight{{Template: 104, Weight: 1}}},
			},
		},
		// `[etc]` 有带是 2026-10-09 起的口径（configs/inventory.current37.json 补了这一条，
		// 依据是 cashshop pilot 的「[etc] 消耗品兜底、落 Use 页」）；`[upgradable legacy]` 仍然没有。
		BagRules: inventory.BagRules{Source: h, Slots: map[string][2]uint16{"[material]": {121, 121}, "[etc]": {65, 120}}, MissingStackLimit: 10},
	}
}

func moonSourceRun() *dungeon.Session {
	run := &dungeon.Session{RunID: strings.Repeat("ab", 16), Definition: catalog.DungeonDefinition{ID: 100004137}}
	run.MarkCompleted()
	return run
}

func moonSourceRole(s *Service) Role {
	return Role{AccountID: 7, ID: 9, ConfigVersion: s.Catalog.Source.SaveIdentity()}
}

// 族别按条目名判定：Oath/Crystal → 誓约族；Equipment/Weapon → 装备族；其余不参与。
func TestMoonSourceFamiliesByName(t *testing.T) {
	cases := []struct {
		name              string
		oath, equipment   bool
	}{
		{"SetEquipmentReward", false, true},
		{"WeaponEquipmentReward", false, true},
		{"SetOathPrimerReward", true, false},
		// 已知例外：名字是 Equipment，组里装的却是 "Dim Oath Crystal" ⇒ 按名字落进装备族。
		// 见实现里的说明：今天只影响跳过日志，不影响实发；誓约族将来能发放时要改判据。
		{"RareEquipmentReward", false, true},
		{"MonsterCard_ExpectationReward_0", false, false},
		{"Mine_ExpectationReward_3", false, false},
	}
	for _, c := range cases {
		if got := moonFamilyOath(c.name); got != c.oath {
			t.Fatalf("moonFamilyOath(%q) = %v", c.name, got)
		}
		if got := moonFamilyEquipment(c.name); got != c.equipment {
			t.Fatalf("moonFamilyEquipment(%q) = %v", c.name, got)
		}
	}
}

// 倍数按类别取；源里没声明该类别时 ok=false（不是 0 次）。
func TestMoonSourceMultipleLookup(t *testing.T) {
	block := catalog.DungeonRewardBlock{Multiple: []catalog.DungeonRewardMultiple{
		{Kind: 2, Normal: 7, Matching: 6}, {Kind: 3, Normal: 3, Matching: 2},
	}}
	if v, ok := moonMultiple(block, 2); !ok || v != 7 {
		t.Fatalf("类别2 = %d,%v", v, ok)
	}
	if v, ok := moonMultiple(block, 3); !ok || v != 3 {
		t.Fatalf("类别3 = %d,%v", v, ok)
	}
	if _, ok := moonMultiple(block, 4); ok {
		t.Fatal("源里没声明的类别必须 ok=false")
	}
	// 100004306 那种只有类别 2 的块：类别 3 必须不存在。
	only := catalog.DungeonRewardBlock{Multiple: []catalog.DungeonRewardMultiple{{Kind: 2, Normal: 13}}}
	if _, ok := moonMultiple(only, 3); ok {
		t.Fatal("只有类别2的块不应给出类别3")
	}
}

// 固定产物：实测数量优先，其余各 1 件；去重 + 排序。
func TestMoonSourceFixedChoices(t *testing.T) {
	got := MoonSourceFixedChoices([]uint32{10362432, 10362429, 10404337, 10362429, 0},
		map[uint32]uint32{10362429: 60, 10362432: 15})
	want := []MoonRewardChoice{
		{Template: 10362429, Count: 60},
		{Template: 10362432, Count: 15},
		{Template: 10404337, Count: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("固定产物 = %+v，期望 %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("固定产物[%d] = %+v，期望 %+v", i, got[i], want[i])
		}
	}
}

// 装备池只取交集（DropPool 里已有的一定发得出去，交集只会更窄）。
func TestMoonSourceEquipmentPoolIsIntersection(t *testing.T) {
	pool := []inventory.EquipmentDrop{{ID: 501}, {ID: 502}, {ID: 503}}
	got := MoonSourceEquipmentPool(pool, []uint32{502, 999})
	if len(got) != 1 || got[0].ID != 502 {
		t.Fatalf("交集 = %+v", got)
	}
	if MoonSourceEquipmentPool(pool, nil) != nil {
		t.Fatal("没有候选时必须是 nil")
	}
}

// 端到端（fixture）：率按千分比、每个类别掷各自倍数、发不出去的要进 Skipped。
func TestMoonSourcePlanRatesMultiplesAndSkips(t *testing.T) {
	s := moonSourceFixture()
	block := catalog.DungeonRewardBlock{
		Contents: "normal",
		Groups:   []int32{2, 7, 1, 8},
		Multiple: []catalog.DungeonRewardMultiple{{Kind: 2, Normal: 2, Matching: 0}, {Kind: 3, Normal: 1, Matching: 0}},
		Special: []catalog.DungeonSpecialReward{
			{Name: "SetEquipmentReward", Rate: 1000, List: 10326880, Count: 1, Group: 7},
			{Name: "SetOathPrimerReward", Rate: 1000, List: 10326880, Count: 1, Group: 9}, // [etc] ⇒ 发不出去
			{Name: "MonsterCard_ExpectationReward_0", Rate: 0, List: 10326884, Count: 1, Group: 8},
		},
	}
	p := MoonSourcePolicy{
		Source:            s.Catalog.Source.SaveIdentity(),
		SettlementDungeon: 100004137,
		Block:             block,
		Fixed:         []MoonRewardChoice{{Template: 102, Count: 3}},
		// 装备掷骰的两个池故意留空：它必须进 Skipped 而不是静默消失。
		// 套装盒 / 誓约盒各给一个「只有一件」的展开池：断言它们改发装备、不再发占位件。
		SetEquipmentPool:  []MoonOathPoolMember{{Template: 102, Weight: 1000, Rarity: 3}},
		OathPool:          []MoonOathPoolMember{{Template: 101, Weight: 1000, Rarity: 3}},
		Level:             115,
		Rank:              3,
	}
	out, e := s.PlanMoonRewardsFromSource(moonSourceRole(s), moonSourceRun(), p)
	if e != nil {
		t.Fatal(e)
	}
	// 四件：固定 1 + 套装盒展开 1 + 誓约盒展开 1 + 誓约掷骰 1。
	// 卡片条目率 0 必不中；装备掷骰因两个池为空全部进 Skipped；
	// 倍数作用在**掷骰次数**上（tooltip 的「效率」口径），不是另开掷骰。
	if out.Grants != 4 {
		t.Fatalf("实发 = %d，期望 4（固定 + 套装盒展开 + 誓约盒展开 + 誓约掷骰）", out.Grants)
	}
	if out.Plan.Grants[0].Template != 102 || out.Plan.Grants[0].Count != 3 {
		t.Fatalf("固定产物未按实测数量发放：%+v", out.Plan.Grants[0])
	}
	if out.Plan.Grants[1].Template != 102 || out.Plan.Grants[1].Count != 1 {
		t.Fatalf("套装盒应改发套装装备池里的 102：%+v", out.Plan.Grants[1])
	}
	if out.Plan.Grants[2].Template != 101 || out.Plan.Grants[2].Count != 1 {
		t.Fatalf("誓约盒应改发誓约装备池里的 101：%+v", out.Plan.Grants[2])
	}
	if out.Plan.Grants[3].Template != 101 {
		t.Fatalf("誓约掷骰应发 101：%+v", out.Plan.Grants[3])
	}
	// 掷骰 = 装备族 Normal(2) + 誓约族 Normal(1)；**每件一格**（帧的每座上限 126 件，
	// 不是 8 格 —— 见 conquest_reward115.go 的 [8][] 是座位数）。
	if out.Rolls != 3 {
		t.Fatalf("掷骰次数 = %d，期望 3（装备2 + 誓约1）", out.Rolls)
	}
	// 跳过项必须写清楚原因（`[etc]` 有了带之后，这里只剩"装备池为空"那条）。
	if len(out.Skipped) < 1 {
		t.Fatalf("跳过项 = %+v，期望至少 1 条", out.Skipped)
	}
	var sawPool bool
	for _, sk := range out.Skipped {
		if sk.Reason == "源装备组的成员里没有可入包装备" {
			sawPool = true
		}
	}
	if !sawPool {
		t.Fatalf("跳过原因不全：%+v", out.Skipped)
	}
	if !strings.Contains(out.Summary, "块=normal") || !strings.Contains(out.Summary, "倍数[装备=2 誓约=1]") {
		t.Fatalf("摘要 = %q", out.Summary)
	}
}

// 门禁：别人的 run / 没通关 / 身份不符都要被拒（与既有 PlanMoonReward 同源判据）。
func TestMoonSourcePlanGates(t *testing.T) {
	s := moonSourceFixture()
	role := moonSourceRole(s)
	p := MoonSourcePolicy{Source: role.ConfigVersion, SettlementDungeon: 100004137, Block: catalog.DungeonRewardBlock{
		Special: []catalog.DungeonSpecialReward{{Name: "SetEquipmentReward", Rate: 1000, Count: 1, Group: 7}},
	}}

	// 未通关
	open := &dungeon.Session{RunID: strings.Repeat("ab", 16), Definition: catalog.DungeonDefinition{ID: 100004137}}
	if _, e := s.PlanMoonRewardsFromSource(role, open, p); e == nil {
		t.Fatal("未通关不应产出奖单")
	}
	// 别的副本
	other := moonSourceRun()
	other.Definition.ID = 100004138
	if _, e := s.PlanMoonRewardsFromSource(role, other, p); e == nil {
		t.Fatal("非 100004137 不应产出奖单")
	}
	// 身份不符
	bad := role
	bad.ConfigVersion = strings.Repeat("b", 64)
	if _, e := s.PlanMoonRewardsFromSource(bad, moonSourceRun(), p); e == nil {
		t.Fatal("身份不符不应产出奖单")
	}
	// 策略身份与目录不符
	p2 := p
	p2.Source = strings.Repeat("c", 64)
	if _, e := s.PlanMoonRewardsFromSource(role, moonSourceRun(), p2); e == nil {
		t.Fatal("策略身份与目录不符不应产出奖单")
	}
}

// 誓约族的判据只看**声明名**：含 oath / crystal。
//
// 依据（2026-10-09 PVF 取证）：`SetOathPrimerReward` 的组 21468 装的是 12 个
// 「X Oath/Crystal Set」盒（[etc]、没有产出段、玩家打不开）⇒ 这条声明必须改发
// 誓约/结晶**装备**。`RareEquipmentReward`（组 21470「Dim Oath Crystal」）的边界
// 见实现里的说明：它是普通（rare）档结晶，今天仍按原样发。
func TestMoonOathDeclarationByName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"SetOathPrimerReward", true},
		{"SetEquipmentReward", false},
		{"WeaponEquipmentReward", false},
		{"RareEquipmentReward", false},
		{"MonsterCard_ExpectationReward_0", false},
	}
	s := moonSourceFixture()
	for _, c := range cases {
		if got := s.MoonOathDeclaration(c.name); got != c.want {
			t.Fatalf("MoonOathDeclaration(%q) = %v，期望 %v", c.name, got, c.want)
		}
	}
}

// 没有装备目录时誓约池必须**空 + 说明原因**，不能静默给一个会发低档货的池。
func TestMoonOathEquipmentPoolNeedsCatalog(t *testing.T) {
	s := moonSourceFixture()
	if pool, note := s.MoonOathEquipmentPool(); pool != nil || note == "" {
		t.Fatalf("Equipment=nil 时应为空池并给出原因：%v %q", pool, note)
	}
}

// 誓约掷骰按源权重抽：权重为 0 的成员永远抽不到（等权退化的反面）。
func TestMoonSourceOathRollUsesWeights(t *testing.T) {
	s := moonSourceFixture()
	sink := &moonSourceSink{svc: s, plan: &MoonRewardPlan{}}
	s.rollMoonSourceOath(sink, "seed", 5, MoonSourcePolicy{OathPool: []MoonOathPoolMember{
		{Template: 101, Weight: 1000, Family: "oath", Rarity: 3},
		{Template: 102, Weight: 0, Family: "primer", Rarity: 6},
	}})
	if len(sink.plan.Grants) != 5 {
		t.Fatalf("掷骰应发 5 件：%+v", sink.plan.Grants)
	}
	for _, g := range sink.plan.Grants {
		if g.Template != 101 {
			t.Fatalf("权重 0 的成员不该被抽中：%+v", g)
		}
	}
}

// 一组声明都发不出去时，计划必须报错而不是给一张空奖单。
func TestMoonSourcePlanRefusesEmptyResult(t *testing.T) {
	s := moonSourceFixture()
	p := MoonSourcePolicy{
		Source:            s.Catalog.Source.SaveIdentity(),
		SettlementDungeon: 100004137,
		Block: catalog.DungeonRewardBlock{Special: []catalog.DungeonSpecialReward{
			{Name: "SetOathPrimerReward", Rate: 1000, Count: 1, Group: 10}, // [upgradable legacy]：仍无槽位带
		}},
	}
	if _, e := s.PlanMoonRewardsFromSource(moonSourceRole(s), moonSourceRun(), p); e == nil {
		t.Fatal("全都发不出去时应报错")
	}
}
// 分组判据**只看堆叠物目录**：只有装备模板的组必须是"产出池"，绝不能落到固定产物里。
//
// 这条钉住一个实测过的坑：早先用"组 ∩ 可入包装备 非空"判定，Equipment 为 nil 或目录不全时
// 交集恒空 ⇒ 100004137 的 [21251, 3, 21291] 里那 803 个装备模板（组 3 有 792 件）
// 全被当成"固定产物"，一次翻牌会发 800 多件装备。
func TestMoonSourceSplitGroupsDoesNotDependOnEquipmentCatalog(t *testing.T) {
	s := moonSourceFixture()
	// 组 7 是堆叠物 101；21291 是堆叠物 102（固定产物组）；新增组 11 只有装备模板
	// （fixture 的 Items 里查不到 ⇒ 视为装备）。
	s.Catalog.DropGroups = append(s.Catalog.DropGroups,
		catalog.DropGroup{ID: 21291, Explicit: []catalog.DropWeight{{Template: 102, Weight: 1}}},
		catalog.DropGroup{ID: 11, Explicit: []catalog.DropWeight{{Template: 100051304, Weight: 10}, {Template: 100101187, Weight: 10}}})
	s.Equipment = nil // 目录没装也不能影响判据
	pool, fixed := s.SplitMoonSourceGroups([]uint32{11, 7, 21291})
	if len(pool) != 1 || pool[0] != 11 {
		t.Fatalf("产出池 = %v，期望 [11]（只有装备模板的组）", pool)
	}
	if len(fixed) != 2 || fixed[0] != 7 || fixed[1] != 21291 {
		t.Fatalf("固定组 = %v，期望 [7 21291]（含堆叠物）", fixed)
	}
	// 固定产物的推导也不该把装备模板带进来。
	got := MoonSourceFixedChoices(s.MoonSourceGroupMembers(fixed), nil)
	if len(got) != 2 || got[0].Template != 101 || got[1].Template != 102 {
		t.Fatalf("固定产物 = %+v，期望只有两个堆叠物", got)
	}
}

// 品级权重必须**在池里真有模板的那几档上重归一化**（业主 2026-10-09 口径：
// 把无模板那几档占的比例按权重摊给有模板的几档）。
//
// 源表实测 `700000 964900 990000 1000000 1000001…` ⇒ rarity0 70% / r1 26.49% /
// r2 2.51% / r3 1% / r4..8 各 1e-6；本副本池里只有 2/3/4/6 ⇒ 照原表掷有 96.5% 落空。
// 重归一化后 r2 = 25100/35102 ≈ 71.5%、r3 = 10000/35102 ≈ 28.5%。
func TestMoonRarityWeightsRenormalizesOverPool(t *testing.T) {
	table := []float64{700000, 964900, 990000, 1000000, 1000001, 1000002, 1000003, 1000004, 1000005}
	pool := map[int32][]uint32{2: {1}, 3: {2}, 4: {3}, 6: {4}}
	tiers, weights := MoonRarityWeights(table, pool)
	want := []int32{2, 3, 4, 6}
	if len(tiers) != len(want) {
		t.Fatalf("档位 = %v，期望 %v", tiers, want)
	}
	for i := range want {
		if tiers[i] != want[i] {
			t.Fatalf("档位[%d] = %v，期望 %v", i, tiers, want)
		}
	}
	if math.Abs(weights[0]-0.71506) > 1e-4 || math.Abs(weights[1]-0.28488) > 1e-4 {
		t.Fatalf("权重 = %v，期望 [≈0.7151 ≈0.2849 …]", weights)
	}
	sum := 0.0
	for _, w := range weights {
		sum += w
	}
	if math.Abs(sum-1) > 1e-9 {
		t.Fatalf("权重和 = %v，期望 1", sum)
	}
	// 池里只有一档 ⇒ 归一化成 1（这样"退到最近品级"那段就永远不需要了）。
	if got, w := MoonRarityWeights(table, map[int32][]uint32{3: {7}}); len(got) != 1 || math.Abs(w[0]-1) > 1e-9 {
		t.Fatalf("单档池 = %v %v", got, w)
	}
	// 没有可用档 / 表太短 ⇒ nil（调用方记跳过，不编造分布）。
	if got, w := MoonRarityWeights(table, nil); got != nil || w != nil {
		t.Fatalf("空池应返回 nil：%v %v", got, w)
	}
	if got, w := MoonRarityWeights([]float64{1, 2}, pool); got != nil || w != nil {
		t.Fatalf("短表应返回 nil：%v %v", got, w)
	}
}
