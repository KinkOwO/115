package loot

import (
	"reflect"
	"testing"

	"dfolan/internal/catalog"
	"dfolan/internal/modpolicy"
)

// dropRateWorldCatalog / dropRateMobCatalog 是本文件所有用例共用的最小目录，
// 形状与改动前黄金值的采集现场**逐字段一致**（权重 850/830/1000 与 500/500）。
func dropRateWorldCatalog() catalog.LootCatalog {
	return catalog.LootCatalog{
		OrdinaryWorldDropPercent: 100,
		WorldDrop: &catalog.WorldDropTable{Levels: map[uint32]catalog.WorldDropLevel{
			15: {Items: []catalog.MonsterItemPair{{Template: 1, Value: 850}, {Template: 2, Value: 830}, {Template: 7, Value: 1000}}},
		}},
		Items: map[uint32]catalog.LootItem{
			1: {ID: 1, Kind: "stackable"},
			2: {ID: 2, Kind: "stackable"},
			7: {ID: 7, Kind: "stackable"},
		},
	}
}

func dropRateMobCatalog() catalog.LootCatalog {
	return catalog.LootCatalog{
		OrdinaryMonsterItemRate: 1000, // 环境变量 10 → 目录里 10*100 = 1000（= 10%）
		Items: map[uint32]catalog.LootItem{
			1: {ID: 1, Kind: "stackable"},
			2: {ID: 2, Kind: "stackable"},
		},
	}
}

func dropRateMobTable() catalog.MonsterItemTable {
	return catalog.MonsterItemTable{Declared: true, Items: []catalog.MonsterItemPair{{Template: 1, Value: 500}, {Template: 2, Value: 500}}}
}

