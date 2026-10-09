package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"fmt"
	"log"
	"time"
)

func (w *worldSession) resetCards() {
	w.cardPlan = nil
	w.cardReceipt = nil
	w.cardScrolled = false
	w.cardLayoutSent = false
	w.cardAutoPickAt = time.Time{}
}
func (w *worldSession) cardsReady() error {
	if w == nil || w.loot == nil || w.activeDungeon == nil || !w.activeDungeon.Completed() || !w.resultSent || w.cardPlan == nil || w.cardPlan.Run != w.activeDungeon.RunID {
		return fmt.Errorf("cards before owned settlement")
	}
	return nil
}
func (w *worldSession) cardSnapshot() []byte {
	index := -1
	if w.cardReceipt != nil {
		index = int(w.cardReceipt.Index)
	}
	p, _ := protocol.CardSelected(index)
	return p
}
func (w *worldSession) cardStage(id uint16, p []byte) ([]outboundPacket, error) {
	if e := w.cardsReady(); e != nil {
		return nil, e
	}
	if len(p) != 0 {
		return nil, fmt.Errorf("card stage has unexpected body")
	}
	if id == 69 {
		return []outboundPacket{{"card_scroll_ack", 1, 69, []byte{1}}}, nil
	}
	if id != 70 || !w.cardScrolled {
		return nil, fmt.Errorf("card layout before scroll")
	}
	return []outboundPacket{{"card_layout_ack", 1, 70, protocol.CardLayout()}}, nil
}
func (w *worldSession) grantFreeCard(index byte) ([]outboundPacket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	role, receipt, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).PickCard(ctx, w.role, w.activeDungeon, *w.cardPlan, index)
	if e != nil {
		return nil, e
	}
	w.role = role
	bag, e := w.items.Bootstrap(workflow.InventoryRole(role))
	if e != nil {
		return nil, e
	}
	w.cardReceipt = &receipt
	w.cardAutoPickAt = time.Time{}
	return []outboundPacket{{"card_inventory_committed", 0, 13, bag}, {"card_selection_ack", 1, 71, w.cardSnapshot()}}, nil
}

