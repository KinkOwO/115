package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestBakalFailedRestartCreatesPreviouslyDefeatedGatekeeper(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Create: protocol.RaidCreateRequest{Kind: 8, Title: "retry"}, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: 12, MemberPosition: 1, MemberArea: 2}
	w.bakalName = w.role.Name
	var id uint32
	for _, spawn := range w.bakal.MonsterPlacements() {
		if spawn.Name == "gerda" {
			for _, loc := range w.bakalRules.Locations {
				if loc.Index == spawn.Location {
					id = loc.Dungeon
				}
			}
		}
	}
	grid := bakalNativeBossGrid(t, w, "gerda")
	if _, e := w.bakalDungeonEntry(id, 0, &grid, at, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	for _, actor := range w.activeDungeon.Monsters {
		if actor.Template == w.bakalRules.Monsters["gerda"].Template {
			w.activeDungeon.Dead[actor.Entity] = true
		}
	}
	if _, e := w.bakalConfirmDefeats(at); e != nil {
		t.Fatal(e)
	}
	if !w.bakal.IsLocationDefeated(w.bakalLocation) {
		t.Fatal("test did not remember first-run defeat")
	}
	w.bakal.Tick(at.Add(time.Duration(w.bakalRules.PhaseTimeOverSecs+1) * time.Second))
	if w.bakal.Stage() != legion.BakalOpeningFailed {
		t.Fatal("source did not fail")
	}
	w.activeDungeon = nil
	w.bakalCurrent = 0
	start, _ := hex.DecodeString("88f25f0000000000feffffffff00010000000000000000000000000000000000")
	at = at.Add(2 * time.Hour)
	if _, _, e := c.bakalStartRaid(start, at); e != nil {
		t.Fatal(e)
	}
	at = at.Add(time.Duration(w.bakalRules.StartDelaySecs) * time.Second)
	w.bakal.Tick(at)
	if _, e := w.bakalDungeonEntry(id, 0, &grid, at, false); e != nil {
		t.Fatal(e)
	}
	_, plan, e := c.bakalLoadingDone(nil)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, p := range plan {
		if p.ID == 2194 && binary.LittleEndian.Uint32(p.Payload[7:]) == w.bakalRules.Monsters["gerda"].Template {
			n++
		}
	}
	if n != 1 || w.bakal.IsLocationDefeated(w.bakalLocation) {
		t.Fatal("new run suppressed old defeated gatekeeper")
	}
	if p, e := w.bakalLoadedBoss(); e != nil || len(p) != 0 {
		t.Fatal("new run spawned duplicate gatekeeper", e)
	}
}

func TestBakalLifecyclePublishesNativeActiveDescriptorAndEnds(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	w.bakalRecruitment = &protocol.RaidRecruitment{ID: 1, Create: protocol.RaidCreateRequest{Kind: 8, Title: "raid"}, Leader: w.role.WireID, LeaderName: w.role.Name, MemberCount: 1, MemberMax: 12, MemberPosition: 1, MemberArea: 2}
	plan, e := w.bakalLifecyclePlan()
	if e != nil || len(plan) != 1 {
		t.Fatal(e)
	}
	body := plan[0].Payload
	off := 16 + len(w.bakalRecruitment.Create.Title)
	if binary.LittleEndian.Uint32(body[4:]) != 2 || body[off] != 8 || body[off+1] != 2 || body[off+2] != 0 || binary.LittleEndian.Uint32(body[off+3:]) != uint32(w.bakal.ActiveSince().Unix()) {
		t.Fatal("native state/phase/start not synchronized")
	}
	if p, e := w.bakalLifecyclePlan(); e != nil || len(p) != 0 {
		t.Fatal("unchanged lifecycle repeated")
	}
	w.bakal.Tick(at.Add(time.Duration(w.bakalRules.PhaseTimeOverSecs+1) * time.Second))
	plan, e = w.bakalLifecyclePlan()
	if e != nil || len(plan) != 1 {
		t.Fatal(e)
	}
	body = plan[0].Payload
	if body[off+1] != 0 || body[off+2] != 255 || binary.LittleEndian.Uint32(body[off+3:]) != 0 {
		t.Fatal("ended raid retained active descriptor")
	}
}

func TestBakalCapturedDefeatedBlonaArenaEntersNormalRoute(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	// 20261007_021400 first run: Blona died 02:19:26; UI entered
	// the empty arena at02:19:43 and stayed there until failure.
	grid := [2]byte{0, 1}
	id := uint32(100003154)
	if _, e := w.bakalDungeonEntry(id, 0, &grid, at, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	for _, actor := range w.activeDungeon.Monsters {
		if actor.Template == w.bakalRules.Monsters["blona"].Template {
			w.activeDungeon.Dead[actor.Entity] = true
		}
	}
	if _, e := w.bakalConfirmDefeats(at); e != nil {
		t.Fatal(e)
	}
	body := make([]byte, 48)
	binary.LittleEndian.PutUint32(body[13:], id)
	binary.LittleEndian.PutUint32(body[25:], 1)
	if handled, _, e := c.enterBakalPortal(2062, body, at); !handled || e != nil {
		t.Fatal("cleared arena return refused", e)
	}
	if [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} != [2]byte{0, 0} {
		t.Fatal("entered cleared arena without death exit")
	}
	if _, p, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	} else {
		for _, row := range p {
			if row.ID == 2194 && binary.LittleEndian.Uint32(row.Payload[7:]) == w.bakalRules.Monsters["blona"].Template {
				t.Fatal("cleared arena fabricated respawn")
			}
		}
	}
}
