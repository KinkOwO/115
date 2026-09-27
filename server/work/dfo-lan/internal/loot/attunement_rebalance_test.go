package loot

import (
	"math"
	"testing"
)

// 调参层改的是**玩家实际看到的比例**，所以这些测试钉的是精确数字，而不是
// 「大概变大了」。数值一旦漂移，就应该有人来看一眼。
//
// 默认幅度是 25%（低档各减 25%，减掉的按高档现比例补）。这个数刻意不写成「减半」：
// 减半会把神器推到 49%，等于把「稀有」这一档废掉。

// TestRebalanceOmenHalvesIdle 钉住征兆每行「① 无事发生」减半后的份额，
// 以及腾出份额的分配：行 1..3 在 ② / ③ 之间对半分（奇数多出的 1 给 ③），
// 行 0 没有 ③（官方该行 [drop prob] 就是 0），所以全部并入 ②。
func TestRebalanceOmenHalvesIdle(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	const dungeon = uint32(100005014)
	before := a.OmenStages(dungeon)
	if len(before) != 5 {
		t.Fatalf("omen stages = %d, want 5", len(before))
	}
	if _, _, err := a.ApplyRebalance(Rebalance{OmenHalveIdle: true}); err != nil {
		t.Fatal(err)
	}
	got := a.OmenStages(dungeon)
	want := []struct{ obtain, drop uint32 }{
		{550000, 0},      // 行 0：10% + 45%，没有 ③ 可补
		{175000, 675000}, // 行 1：无事 30% → 15%，腾出 15% 对半分
		{225000, 525000}, // 行 2：无事 50% → 25%
		{242500, 472500}, // 行 3：无事 57% → 28.5%
		{0, 1000000},     // 行 4：满档直接结算，本来就没有「无事」
	}
	for i := range want {
		if got[i].ObtainProb != want[i].obtain || got[i].DropProb != want[i].drop {
			t.Fatalf("row %d = obtain %d drop %d, want obtain %d drop %d",
				i, got[i].ObtainProb, got[i].DropProb, want[i].obtain, want[i].drop)
		}
	}
	// 前四行必须真的变了，否则上面的断言在「改前 == 改后」时也会静默通过。
	for i := 0; i < 4; i++ {
		if before[i].ObtainProb == got[i].ObtainProb && before[i].DropProb == got[i].DropProb {
			t.Fatalf("row %d did not change (obtain %d drop %d)", i, got[i].ObtainProb, got[i].DropProb)
		}
		wantIdle := (1000000 - int(before[i].ObtainProb) - int(before[i].DropProb)) / 2
		if idle := 1000000 - int(got[i].ObtainProb) - int(got[i].DropProb); idle != wantIdle {
			t.Fatalf("row %d idle = %d, want %d", i, idle, wantIdle)
		}
	}
}

// TestRebalanceTiltsFixedPool 钉住 fixed 池「低档各减 25%、高档定量补」的结果。
func TestRebalanceTiltsFixedPool(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	const dungeon, maze = uint32(100005014), uint32(0)
	before := a.FixedTiers(dungeon, maze)
	if _, _, err := a.ApplyRebalance(Rebalance{FixedTiltPercent: 25}); err != nil {
		t.Fatal(err)
	}
	got := a.FixedTiers(dungeon, maze)
	want := map[string]uint32{
		"normal":    353001,
		"rare":      204000,
		"unique":    346482,
		"legendary": 62367,
		"epic":      27718,
		"primeval":  2599,
		"luck15":    3333,
		"luck30":    500,
	}
	var sum uint64
	for tier, w := range want {
		if got[tier] != w {
			t.Fatalf("%s = %d, want %d", tier, got[tier], w)
		}
		sum += uint64(got[tier])
	}
	if sum != attunementWeightSpace {
		t.Fatalf("rebalanced fixed pool sums to %d, want %d", sum, attunementWeightSpace)
	}
	// 低档确实降了、高档确实升了 —— 否则「对称的 bug」也能让上面那张表通过。
	for _, tier := range []string{"normal", "rare"} {
		if got[tier] >= before[tier] {
			t.Fatalf("%s did not go down: %d -> %d", tier, before[tier], got[tier])
		}
	}
	for _, tier := range []string{"unique", "legendary", "epic", "primeval"} {
		if got[tier] <= before[tier] {
			t.Fatalf("%s did not go up: %d -> %d", tier, before[tier], got[tier])
		}
	}
	// 幸运事件的两条**必须原样不动**：那是另一条线（Mystical Fortune），
	// 「补给高档」不该顺手把它改掉。
	if got["luck15"] != before["luck15"] || got["luck30"] != before["luck30"] {
		t.Fatalf("luck tiers moved: %d/%d -> %d/%d",
			before["luck15"], before["luck30"], got["luck15"], got["luck30"])
	}
	// 高档被放大**同一个倍数** ⇒ 稀有度阶梯的形状不变，变的只是总量。
	// 这是「按现有比例补」与「等分 / 越稀有越多」的分水岭，所以单独钉一条。
	for _, pair := range [][2]string{{"unique", "legendary"}, {"unique", "primeval"}, {"legendary", "epic"}} {
		was := float64(before[pair[0]]) / float64(before[pair[1]])
		now := float64(got[pair[0]]) / float64(got[pair[1]])
		if math.Abs(was-now) > was*0.01 {
			t.Fatalf("%s : %s was %.4f, now %.4f — the tier ladder changed shape",
				pair[0], pair[1], was, now)
		}
	}
}

