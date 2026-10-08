package main

// 征兆（omen）的角色存档读写，以及「隐藏 BOSS 由征兆驱动」的判定。
//
// 为什么征兆要落库、而不是留在内存账本里：见 internal/database/omen_state.go 的文件头
// —— 它是角色存档级的占位标记，不是道具（全库没有一件「征兆」物品），也不属于某一次
// 服务会话。内存账本在重启后归零，玩家永远攒不满四档。
//
// 服务端这一半的完整链路：
//
//	进本 loading          loadOmenRunState   读存档 → held + orthaire_pending
//	                       ├─ omen_info.go     按 held 编 noti 2836（征兆 UI 唯一数据源）
//	                       └─ oath_info.go     按 pending 决定 noti 2838 的档位
//	天平死亡（结算那一刻）  noteOmenSettlement 写回 held；满档结算额外置 pending
//	通关确认              clearOmenOrthaier  清 pending（掉线不该吞掉这次机会）
//
// ⚠️ 2026-10-07 变更：**隐藏 BOSS 不再由征兆驱动**。
//
// 业主 2026-10-07 指出「征兆是独立系统，不该与天平的掉率耦合」；客户端脚本
// （`primer_proc.act` 的 `nox_index_checker`）也证实：两个隐藏 BOSS 只看 2838 的两个档位
//
//	oath_max == 45                   -> 奥尔特尔（太初级誓约那条线）
//	oath_max < 45 && primer_max == 45 -> 监视者（太初级星蕴石那条线）
//
// ⇒ 所以 BOSS 由 internal/loot 预掷的档位自然决定（见 attunement_plan.go）。
//
// 下面这套 `orthaire_pending` 是 2026-09-27「B1」留下的**惰性状态**：读写都还在（存档列
// 保留，§0.4 存档兼容不删列），但**已经没有任何决策读它**。待后续清理时再一并摘除。

import (
	"context"
	"fmt"
	"log"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/loot"
)

// omenStateTimeout 是一次存档读写的上限。写法与 oath_progress.go 一致：卡住的读写
// 不该把玩家的进本流程一起拖住，但绝不能静默吞掉。
const omenStateTimeout = 5 * time.Second

// omenStore returns the persistence handle injected by the composition root.
func (w *worldSession) omenStore() *database.Store {
	if w == nil {
		return nil
	}
	return w.store
}

// omenStagesCount 报告这个副本的征兆阶段行数（0 = 这个副本没有征兆系统）。
func (w *worldSession) omenStagesCount(dungeon uint32) int {
	if w == nil || w.loot == nil || w.loot.Attunement == nil {
		return 0
	}
	return w.loot.Attunement.OmenStagesCount(dungeon)
}

// omenStageIDs 返回每一档的奖励预览模板（见 loot.AttunementRewards.OmenStageIDs）。
func (w *worldSession) omenStageIDs(dungeon uint32) []uint32 {
	if w == nil || w.loot == nil || w.loot.Attunement == nil {
		return nil
	}
	return w.loot.Attunement.OmenStageIDs(dungeon)
}

// omenStateEnabled 说明「征兆 = 角色存档状态」这条线在这个副本上是否生效。
//
// 它要求两件事同时成立：开关打开，且这个副本真的带 [coupon drop table]。后者保证
// 其它副本连一次存档读写都不做。
func (w *worldSession) omenStateEnabled(dungeon uint32) bool {
	return w != nil && w.omenState && w.omenStagesCount(dungeon) > 0
}

// loadOmenRunState 在进本时把征兆存档读进会话。
//
// 读错误**往上抛**，不回落成 0：静默归零会让玩家的征兆无声清零、隐藏 BOSS 无声消失，
// 那是比启动时报错难查得多的行为变更（与 oath_progress.go 同一条）。
func (w *worldSession) loadOmenRunState(dungeonID uint32) error {
	w.omenHeldRun, w.omenOrthaierDue, w.omenHeldReady = 0, false, false
	if !w.omenStateEnabled(dungeonID) {
		return nil
	}
	stages := w.omenStagesCount(dungeonID)

	// 诊断入口 -omen-hold：把玩家直接放到指定阶段，省掉刷场次。每个会话只应用一次，
	// 之后交回正常的累积/结算路径；但**写回存档**，否则重启后这次摆放就白摆了。
	if w.omenHold >= 0 && !w.omenHoldApplied {
		w.omenHoldApplied = true
		held := w.omenHold
		if held > stages-1 {
			held = stages - 1
		}
		if err := w.saveOmenHeld(dungeonID, uint32(held)); err != nil {
			return err
		}
		w.omenHeldRun, w.omenHeldReady = uint32(held), true
		log.Printf("omen state: diagnostic -omen-hold pinned dungeon %d to stage %d", dungeonID, held)
		return nil
	}

	store := w.omenStore()
	if store == nil {
		return fmt.Errorf("omen state read: no store (is the world service wired?)")
	}
	ctx, cancel := context.WithTimeout(context.Background(), omenStateTimeout)
	defer cancel()
	st, err := store.OmenState(ctx, w.role.ID, int64(dungeonID))
	if err != nil {
		return fmt.Errorf("omen state read: %w", err)
	}
	if st.Held > stages-1 {
		st.Held = stages - 1
	}
	w.omenHeldRun, w.omenOrthaierDue, w.omenHeldReady = uint32(st.Held), st.OrthaierPending, true
	return nil
}

