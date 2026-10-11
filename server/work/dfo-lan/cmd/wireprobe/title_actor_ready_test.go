package main

import (
	"bytes"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"testing"
)

func TestDungeonTitleReadinessRejectsUnrelatedOrInvalidCommands(t *testing.T) {
	for _, tc := range []struct {
		name     string
		op       uint16
		body     int
		verified bool
	}{
		{"process scan", 2127, 16, true}, {"loading finished", 37, 16, true},
		{"unverified", 2423, 16, false}, {"short", 2423, 15, true}, {"long", 2423, 17, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &worldSession{titleRestoreCharacter: 7, role: database.Character{ID: 7}, activeDungeon: &dungeon.Session{}, characters: &character.Service{}}
			got, err := w.dungeonTitleActorReady(tc.op, make([]byte, tc.body), tc.verified)
			if err != nil || len(got) != 0 || w.titleRestoreCharacter != 7 {
				t.Fatalf("premature consumption: %v %v pending=%d", got, err, w.titleRestoreCharacter)
			}
		})
	}
}

func TestDungeonTitleReadinessUsesCurrentSelectionAndRejectsStaleCharacter(t *testing.T) {
	for _, tc := range []struct {
		name    string
		role    int64
		skin    uint32
		dungeon bool
		want    bool
	}{
		{"changed selection", 7, 2003, true, true}, {"cancelled selection", 7, 0, true, false},
		{"different character", 8, 2003, true, false}, {"left dungeon", 7, 2003, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := inventory.SaveBag([]byte(`{"level":115,"future":{"keep":true}}`), inventory.Bag{Version: "ordinary-bag-v1", TitleSkin: tc.skin, TitleBook: map[byte][]inventory.TitleBookEntry{4: {{Equipment: inventory.BagEquipment{Slot: 10, Template: 2003}}}}})
			if err != nil {
				t.Fatal(err)
			}
			w := &worldSession{titleRestoreCharacter: 7, role: database.Character{ID: tc.role, State: raw}, characters: &character.Service{}}
			if tc.dungeon {
				w.activeDungeon = &dungeon.Session{}
			}
			got, err := w.dungeonTitleActorReady(2423, make([]byte, 16), true)
			if err != nil || (len(got) == 1) != tc.want || w.titleRestoreCharacter != 0 || !bytes.Equal(w.role.State, raw) {
				t.Fatalf("stale/read-only replay: %v %v", got, err)
			}
			if tc.want && !bytes.Equal(got[0].Payload, protocol.TitleAppearanceSuccess(tc.skin)) {
				t.Fatal("cached old selection replayed")
			}
		})
	}
}
