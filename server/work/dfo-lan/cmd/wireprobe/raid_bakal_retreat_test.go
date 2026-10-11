package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/raid"
	"testing"
	"time"
)

func TestBakalCapturedRetreatReturnsToCampWithoutClear(t *testing.T) {
	now := time.Now()
	rules := catalog.RaidEntrance{MemberMax: 12, StartMinimum: 1, PartyMax: 3, Bakal: &catalog.BakalRaidRules{
		PhaseMax: 1, TimeLimit: 9999, Dungeons: []catalog.RaidPhaseDungeon{{ID: 100003160, State: "open"}},
		Monsters:           []catalog.RaidMonsterPlacement{{Location: 23, Kind: "basilisk"}},
		MonsterDefinitions: map[string]catalog.BakalRaidMonster{"basilisk": {ID: 109014480}},
		Slots:              map[uint32]catalog.BakalRaidSlot{23: {Kind: "basilisk", Dungeon: 100003160}, 52: {Kind: "camp", TownArea: 2}}}}
	o, err := raid.PrepareBakalOpening(rules, []raid.Member{{Actor: 2, Position: 1}}, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = o.Activate(now); err != nil {
		t.Fatal(err)
	}
	run := &dungeon.Session{Loaded: true}
	w := &worldSession{role: database.Character{ID: 2, WireID: 2}, channelType: 82, raidWaiting: true, bakalOpening: o, bakalRules: rules.Bakal, bakalTown: 152, bakalParty: 1,
		activeDungeon: run, state: database.WorldState{Position: database.WorldPosition{Town: 152, Area: 2, X: 700, Y: 300}}}
	p := make([]byte, 16)
	p[0], p[1], p[2] = 2, 2, 1
	if _, plan, err := w.settlementExit(p); err != nil || len(plan) != 1 || plan[0].Name != "settlement_focus_ack" || w.activeDungeon != run {
		t.Fatal("focus clears active battle", plan, err)
	}
	p[0] = 1
	pending, plan, err := w.settlementExit(p)
	if err != nil || pending != nil || len(plan) != 5 {
		t.Fatal("captured retreat rejected", plan, err)
	}
	for i, id := range []uint16{72, 3, 23, 24, 2285} {
		if plan[i].ID != id {
			t.Fatal("missing camp return packet", plan)
		}
	}
	if plan[0].Name != "settlement_exit_ack" || !bytes.Equal(plan[0].Payload, []byte{1, 1, 2}) || w.selectingDungeon || run.Completed() || !o.Active() {
		t.Fatal("retreat settled or ended raid")
	}
	w.role.WireID = 3
	if _, _, err = w.settlementExit(p); err == nil {
		t.Fatal("foreign member allowed retreat")
	}
	w.role.WireID = 2
	if _, err = w.bakalNativeCampReturn(3); err == nil || w.activeDungeon != run {
		t.Fatal("foreign camp changed owned combat")
	}
	plan, err = w.bakalNativeCampReturn(2)
	if err != nil || len(plan) != 5 || plan[0].ID != 2074 || plan[0].Kind != 1 || !bytes.Equal(plan[0].Payload, []byte{1}) || w.activeDungeon != nil || !o.Active() || run.Completed() {
		t.Fatal("native camp return settled raid or retained combat", plan, err)
	}
}
