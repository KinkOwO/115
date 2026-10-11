package loot

import "fmt"

// 征兆系统（omen）是千海之空两个深渊共有的通关摸奖机制。它没有独立的表：
// 表就是奖励表里的 [coupon drop table]，一行一个阶段。
//
// 两列概率读作**互斥的三选一门槛**，行号 = 玩家结算前持有的征兆数：
//
//	roll < [obtain prob]                 -> 获得一个征兆（持有 +1）
//	roll < [obtain prob] + [drop prob]   -> 结算该阶段奖励（抽该行 list 一次），持有归零
//	否则                                  -> 无变化
//
// 这个读法有两条互相独立的真值撑着，缺一条都不够：
//
//  1. 官方对玩家的说明是「征兆持有 1-3 个时，会从 获得一个征兆 / 结算征兆奖励
//     / 无变化 中随机一个」——那就是三个分支，不是两次独立判定。
//  2. 这两列的合计（本表 obtain 400000、drop 2330000）都不落在百万空间，而文件里
//     其它每一张表都恰好落。三选一的模型**不需要**它们落，因为「无变化」拿走了
//     剩余空间；这同时解释了为什么此前找不到可证伪的不变量。
//
// 行与官方四阶段逐条对应（三条独立对位）：条目数 3/3/2/1 等于官方奖励表的件数、
// 各档主奖励盒开出来恰好是该档稀有度（神器 unique / 传说 legendary / 史诗 epic /
// 太初 primeval）、第 4 行 [drop prob] = 1000000 即「累积满 4 阶段必定触发奖励结算」。
//
// 官方规则（业主 2026-09-27 提供，见 docs §38）：
//
//	① 无征兆通关 -> 有概率激活第一个征兆（永远是神器）；
//	② 持有征兆通关 -> 三选一：无事发生 / 额外激活更高品质一个 / 结算并重置；
//	③ 满 4 个 -> 直接结算；
//	④ **奖励可以兼得**：结算时按**已激活的每一档**各给 1 个
//	   （激活神器+传说+史诗时结算 = 三段奖励各 1 个）。
//
// 第 ④ 条就是 payOmenStages：一次结算对行 1..持有数 各抽一次 list。此前这里只抽
// 当前行一次，等于**少发**（满档 1 件 vs 4 件），是 §38.3 的第 1 号差距。
//
// 结算后持有归零由官方第 ② 条「结算征兆**并重置**」直接给出，不再是推断。

// OmenPityMisses 是小深渊征兆的保底：**连续 OmenPityMisses 次通关都没有触发征兆**时，
// 这一场直接补 1 阶（官方设定，业主 2026-10-08 提供）。
//
// 计数按**角色**记（存档 character_omen_state.misses），规则取自业主原话：
//
//	记录玩家没触发征兆的次数，一旦有征兆了就归 0，即便征兆当轮不结算；
//	直到征兆被结算后，重新计数。
//
// 落到代码里就是三句：计数只在「这一场开始前一个征兆都没有」时 +1；
// 遇到「获得一个征兆」或「结算」都归 0；到 30 就直接补一阶并归 0。
//
// 只有带 [coupon drop table] 的副本会推进它（当前只有小深渊 100005014）——
// 没有征兆表的副本在 AdvanceOmen 开头就原样返回了，连计数都不碰。
const OmenPityMisses = 30

// OmenStage 是征兆表的一行。
type OmenStage struct {
	// Index 是行号，也是结算这一行时玩家结算前持有的征兆数。
	Index uint32
	// ObtainProb 是「本次通关获得一个征兆」的阈值（百万空间）。
	ObtainProb uint32
	// DropProb 是「本次通关结算这一阶段奖励」的宽度，紧跟 ObtainProb 之后。
	DropProb uint32
	// entries 是该阶段结算时抽的 [drop list]。
	entries []attunementEntry
}

// OmenStages 返回副本的征兆阶段表，按行序。没有 [coupon drop table] 的副本
// 返回 nil —— 那正是「这个副本没有征兆系统」，调用方据此完全不动种子。
func (a *AttunementRewards) OmenStages(dungeon uint32) []OmenStage {
	if a == nil {
		return nil
	}
	t, ok := a.byDungeon[dungeon]
	if !ok || len(t.Coupons) == 0 {
		return nil
	}
	out := make([]OmenStage, 0, len(t.Coupons))
	for i, c := range t.Coupons {
		out = append(out, OmenStage{
			Index:      uint32(i),
			ObtainProb: c.ObtainProb,
			DropProb:   c.DropProb,
			entries:    c.Entries,
		})
	}
	return out
}

// OmenStagesCount 报告副本的征兆阶段行数（0 = 没有征兆系统）。
func (a *AttunementRewards) OmenStagesCount(dungeon uint32) int {
	if a == nil {
		return 0
	}
	t, ok := a.byDungeon[dungeon]
	if !ok {
		return 0
	}
	return len(t.Coupons)
}

