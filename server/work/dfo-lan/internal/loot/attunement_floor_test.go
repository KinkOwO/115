package loot

import (
	"strings"
	"testing"
)

// syntheticAttunement 造一张最小可用的合成表：一个副本，fixed 池按给定条目。
//
// 用来构造**真实源表里不会出现的形状**（池子里带保底档以下的条目）。
// 四张真实 CTP 的 fixed 池本来就只写保底档及以上，所以那条路径在真表上是恒等变换；
// 合成数据的存在价值是：把「保底是下界而不是结果」这件事变成可断言的行为。
func syntheticAttunement(t *testing.T, dungeon uint32, fixed []attunementEntry) *AttunementRewards {
	t.Helper()
	a, err := NewAttunementRewards(AttunementRewards{
		Model: AttunementModel,
		Tables: []attunementDungeon{{
			Path: "synthetic.ctp", SHA256: strings.Repeat("0", 64), Dungeon: dungeon,
			Fixed: []attunementFixed{{Maze: 0, Entries: fixed}},
			Additional: []attunementAdditional{{
				EffectIndex: 1, SelectProb: attunementWeightSpace, DropCount: 1,
				Entries: []attunementEntry{{Tier: "epic", Weight: attunementWeightSpace, Item: 5000}},
			}},
		}},
	})
	if err != nil {
		t.Fatalf("synthetic table: %v", err)
	}
	return a
}

// TestPlanRunKeepsTheWeightedRollAboveTheFloor 钉住业主 2026-10-08 的口径：
// **保底是掷骰的下界，不是结果** —— 掷出来完全可以更高，而且仍然是按权重掷。
//
// 断言两件事：
//  1. 只会掷出池子里真实存在的档位（不会凭空多出别的档）；
//  2. 各档的出现次数**随权重单调下降**（900000 / 90000 / 9000 / 1000）。
//     如果是「保底档固定 + 平摊更高档」这类实现，第 2 条就会挂。
func TestPlanRunKeepsTheWeightedRollAboveTheFloor(t *testing.T) {
	a := syntheticAttunement(t, 900001, []attunementEntry{
		{Tier: "normal", Weight: 900000, Item: 1},
		{Tier: "rare", Weight: 90000, Item: 2},
		{Tier: "epic", Weight: 9000, Item: 3},
		{Tier: "primeval", Weight: 1000, Item: 4},
	})
	seen := map[uint32]int{}
	const seeds = 10000
	for seed := uint32(1); seed <= seeds; seed++ {
		tiers, _, err := a.PlanRun(seed, 900001, 0)
		if err != nil {
			t.Fatalf("PlanRun(seed=%d): %v", seed, err)
		}
		// 保底 = fixed 池里最低的那个六档稀有度。源表把「这个砝码保证什么」写成了
		// 「这张表里最低的那一档」，所以我们读的是源，不是另立的策略常量。
		if tiers.Floor != 40 || tiers.FloorLabel != "normal" {
			t.Fatalf("floor = %d(%s), want 40(normal)", tiers.Floor, tiers.FloorLabel)
		}
		if tiers.Primer < tiers.Floor {
			t.Fatalf("primer %d 低于保底 %d", tiers.Primer, tiers.Floor)
		}
		seen[tiers.Primer]++
	}
	for grade := range seen {
		switch grade {
		case 40, 41, 44, 45:
		default:
			t.Fatalf("掷出池子里没有的档位 %d：%v", grade, seen)
		}
	}
	order := []uint32{40, 41, 44, 45}
	for i := 1; i < len(order); i++ {
		if seen[order[i-1]] <= seen[order[i]] {
			t.Fatalf("各档次数没有随权重下降（%v）：保底把掷骰改成了别的分布", seen)
		}
	}
	if seen[45] == 0 {
		t.Fatalf("10000 次一次最高档都没有（期望 ~10 次）：%v", seen)
	}
}

// TestPlanRunRollsAboveTheFloorNotOnIt 用「池里最低档 = 保底」的真实形状证明：
// 保底档与更高档都要出，而不是钉在保底上（业主口径的原话：可以随机出更高品质）。
func TestPlanRunRollsAboveTheFloorNotOnIt(t *testing.T) {
	a := syntheticAttunement(t, 900002, []attunementEntry{
		{Tier: "epic", Weight: 914300, Item: 11},
		{Tier: "primeval", Weight: 85700, Item: 12},
	})
	seen := map[uint32]int{}
	for seed := uint32(1); seed <= 3000; seed++ {
		tiers, _, err := a.PlanRun(seed, 900002, 0)
		if err != nil {
			t.Fatalf("PlanRun(seed=%d): %v", seed, err)
		}
		if tiers.Floor != 44 || tiers.FloorLabel != "epic" {
			t.Fatalf("floor = %d(%s), want 44(epic)", tiers.Floor, tiers.FloorLabel)
		}
		if tiers.Primer < tiers.Floor {
			t.Fatalf("primer %d 低于保底 %d", tiers.Primer, tiers.Floor)
		}
		seen[tiers.Primer]++
	}
	if seen[44] == 0 {
		t.Fatal("保底档自己一次都没出")
	}
	if seen[45] == 0 {
		// 8.57% × 3000 ≈ 257 次，缺席说明「向上随机」没接上。
		t.Fatalf("更高档从没出过（%v）：保底被做成了固定值而不是下界", seen)
	}
}

