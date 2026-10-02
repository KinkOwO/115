package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/hex"
	"strings"
	"testing"
)

func TestRejectedWestCoastSceneGateCannotStartDungeon(t *testing.T) {
	// Native CMD16 body from the 2026-09-28 West Coast capture, following a
	// rejected CMD15 for the same scenario ID.
	body, err := hex.DecodeString("34f2f5050000000000ffff0000000000782f0000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{
		role: storage.Character{ID: 11}, dungeons: &catalog.DungeonCatalog{},
		state: storage.WorldState{Position: storage.WorldPosition{Town: 40, Area: 0}},
		townArrivalScenes: map[uint32]catalog.TownArrivalScene{
			100004404: {QuestID: 12152, Town: 40, Area: 0, DungeonID: 100004404},
		},
	}
	_, _, err = w.selectDungeon(body)
	if err == nil || !strings.Contains(err.Error(), "no approved matching gate") {
		t.Fatalf("rejected scene gate allowed dungeon selection: %v", err)
	}
	if w.activeDungeon != nil {
		t.Fatal("town character was moved into a dungeon session")
	}
}

func TestWestCoastOriginSyncKeepsPendingScene(t *testing.T) {
	w := &worldSession{
		pendingTownArrival: &dungeon.Session{},
		state:              storage.WorldState{Position: storage.WorldPosition{Town: 40, Area: 0, X: 412, Y: 181}},
	}
	actual, err := hex.DecodeString("28000000000000009c01b500002800000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	if !w.isTownArrivalOriginSync(36, actual) {
		t.Fatal("captured CMD36 origin sync would discard the pending scene")
	}
	actual[8]++
	if w.isTownArrivalOriginSync(36, actual) || w.isTownArrivalOriginSync(35, actual) {
		t.Fatal("a different town movement must not look like origin sync")
	}
}

func TestWestCoastSceneSelectsQuestMaze(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	body, err := hex.DecodeString("34f2f5050000000000ffff0000000000782f0000000000000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeDungeonSelection(body)
	if err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(c, r, 96, map[uint16]bool{12152: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Definition.ID != 100004404 || s.Room.Map != 100014298 {
		t.Fatalf("scene selected dungeon %d map %d", s.Definition.ID, s.Room.Map)
	}
}