// saveOmenHeld 把持有档数写回存档。
func (w *worldSession) saveOmenHeld(dungeonID uint32, held uint32) error {
	store := w.omenStore()
	if store == nil {
		return fmt.Errorf("omen held save: no store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), omenStateTimeout)
	defer cancel()
	if err := store.SaveOmenHeld(ctx, w.role.ID, int64(dungeonID), int(held)); err != nil {
		return fmt.Errorf("omen held save: %w", err)
	}
	return nil
}

// saveOmenOrthaier 置 / 清「下一场该出隐藏 BOSS」。
func (w *worldSession) saveOmenOrthaier(dungeonID uint32, pending bool) error {
	store := w.omenStore()
	if store == nil {
		return fmt.Errorf("omen orthaire save: no store")
	}
	ctx, cancel := context.WithTimeout(context.Background(), omenStateTimeout)
	defer cancel()
	if err := store.SetOmenOrthaierPending(ctx, w.role.ID, int64(dungeonID), pending); err != nil {
		return fmt.Errorf("omen orthaire save: %w", err)
	}
	return nil
}

// omenSettlementIsFull 判断这次结算是不是「集齐四档」那一次。
//
// 判据是**结算前**的持有数等于最后一行的行号：官方说的「激活四档时通关直接结算」
// 只有这一种情形，而它同时是四档里唯一 100% 必给的（见 internal/loot/omen.go）。
// 抽成纯函数是为了能单独测 —— 隐藏 BOSS 的触发条件不该只能靠打一局来验证。
func omenSettlementIsFull(outcome loot.OmenOutcome, stages int) bool {
	return outcome.Paid && stages > 0 && int(outcome.Held) == stages-1
}

// noteOmenSettlement 在一次征兆结算之后落库。
//
// 持有数一律写回；**满档**结算额外置「下一场该出隐藏 BOSS」。判据是结算前的持有数
// 等于最后一行的行号 —— 那正是官方说的「激活四档时通关直接结算」，也是唯一一个
// 100% 必给的档（见 internal/loot/omen.go）。
func (w *worldSession) noteOmenSettlement(outcome loot.OmenOutcome) error {
	dungeonID := outcome.Dungeon
	if !w.omenStateEnabled(dungeonID) {
		return nil
	}
	if err := w.saveOmenHeld(dungeonID, outcome.After); err != nil {
		return err
	}
	full := omenSettlementIsFull(outcome, w.omenStagesCount(dungeonID))
	if full {
		if err := w.saveOmenOrthaier(dungeonID, true); err != nil {
			return err
		}
		log.Printf("omen state: full settlement on dungeon %d (stages %v) -> hidden boss pending",
			dungeonID, outcome.PaidStages)
	}
	// 内存账本已经往前走了，会话里的快照跟着走，免得同一场里再读一次旧值。
	w.omenHeldRun = outcome.After
	return nil
}

// clearOmenOrthaier 在**通关确认之后**清掉「下一场该出隐藏 BOSS」。
//
// 时机与 oath_progress.go 的归零一致：进本时就清，会让掉线或退出吞掉已经攒到的
// 那一次奥尔泰尔。
func (w *worldSession) clearOmenOrthaier(dungeonID uint32) error {
	if !w.omenState || !w.omenOrthaierDue {
		return nil
	}
	if err := w.saveOmenOrthaier(dungeonID, false); err != nil {
		return err
	}
	w.omenOrthaierDue = false
	log.Printf("omen state: hidden boss consumed on dungeon %d clear", dungeonID)
	return nil
}
