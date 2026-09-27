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
// 行与官方四阶段逐条对应（三条独立对位）：条目数 3/3/2/1 等于官方斜杠分段数、
// 档位 粉/传说/史诗/太初 等于 unique/legendary/epic/primeval、第 4 行
// [drop prob] = 1000000 即「累积满 4 阶段必定触发奖励结算」。
//
// 结算后持有归零是**推断**：第 4 行 100% 结算，若不消耗则玩家此后每场必得太初。
// 数据没有第二处能证伪它，所以它单独写在这里、单独有测试，改的时候只有一处。

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
	// Gained 表示这一场获得了一个征兆。
	Gained bool
	// Paid 表示这一场结算了阶段奖励。
	Paid bool
	// Stage 是结算的阶段（只有 Paid 时有意义，等于 Held）。
	Stage uint32
	// Awards 是阶段奖励的**包装**，由调用方按既有开箱路径展开。
	Awards []Award
	// Seed 是推进后的种子。
	Seed uint32
}

// AdvanceOmen 推进一场通关的征兆状态。
//
// 没有征兆表的副本、或表里没有行的副本，原样返回种子与持有数：这条线对其它
// 副本是惰性的，和禁用的章节掉落一样。
func (a *AttunementRewards) AdvanceOmen(seed, dungeon, held uint32) (OmenOutcome, error) {
	out := OmenOutcome{Dungeon: dungeon, Held: held, After: held, Seed: seed}
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
	roll := rng.Next(attunementWeightSpace)
	switch {
	case roll < stage.ObtainProb:
		out.Gained = true
		if int(out.After)+1 < len(stages) {
			out.After++
		}
	case roll < stage.ObtainProb+stage.DropProb && len(stage.entries) > 0:
		e, err := pickAttunement(&rng, stage.entries)
		if err != nil {
			return out, err
		}
		out.Paid = true
		out.Awards = []Award{{Template: e.Item, Amount: 1}}
		out.After = 0
	}
	out.Seed = rng.Seed
	return out, nil
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
