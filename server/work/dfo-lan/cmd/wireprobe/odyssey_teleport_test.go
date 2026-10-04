package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"dfolan/internal/testfixture"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestOdysseyJournalAreaTransition(t *testing.T) {
	cat, err := catalog.LoadWorld(testfixture.CatalogPath(t, "world"))
	if err != nil {
		t.Fatal(err)
	}
	growth, err := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if err != nil {
		t.Fatal(err)
	}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	w := &worldSession{service: &world.Service{Catalog: cat}, progression: &character.ProgressionService{Odyssey: growth}, level: 35, odyssey: true,
		role:  database.Character{Request: req, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"odyssey_completed_dungeons":[100004934,100004935,100004936]}`)},
		state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 1, X: 544, Y: 311}}}
	b, _ := hex.DecodeString("280000000400000081032a01052600000001000000000000")
	r, err := protocol.DecodeAreaChangeRequest(b)
	if err != nil {
		t.Fatal(err)
	}
	next, err := w.areaTransition(r)
	if err != nil || next.Town != 40 || next.Area != 4 || next.X != 897 || next.Y != 298 || next.Return != nil {
		t.Fatal(next, err)
	}
	w.level = 1
	if _, err = w.areaTransition(r); err == nil {
		t.Fatal("level bypass")
	}
	w.level = 35
	stale := r
	stale.PreviousArea = 2
	if _, err = w.areaTransition(stale); err == nil {
		t.Fatal("stale origin accepted")
	}
	w.activeDungeon = &dungeon.Session{}
	if _, err = w.areaTransition(r); err == nil {
		t.Fatal("in-dungeon teleport accepted")
	}
	w.activeDungeon = nil
	w.role.Request = nil
	if _, err = w.areaTransition(r); err == nil {
		t.Fatal("normal character bypassed adjacency")
	}
}
