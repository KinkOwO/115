package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
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
		copy := *w
		copy.activeDungeon = nil
		request := make([]byte, 32)
		binary.LittleEndian.PutUint32(request, w.activeDungeon.Definition.ID)
		binary.LittleEndian.PutUint16(request[9:], 65535)
		binary.LittleEndian.PutUint32(request[16:], uint32(w.activeDungeon.Maze.Quest))
		pending, route, e = copy.selectDungeon(request)
	case 1:
		copy := *w
		copy.activeDungeon = nil
		route, e = copy.dungeonGate(make([]byte, 8))
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