// TestDropRateUnsetIsBitIdenticalToPreChange 是"没有 mod 设置倍率时行为与改动前一致"的依据：
// 下面每条 Awards/NextSeed 都是**改动前**的同一份代码在同一份目录与 seed 上跑出来的输出
// （2026-10-07 先跑原函数采集，再改 world_drop.go / monster_items.go）。改完必须一模一样
// —— 既是最坏情况下的回退保障，也证明零值路径连 RNG 抽签顺序都没变。
func TestDropRateUnsetIsBitIdenticalToPreChange(t *testing.T) {
	defer modpolicy.Reset()
	modpolicy.Reset() // 零值 = 没有 mod 设置倍率

	world := dropRateWorldCatalog()
	worldHits := []struct {
		seed        uint32
		template    uint32
		next, level uint32
	}{
		{34, 7, 3848457116, 15},
		{37, 2, 1626854279, 15},
		{157, 7, 2957054015, 15},
		{515, 1, 4133754485, 15},
		{666, 7, 3982228052, 15},
		{1653, 2, 3219334999, 15},
	}
	for _, want := range worldHits {
		out, err := rollWorldItems(world, want.seed, byte(want.level))
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Awards) != 1 || out.Awards[0].Template != want.template || out.Awards[0].Amount != 1 || out.NextSeed != want.next {
			t.Fatalf("世界掉落零值路径变了：seed%d got%+v next%d want template%d next%d", want.seed, out.Awards, out.NextSeed, want.template, want.next)
		}
	}
	// 没中的 seed 也要逐位相同（抽签照样推进种子）。
	if out, err := rollWorldItems(world, 1, 15); err != nil || len(out.Awards) != 0 || out.NextSeed != 662824084 {
		t.Fatalf("世界掉落未中时也变了：%+v next%d err%v", out.Awards, out.NextSeed, err)
	}
	// 10000（既有上限口径）四条。
	world.OrdinaryWorldDropPercent = 10000
	for _, want := range []struct {
		seed, template, next uint32
	}{{1, 2, 2516284547}, {2, 1, 1775750268}, {3, 2, 1035215989}, {4, 7, 294681710}} {
		out, err := rollWorldItems(world, want.seed, 15)
		if err != nil || len(out.Awards) != 1 || out.Awards[0].Template != want.template || out.NextSeed != want.next {
			t.Fatalf("世界掉落百分百路径变了：seed%d %+v next%d err%v", want.seed, out.Awards, out.NextSeed, err)
		}
	}

	mob := dropRateMobCatalog()
	table := dropRateMobTable()
	for _, want := range []struct {
		seed, template, next uint32
	}{{39, 2, 145785721}, {77, 1, 2070254191}, {164, 2, 2068281358}, {1147, 2, 4267525421}} {
		out, err := rollMonsterItems(mob, table, want.seed)
		if err != nil || len(out.Awards) != 1 || out.Awards[0].Template != want.template || out.NextSeed != want.next {
			t.Fatalf("小怪池零值路径变了：seed%d %+v next%d err%v", want.seed, out.Awards, out.NextSeed, err)
		}
	}
	if out, err := rollMonsterItems(mob, table, 1); err != nil || len(out.Awards) != 0 || out.NextSeed != 662824084 {
		t.Fatalf("小怪池未中时也变了：%+v next%d err%v", out.Awards, out.NextSeed, err)
	}
	mob.OrdinaryMonsterItemRate = 10000
	for _, want := range []struct {
		seed, template, next uint32
	}{{1, 2, 2516284547}, {2, 2, 1775750268}, {3, 2, 1035215989}, {4, 1, 294681710}} {
		out, err := rollMonsterItems(mob, table, want.seed)
		if err != nil || len(out.Awards) != 1 || out.Awards[0].Template != want.template || out.NextSeed != want.next {
			t.Fatalf("小怪池百分百路径变了：seed%d %+v next%d err%v", want.seed, out.Awards, out.NextSeed, err)
		}
	}
	// 1.00 倍（显式写 100）也必须与"未设置"逐位相同。
	modpolicy.ConfigureDrops(modpolicy.DropRules{WorldPercent: 100, MonsterItemPercent: 100, Source: "test"})
	world.OrdinaryWorldDropPercent = 100
	for _, want := range worldHits {
		out, err := rollWorldItems(world, want.seed, byte(want.level))
		if err != nil || len(out.Awards) != 1 || out.Awards[0].Template != want.template || out.NextSeed != want.next {
			t.Fatalf("1.00 倍与未设置不一致：seed%d %+v next%d err%v", want.seed, out.Awards, out.NextSeed, err)
		}
	}
	mob.OrdinaryMonsterItemRate = 1000
	if out, err := rollMonsterItems(mob, table, 39); err != nil || len(out.Awards) != 1 || out.Awards[0].Template != 2 || out.NextSeed != 145785721 {
		t.Fatalf("1.00 倍与未设置不一致：%+v next%d err%v", out.Awards, out.NextSeed, err)
	}
}