// autoPickSettlementCard 在「翻牌布局已发出、玩家一直没选」时替他选**第一张免费牌**。
//
// 与手动选牌及退出结算共用同一领取事务（grantFreeCard(0)）；仅从布局发出开始计时，
// 重复请求不会延长等待，已领过（cardReceipt != nil）就不再动。
//
// 这套机制原本只给黑鸦开（判据写在副本号上），其它副本 —— 蔚蓝号也不例外 ——
// 倒计时结束后服务端什么都不发，客户端就停在「一张都没翻」的状态
// （业主实机 2026-10-04）。翻牌界面本来就是同一套 69/70/71，所以这里放开到
// **所有已结算的副本**；黑鸦那两个事件名保留（有测试与日志在依赖它们）。
func (w *worldSession) autoPickSettlementCard(now time.Time) ([]outboundPacket, error) {
	if w == nil || w.cardAutoPickAt.IsZero() || now.Before(w.cardAutoPickAt) {
		return nil, nil
	}
	if w.activeDungeon == nil || w.cardReceipt != nil || !w.cardLayoutSent || w.cardsReady() != nil {
		w.cardAutoPickAt = time.Time{}
		return nil, nil
	}
	packets, err := w.grantFreeCard(0)
	if err != nil {
		// 未提交则保留奖单，已提交但通知失败则幂等重取；满包不吞奖。
		w.cardAutoPickAt = now.Add(5 * time.Second)
		return nil, err
	}
	if w.activeDungeon.Definition.ID == blackPurgatorySquadDungeon {
		packets[0].Name = "黑鸦自动翻牌背包同步"
		packets[1].Name = "黑鸦自动翻牌选牌确认"
	}
	// 末世录：翻牌结束 → 这时才发终局 State3 启动通关视频（业主口径：
	// 「通关视频应该是在翻牌后才出现」）。
	if moviePackets, _ := w.readyApocalypseClearMovie(); len(moviePackets) > 0 {
		packets = append(packets, moviePackets...)
	}

	return packets, nil
}
func (w *worldSession) cardPick(p []byte) ([]outboundPacket, error) {
	if e := w.cardsReady(); e != nil {
		return nil, e
	}
	r, e := protocol.DecodeCardSelection(p)
	if e != nil {
		return nil, e
	}
	if !w.cardLayoutSent {
		return nil, fmt.Errorf("card choice before reveal")
	}
	if r.Side != 0 {
		return nil, fmt.Errorf("paid card policy is not enabled")
	}
	packets, e := w.grantFreeCard(r.Index)
	if e != nil {
		return nil, e
	}
	// 末世录：玩家点牌 = 翻牌结束 → 发终局 State3 启动通关视频。
	if moviePackets, _ := w.readyApocalypseClearMovie(); len(moviePackets) > 0 {
		packets = append(packets, moviePackets...)
	}

	return packets, nil
}
func (w *worldSession) settlementExit(p []byte) (*dungeon.Session, []outboundPacket, error) {
	r, e := protocol.DecodeSettlementExit(p)
	if e != nil {
		return nil, nil, e
	}
	// [EPLP-SEAMLESS-UNSETTLED] 小深渊现在不发 NOTI31、只发 NOTI261（见
	// dungeon_flow.go 的通关分支），所以客户端发来的 CMD72 `01 05 01`
	// 到达时**没有**已提交的翻牌事务，cardsReady() 必然失败。
	//
	// 2026-10-07 18:11 实机（会话 `_181140_`）证明：选项 5 只回「focus ACK」而
	// 不驱动入场时，客户端会退回它自己的直进路径、自己补发
	// `CMD2062 DUNGEON_DIRECT_MOVE`，我们再按 2062 回一套「NOTI15 + NOTI27 +
	// 入场序列」—— 结果声音与技能都正常，**画面却停在上一次清图那一帧**。
	//
	// 对照黄金样本 `_170534_` 的客户端 trace：那一次**整场没有 CMD2062**，
	// 入场完全由 CMD72 的应答驱动，模块序列
	// `MAIN_GAME(3) -> SELECT_DUNGEON(2) -> MAIN_GAME(3)` + `Enter Dungeon`，
	// 与本次逐条同形（连 VMem 增量模式都一样）。
	// ⇒ 定案：选项 5 必须**由应答当场驱动入场**，不能让客户端自己走 2062。
	unsettledSeamless := r.Option == protocol.SettlementExitSeamless && w.cardsReady() != nil
	// [ISPINS-ARENA-BOSS] 伊斯大陆的 CMD72 全部不走通用翻牌/结算：官服 s4
	// 整场没有一帧 69/70/71（阶段奖励由 N2256/N2252 承载），撤退休退
	// （source=0）更发生在副本未完成时。回城复用 leaveDungeon（回到进本前
	// 位置=待机区），弃用 42 回执、只发 ACK72；主循环按 settlement_exit_ack
	// 清 activeDungeon。不保持选图（option 1 在军团流程没有「选图」语义）。
	// 撤退不算通关：run.cleared 不动，玩家重新 CMD2043 开战时官服语义本就
	// 是整场作废重开。
	if w.ispins != nil && w.activeDungeon != nil {
		ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
		if r.State == 2 {
			// Focus only updates the button selection. Naming this as an exit
			// triggers main.go's post-send activeDungeon/card cleanup, so the
			// subsequent state1 request loses its owned Ispins settlement.
			return nil, []outboundPacket{ack}, nil
		}
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, nil, e
		}
		w.selectingDungeon = false
		ack.Name = "settlement_exit_ack"
		if w.ispins.finalDone && w.ispins.storyFinished {
			w.ispinsRepeatPending = true
			w.ispins = nil
		}
		return nil, append([]outboundPacket{ack}, route[1:]...), nil
	}
	// 维纳斯阶段本的 CMD72 全部不走通用翻牌/结算。形状：ACK72 后走
	// leaveDungeon 回维纳斯待机区，主循环按 settlement_exit_ack 清
	// activeDungeon。run 复位为全新未选状态（不再作废——2026-10-04 13:33
	// 实机：作废后客户端带着已选难度残留，回城自动弹难度窗、进入地下城/
	// 更改难度全被静默拒绝，作战窗口状态机卡死），序列末尾垫等待态 N2655
	// 让客户端作战窗口从「未选难度」重启。DGN 声明 [no giveup panalty]，
	// 无惩罚语义只影响未实现的次数账本。
	if w.venus != nil && w.activeDungeon != nil && legion.IsVenusStageDungeon(w.activeDungeon.Definition.ID) {
		ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
		if r.State == 2 {
			// Focus only updates the button selection; keep the run owned.
			return nil, []outboundPacket{ack}, nil
		}
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, nil, e
		}
		w.selectingDungeon = false
		ack.Name = "settlement_exit_ack"
		if w.venus.finalDone {
			// 通关视频后的返回城镇：run 已结束——末尾发关闭态 N2655（State0，
			// 兜底保证右上角面板与遗物显示消失；视频恢复时的 leave 态已发过）
			// 并作废 run：Open 不得再进图，重新开团走门（CMD2043）。
			closed := outboundPacket{"venus_info_closed", 0, legion.NotiVenusInfo, legion.VenusClosedInfo()}
			w.venus = nil
			return nil, append(append([]outboundPacket{ack}, route[1:]...), closed), nil
		}
		// BUG3（第二十二轮）：撤退不清进度——保留 cleared/stage，回待机区
		// 重新选难度开战后从撤退的下一关继续（用户口径）。第三十三轮起难度
		// 与遗物一并保留（2290 规格：官服 C72 返回等待区「保留原run、难度
		// 及遗物」）：waiting 向量带权威已选难度（VenusChosenInfo），客户端
		// 按钮意图 getter 按 Choice≠FF 直接进「变更提示」（已选择X。确定要
		// 进入吗？，变更难度按钮置灰）——不再弹三卡片自由重选窗，重选低档
		// 造成的终点回退类状态分裂（172342 会话「stage 3 beyond endpoint」）
		// 从根上不可能发生。
		kept := w.venus.clearedCount()
		choice := w.venus.choice
		relicMask := w.venus.relicMask
		w.venus.resetRun()
		w.venus.choice = choice
		w.venus.relicMask = relicMask
		w.venus.entered = true
		for i := 0; i < kept; i++ {
			w.venus.cleared[i] = true
		}
		trailing := legion.VenusReopenInfo(kept)
		if choice != 0xff {
			trailing = legion.VenusChosenInfo(choice, kept)
		}
		plan := append(append([]outboundPacket{ack}, route[1:]...),
			outboundPacket{"venus_info_waiting", 0, legion.NotiVenusInfo, trailing})
		return nil, plan, nil
	}
	// 苏醒之森阶段本的 CMD72 不走通用翻牌/结算（维纳斯同款形状）。演出后
	// 退场作废 run；未终局的退场（中途 ESC）保持 run（cleared 保留，重进
	// 按已清关序列继续）。
	//
	// ★ 2026-10-09（业主实机）：Extreme 终局演出结束后**由玩家点右上角
	// 「返回城镇」**（官服 22:08:33.821 c2s CMD72 → 22:08:34.081 ACK），
	// 而那时副本会话已在 forestResult（CMD46）收尾（activeDungeon 为空）。
	// 只认 activeDungeon != nil 会让这一请求落到通用路径被 cardsReady 拒成
	// 「cards before owned settlement」，按钮永远没反应 —— 与末世录
	// 2026-10-08 那个「点击返回城镇没有任何反应」是同一类缺口。所以终局
	// （forest.finalDone）也在这条分支里放行。
	forestStageOpen := w.forest != nil && w.activeDungeon != nil && legion.IsForestStageDungeonAny(w.activeDungeon.Definition.ID)
	forestFinale := w.forest != nil && w.forest.finalDone
	if w.forest != nil && (forestStageOpen || forestFinale) {
		ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
		if r.State == 2 {
			return nil, []outboundPacket{ack}, nil
		}
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, nil, e
		}
		w.selectingDungeon = false
		ack.Name = "settlement_exit_ack"
		plan := append([]outboundPacket{ack}, route[1:]...)
		if w.forest.finalDone {
			// 终局（视频播完点「返回城镇」）：本局结束，作废 run。右上角面板由
			// 随后离队时的 N2565 state0e 收起（官服 22:08:45.691）。
			w.forest = nil
			return nil, plan, nil
		}
		// 中途撤退：进度保留（cleared 留在 run 里），但右上角倒计时必须重置
		//（业主 2026-10-09：「点击撤退出去…右上角的倒计时没有刷新」）。
		if stage := w.forestStageOfActiveRun(); stage >= 0 {
			plan = append(plan, w.forestStageTimerReset(time.Now(), stage, "retreat")...)
		}
		return nil, plan, nil
	}
	// 末世录阶段本的 CMD72 同样不走通用翻牌/结算。
	//
	// 实机 BUG（2026-10-08 15:40，业主报「点击返回城镇没有任何反应」）：
	// 全清翻牌之后客户端发 CMD72（`01 02 01 00…` → State1/Option2=返回城镇），
	// 此前没有这一分支，落到通用路径被 cardsReady() 拒成
	// `dungeon_request_refused reason="cards before owned settlement"`，
	// 客户端只收到 ErrCode 4，于是按钮永远没反应。
	//
	// ★ 未完成的撤退**不是取消**（规格：0072-结算离场.md「已推翻」段 G0452 /
	// 2895-LEGIONINFO末世录状态.md G0452 / 2045-LEGIONENTERDUNGEON.md G0452）：
	//
	//	「末世录119的未完成C72撤退不再等价于cancel；保留同一在线作战，回城重建后
	//	 恢复N2895 State2和原阶段，C2045继续当前Boss。主动C2044放弃才走取消。」
	//	「已暂停的同一作战走ResumeApocalypse：阶段须等于保存值，全员回城写入完成
	//	 才重建当前未完成Boss，保留RunID、难度与已清路线。」
	//
	// 所以这里走**挂起**路径：Ticket(ID)/Choice/Stage/Cleared 全部保留，
	// 只置 Suspended；回城后恢复 State2 + 原 Choice/Stage（run 恢复），
	// 玩家再点开始时用 CMD2045 带着保存阶段续关。
	// （此前的实现照抄维纳斯做了 run.Reset()，等于把进度清成 0 —— 那正是
	//   2 号「撤退后 UI 消失、只能从第 1 关重开」这个最大 BUG 的成因。）
	if w.apocalypse != nil && w.activeDungeon != nil && legion.IsApocalypseStageDungeon(w.activeDungeon.Definition.ID) {
		ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
		if r.State == 2 {
			// 焦点帧只更新按钮选中态，不能当成退场（当成退场会让主循环
			// 清掉 activeDungeon，随后 state1 的请求就失去自己的结算上下文）。
			return nil, []outboundPacket{ack}, nil
		}
		route, e := w.leaveDungeon()
		if e != nil {
			return nil, nil, e
		}
		w.selectingDungeon = false
		ack.Name = "settlement_exit_ack"
		run := w.apocalypse
		plan := append([]outboundPacket{ack}, route[1:]...)
		if run.Rewarded {
			// 已领奖的退场：这一局确实结束了，作废 run 并补一帧 N2895 **关闭**
			// 右上角面板。
			//
			// ★ 2026-10-08 修正：这里原本发的是**等待态 State2**（Choice=FF、
			// Stage=0），那是「准备开下一局」的形态，客户端因此**保留**军团面板
			// —— 业主报「翻牌结束后右上角 UI 还在，退出也有 UI 残留」就是这个
			// 成因。规格 2895-LEGIONINFO末世录状态.md / 0072-结算离场.md 的
			// G0452 写得很清楚：
			//
			//	「末世录未完成撤退**先 State0 清旧副本/界面**，再在本人城镇重建后
			//	  恢复 State2 和原 Choice/Stage/目标通关标记」
			//
			// 也就是 **State0 才是「收起军团窗口」的那一帧**（14069ABF0 的语义）。
			// 通关后这一局已经结束、不需要恢复，所以直接停在 State0。
			// 通关关闭态的形态按 2 号权威抓包 idx=654 构造（State0 + 保留
			// 本次通关的阶段记录），不能发「全 ff 的清空态」——那会连阶段记录
			// 一起抹掉，与抓包不符。
			cleared := run.Stage - 1
			run.Reset()
			w.apocalypsePending = nil
			w.apocalypseAdvancePending = nil
			// ★ choice 必须归 **FF**（维纳斯 `VenusClosedInfo` 同款）：
			// 保留难度会让 NPC 继续挂出「开始作战」——业主 2026-10-08 报
			// 「退出没有残留，但是 NPC 头上又出现了开始」，而 2 号也有同样的
			// BUG。规格 2895 G0376：「State0（14069ABF0 在该值关闭军团窗口并
			// **清除选择标记**）」，所以这一帧要的是「未选难度 + 关闭」。
			plan = append(plan, outboundPacket{
				"apocalypse_completed_ui_cleared", 0, legion.NotiLegionInfo,
				legion.ApocalypseInfo(legion.ApocalypseCompletedInfo(0xff, cleared)),
			})
			return nil, plan, nil
		}
		// 未完成的撤退：挂起同一作战。先按 State0 清旧副本/界面（规格 G0452：
		// 「未完成撤退先State0清旧副本/界面，再在本人城镇重建后恢复State2」），
		// 然后立刻恢复 State2 + 原阶段 —— 也就是客户端等价于「原状态但不在图里」，
		// 右上角面板保留，重新点开始就走 2045 续关。
		run.Suspended = true
		w.apocalypsePending = nil
		w.apocalypseAdvancePending = nil
		plan = append(plan,
			outboundPacket{"apocalypse_retreat_cleared", 0, legion.NotiLegionInfo, legion.ApocalypseInfo(legion.ApocalypseClosedInfo())},
			outboundPacket{"apocalypse_retreat_restored", 0, legion.NotiLegionInfo, w.legion.apocalypseInfo()},
		)
		return nil, plan, nil
	}
	if !unsettledSeamless {
		if e = w.cardsReady(); e != nil {
			return nil, nil, e
		}
	}
	ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
	if r.State == 2 {
		return nil, []outboundPacket{ack}, nil
	}
	// Routing follows the decoded request, independently of the ACK envelope.
	w.selectingDungeon = r.Option == 1
	// Preflight routing before granting an automatic unclaimed free card.
	var pending *dungeon.Session
	var route []outboundPacket
	switch r.Option {
	case 0:
		// dstr 479 "Restart the dungeon.": reopen the run that was just
		// settled, behind the dungeon-select head. See restartDungeon.
		pending, route, e = w.restartDungeon()
	case 1:
		copy := *w
		copy.activeDungeon = nil
		route, e = copy.dungeonGate(make([]byte, 8))
	case protocol.SettlementExitSeamless:
		// 无缝续刷（CMD72 选项 5）。复用 restartDungeon —— 它已经是
		// 「ACK15 + NOTI27 + 入场序列」的形状，正是这份修复需要的顺序
		// （两份外部文档在「要不要发 NOTI27」上互相矛盾，较晚的那份明确纠正
		// 了较早的「省略 NOTI27」，理由是该清理函数同时负责卸载
		// onExitModule_SeamlessLoading 的旧地图状态）。
		// ACK 里的 option 原样保留 5（SettlementExitSuccess 用 r.Option），
		// 客户端据此走无缝重置路径而不是普通重开。
		//
		// 准入：只有**已通关**的那一次可以借它续刷；未通关时这个选项
		// 不该出现，出现也拒绝，避免把未完成的挑战重开成新一轮。
		if w.activeDungeon == nil || !w.activeDungeon.Completed() {
			return nil, nil, fmt.Errorf("seamless rechallenge before a committed clear")
		}
		// ⚠️ 2026-10-07 18:44 实机**证伪**：小深渊一度改走「原地重置」（`seamlessRoomReset` ——
		// 只发 ACK + NOTI28 + NOTI29 + 两条线档位 + 疲劳、不切模块、不重喂角色状态），
		// 客户端收到 `ENUM_NOTIPACKET_START_MAP` 之后**立刻 0xC0000005**
		//（client.log `exit=0xC0000005`；trace 最后三帧就是 DUNGEON_INFO / START_MAP / FATIGUE）。
		// ⇒ 客户端必须先把「加载状态」建起来（NOTI27 切模块那一段，也就是屏幕上闪过的那一帧
		// map select）才吃得下 START_MAP。这条捷径不再走；`seamlessRoomReset` 留在
		// dungeon_flow.go 里只作取证，**不要接线**。
		log.Printf("seamless rechallenge drives the entry itself: dungeon=%d state=%d settled=%v", w.activeDungeon.Definition.ID, r.State, !unsettledSeamless)
		// 无缝续刷：让进图序列跳过 NOTI2（角色对象重建）—— buff/召唤物要延续。
		// 置位后**不在这里清**：进图序列要跳过 NOTI2，CMD37 那一步还要跳过 1361
		//（增益强化注册 = 重绑 buff），两处都靠它。
		w.seamlessRetry = true
		pending, route, e = w.restartDungeon()
	case 2, 3:
		// Current scenario option3 is "Start Next Quest". A town objective
		// returns to the owned town; preserve the active quest for NPC handling.
		route, e = w.leaveDungeon()
		if e == nil {
			route = route[1:]
		}
	}
	if e != nil {
		return nil, nil, e
	}
	var plan []outboundPacket
	// 无结算的无缝续刷没有翻牌事务（cardPlan == nil），补发免费牌会直接报错。
	if !unsettledSeamless && w.cardReceipt == nil {
		plan, e = w.grantFreeCard(0)
		if e != nil {
			if w.activeDungeon.Definition.ID != blackPurgatorySquadDungeon || (r.Option != 2 && r.Option != 3) {
				return nil, nil, e
			}
			// 黑鸦奖单已在通关事务落盘，未领取时由重登恢复。满包或暂时
			// 无法发奖不能阻断回城；不发送领取成功，也不删除待领奖单。
			log.Printf("黑鸦翻牌暂未领取，保留奖单等待重登补发：角色=%d 挑战=%s 错误=%v", w.role.ID, w.cardPlan.Run, e)
			plan = nil
		}
	}
	ack.Name = "settlement_exit_ack"
	plan = append(plan, ack)
	plan = append(plan, route...)
	return pending, plan, nil
}

