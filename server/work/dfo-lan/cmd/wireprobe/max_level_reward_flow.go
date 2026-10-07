package main

import (
	"context"
	"log"
	"time"
)

// deliverMaxLevelRewardAtTown mails the source's etc/titlebook.etc
// [maxlevel reward] box when a capped character is standing in town. The event
// key is the receipt, so every town return retries an undelivered gift and a
// delivered one stays silent. No notification packet is pushed: the mailbox
// alarm reads the durable delivery at login, the same path the Odyssey honor
// box already uses.
func (w *worldSession) deliverMaxLevelRewardAtTown() {
	if w == nil || w.progression == nil || w.progression.MaxLevelReward == nil || w.activeDungeon != nil || w.selectingDungeon {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	role, applied, err := w.progression.MaxLevelRewardMail(ctx, w.role)
	if err != nil {
		log.Printf("满级礼盒待投递：character=%d: %v", w.role.ID, err)
		return
	}
	w.role = role
	if applied {
		log.Printf("满级礼盒已投递：character=%d template=%d", role.ID, w.progression.MaxLevelReward.Template)
	}
}
