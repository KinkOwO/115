package main

import (
	"bytes"
	"dfolan/internal/game/wire"
	"dfolan/internal/loot"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestApocalypsePickupWindowDefersWithoutBlockingAndCommitsAfterDelivery(t *testing.T) {
	for _, broken := range []bool{false, true} {
		s, w, body := combatProgressFixture(t)
		a, peer := net.Pipe()
		t.Cleanup(func() { a.Close(); peer.Close() })
		log := func(map[string]any) {}
		c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: log, output: newConnectionOutput(a, make([]byte, wire.SessionKeyBytes), "test", log)}
		t.Cleanup(c.cancelApocalypsePortal)
		now := time.Now()
		c.noteApocalypsePickupWindow(now)
		deadline := c.apocalypsePickup.Deadline
		if !deadline.Equal(now.Add(5 * time.Second)) {
			t.Fatal("pickup deadline is not five seconds from death")
		}
		c.noteApocalypsePickupWindow(now.Add(time.Second))
		if !c.apocalypsePickup.Deadline.Equal(deadline) {
			t.Fatal("duplicate death extends wait")
		}
		old, route := w.activeDungeon, w.apocalypseRun
		drops := &loot.Session{}
		w.drops = drops
		// No network reader is active: immediate return proves this does not
		// sleep or write a loading frame in the request/pickup loop.
		if c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 2062}, verified: true, plaintext: body}) != dispatchHandled || c.apocalypsePendingPortal == nil {
			t.Fatal("validated portal was not queued")
		}
		timer := c.apocalypsePortalTimer
		if !c.deferApocalypsePortal(body, now.Add(time.Second)) || c.apocalypsePortalTimer != timer {
			t.Fatal("duplicate portal replaced timer")
		}
		if err := c.releaseApocalypsePortal(deadline.Add(-time.Nanosecond)); err != nil || w.activeDungeon != old || w.apocalypseRun != route {
			t.Fatal("early scene change", err)
		}
		if w.drops != drops {
			t.Fatal("pickup window lost outstanding drops")
		}
		// Inventory can change while waiting; the delayed preparation must use
		// the current actor snapshot and preserve it through the scene change.
		w.role.State = []byte(`{"future":{"picked_up":true}}`)
		latest := append([]byte(nil), w.role.State...)
		prepared, err := c.legionState.prepareCombatPortal(w, body)
		if err != nil {
			t.Fatal(err)
		}
		peer.SetDeadline(time.Now().Add(5 * time.Second))
		done := make(chan error, 1)
		go func() { done <- c.releaseApocalypsePortal(deadline) }()
		for i, packet := range prepared.Packets {
			if broken && i == len(prepared.Packets)-1 {
				peer.Close()
				break
			}
			frame := readDispatchServerFrame(t, peer)
			if frame[0] != packet.Kind || binary.LittleEndian.Uint16(frame[1:]) != packet.ID {
				t.Fatal("wrong delayed load frame")
			}
		}
		err = <-done
		if broken {
			if err == nil || w.activeDungeon != old || w.apocalypseRun != route || w.drops != drops {
				t.Fatal("partial write adopted new scene")
			}
		} else if err != nil || w.activeDungeon == old || w.apocalypseRun.Stage != route.Stage+1 || !bytes.Equal(w.role.State, latest) || w.drops != nil {
			t.Fatal("delayed transition lost scene or actor snapshot", err)
		}
		if c.apocalypsePendingPortal != nil || c.apocalypsePortalTimer != nil {
			t.Fatal("timer still active after delivery")
		}
	}
}

func TestApocalypsePickupWindowDiscardsChangedOwnerAndSource(t *testing.T) {
	changes := []func(*gameConnection){
		func(c *gameConnection) { c.worldState.activeDungeon = nil },
		func(c *gameConnection) { c.worldState.role.ID++ },
		func(c *gameConnection) { c.worldState.activeDungeon.RunID += "new" },
		func(c *gameConnection) { c.worldState.apocalypseRun.Stage++ },
		func(c *gameConnection) { c.worldState.apocalypseRun.CatalogHash += "new" },
		func(c *gameConnection) { c.legionState.catalog.SHA256 += "new" },
	}
	for i, change := range changes {
		s, w, body := combatProgressFixture(t)
		c := &gameConnection{worldState: w, legionState: *s, event: func(map[string]any) {}}
		now := time.Now()
		c.noteApocalypsePickupWindow(now)
		if !c.deferApocalypsePortal(body, now) {
			t.Fatal("no queued portal")
		}
		change(c)
		if err := c.releaseApocalypsePortal(now.Add(5 * time.Second)); err != nil || c.apocalypsePortalTimer != nil || c.apocalypsePendingPortal != nil {
			t.Fatal("stale timer not discarded", i, err)
		}
		// No output connection exists: any attempted stale frame would panic.
	}
}

func TestApocalypsePickupWindowRejectsLivingBossAndMalformedPortal(t *testing.T) {
	s, w, body := combatProgressFixture(t)
	c := &gameConnection{worldState: w, legionState: *s, bootstrapped: true, event: func(map[string]any) {}}
	now := time.Now()
	w.deathSent[4096] = false
	c.noteApocalypsePickupWindow(now)
	if c.deferApocalypsePortal(body, now) {
		t.Fatal("undelivered death opened pickup window")
	}
	w.deathSent[4096] = true
	c.noteApocalypsePickupWindow(now)
	body[13]++
	if c.dispatchLegion(&clientRequest{frame: wire.Frame{Type: 1, ID: 2062}, verified: true, plaintext: body}) != dispatchHandled || c.apocalypsePendingPortal != nil {
		t.Fatal("malformed portal queued")
	}
}
