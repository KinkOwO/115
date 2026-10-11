package main

import (
	"fmt"
	"time"
)

// User-requested local test policy: five seconds from the delivered boss death,
// not five more seconds after C2062 (native sender has a 5000ms retry timer).
const apocalypsePickupWindow = 5 * time.Second

type apocalypsePickupOwner struct {
	Character   int64
	Run, Source string
	Deadline    time.Time
}
type apocalypsePendingPortal struct {
	Owner apocalypsePickupOwner
	Stage uint32
	Body  []byte
}

func (client *gameConnection) noteApocalypsePickupWindow(now time.Time) {
	w := client.worldState
	if w == nil || w.apocalypseRun == nil || w.activeDungeon == w.apocalypseNavigation {
		return
	}
	r, err := client.legionState.validateRoute(w)
	if err != nil || !apocalypseBossDeathDelivered(w, r) {
		return
	}
	if client.apocalypsePickup.Run == w.activeDungeon.RunID && client.apocalypsePickup.Character == w.role.ID {
		return
	}
	client.apocalypsePickup = apocalypsePickupOwner{Character: w.role.ID, Run: w.activeDungeon.RunID, Source: r.CatalogHash, Deadline: now.Add(apocalypsePickupWindow)}
}

// Called only after the existing portal admission has validated the body.
// Keeping this work in the connection select loop leaves pickup and retreat
// requests available throughout the grace period; no timer mutates run state.
func (client *gameConnection) deferApocalypsePortal(body []byte, now time.Time) bool {
	w := client.worldState
	owner := client.apocalypsePickup
	if w == nil || w.activeDungeon == nil || w.apocalypseRun == nil || owner.Character != w.role.ID || owner.Run != w.activeDungeon.RunID || owner.Source != w.apocalypseRun.CatalogHash || !now.Before(owner.Deadline) {
		return false
	}
	if client.apocalypsePendingPortal != nil {
		return true
	}
	client.apocalypsePendingPortal = &apocalypsePendingPortal{Owner: owner, Stage: w.apocalypseRun.Stage, Body: append([]byte(nil), body...)}
	client.apocalypsePortalTimer = time.NewTimer(owner.Deadline.Sub(now))
	client.event(map[string]any{"kind": "apocalypse_pickup_window_waiting", "run": owner.Run, "wait_millis": owner.Deadline.Sub(now).Milliseconds(), "policy_seconds": 5})
	return true
}

func (client *gameConnection) apocalypsePortalTicks() <-chan time.Time {
	if client.apocalypsePortalTimer == nil {
		return nil
	}
	return client.apocalypsePortalTimer.C
}
func (client *gameConnection) cancelApocalypsePortal() {
	if client.apocalypsePortalTimer != nil {
		client.apocalypsePortalTimer.Stop()
	}
	client.apocalypsePortalTimer = nil
	client.apocalypsePendingPortal = nil
}
func (client *gameConnection) releaseApocalypsePortal(now time.Time) error {
	pending := client.apocalypsePendingPortal
	if pending == nil || now.Before(pending.Owner.Deadline) {
		return nil
	}
	client.cancelApocalypsePortal()
	w := client.worldState
	if w == nil || w.activeDungeon == nil || w.apocalypseRun == nil || w.role.ID != pending.Owner.Character || w.activeDungeon.RunID != pending.Owner.Run || w.apocalypseRun.Stage != pending.Stage || w.apocalypseRun.CatalogHash != pending.Owner.Source {
		client.event(map[string]any{"kind": "apocalypse_stale_pickup_portal_discarded", "run": pending.Owner.Run})
		return nil
	}
	// Rebuild after pickup: use the latest saved inventory and source owner,
	// never a previously prepared loading projection or stale actor snapshot.
	result, err := client.legionState.prepareCombatPortal(w, pending.Body)
	if err != nil {
		client.event(map[string]any{"kind": "apocalypse_delayed_portal_refused", "reason": err.Error()})
		return nil
	}
	if err = client.deliverApocalypseRoute(result); err != nil {
		return fmt.Errorf("delayed apocalypse portal: %w", err)
	}
	client.event(map[string]any{"kind": "apocalypse_pickup_window_completed", "run": pending.Owner.Run, "policy_seconds": 5})
	return nil
}

func (client *gameConnection) deliverApocalypseRoute(result legionResult) error {
	if err := client.sendPlan(result.Packets, client.logCharacterResponseBody); err != nil {
		return err
	}
	w := client.worldState
	w.apocalypseRun = result.Route
	client.legionState.session.Phase = byte(result.Route.Stage)
	client.legionState.adoptProjection(result.Packets)
	if result.Combat != nil {
		client.cancelApocalypsePortal()
		client.apocalypsePickup = apocalypsePickupOwner{}
		w.activeDungeon = result.Combat
		w.deathSent = map[uint16]bool{}
		w.drops = nil
		w.resetCards()
		w.completionSent, w.resultSent = false, false
		w.completionErr = nil
	}
	for _, note := range result.Events {
		client.event(note)
	}
	return nil
}
