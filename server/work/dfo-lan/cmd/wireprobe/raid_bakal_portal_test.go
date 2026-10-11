package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/raid"
	"encoding/binary"
	"encoding/hex"
	"testing"
	"time"
)

func TestBakalCapturedCampPortalRequiresRealActiveMember(t *testing.T) {
	p, _ := hex.DecodeString("2e208b4dfe7f0000c8cf8f96015aedf5050000000003000000000000006f0500001c0100006400000064000000000000")
	now := time.Now()
	rules := catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3, Bakal: &catalog.BakalRaidRules{PhaseMax: 1, StartDelay: 3, TimeLimit: 9999, Dungeons: []catalog.RaidPhaseDungeon{{ID: 100003162, State: "open"}}, Monsters: []catalog.RaidMonsterPlacement{{Location: 24, Kind: "bakal"}}, MonsterDefinitions: map[string]catalog.BakalRaidMonster{"bakal": {Kind: "bakal", ID: 109014482, SpecificMap: -1}}, Slots: map[uint32]catalog.BakalRaidSlot{24: {ID: 24, Kind: "bakal", Dungeon: 100003162, Maps: []uint32{77}}, 52: {ID: 52, Kind: "camp", TownArea: 2}}}}
	opening, err := raid.PrepareBakalOpening(rules, []raid.Member{{Actor: 2, Position: 1}}, now.Add(-4*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: database.Character{ID: 2, WireID: 2}, channelType: 82, raidWaiting: true, bakalOpening: opening, bakalRules: rules.Bakal, bakalTown: 152, bakalParty: 1, soloPartyReady: true}
	w.state.Position = database.WorldPosition{Town: 152, Area: 2}
	if _, _, err := w.bakalPortalSelection(p, now); err == nil {
		t.Fatal("unstarted raid admitted portal")
	}
	if err := opening.Activate(now); err != nil {
		t.Fatal(err)
	}
	sel, grid, err := w.bakalPortalSelection(p, now)
	if err != nil || sel.ID != 100003162 || sel.Party != 65535 || grid != [2]int32{3, 0} {
		t.Fatal("actual native portal rejected", sel, grid, err)
	}
	w.role.WireID = 3
	if _, _, err := w.bakalPortalSelection(p, now); err == nil {
		t.Fatal("foreign member admitted")
	}
	w.role.WireID = 2
	w.activeDungeon = &dungeon.Session{}
	if _, _, err := w.bakalPortalSelection(p, now); err == nil {
		t.Fatal("replayed portal replaced active run")
	}
	w.activeDungeon = nil
	binary.LittleEndian.PutUint32(p[13:], 100004947)
	if _, _, err := w.bakalPortalSelection(p, now); err == nil {
		t.Fatal("foreign dungeon admitted")
	}
}