// TestScaleDropPercentIntegerMathAndClamp 覆盖倍率的取值算术：1.00 / 5.00 / 未设置 /
// 超上限截断 / 整数截断 / 不溢出。100 = 1.00 倍。
func TestScaleDropPercentIntegerMathAndClamp(t *testing.T) {
	for _, tc := range []struct {
		base, multiplier, limit, want uint32
		note                          string
	}{
		{100, 0, worldDropMaxPercent, 100, "未设置：原样返回（连截断都不做）"},
		{0, 0, worldDropMaxPercent, 0, "未设置 + 关着的池子"},
		{100, 100, worldDropMaxPercent, 100, "1.00 倍"},
		{100, 500, worldDropMaxPercent, 500, "世界掉落 5 倍"},
		{1000, 500, MonsterItemDropDenominator, 5000, "小怪池 10% → 50%"},
		{10000, 500, worldDropMaxPercent, 10000, "超上限：截断而不是溢出"},
		{10000, 500, MonsterItemDropDenominator, 10000, "小怪池另一侧上限"},
		{2000, 500, worldDropMaxPercent, 10000, "正好顶到上限"},
		{1, 150, worldDropMaxPercent, 1, "整数截断（1.5 → 1）"},
		{3, 150, worldDropMaxPercent, 4, "整数截断（4.5 → 4）"},
		{0, 500, worldDropMaxPercent, 0, "倍率不能把关掉的池子打开"},
		{4294967295, 0, worldDropMaxPercent, 4294967295, "未设置：超大 base 也原样"},
		{4294967295, 4000000000, worldDropMaxPercent, 10000, "乘数极大也不溢出（uint64 中间量）"},
	} {
		if got := scaleDropPercent(tc.base, tc.multiplier, tc.limit); got != tc.want {
			t.Errorf("%s：scaleDropPercent(%d,%d,%d)=%d，want %d", tc.note, tc.base, tc.multiplier, tc.limit, got, tc.want)
		}
	}

	// 走真实消费函数：目录值 100 × mod 500 = 500（世界），1000 × 500 = 5000（小怪池）。
	defer modpolicy.Reset()
	modpolicy.Reset()
	world := dropRateWorldCatalog()
	mob := dropRateMobCatalog()
	if got := effectiveWorldDropPercent(world); got != 100 {
		t.Fatalf("未设置时世界掉落应保持目录值：%d", got)
	}
	if got := effectiveMonsterItemRate(mob); got != 1000 {
		t.Fatalf("未设置时小怪池应保持目录值：%d", got)
	}
	modpolicy.ConfigureDrops(modpolicy.DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "test"})
	if got := effectiveWorldDropPercent(world); got != 500 {
		t.Fatalf("5 倍世界掉落：%d", got)
	}
	if got := effectiveMonsterItemRate(mob); got != 5000 {
		t.Fatalf("5 倍小怪池：%d", got)
	}
	// 上限：目录值已经顶格时，倍率只把它压回上限，不外溢。
	world.OrdinaryWorldDropPercent = 10000
	mob.OrdinaryMonsterItemRate = 10000
	if got := effectiveWorldDropPercent(world); got != worldDropMaxPercent {
		t.Fatalf("世界掉落上限截断：%d", got)
	}
	if got := effectiveMonsterItemRate(mob); got != MonsterItemDropDenominator {
		t.Fatalf("小怪池上限截断：%d", got)
	}
}

// worldDropOutcomes / mobDropOutcomes 收集"哪些 seed 掉了什么"，用于比较概率与顺序关系。
func worldDropOutcomes(t *testing.T, c catalog.LootCatalog, seeds int) map[uint32]uint32 {
	t.Helper()
	got := map[uint32]uint32{}
	for i := 1; i <= seeds; i++ {
		out, err := rollWorldItems(c, uint32(i), 15)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Awards) > 1 {
			t.Fatalf("seed%d 一次抽多件：%+v", i, out.Awards)
		}
		if len(out.Awards) == 1 {
			got[uint32(i)] = out.Awards[0].Template
		}
	}
	return got
}

func mobDropOutcomes(t *testing.T, c catalog.LootCatalog, table catalog.MonsterItemTable, seeds int) map[uint32]uint32 {
	t.Helper()
	got := map[uint32]uint32{}
	for i := 1; i <= seeds; i++ {
		out, err := rollMonsterItems(c, table, uint32(i))
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Awards) > 1 {
			t.Fatalf("seed%d 一次抽多件：%+v", i, out.Awards)
		}
		if len(out.Awards) == 1 {
			got[uint32(i)] = out.Awards[0].Template
		}
	}
	return got
}

