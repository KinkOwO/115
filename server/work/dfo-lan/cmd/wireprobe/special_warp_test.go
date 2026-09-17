package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func specialWarpFixture(t *testing.T) *worldSession {
	t.Helper()
	c, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	g, e := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	return &worldSession{account: 7, level: 38, service: &world.Service{Catalog: c}, progression: &character.ProgressionService{Odyssey: g},
		role:  storage.Character{ID: 14, AccountID: 7, WireID: 14, Request: req, ConfigVersion: g.Source, State: json.RawMessage(`{"odyssey_completed_dungeons":[100004934,100004935,100004936,100004937,100004938]}`)},
		state: storage.WorldState{Position: storage.WorldPosition{Town: 40, Area: 4, X: 849, Y: 263}}}
}

func TestSpecialWarpPreparationAndDarkelfDestination(t *testing.T) {
	w := specialWarpFixture(t)
	old := w.state
	plan, e := w.prepareSpecialWarp(nil)
	if e != nil || len(plan) != 1 {
		t.Fatal(plan, e)
	}
	if plan[0].Kind != 0 || plan[0].ID != 365 || hex.EncodeToString(plan[0].Payload) != "04010e0000000000" {
		t.Fatal(plan)
	}
	if w.state != old {
		t.Fatal("preparation changed position")
	}
	for i := 0; i < 32; i++ {
		if !retainRequestBody(2261, map[uint16]int{2261: 99}) {
			t.Fatal("preparation stopped being decoded")
		}
		p, e := w.prepareSpecialWarp(nil)
		if e != nil || len(p) != 0 {
			t.Fatal("duplicate animation", p, e)
		}
	}
	r := protocol.AreaChangeRequest{Town: 41, Area: 2, X: 569, Y: 218, Flag: 5, PreviousTown: 40, PreviousArea: 4}
	next, e := w.areaTransition(r)
	if e != nil || next.Town != 41 || next.Area != 2 {
		t.Fatal(next, e)
	}
	r.X = 1
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("altered target admitted")
	}
	r.X = 569
	w.role.State = json.RawMessage(`{"odyssey_completed_dungeons":[100004934,100004935,100004936,100004937]}`)
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("missing clear bypassed")
	}
	w = specialWarpFixture(t)
	w.specialWarpPending = true
	if e = w.handle(36, []byte{1}, nil, nil); e == nil || w.specialWarpPending {
		t.Fatal("malformed move retained pending")
	}
	w.specialWarpPending = true
	clearSelectedWorld(w)
	if w.specialWarpPending {
		t.Fatal("pending survived character switch")
	}
	t.Log("MODIFIED: CMD2261 empty -> NOTI365 04010e0000000000; no position mutation; source Darkelf41/2 admitted after confirmed clears")
}

func TestSpecialWarpRejectsInvalidContext(t *testing.T) {
	for _, kind := range []string{"body", "role", "owner", "actor", "dungeon", "selection", "geometry", "level"} {
		t.Run(kind, func(t *testing.T) {
			w := specialWarpFixture(t)
			var p []byte
			switch kind {
			case "body":
				p = []byte{0}
			case "role":
				w.role.ID = 0
			case "owner":
				w.role.AccountID++
			case "actor":
				w.role.WireID = 65535
			case "dungeon":
				w.activeDungeon = &dungeon.Session{}
			case "selection":
				w.selectingDungeon = true
			case "geometry":
				w.state.Position.X = 65535
			case "level":
				w.level = 0
			}
			if plan, e := w.prepareSpecialWarp(p); e == nil || len(plan) != 0 || w.specialWarpPending {
				t.Fatal(plan, e)
			}
		})
	}
}
