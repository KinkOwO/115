package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"encoding/binary"
	"encoding/hex"
	"testing"
)

func TestBakalAlivePhaseCinematicLoadsSecondMapOnce(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	for _, d := range w.bakalRules.Dungeons {
		if d.Type == "sparazzi" || d.Type == "skasa" || d.Type == "hisma" {
			loc, e := legionLocationForTest(w, d.Index)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = w.bakal.EnterDungeon(d.Index, loc, at); e != nil {
				t.Fatal(e)
			}
			if e = w.bakal.LoadingDone(d.Index); e != nil {
				t.Fatal(e)
			}
			if _, e = w.bakal.DefeatMonster(d.Index, loc, at); e != nil {
				t.Fatal(e)
			}
		}
	}
	grid := [2]byte{1, 1}
	if _, e := w.bakalDungeonEntry(100003149, 0, &grid, at, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	first := w.bakalRules.Monsters["bakal"].Template
	for _, actor := range w.activeDungeon.Monsters {
		if actor.Template == first && w.activeDungeon.Dead[actor.Entity] {
			t.Fatal("first actor already dead")
		}
	}
	warp, _ := hex.DecodeString("8060aa840100000089909a4501010000000500000001000000ffff7c01c201ffff320032000000000000000000000000")
	_, plan, e := c.bakalScriptWarp(warp)
	if e != nil {
		t.Fatal(e)
	}
	mapCount := 0
	for _, p := range plan {
		if p.ID == 29 {
			mapCount++
		}
		if p.ID == 2194 {
			t.Fatal("second actor created before map loading")
		}
	}
	if mapCount != 1 || w.activeDungeon.Loaded || !w.bakal.SecondPhase() {
		t.Fatal("phase did not publish unloaded second map")
	}
	_, repeat, e := c.bakalScriptWarp(warp)
	if e != nil || len(repeat) != 1 || repeat[0].ID != 2070 {
		t.Fatal("duplicate cinematic replayed map/symbols", e)
	}
	_, loaded, e := c.bakalLoadingDone(nil)
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, p := range loaded {
		if p.ID == 2194 {
			n++
			if binary.LittleEndian.Uint32(p.Payload[7:]) != w.bakalRules.Monsters["bakal"].SecondTemplate {
				t.Fatal("wrong second-stage template")
			}
		}
	}
	if n != 1 {
		t.Fatalf("second actor creates=%d", n)
	}
}

func legionLocationForTest(w *worldSession, id uint32) (uint32, error) {
	return legion.BakalLocationOfDungeon(w.bakalRules, id)
}

func TestBakalNativeSameGridPortalPreservesLivingBoss(t *testing.T) {
	c, at := bakalNativeClient(t)
	w := c.worldState
	grid := [2]byte{1, 1}
	if _, e := w.bakalDungeonEntry(100003149, 0, &grid, at, false); e != nil {
		t.Fatal(e)
	}
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	h, _ := hex.DecodeString("01014e0100003a01000000e17c040000000000000000000000000000002100000000000000000000000000000000000000000000000000000000000000b700736d02004e0100003a010000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000010000000505c80280010000000000000000004dedf5050000000000")
	r, e := protocol.DecodeDungeonRoomTransition(h)
	if e != nil {
		t.Fatal(e)
	}
	before := w.activeDungeon.NextEntity
	next, plan, e := w.moveDungeonRoomDecoded(r)
	if e != nil {
		t.Fatal(e)
	}
	if next.NextEntity != before || next.Room.Map != w.activeDungeon.Room.Map {
		t.Fatal("same-room portal changed scene/actor identity")
	}
	if len(plan) < 2 || plan[1].ID != 29 || len(plan[1].Payload) != 34 || plan[1].Payload[31] != 0 {
		t.Fatal("portal did not retain native scene objects")
	}
	w.activeDungeon = next
	if _, _, e := c.bakalLoadingDone(nil); e != nil {
		t.Fatal(e)
	}
	if p, e := w.bakalLoadedBoss(); e != nil || len(p) != 0 {
		t.Fatal("portal recreated living boss", e)
	}
}