// OmenTemplates 列出征兆表能发的所有项（未展开的包装）。它们不动固定/追加表，
// 但同样要在启动期校验：一旦开始抽它们，一个开不出产物的包装就是玩家看得见
// 的「掉了个用不了的东西」。
func (a *AttunementRewards) OmenTemplates() []uint32 {
	if a == nil {
		return nil
	}
	seen := map[uint32]bool{}
	var out []uint32
	for _, t := range a.Tables {
		for _, c := range t.Coupons {
			for _, e := range c.Entries {
				if e.Item != 0 && !seen[e.Item] {
					seen[e.Item] = true
					out = append(out, e.Item)
				}
			}
		}
	}
	return out
}

// OmenStageTemplates 列出一个阶段能发的项目。对账要用它：官方四阶段各自
// 对应一个档位的星蕴石，产出串档就是读错了行。
func (a *AttunementRewards) OmenStageTemplates(dungeon, stage uint32) []uint32 {
	if a == nil {
		return nil
	}
	tb, ok := a.byDungeon[dungeon]
	if !ok || int(stage) >= len(tb.Coupons) {
		return nil
	}
	seen := map[uint32]bool{}
	var out []uint32
	for _, e := range tb.Coupons[stage].Entries {
		if e.Item != 0 && !seen[e.Item] {
			seen[e.Item] = true
			out = append(out, e.Item)
		}
	}
	return out
}

// OmenOutcome 是一场通关对征兆状态的全部改变。字段是给日志用的：玩家报告的
// 「这把给了什么」应当能从事件流里直接读出来，而不是靠反推掉落物属于哪一档。
type OmenOutcome struct {
	Dungeon uint32
	// Seq 是这本账的自增序号。调用方靠它判断「这一条是否已经报过」，
	// 免得同一场里每只怪的死亡都重报一次结算。
	Seq uint64
	// Held 是这一场开始前持有的征兆数，也就是选中的行号。
	Held uint32
	// After 是这一场结束后持有的征兆数。
	After uint32
	// Misses 是这一场开始前「连续未触发」的计数（见 OmenPityMisses）。
	Misses uint32
	// MissesAfter 是这一场结束后的计数。和 Held/After 一样拆成两个：落库发生在
	// **发布**那一刻（见 omen_ledger.go 的 commit），发布前要能验证这本账有没有
	// 在期间被别人动过。
	MissesAfter uint32
	// Pity 表示这一场的一阶是**保底补的**，不是掷出来的（日志与实机验收要用）。
	Pity bool

	// Gained 表示这一场获得了一个征兆。
	Gained bool
	// Paid 表示这一场结算了阶段奖励。
	Paid bool
	// Stage 是结算时用的行号（只有 Paid 时有意义，等于结算前的 Held）。
	Stage uint32
	// PaidStages 是**实际发了奖励的档位**，升序。官方「奖励可以兼得」⇒
	// 结算持有 N 个时这里是 1..N；它等于 Awards 里每一项来自哪一行。
	PaidStages []uint32
	// Awards 是阶段奖励的**包装**，由调用方按既有开箱路径展开。
	Awards []Award
	// Seed 是推进后的种子。
	Seed uint32
}

// AdvanceOmen 推进一场通关的征兆状态。
//
// 没有征兆表的副本、或表里没有行的副本，原样返回种子与持有数：这条线对其它
// 副本是惰性的，和禁用的章节掉落一样。
func (a *AttunementRewards) AdvanceOmen(seed, dungeon, held, misses uint32) (OmenOutcome, error) {
	out := OmenOutcome{Dungeon: dungeon, Held: held, After: held, Misses: misses, MissesAfter: misses, Seed: seed}
	stages := a.OmenStages(dungeon)
	if len(stages) == 0 {
		return out, nil
	}
	// 持有数封顶到最后一行：表只有 5 行（0..4），而第 4 行的 [obtain prob] 是 0，
	// 所以正常路径永远不会溢出。封顶是防御，代价是「溢出时按满阶段结算」，
	// 比越界 panic 或按第 0 行结算都更接近设计。
	if int(out.Held) >= len(stages) {
		out.Held = uint32(len(stages) - 1)
	}
	stage := stages[out.Held]
	out.Stage = stage.Index
	rng := RNG{seed}
	// 满档：官方第 ③ 条「激活 4 个征兆时通关则直接结算」，不掷骰。
	// （行 4 的 [drop prob] 本来就是 1000000，所以这与按行判定等价；
	// 写成显式分支是为了让「满档」这件事在代码里看得见，而不是靠表里的常量。）
	if int(out.Held) == len(stages)-1 {
		out.Paid = true
		if err := a.payOmenStages(&rng, stages, out.Held, &out); err != nil {
			return out, err
		}
		out.After = 0
		out.MissesAfter = 0
		out.Seed = rng.Seed
		return out, nil
	}
	roll := rng.Next(attunementWeightSpace)
	switch {
	case roll < stage.ObtainProb:
		out.Gained = true
		if int(out.After)+1 < len(stages) {
			out.After++
		}
		// 有征兆了就归 0（官方口径）：不管这一场结不结算。
		out.MissesAfter = 0
	case roll < stage.ObtainProb+stage.DropProb:
		out.Paid = true
		if err := a.payOmenStages(&rng, stages, out.Held, &out); err != nil {
			return out, err
		}
		out.After = 0
		// 结算之后重新计数（官方口径）。
		out.MissesAfter = 0
	default:
		// 无事发生。只有「本来就一个征兆都没有」的那些通关才算「没触发」：
		// 持有 ≥1 时按口径要等这次征兆结算之后才重新计数，所以这里不动它。
		if out.Held == 0 {
			out.MissesAfter = out.Misses + 1
			if out.MissesAfter >= OmenPityMisses {
				// 保底：这一场直接补 1 阶，不掷骰、也不额外消耗随机数。
				out.MissesAfter = 0
				out.Gained, out.Pity = true, true
				if int(out.After)+1 < len(stages) {
					out.After++
				}
			}
		}
	}
	out.Seed = rng.Seed
	return out, nil
}


