package loot

import "testing"

// omenDungeon 是唯一带 [coupon drop table]（= 征兆阶段表）的副本：千海之空小深渊。
const omenDungeon = 100005014

// TestOathTierCofferMapsFourTiers 钉住「天平档位 → 星蕴石自选套装罐子」这条线。
//
// 业主 2026-10-01 定调：天平档位与征兆是**两条平行的线**，各发各的（同一场都触发就
// 各自兑现一份）。四档对四个罐子，内容是 §38.2 实测展开的：
//
//	unique(42)    → 10416150 → 12 × rarity 3（神器）
//	legendary(43) → 10417545 → 12 × rarity 6（传说）
//	epic(44)      → 10417552 → 12 × rarity 4（史诗）
//	primeval(45)  → 10417571 → 12 × rarity 8（太初）
//
// normal(40) / rare(41) 不发 —— 官方奖励表里没有 rarity 2 的罐子，行 0 的条目数是 0，
// 国服 1710 场里 32.05% 也正是「不变色、不出东西」。
func TestOathTierCofferMapsFourTiers(t *testing.T) {
	cases := []struct {
		tier uint16
		want uint32
	}{
		{0, 0},         // 非深渊 / 未下发
		{40, 0},        // normal   不发
		{41, 0},        // rare     不发
		{42, 10416150}, // unique
		{43, 10417545}, // legendary
		{44, 10417552}, // epic
		{45, 10417571}, // primeval
		{70, 10417571}, // rainbow1 取最高档
		{71, 10417571}, // rainbow2
	}
	for _, c := range cases {
		if got := oathTierCoffer(c.tier); got != c.want {
			t.Errorf("oathTierCoffer(%d) = %d, want %d", c.tier, got, c.want)
		}
	}
}

// TestOmenStagesReadTheShippedCouponRows 钉住读法的**数据面**：五行、两列门槛、
// 第 4 行 100% 结算。这三样是官方四阶段在文件里的样子，任何一处变动都意味着
// 读法要重新论证，而不是继续按老读法发奖。
func TestOmenStagesReadTheShippedCouponRows(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stages := a.OmenStages(omenDungeon)
	if len(stages) != 5 {
		t.Fatalf("omen stages = %d, want 5 (stage 0..4)", len(stages))
	}
	want := []struct {
		obtain, drop uint32
		entries      int
	}{
		{100000, 0, 0},
		{100000, 600000, 3},
		{100000, 400000, 3},
		{100000, 330000, 2},
		{0, attunementWeightSpace, 1},
	}
	for i, w := range want {
		if stages[i].Index != uint32(i) {
			t.Fatalf("stage %d reports index %d", i, stages[i].Index)
		}
		if stages[i].ObtainProb != w.obtain || stages[i].DropProb != w.drop {
			t.Fatalf("stage %d = obtain %d drop %d, want obtain %d drop %d",
				i, stages[i].ObtainProb, stages[i].DropProb, w.obtain, w.drop)
		}
		if len(stages[i].entries) != w.entries {
			t.Fatalf("stage %d carries %d entries, want %d", i, len(stages[i].entries), w.entries)
		}
	}
	// 第 4 行就是「累积满 4 阶段必定触发奖励结算」，也就是官方承诺的保底。
	if stages[4].DropProb != attunementWeightSpace {
		t.Fatalf("the last stage must always settle, got drop %d", stages[4].DropProb)
	}
	// 其它副本没有征兆系统，这条线对它们必须是惰性的。
	for _, d := range []uint32{100005066, 100005067, 100005068} {
		if n := a.OmenStagesCount(d); n != 0 {
			t.Fatalf("dungeon %d reports %d omen stage(s), want 0", d, n)
		}
	}
	if n := len(a.OmenTemplates()); n != 9 {
		t.Fatalf("omen templates = %d, want 9 (3+3+2+1)", n)
	}
}