// TestDropRateMultiplierAppliesAtRuntime 用真实抽取函数钉住 mod 倍率的效果与**运行期可变**：
// 同一份目录对象，改 modpolicy 前后概率就变（不需要重建目录），撤销后逐位回到基线。
func TestDropRateMultiplierAppliesAtRuntime(t *testing.T) {
	defer modpolicy.Reset()
	modpolicy.Reset()
	const seeds = 2000

	world := dropRateWorldCatalog()
	mob := dropRateMobCatalog()
	table := dropRateMobTable()

	worldBase := worldDropOutcomes(t, world, seeds)
	mobBase := mobDropOutcomes(t, mob, table, seeds)
	t.Logf("未设置：世界掉落 %d/%d，小怪池 %d/%d", len(worldBase), seeds, len(mobBase), seeds)
	if len(worldBase) < 30 || len(worldBase) > 90 {
		t.Fatalf("世界掉落基线概率异常：%d/%d", len(worldBase), seeds)
	}
	if len(mobBase) < 150 || len(mobBase) > 300 {
		t.Fatalf("小怪池基线概率异常：%d/%d", len(mobBase), seeds)
	}

	// mod 在 boot 里设：×5（目录对象一个字节都没动）。
	modpolicy.ConfigureDrops(modpolicy.DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "odyssey.hardcore"})
	worldFive := worldDropOutcomes(t, world, seeds)
	mobFive := mobDropOutcomes(t, mob, table, seeds)
	t.Logf("×5：世界掉落 %d/%d，小怪池 %d/%d", len(worldFive), seeds, len(mobFive), seeds)
	if len(worldFive) <= len(worldBase)*3 || len(mobFive) <= len(mobBase)*3 {
		t.Fatalf("×5 没有把概率拉起来：世界 %d→%d，小怪 %d→%d", len(worldBase), len(worldFive), len(mobBase), len(mobFive))
	}
	if len(worldFive) > seeds || len(mobFive) > seeds {
		t.Fatalf("概率不可能超过 100%%：世界 %d，小怪 %d", len(worldFive), len(mobFive))
	}
	// 阈值只增不减 ⇒ 原来掉的 seed 现在必掉，且掉的还是同一件（同 seed 的第一次抽签不受阈值影响）。
	for seed, template := range worldBase {
		if got, ok := worldFive[seed]; !ok || got != template {
			t.Fatalf("世界掉落 ×5 后 seed%d 变了：%d → %d（在=%v）", seed, template, got, ok)
		}
	}
	for seed, template := range mobBase {
		if got, ok := mobFive[seed]; !ok || got != template {
			t.Fatalf("小怪池 ×5 后 seed%d 变了：%d → %d（在=%v）", seed, template, got, ok)
		}
	}

	// 撤销 = 回到 1.00 倍，逐位相同（同一 seed 同一结果）。
	modpolicy.ConfigureDrops(modpolicy.DropRules{})
	if got := worldDropOutcomes(t, world, seeds); !reflect.DeepEqual(got, worldBase) {
		t.Fatal("撤销世界掉落倍率后没有回到基线")
	}
	if got := mobDropOutcomes(t, mob, table, seeds); !reflect.DeepEqual(got, mobBase) {
		t.Fatal("撤销小怪池倍率后没有回到基线")
	}
}

// TestOrdinaryDropPoolsEnabledUsesRuntimeRate 钉住 session.go 的池子开关：
// 它必须看**实际**速率（含 mod 倍率），且倍率不能把 base=0 的池子凭空打开。
func TestOrdinaryDropPoolsEnabledUsesRuntimeRate(t *testing.T) {
	defer modpolicy.Reset()
	modpolicy.Reset()
	c := catalog.LootCatalog{}
	if ordinaryDropPoolsEnabled(c) {
		t.Fatal("两档都关着时不该开")
	}
	modpolicy.ConfigureDrops(modpolicy.DropRules{WorldPercent: 500, MonsterItemPercent: 500, Source: "test"})
	if ordinaryDropPoolsEnabled(c) {
		t.Fatal("base 为 0 时倍率不能凭空打开池子")
	}
	c.OrdinaryMonsterItemRate = 1000
	if !ordinaryDropPoolsEnabled(c) {
		t.Fatal("小怪池开着时应当开")
	}
	c.OrdinaryMonsterItemRate = 0
	c.WorldDrop = &catalog.WorldDropTable{}
	if ordinaryDropPoolsEnabled(c) {
		t.Fatal("没有世界掉落表条目时不该开")
	}
	c.OrdinaryWorldDropPercent = 100
	if !ordinaryDropPoolsEnabled(c) {
		t.Fatal("世界掉落开着时应当开")
	}
	// 运行期可变：只改 mod 策略不改目录，开关状态就能翻转。
	c.OrdinaryMonsterItemRate = 0
	c.WorldDrop = nil
	modpolicy.ConfigureDrops(modpolicy.DropRules{})
	if ordinaryDropPoolsEnabled(c) {
		t.Fatal("撤销倍率后不该还开着")
	}
}
