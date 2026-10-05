package main

import (
	"context"
	"dfolan/internal/workflow"
	"log"
	"time"
)

func (w *worldSession) flushBoostGraduationMail() {
	if w == nil || w.loot == nil || w.boostup == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lsvc := &workflow.LootService{Store: w.store, Loot: w.loot}
	settledAny := false
	for _, bonus := range []bool{false, true} {
		deliver := lsvc.DeliverBoostGraduationMail
		if bonus {
			deliver = lsvc.DeliverBoostLevelBonusMail
		}
		next, settled, err := deliver(ctx, w.role, w.boostup)
		if next.ID == w.role.ID {
			next.WireID = w.role.WireID
			w.role = next
		}
		settledAny = settledAny || settled
		if err != nil {
			log.Printf("boost graduation mail pending role=%d level_bonus=%v: %v", w.role.ID, bonus, err)
		}
	}
	if settledAny && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
}
