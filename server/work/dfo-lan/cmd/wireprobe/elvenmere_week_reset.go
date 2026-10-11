package main

import (
	"dfolan/internal/dungeon"
	"time"
)

// Run on the connection's serialized existing timer. A reset terminates the
// old run using the established town-return notifications, without fabricating
// a CMD42 acknowledgement, granting rewards or deleting persisted receipts.
func (w *worldSession) elvenmereWeekReset(now time.Time) ([]outboundPacket, error) {
	if w == nil || w.activeDungeon == nil || w.activeDungeon.Definition.ID != 100003126 || w.activeDungeon.ElvenmereWeekStart == 0 || dungeon.ElvenmereWeekStart(now) == w.activeDungeon.ElvenmereWeekStart {
		return nil, nil
	}
	leave, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	var plan []outboundPacket
	for _, p := range leave {
		if p.Name != "dungeon_leave_ack" {
			plan = append(plan, p)
		}
	}
	w.activeDungeon = nil
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.pilotDeath = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.resetCards()
	return plan, nil
}