// restartDungeon reopens the run that has just been settled. The settlement
// panel's option 0 is dstr 479 "Restart the dungeon." - the same map as a
// fresh run, not a return to town (option 2 is dstr 481, "Return to town.").
//
// The client is still sitting on the settlement panel when it sends CMD 72,
// and the native handler has already torn the instance module down; an entry
// sequence pushed straight back arrives at a dismantled scene and takes the
// client out with 0xC0000005 (live 20260922T193325, the only option 0 of that
// session). The post-clear "next story dungeon" gate (CMD 2062) hit exactly
// this and solved it by raising the client to the dungeon-select UI first -
// ACK 15 + NOTI 27 - and only then replaying the CMD 16 entry sequence. This
// reuses that shape with the finished run's own id and maze quest, so "again"
// is a direct reopen and never routes through the town the way leaveDungeon
// does.
func (w *worldSession) restartDungeon() (*dungeon.Session, []outboundPacket, error) {
	if w == nil || w.dungeons == nil || w.role.ID == 0 {
		return nil, nil, fmt.Errorf("dungeon catalog or character unavailable")
	}
	old := w.activeDungeon
	if old == nil {
		return nil, nil, fmt.Errorf("retry without an active dungeon")
	}
	if old.Definition.ID == blackPurgatorySquadDungeon {
		return nil, nil, fmt.Errorf("黑鸦挑战结束，请返回大厅重新创建队伍")
	}
	if old.Definition.Tower != nil {
		return nil, nil, fmt.Errorf("%s tower does not allow settlement retry", old.Definition.Tower.Key)
	}
	copy := *w
	copy.activeDungeon = nil
	sel := protocol.DungeonSelection{ID: old.Definition.ID, Party: 65535, Quest: uint32(old.Maze.Quest)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The same entry gate the ordinary selection applies, minus its town-only
	// check: the character is inside a run, so there is no PVF [dungeon gate]
	// area under its feet to stand on.
	if w.fatigue != nil && !old.Definition.NoFatigue && w.fatigue.EnterCostFor(old.Definition.ID) > 0 {
		fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if fp.Used >= fp.Limit {
			return nil, nil, database.ErrFatigueExhausted
		}
	}
	accepted, e := copy.acceptedQuestIDs(ctx)
	if e != nil {
		return nil, nil, e
	}
	s, e := dungeon.Select(*copy.dungeons, sel, copy.level, accepted)
	if e != nil {
		return nil, nil, e
	}
	entry, e := copy.dungeonEntryPlan(context.Background(), "dungeon_select_ack", 16, sel, s)
	if e != nil {
		return nil, nil, e
	}
	// ⚠️ 2026-10-07 19:14 实机**证伪**：无缝续刷曾试过「去掉 NOTI27、只留 NOTI15 门应答」
	// （`seamlessSelectionHead`），客户端再次 0xC0000005。
	// ⇒ **NOTI27 是必需的加载握手**：切模块那一步（屏幕上那一帧 map select）就是客户端
	// 建立加载状态的过程，`START_MAP` 必须有它在前。`seamlessSelectionHead` 只作取证，不要接线。
	// 无缝续刷（选项 5）要用 NOTI27 的「继续挑战」形态：头字节 `relay` = 1。
	// 官服抓包里同一个副本冷进场是 0x00、两次继续都是 0x01（next178 §3），
	// 而本仓此前恒为 0（客户端因此一直走「新副本」那条加载路径）。
	// 选项 0（普通重开）保持 relay = 0。
	return s, append(dungeonSelectionHeadFor(copy.dungeonRelayFlag() != 0), entry...), nil
}
