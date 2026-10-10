package main

import (
	"dfolan/internal/legion"
	"time"
)

// The native captured dead-player revival offer lasts ten seconds before
// returning to camp; recovery AFTER camp is the separate source REVIVAL TIME.
// This schedule is consumed on the connection's serial tick, never a goroutine.
func (w *worldSession) scheduleBakalDeathReturn(now time.Time) {
	if w.bakal == nil || w.bakal.Stage() != legion.BakalOpeningActive || w.pilotDeath == nil || !w.pilotDeath.Dead || w.activeDungeon == nil || !w.activeDungeon.Loaded || !w.activeDungeon.RaidManaged || w.pilotDeath.Run != w.activeDungeon.RunID {
		return
	}
	d := w.pilotDeath
	if w.bakalDeathReturnRun == d.Run && w.bakalDeathReturnSequence == d.Sequence && !w.bakalDeathReturnAt.IsZero() {
		return
	}
	w.bakalDeathReturnRun, w.bakalDeathReturnSequence = d.Run, d.Sequence
	w.bakalDeathReturnAt = now.Add(deathFailTimeout)
}

func (client *gameConnection) tickBakalDeathReturn(now time.Time) ([]outboundPacket, error) {
	w := client.worldState
	if w.bakalDeathReturnAt.IsZero() {
		return nil, nil
	}
	d := w.pilotDeath
	if w.bakal == nil || w.bakal.Stage() != legion.BakalOpeningActive || w.bakal.NeedsCampReturn() || w.activeDungeon == nil || !w.activeDungeon.RaidManaged || w.activeDungeon.RunID != w.bakalDeathReturnRun || d == nil || !d.Dead || d.Run != w.bakalDeathReturnRun || d.Sequence != w.bakalDeathReturnSequence {
		w.bakalDeathReturnAt = time.Time{}
		return nil, nil
	}
	if now.Before(w.bakalDeathReturnAt) {
		return nil, nil
	}
	plan, err := w.bakalRetreatPlanAt(now, legion.BakalReturnDeath)
	if err == nil {
		client.event(map[string]any{"kind": "bakal_death_return", "raid": w.bakalRun, "recovery_until": w.bakal.RecoveryUntil().Unix()})
	}
	return plan, err
}