// TestAdvanceOmenIsAThreeWayChoice 一次通关只有一个分支：获得一个征兆、
// 结算奖励、或什么都不发生。官方对玩家的说明就是这三选一，所以「同时累积又
// 结算」是读错的直接证据。
func TestAdvanceOmenIsAThreeWayChoice(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	const runs = 40000
	var gained, paid, none int
	for i := uint32(1); i <= runs; i++ {
		out, err := a.AdvanceOmen(i*2654435761+1, omenDungeon, 1)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if out.Gained && out.Paid {
			t.Fatalf("seed %d both gained a mark and settled a stage", i)
		}
		switch {
		case out.Gained:
			gained++
			if out.After != 2 {
				t.Fatalf("seed %d gained but ended at %d, want 2", i, out.After)
			}
			if len(out.Awards) != 0 {
				t.Fatalf("seed %d gained a mark and still paid %v", i, out.Awards)
			}
		case out.Paid:
			paid++
			if out.After != 0 {
				t.Fatalf("seed %d settled but ended at %d, want 0", i, out.After)
			}
			if len(out.Awards) != 1 {
				t.Fatalf("seed %d settled %d award(s), want 1", i, len(out.Awards))
			}
		default:
			none++
			if out.After != 1 {
				t.Fatalf("seed %d did nothing but ended at %d, want 1", i, out.After)
			}
		}
	}
	if gained == 0 || paid == 0 || none == 0 {
		t.Fatalf("all three branches must be reachable: gained %d paid %d none %d", gained, paid, none)
	}
	// 阶段 1 声明的是 10% 累积、60% 结算。只做区间断言，不去追 RNG 的具体
	// 偏斜：这里要证明的是「两列门槛确实各自生效」，不是复刻随机数发生器。
	if r := float64(paid) / runs; r < 0.4 || r > 0.75 {
		t.Fatalf("stage 1 settled on %.1f%% of clears, want the 60%% the row declares", r*100)
	}
	if r := float64(gained) / runs; r < 0.05 || r > 0.25 {
		t.Fatalf("stage 1 granted a mark on %.1f%% of clears, want the 10%% the row declares", r*100)
	}
}

// TestAdvanceOmenStageZeroNeverSettlesAndStageFourAlwaysDoes 两端的硬断言：
// 第 0 行 drop 是 0（还没攒到征兆，不可能结算），第 4 行 drop 是满空间
// （攒满了就保底）。这两条不依赖随机数的分布。
func TestAdvanceOmenStageZeroNeverSettlesAndStageFourAlwaysDoes(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for i := uint32(1); i <= 2000; i++ {
		seed := i * 2246822519
		out, err := a.AdvanceOmen(seed, omenDungeon, 0)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if out.Paid {
			t.Fatalf("seed %d settled on stage 0, whose drop prob is 0", i)
		}
		if out.After != out.Held && out.After != 1 {
			t.Fatalf("seed %d ended at %d, want 0 or 1", i, out.After)
		}
		full, err := a.AdvanceOmen(seed, omenDungeon, 4)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if !full.Paid {
			t.Fatalf("seed %d did not settle a full omen run", i)
		}
		if full.Gained {
			t.Fatalf("seed %d gained a mark while already full", i)
		}
		if full.Stage != 4 || full.After != 0 {
			t.Fatalf("seed %d settled stage %d and ended at %d, want stage 4 -> 0", i, full.Stage, full.After)
		}
		// 满档结算 = 四档各一件（官方「奖励可以兼得」）。第 4 行只有一项（权重 100%），
		// 所以最后那件是固定的保底项 10417571。
		if len(full.Awards) != 4 || len(full.PaidStages) != 4 {
			t.Fatalf("seed %d a full settle paid %d award(s) over %d stage(s), want 4 each",
				i, len(full.Awards), len(full.PaidStages))
		}
		if full.Awards[3].Template != 10417571 {
			t.Fatalf("seed %d the primeval entry is %v, want 10417571", i, full.Awards[3])
		}
		for k, st := range full.PaidStages {
			if st != uint32(k+1) {
				t.Fatalf("seed %d paid stages %v, want 1..4", i, full.PaidStages)
			}
		}
	}
}

// TestAdvanceOmenIgnoresDungeonsWithoutStages 没有征兆表的副本连种子都不动 ——
// 与禁用的章节掉落同一条约定，避免悄悄改变其它副本的随机序列。
func TestAdvanceOmenIgnoresDungeonsWithoutStages(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	const seed = 0xdeadbeef
	out, err := a.AdvanceOmen(seed, 100005067, 3)
	if err != nil {
		t.Fatalf("advance: %v", err)
	}
	if out.Seed != seed || out.Held != 3 || out.After != 3 || out.Gained || out.Paid {
		t.Fatalf("a dungeon without omen stages changed state: %+v", out)
	}
	if len(out.Awards) != 0 {
		t.Fatalf("a dungeon without omen stages paid %v", out.Awards)
	}
}

// TestOmenSettlementPaysEveryActivatedStage 是官方第 ④ 条的直接断言：
// 「奖励可以兼得」—— 持有 N 个征兆时结算，就对已激活的 1..N 行各抽一次，
// 各出一件，且每一件都必须来自它自己那一行的 [drop list]。
//
// 这条曾经是错的（只发当前行一件），所以它单独一条测试：改回来会立刻红。
func TestOmenSettlementPaysEveryActivatedStage(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	stages := a.OmenStages(omenDungeon)
	if len(stages) < 5 {
		t.Fatalf("need the five shipped rows, got %d", len(stages))
	}
	for held := uint32(1); held <= 4; held++ {
		// 固定 seeds 里挑出「结算」那一支，确保走的是发放路径。
		paid := 0
		for i := uint32(1); i <= 5000 && paid < 5; i++ {
			seed := i*2654435761 + 11
			out, err := a.AdvanceOmen(seed, omenDungeon, held)
			if err != nil {
				t.Fatalf("advance: %v", err)
			}
			if !out.Paid {
				continue
			}
			paid++
			if len(out.Awards) != int(held) || len(out.PaidStages) != int(held) {
				t.Fatalf("held %d settled %d award(s) over %v, want %d",
					held, len(out.Awards), out.PaidStages, held)
			}
			for k, st := range out.PaidStages {
				if st != uint32(k+1) {
					t.Fatalf("held %d paid stages %v, want 1..%d", held, out.PaidStages, held)
				}
				row := map[uint32]bool{}
				for _, e := range stages[st].entries {
					row[e.Item] = true
				}
				if !row[out.Awards[k].Template] {
					t.Fatalf("held %d award #%d = %d is not in row %d's list",
						held, k, out.Awards[k].Template, st)
				}
			}
		}
		if paid == 0 {
			t.Fatalf("held %d never settled in 5000 seeds", held)
		}
	}
}

