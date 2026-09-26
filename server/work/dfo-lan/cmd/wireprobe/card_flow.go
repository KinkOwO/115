package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

func (w *worldSession) resetCards() {
	w.cardPlan = nil
	w.cardReceipt = nil
	w.cardScrolled = false
	w.cardLayoutSent = false
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
	role, receipt, _, e := w.loot.PickCard(ctx, w.role, w.activeDungeon, *w.cardPlan, index)
	if e != nil {
		return nil, e
	}
	w.role = role
	w.cardReceipt = &receipt
	bag, e := w.loot.Bootstrap(role)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"card_inventory_committed", 0, 13, bag}, {"card_selection_ack", 1, 71, w.cardSnapshot()}}, nil
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
	return w.grantFreeCard(r.Index)
}
func (w *worldSession) settlementExit(p []byte) (*dungeon.Session, []outboundPacket, error) {
	r, e := protocol.DecodeSettlementExit(p)
	if e != nil {
		return nil, nil, e
	}
	if e = w.cardsReady(); e != nil {
		return nil, nil, e
	}
	ack := outboundPacket{"settlement_focus_ack", 1, 72, protocol.SettlementExitSuccess(r)}
	if r.State == 2 {
		return nil, []outboundPacket{ack}, nil
	}
	// The selection flag is set from the decoded request, not read back out of
	// the outgoing acknowledgement. The gateway used to do
	// "worldState.selectingDungeon = p.Payload[2] == 1", which only worked
	// while the ack happened to be three bytes wide; narrowing it to its
	// native two bytes turned that line into index out of range [2] with
	// length 2 and killed the whole process, dropping every connected player.
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
	if w.cardReceipt == nil {
		plan, e = w.grantFreeCard(0)
		if e != nil {
			return nil, nil, e
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
	copy := *w
	copy.activeDungeon = nil
	sel := protocol.DungeonSelection{ID: old.Definition.ID, Party: 65535, Quest: uint32(old.Maze.Quest)}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The same entry gate the ordinary selection applies, minus its town-only
	// check: the character is inside a run, so there is no PVF [dungeon gate]
	// area under its feet to stand on.
	if w.fatigue != nil && !old.Definition.NoFatigue && w.fatigue.Rules.RoomCost > 0 {
		fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
		if err != nil {
			return nil, nil, err
		}
		if fp.Used >= fp.Limit {
			return nil, nil, storage.ErrFatigueExhausted
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
	entry, e := copy.dungeonEntryPlan("dungeon_select_ack", 16, sel, s)
	if e != nil {
		return nil, nil, e
	}
	return s, append(dungeonSelectionHead(), entry...), nil
}