// TestRebalanceTiltRejectsTotalCut 守住「把低档清空」这种极端配置 —— 那等于把
// 稀有度体系砍掉一半，不该能通过一个数字悄悄上线。
func TestRebalanceTiltRejectsTotalCut(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.ApplyRebalance(Rebalance{FixedTiltPercent: 100}); err == nil {
		t.Fatal("a 100% tilt must be refused")
	}
	if _, _, err := a.ApplyRebalance(Rebalance{FixedTiltPercent: 250}); err == nil {
		t.Fatal("a tilt above 100% must be refused")
	}
}

// TestRebalanceLeavesHighTierTablesAlone 说明这条变换的作用域：大深渊的三张表
// 里根本没有普通 / 稀有档（它们的池子本身就是按难度给的 unique/legendary/epic），
// 所以「削低档」对它们应当是零改动。
func TestRebalanceLeavesHighTierTablesAlone(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	const dungeon, maze = uint32(100005066), uint32(0)
	before := a.FixedTiers(dungeon, maze)
	if len(before) == 0 {
		t.Fatalf("the high-tier table has no fixed pool to compare")
	}
	for tier := range before {
		if fixedDemotedTiers[tier] {
			t.Fatalf("precondition broken: %s appears in the high-tier pool", tier)
		}
	}
	if _, _, err := a.ApplyRebalance(Rebalance{OmenHalveIdle: true, FixedTiltPercent: 25}); err != nil {
		t.Fatal(err)
	}
	got := a.FixedTiers(dungeon, maze)
	for tier, w := range before {
		if got[tier] != w {
			t.Fatalf("dungeon %d %s = %d, want it untouched at %d", dungeon, tier, got[tier], w)
		}
	}
}

// TestRebalanceDisabledIsANoOp 守住「默认关闭 ⇒ 与官方表逐字节一致」这条承诺。
func TestRebalanceDisabledIsANoOp(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatal(err)
	}
	const dungeon = uint32(100005014)
	beforeStages := a.OmenStages(dungeon)
	beforeTiers := a.FixedTiers(dungeon, 0)
	rows, groups, err := a.ApplyRebalance(Rebalance{})
	if err != nil {
		t.Fatal(err)
	}
	if rows != 0 || groups != 0 {
		t.Fatalf("disabled rebalance touched %d rows / %d groups", rows, groups)
	}
	for i, st := range a.OmenStages(dungeon) {
		if st.ObtainProb != beforeStages[i].ObtainProb || st.DropProb != beforeStages[i].DropProb {
			t.Fatalf("row %d moved with the rebalance disabled", i)
		}
	}
	for tier, w := range beforeTiers {
		if a.FixedTiers(dungeon, 0)[tier] != w {
			t.Fatalf("tier %s moved with the rebalance disabled", tier)
		}
	}
}

// TestRebalanceTiltTable 把几档倾斜幅度的实际结果打出来，供挑幅度时对照。
// 顺带断言「幅度越大高档越多」是单调的 —— 这条性质写错了不会报错，只会静默变味。
func TestRebalanceTiltTable(t *testing.T) {
	const dungeon, maze = uint32(100005014), uint32(0)
	var prevUpper uint64
	for _, tilt := range []uint32{0, 15, 25, 35, 50} {
		a, err := LoadAttunementRewards(attunementConfig)
		if err != nil {
			t.Fatal(err)
		}
		if _, _, err := a.ApplyRebalance(Rebalance{FixedTiltPercent: tilt}); err != nil {
			t.Fatal(err)
		}
		tiers := a.FixedTiers(dungeon, maze)
		var upper uint64
		for _, k := range []string{"unique", "legendary", "epic", "primeval"} {
			upper += uint64(tiers[k])
		}
		if upper < prevUpper {
			t.Fatalf("tilt %d%% moved the high tiers backwards: %d after %d", tilt, upper, prevUpper)
		}
		prevUpper = upper
		t.Logf("tilt %2d%%: normal %6d · rare %6d · unique %6d · legendary %5d · epic %5d · primeval %4d | 高档合计 %.1f%%",
			tilt, tiers["normal"], tiers["rare"], tiers["unique"], tiers["legendary"], tiers["epic"], tiers["primeval"],
			float64(upper)/10000)
	}
}