// payOmenStages 结算**已激活的每一档**：官方「奖励可以兼得」——
// 持有 held 个征兆时结算，就对行 1..held 各抽一次 [drop list]，各出一件。
//
// 行 0 不在范围内（它没有 list，也不代表任何已激活档位）。空 list 的行跳过：
// ValidateOmen 已经保证「声称会结算却没有 list」的表进不来。
func (a *AttunementRewards) payOmenStages(rng *RNG, stages []OmenStage, held uint32, out *OmenOutcome) error {
	for i := uint32(1); i <= held && int(i) < len(stages); i++ {
		st := stages[i]
		if len(st.entries) == 0 {
			continue
		}
		e, err := pickAttunement(rng, st.entries)
		if err != nil {
			return err
		}
		out.Awards = append(out.Awards, Award{Template: e.Item, Amount: 1})
		out.PaidStages = append(out.PaidStages, i)
	}
	return nil
}

// payableTemplates 列出一次通关真能放到地上的所有模板：固定表、追加表，加上
// 征兆阶段。hidden 行不在其中 —— 没有任何东西抽它们。
//
// 它和 RolledTemplates 分开，是因为两者的**动词**不同：前者是「会被抽」，
// 后者是「会被发」。征兆阶段由 AdvanceOmen 抽，所以它们必须同样接受开箱校验，
// 但要收在征兆这一侧，别让固定/追加那条线的判据跟着动。
func (a *AttunementRewards) payableTemplates() []uint32 {
	seen := map[uint32]bool{}
	var out []uint32
	for _, id := range append(a.RolledTemplates(), a.OmenTemplates()...) {
		if id != 0 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// ValidateOmen 检查征兆表本身是否自洽，把它能发的东西一并交给奖励开箱校验。
//
// 两条被拒的形态都对应「玩家会看见一个用不了的奖励」：一条声称会结算却没有
// 列表的行，和一个开不出产物的包装。第 4 行（保底）正是靠前一条保护 —— 它是
// 整个系统的承诺，不能是空的。
func (a *AttunementRewards) ValidateOmen() error {
	if a == nil {
		return nil
	}
	for i := range a.Tables {
		t := &a.Tables[i]
		for j, c := range t.Coupons {
			if c.DropProb > 0 && len(c.Entries) == 0 {
				return fmt.Errorf("dungeon %d omen stage %d can pay (%d) but carries no drop list",
					t.Dungeon, j, c.DropProb)
			}
		}
	}
	return nil
}

// OmenStageIDs returns the reward-preview template of every stage row: the
// heaviest entry of that row's [drop list], which is exactly the main reward box
// of the official table (row 1 = 10416150, row 2 = 10417545, row 3 = 10417552,
// row 4 = 10417571 -- see the positional match in docs section 38.2).
//
// It is not a drop. These are the values of the "omen id" u32 array inside every
// noti 2836 record: the client looks them up to draw the reward preview. Holding N
// stages means writing the first N of them; row 0 carries no entries ("a chance to
// activate the first omen") so it stays 0 and therefore never shows up in a payload.
func (a *AttunementRewards) OmenStageIDs(dungeon uint32) []uint32 {
	if a == nil {
		return nil
	}
	tb, ok := a.byDungeon[dungeon]
	if !ok {
		return nil
	}
	out := make([]uint32, 0, len(tb.Coupons))
	for _, c := range tb.Coupons {
		var item, weight uint32
		for _, e := range c.Entries {
			if e.Item != 0 && e.Weight > weight {
				item, weight = e.Item, e.Weight
			}
		}
		out = append(out, item)
	}
	return out
}
