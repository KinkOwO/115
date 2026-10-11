package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"testing"
)

func TestBakalTeamFreezesSelectedSource(t *testing.T) {
	normal := &catalog.BakalRaidRules{Mode: catalog.BakalNormal, InitialVariables: map[string]int32{"hp": 1}}
	hard := &catalog.BakalRaidRules{Mode: catalog.BakalHard, InitialVariables: map[string]int32{"hp": 2}}
	entry := catalog.RaidEntrance{Bakal: normal, BakalHard: hard, PartyMax: 3, HardPartyMax: 2, HardLimitFame: 22}
	for _, mode := range []byte{0, 1} {
		got, err := bakalEntryForTeam(entry, protocol.RaidCreateRequest{Kind: 8, Options: [6]byte{0, 0, 0, 0, mode, 0}})
		if err != nil || got.Bakal.Mode != catalog.BakalMode(mode) || got.Bakal.InitialVariables["hp"] != int32(mode)+1 {
			t.Fatal(got, err)
		}
		if mode == 1 && got.PartyMax != 2 {
			t.Fatal("hard party source not selected")
		}
	}
	entry.BakalHard = nil
	if _, err := bakalEntryForTeam(entry, protocol.RaidCreateRequest{Kind: 8, Options: [6]byte{0, 0, 0, 0, 1, 0}}); err == nil {
		t.Fatal("missing hard source fell back to normal")
	}
	if entry.Bakal != normal || entry.PartyMax != 3 {
		t.Fatal("shared entry mutated")
	}
}