// TestPlanRunHasNoFloorWhenNothingIsComparable 钉住兼容面：
// 池里没有可比较的六档稀有度时（例如只有两档幸运），保底为 0，掷骰与旧行为一致。
func TestPlanRunHasNoFloorWhenNothingIsComparable(t *testing.T) {
	a := syntheticAttunement(t, 900003, []attunementEntry{
		{Tier: "luck15", Weight: 900000, Item: 21},
		{Tier: "luck30", Weight: 100000, Item: 22},
	})
	for seed := uint32(1); seed <= 500; seed++ {
		tiers, _, err := a.PlanRun(seed, 900003, 0)
		if err != nil {
			t.Fatalf("PlanRun(seed=%d): %v", seed, err)
		}
		if tiers.Floor != 0 || tiers.FloorLabel != "" {
			t.Fatalf("floor = %d(%s), want 0", tiers.Floor, tiers.FloorLabel)
		}
		if tiers.Primer != 70 && tiers.Primer != 71 {
			t.Fatalf("primer = %d, want 70/71", tiers.Primer)
		}
	}
}

// TestPickTierAboveRefusesAnUncomparableLabel 钉住「不猜」：
// 保底存在时，一个既不是六档稀有度也不是两档幸运的标签必须**报错**，
// 而不是被当成「算不算达标都行」静默留下（猜错的代价是发出低于保底的奖励）。
func TestPickTierAboveRefusesAnUncomparableLabel(t *testing.T) {
	rng := RNG{1}
	_, err := pickTierAbove(&rng, []attunementEntry{
		{Tier: "voidsoul", Weight: 1000000, Item: 31},
	}, 44)
	if err == nil {
		t.Fatal("无法比较的标签必须报错")
	}
	if !strings.Contains(err.Error(), "voidsoul") {
		t.Fatalf("错误里要点名那个标签：%v", err)
	}
	// 保底为 0 时同样的池子按旧行为原样掷（非调律副本 / 表缺失那条路不能被牵连）。
	rng = RNG{1}
	got, err := pickTierAbove(&rng, []attunementEntry{
		{Tier: "voidsoul", Weight: 1000000, Item: 31},
	}, 0)
	if err != nil || got != "voidsoul" {
		t.Fatalf("floor=0 时应原样掷出：%q %v", got, err)
	}
}

// TestShippedTablesDeclareTheGuaranteedFloor 把**官方口径**与**源表自己**对起来：
//
//	传说砝码 ⇒ 保底 传说~太初；史诗砝码 ⇒ 保底 史诗~太初（业主 2026-10-07 的 6 张图）
//
// 三个砝码档各是一张独立的 CTP（unique / legendary / epic.ctp），文件里最低的那一档
// 就是它承诺的保底。这条测试的价值在于：**保底是从源表推出来的**，不是我们另写的一行
// 常量 —— 源表换了内容（最低档被改），这里的期望值立刻失配。
func TestShippedTablesDeclareTheGuaranteedFloor(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cases := []struct {
		name    string
		dungeon uint32
		floor   uint32
		label   string
	}{
		{"100005067 传说砝码", 100005067, 43, "legendary"},
		{"100005068 史诗砝码", 100005068, 44, "epic"},
		{"100005066 无砝码", 100005066, 42, "unique"},
		{"100005014 小深渊", 100005014, 40, "normal"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tiers, _, err := a.PlanRun(1, c.dungeon, 0)
			if err != nil {
				t.Fatalf("PlanRun: %v", err)
			}
			if tiers.Floor != c.floor || tiers.FloorLabel != c.label {
				t.Fatalf("副本 %d 的保底 = %q(%d)，官方/源表口径是 %q(%d)",
					c.dungeon, tiers.FloorLabel, tiers.Floor, c.label, c.floor)
			}
			if tiers.Primer < tiers.Floor {
				t.Fatalf("primer %d 低于保底 %d", tiers.Primer, tiers.Floor)
			}
		})
	}
}

// TestAnimationGradeFollowsTheClientTable 钉住「本场档位 → 客户端演出那一格」的换算。
//
// 依据（实测 2026-10-08）：四格演出按 [Unique, Legendary, Epic, Primeval] 存，
// 索引 = 值 − 40 —— 业主发 42 看到 EpicDrop、发 40 看到最低那格；
// 而大深渊保底是传说(43) ⇒ 索引 3 = Primeval、44/45 越界兜底 ⇒「每次都播太初」。
// 我们的阶梯比演出表晚两格 ⇒ 必须 −2。
func TestAnimationGradeFollowsTheClientTable(t *testing.T) {
	cases := []struct {
		tier uint32
		slot uint32
		ok   bool
	}{
		{42, 40, true}, // 独有 → UniqueDrop
		{43, 41, true}, // 传说 → LegendaryDrop
		{44, 42, true}, // 史诗 → EpicDrop
		{45, 43, true}, // 太初 → PrimevalDrop
		{70, 43, true}, // 幸运（独立轴）→ 取最高格
		{71, 43, true},
		{40, 0, false}, // 普通：演出表从「独有」起，没有格
		{41, 0, false}, // 高级：同上
		{0, 0, false},  // 非调律副本
		{72, 0, false}, // 模块哨兵值
	}
	for _, c := range cases {
		got, ok := AnimationGrade(c.tier)
		if ok != c.ok || got != c.slot {
			t.Errorf("AnimationGrade(%d) = (%d,%v)，want (%d,%v)", c.tier, got, ok, c.slot, c.ok)
		}
		if ok && AnimationSlot(got) == "" {
			t.Errorf("AnimationGrade(%d) 的格 %d 没有名字（日志要用）", c.tier, got)
		}
	}
	// 大深渊的保底必须落在**能表达的格**上：传说/史诗砝码各一张表的最低档。
	for _, tier := range []uint32{43, 44} {
		if _, ok := AnimationGrade(tier); !ok {
			t.Fatalf("保底档 %d 在演出表里没有格，大深渊会没有演出", tier)
		}
	}
}
