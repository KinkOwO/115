package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"errors"
	"testing"
	"time"
)

func robotDeathFixture(t *testing.T, coins uint32) (*worldSession, *ceraReviveFake, *townRobotTransport, []byte) {
	t.Helper()
	w, s, p := ceraReviveFixture(t, coins)
	tr := nativeAITestTransport(t)
	tr.ownerActor = w.role.WireID
	tr.nativeAI.phase = 2
	tr.nativeAI.lastAck = time.Now()
	w.hub = newLanHub()
	w.peer = &lanPeer{}
	w.hub.townRobots = &townRobotFleet{parties: map[*lanPeer]*townRobotParty{w.peer: {owner: w.peer, dungeon: true, transport: tr}}}
	binary.LittleEndian.PutUint16(p, tr.peers[0].actor)
	return w, s, tr, p
}
func lifeAck(tr *townRobotTransport, now time.Time, seq, mask uint32) []byte {
	b := nativeAIMessage(tr.authorityBody(now), tr.nativeAI.generation, 2)
	binary.LittleEndian.PutUint32(b[76:], 1234)
	ack := append(b[:], make([]byte, 24)...)
	binary.LittleEndian.PutUint64(ack[80:], uint64(now.UnixMilli()))
	binary.LittleEndian.PutUint32(ack[88:], seq)
	binary.LittleEndian.PutUint32(ack[100:], mask)
	return ack
}
func markRobotDead(t *testing.T, tr *townRobotTransport, seq uint32) {
	t.Helper()
	tr.send = func(typ byte, id uint16, body []byte) error {
		if typ != 0 || id != 32 || len(body) != 8 || body[2] != 0 || binary.LittleEndian.Uint16(body) != tr.peers[0].actor {
			t.Fatal("wrong death packet")
		}
		return nil
	}
	now := time.Now()
	tr.observeNativeDeathsLocked(lifeAck(tr, now, seq, 1), now)
	if !tr.nativeAI.deaths[0].notified {
		t.Fatal("death was not notified")
	}
}
func TestRobotDeathNotificationRetryAndStaleRoom(t *testing.T) {
	_, _, tr, _ := robotDeathFixture(t, 2)
	now := time.Now()
	ack := lifeAck(tr, now, 1, 1)
	calls := 0
	fail := true
	tr.send = func(byte, uint16, []byte) error {
		calls++
		if fail {
			return errors.New("closed output")
		}
		return nil
	}
	tr.observeNativeDeathsLocked(ack, now)
	if tr.nativeAI.deaths[0].notified {
		t.Fatal("failed send marked delivered")
	}
	fail = false
	tr.observeNativeDeathsLocked(ack, now)
	tr.observeNativeDeathsLocked(ack, now)
	if calls != 2 {
		t.Fatal("duplicate notification", calls)
	}
	tr.resetNativeAILocked()
	tr.nativeAI.phase = 2
	tr.nativeAI.lastAck = now
	offer := nativeAIMessage(tr.authorityBody(now), tr.nativeAI.generation, 2)
	if _, ok := validNativeAIAck(ack, offer, now); ok {
		t.Fatal("previous room accepted")
	}
	for _, bad := range []struct {
		seq, mask uint32
		sample    time.Time
	}{{1, 8, now}, {2, 1, now}, {1, 1, now.Add(-3 * time.Second)}, {1, 1, now.Add(time.Second)}} {
		tr.observeNativeDeathsLocked(lifeAck(tr, bad.sample, bad.seq, bad.mask), now)
	}
	if tr.nativeAI.deaths[0].death.Dead {
		t.Fatal("invalid game-frame state accepted")
	}
}
func TestRobotReviveChargesOwnerOncePreservesOwnerAndParty(t *testing.T) {
	w, s, tr, p := robotDeathFixture(t, 3)
	markRobotDead(t, tr, 1)
	ownerDeath := w.pilotDeath
	ownerID := w.role.WireID
	generation := tr.nativeAI.generation
	frame := []byte{41, 1}
	plan, handled, err := w.reviveTownRobot(context.Background(), s, p, frame)
	if err != nil || !handled || len(plan) != 3 {
		t.Fatal(plan, handled, err)
	}
	bag, _ := inventory.ReadBag(w.role.State)
	if bag.Coin != 2 || w.role.WireID != ownerID || w.role.ID != 7 || w.pilotDeath != ownerDeath || !ownerDeath.Dead || tr.closed || tr.nativeAI.generation != generation {
		t.Fatal("payer identity/death/party corrupted")
	}
	if plan[0].ID != 41 || plan[1].ID != 32 || binary.LittleEndian.Uint16(plan[1].Payload) != tr.peers[0].actor || binary.LittleEndian.Uint16(plan[1].Payload[4:]) != 1000 {
		t.Fatal("incorrect target restore")
	}
	for _, repeat := range [][]byte{frame, {41, 2}} {
		_, _, _ = w.reviveTownRobot(context.Background(), s, p, repeat)
		bag, _ = inventory.ReadBag(w.role.State)
		if bag.Coin != 2 {
			t.Fatal("double charge")
		}
	}
	now := time.Now()
	tr.observeNativeDeathsLocked(lifeAck(tr, now, 1, 1), now)
	if tr.nativeAI.deaths[0].death.Dead {
		t.Fatal("old dead heartbeat undid revive")
	}
	markRobotDead(t, tr, 2)
	if _, _, err = w.reviveTownRobot(context.Background(), s, p, []byte{41, 3}); err != nil {
		t.Fatal(err)
	}
	bag, _ = inventory.ReadBag(w.role.State)
	if bag.Coin != 1 {
		t.Fatal("next death not charged once")
	}
	if len(s.keys) != 2 {
		t.Fatal("death transaction key collided", s.keys)
	}
	for key := range s.keys {
		if key == "dungeon-life-token-revive:run-cera:3" {
			t.Fatal("player death key reused")
		}
	}
}
func TestRobotReviveCurrencyAndRulesFailuresKeepDead(t *testing.T) {
	for _, mode := range []string{"cera", "empty", "forbidden", "stale", "foreign", "padding", "alive", "loading", "settled"} {
		t.Run(mode, func(t *testing.T) {
			w, s, tr, p := robotDeathFixture(t, 0)
			markRobotDead(t, tr, 1)
			switch mode {
			case "empty":
				s.balance = 14
			case "forbidden":
				w.dungeons.Maps[1] = catalog.ScriptRecord{Cells: []pvf.Token{{Type: 3, Text: "[cannot use coin map]"}}}
			case "stale":
				tr.nativeAI.lastAck = time.Now().Add(-3 * time.Second)
			case "foreign":
				binary.LittleEndian.PutUint16(p, 123)
			case "padding":
				p[7] = 1
			case "alive":
				tr.nativeAI.deaths[0].death.Dead = false
			case "loading":
				tr.roomLoading = true
			case "settled":
				w.resultSent = true
			}
			before := s.balance
			plan, handled, err := w.reviveTownRobot(context.Background(), s, p, []byte{41, 3})
			if !handled {
				t.Fatal("foreign target fell through")
			}
			if mode == "cera" {
				if err != nil || len(plan) != 3 || s.balance != 15 || len(s.grants) != 1 || s.grants[0].AccountID != 11 {
					t.Fatal(plan, err, s.grants)
				}
			} else if err == nil || len(plan) != 0 || s.balance != before {
				t.Fatal("refusal mutated currency", mode, plan, err)
			}
		})
	}
}
func TestRobotReviveDoesNotCaptureOwnerRequest(t *testing.T) {
	w, s, _, _ := robotDeathFixture(t, 1)
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, w.role.WireID)
	if _, handled, err := w.reviveTownRobot(context.Background(), s, p, []byte{41}); handled || err != nil {
		t.Fatal("ordinary revive intercepted")
	}
	if b, _ := protocol.PlayerDeathState(w.role.WireID); len(b) != 8 {
		t.Fatal("protocol fixture")
	}
}