// TestOmenLedgerAccumulatesToTheGuarantee 是这套系统的验收点：从零开始反复
// 通关，累积到第 4 阶段时必定拿到保底那一项。官方承诺的就是这个。
func TestOmenLedgerAccumulatesToTheGuarantee(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	ledger := NewOmenLedger(a)
	if !ledger.Enabled() {
		t.Fatal("ledger reports itself disabled with the shipped table")
	}
	const character = int64(424242)
	reached := 0
	for i := uint32(1); i <= 20000; i++ {
		out, awards, err := ledger.Advance(character, omenDungeon, i*2246822519+7)
		if err != nil {
			t.Fatalf("advance: %v", err)
		}
		if ledger.Held(character) != out.After {
			t.Fatalf("run %d: ledger holds %d, outcome says %d", i, ledger.Held(character), out.After)
		}
		if out.Paid && out.Stage == 4 {
			reached++
			// 满档结算发 4 件（神器/传说/史诗/太初各 1），最后一件是保底的太初项。
			if len(awards) != 4 || awards[3].Template != 10417571 {
				t.Fatalf("run %d: the guarantee paid %v", i, awards)
			}
		}
		if reached > 0 {
			break
		}
	}
	if reached == 0 {
		t.Fatal("20000 clears never accumulated to stage 4, which official text calls a guarantee")
	}
	last, ok := ledger.Last(character)
	if !ok || last.Stage != 4 || last.Seq == 0 {
		t.Fatalf("ledger last = %+v ok=%v", last, ok)
	}
}

// TestOmenLedgerIsSilentWithoutARewardTable 没有奖励表时账本是哑的：不推进、
// 不报错、不消耗种子。
func TestOmenLedgerIsSilentWithoutARewardTable(t *testing.T) {
	ledger := NewOmenLedger(nil)
	if ledger.Enabled() {
		t.Fatal("a ledger without a reward table reports itself enabled")
	}
	const seed = 12345
	out, awards, err := ledger.Advance(1, omenDungeon, seed)
	if err != nil || len(awards) != 0 || out.Seed != seed || ledger.Held(1) != 0 {
		t.Fatalf("ledger without a table acted: %+v %v %v", out, awards, err)
	}
	if _, ok := ledger.Last(1); ok {
		t.Fatal("ledger without a table recorded a settlement")
	}
}

// TestValidateOmenRefusesAStageThatPaysNothing 保底那一行声称 100% 结算，
// 所以它的 drop list 不能是空的 —— 那是整个系统的承诺。
func TestValidateOmenRefusesAStageThatPaysNothing(t *testing.T) {
	path := loadAttunementDoc(t, func(doc map[string]any) {
		tab := attunementTable(t, doc, omenDungeon)
		rows := tab["coupons"].([]any)
		row := rows[4].(map[string]any)
		row["entries"] = []any{}
	})
	a, err := LoadAttunementRewards(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := a.ValidateOmen(); err == nil {
		t.Fatal("a stage that claims to settle but carries no drop list was accepted")
	}
	// 反过来，第 0 行本来就 drop 0 且没有列表，那是合法的「还没攒到」。
	path = loadAttunementDoc(t, nil)
	a, err = LoadAttunementRewards(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if err := a.ValidateOmen(); err != nil {
		t.Fatalf("the shipped table fails its own omen check: %v", err)
	}
}

// TestPayableTemplatesCoverTheOmenRows 一次通关真能放到地上的模板集合必须
// 包含征兆那 9 项 —— 启动期的开箱校验走的就是这个集合，漏掉它们就等于让
// 一个开不出产物的包装直接落到玩家脚下。
func TestPayableTemplatesCoverTheOmenRows(t *testing.T) {
	a, err := LoadAttunementRewards(attunementConfig)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	payable := map[uint32]bool{}
	for _, id := range a.payableTemplates() {
		payable[id] = true
	}
	for _, id := range a.OmenTemplates() {
		if !payable[id] {
			t.Fatalf("omen template %d is not part of what a clear can pay", id)
		}
	}
}
