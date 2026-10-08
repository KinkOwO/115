package loot

import "testing"

// TestTierGradeValuesMatchTheClientLadder 钉住八档的**数值**。
//
// 值域依据（见 cmd/wireprobe/oath_info.go 文件头）：收尾动作是
// `[SET GROUP ACTION] end (max-40)`，而 `o:hp_limit` / `o:delay_time` 都只有 8 项
// ⇒ 合法下标 0..7 ⇒ 八档。40..45 是六档稀有度，70/71 是两档彩虹（神秘的幸运）。
func TestTierGradeValuesMatchTheClientLadder(t *testing.T) {
	want := map[string]uint32{
		"normal": 40, "rare": 41, "unique": 42, "legendary": 43,
		"epic": 44, "primeval": 45, "luck15": 70, "luck30": 71,
	}
	if len(tierGradeValues) != 8 {
		t.Fatalf("tier table has %d rows, want 8", len(tierGradeValues))
	}
	for tier, v := range want {
		if got := TierGradeValue(tier); got != v {
			t.Errorf("TierGradeValue(%q) = %d, want %d", tier, got, v)
		}
		if got := TierForGradeValue(v); got != tier {
			t.Errorf("TierForGradeValue(%d) = %q, want %q", v, got, tier)
		}
	}
	if TierGradeValue("no-such-tier") != 0 || TierForGradeValue(99) != "" {
		t.Fatal("unknown tier/grade must report zero")
	}
}

// TestPlanRunUsesTheShippedTable 钉住预掷：两条线各自落在**表里真实存在的档位**上。
//
// 小深渊的 fixed 池八档（含两档幸运）、additional 池是四档誓约 + 一档「没拿到誓约」，
// 所以 primer 只可能是那 8 个值、oath 只可能是 {40,42,43,44,45}。
// 用一个种子扫一遍，覆盖到多档而不是只试一次。
func TestPlanRunUsesTheShippedTable(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	allowedPrimer := map[uint32]bool{}
	for _, tier := range []string{"normal", "rare", "unique", "legendary", "epic", "primeval", "luck15", "luck30"} {
		allowedPrimer[TierGradeValue(tier)] = true
	}
	allowedOath := map[uint32]bool{40: true, 42: true, 43: true, 44: true, 45: true}

	seenPrimer := map[uint32]bool{}
	for seed := uint32(1); seed <= 4000; seed++ {
		tiers, next, err := a.PlanRun(seed, omenDungeon, 0)
		if err != nil {
			t.Fatalf("PlanRun(seed=%d): %v", seed, err)
		}
		if !allowedPrimer[tiers.Primer] {
			t.Fatalf("primer tier %d is outside the eight tiers", tiers.Primer)
		}
		if !allowedOath[tiers.Oath] {
			t.Fatalf("oath tier %d is not one of 40/42/43/44/45", tiers.Oath)
		}
		seenPrimer[tiers.Primer] = true
		if next == seed {
			t.Fatalf("PlanRun must advance the seed (seed=%d)", seed)
		}
	}
	// 4000 次里六档稀有度应当都出现过（最低档 primeval 0.15% ⇒ 期望 6 次）。
	for _, tier := range []string{"normal", "rare", "unique", "legendary", "epic", "primeval"} {
		if !seenPrimer[TierGradeValue(tier)] {
			t.Errorf("4000 seeds never produced tier %q", tier)
		}
	}
}

// TestRollPlannedFollowsTheGivenTier 钉住「显示哪一档就发哪一档」。
//
// 这条是本次改动的**核心契约**：珠子/天平的档位在进本时定下（随 2838 下发），
// 掉落必须按同一档选池，否则客户端演出与实际奖励不符。
// maze 0 的六个稀有度与两档幸运各只有一个条目 ⇒ 给定档位时结果是确定的。
func TestRollPlannedFollowsTheGivenTier(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	cases := []struct {
		name  string
		tiers RunTiers
		want  []uint32
	}{
		{"normal+没拿到誓约", RunTiers{Primer: 40, Oath: 40}, []uint32{10416103, 10416752}},
		{"rare+unique", RunTiers{Primer: 41, Oath: 42}, []uint32{10416106, 10416141}},
		{"legendary+legendary", RunTiers{Primer: 43, Oath: 43}, []uint32{10416108, 10416142}},
		{"epic+epic", RunTiers{Primer: 44, Oath: 44}, []uint32{10416109, 10416143}},
		{"primeval+primeval", RunTiers{Primer: 45, Oath: 45}, []uint32{10416110, 10416144}},
		{"luck15", RunTiers{Primer: 70}, []uint32{10416119}},
		{"luck30", RunTiers{Primer: 71}, []uint32{10416120}},
		{"只有誓约", RunTiers{Oath: 45}, []uint32{10416144}},
		{"只有固定", RunTiers{Primer: 42}, []uint32{10416107}},
	}
	for _, c := range cases {
		awards, _, err := a.RollPlanned(7, omenDungeon, 0, c.tiers)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if len(awards) != len(c.want) {
			t.Fatalf("%s: %d awards, want %d (%v)", c.name, len(awards), len(c.want), awards)
		}
		for i, w := range c.want {
			if awards[i].Template != w {
				t.Errorf("%s: award[%d] = %d, want %d", c.name, i, awards[i].Template, w)
			}
		}
	}
	// 两条线都没有档位 ⇒ 什么都不发（非调律副本或表缺失）。
	if awards, _, err := a.RollPlanned(7, omenDungeon, 0, RunTiers{}); err != nil || len(awards) != 0 {
		t.Fatalf("空档位应不发奖：%v %v", awards, err)
	}
	// 档位落在表里不存在的格子上 ⇒ 报错（而不是发别的档）。
	if _, _, err := a.RollPlanned(7, omenDungeon, 0, RunTiers{Primer: 46}); err == nil {
		t.Fatal("域外档位必须报错")
	}
}

// TestRollUnchangedWhenNoPlan 钉住兼容面：没有预掷时 Roll 的分布不变。
//
// Roll 现在就是「内部预掷 + 按档选池」，与旧实现同分布（旧实现是整表一次加权掷，
// 而整表权重本来就按 tier 分组、组内权重就是条件权重）。
func TestRollUnchangedWhenNoPlan(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	seen := map[uint32]int{}
	for seed := uint32(1); seed <= 3000; seed++ {
		awards, _, err := a.Roll(seed, omenDungeon, 0)
		if err != nil {
			t.Fatalf("Roll(seed=%d): %v", seed, err)
		}
		if len(awards) == 0 {
			t.Fatalf("Roll(seed=%d) 空奖", seed)
		}
		seen[awards[0].Template]++
	}
	// 普通档有三个条目共 47% ⇒ 3000 次里必然出现，且应当出现多次。
	normal := seen[10416103] + seen[10416104] + seen[10416105]
	if normal == 0 {
		t.Fatalf("3000 次一次普通档都没出：%v", seen)
	}
	if seen[10416110] == 0 {
		// 太初 0.15% ⇒ 期望 4.5 次；缺席说明档位根本没被走到。
		t.Logf("warning: 3000 次未出现太初档（期望 ~4.5 次）")
	}
}
