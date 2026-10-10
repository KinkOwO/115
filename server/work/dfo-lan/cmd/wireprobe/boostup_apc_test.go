package main

import (
	"bytes"
	"context"
	"dfolan/internal/adventureelite"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"encoding/binary"
	"encoding/json"
	"reflect"
	"testing"
)

// Live030159: successful2333/1811 was followed by the account refresh's
// empty1754, releasing the NPC before CMD15. Retain mode3 in every projection.
func TestBoostAPCAccountRefreshKeepsPreparedCompanion(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, companion, _ := eliteEligibilityWorld(t, 115, 3, false)
	w.channelType = 0
	w.boostup = &boostup.Catalog{Town: 222, TeachingAPCs: []boostup.TeachingAPC{{Index: 1, Template: 2504, Event: boostup.EventID}}}
	var err error
	w.role.State, err = boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 4}})
	must115(t, err)
	w.state.Position = database.WorldPosition{Town: 222}
	ctx := context.Background()
	profile, err := w.prepareAdventure(ctx)
	must115(t, err)
	initial, err := w.adventureElitePayload(ctx, profile)
	must115(t, err)
	if !bytes.Equal(initial, []byte{0}) {
		t.Fatal("unsolicited NPC selection", initial)
	}
	selected, err := w.requestBoostAPC(ctx, []byte{1})
	must115(t, err)
	_, err = w.loadBoostAPC()
	must115(t, err)
	for _, phase := range []string{"town", "selection", "dungeon"} {
		w.selectingDungeon = phase == "selection"
		if phase == "dungeon" {
			w.activeDungeon = &dungeon.Session{}
		}
		packets, err := w.refreshAdventureEliteSelections(ctx, profile)
		must115(t, err)
		if len(packets) != 0 {
			t.Fatalf("%s refresh overwrote the loaded event NPC: %+v", phase, packets)
		}
		body, err := w.adventureElitePayload(ctx, profile)
		must115(t, err)
		if !bytes.Equal(body, selected[0].Payload) {
			t.Fatal("event NPC projection changed during training", phase)
		}
	}
	w.selectingDungeon, w.activeDungeon = false, nil
	profile.Data.EliteSelections = map[uint16][3]int64{1: {companion.ID}}
	packets, err := w.refreshAdventureEliteSelections(ctx, profile)
	must115(t, err)
	if len(packets) != 1 || packets[0].ID != 1754 || packets[0].Payload[0] != 2 {
		t.Fatal("changed account view did not retain the NPC row", packets)
	}
	row := packets[0].Payload[len(packets[0].Payload)-531:]
	if binary.LittleEndian.Uint16(row[0x0d:]) != 3 || row[0x10] != 1 || binary.LittleEndian.Uint32(row[0x17:]) != 1 {
		t.Fatal("account refresh removed the source NPC binding")
	}
	w.state.Position.Town = 1
	body, err := w.adventureElitePayload(ctx, database.AccountAdventure{})
	must115(t, err)
	if !bytes.Equal(body, []byte{0}) {
		t.Fatal("NPC selection escaped its event town", body)
	}
	w.state.Position.Town = 222
	body, err = w.adventureElitePayload(ctx, database.AccountAdventure{})
	must115(t, err)
	if !bytes.Equal(body, []byte{0}) {
		t.Fatal("stale event selection restored without a new2333 request")
	}
	_, err = w.requestBoostAPC(ctx, []byte{1})
	must115(t, err)
	w.role.State, err = boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 4, Finished: true}})
	must115(t, err)
	body, err = w.adventureElitePayload(ctx, profile)
	must115(t, err)
	if len(body) != 532 || body[0] != 1 || binary.LittleEndian.Uint16(body[1+0x0d:]) != 1 {
		t.Fatal("graduation did not remove just the transient NPC", body)
	}
}

func TestBoostAPCPrepareNativeSequencePreservesAccountSelections(t *testing.T) {
	t.Setenv(adventureelite.EnvKey, "1")
	w, companion, slot := eliteEligibilityWorld(t, 115, 3, false)
	w.channelType = 0
	w.boostup = &boostup.Catalog{Town: 222, TeachingAPCs: []boostup.TeachingAPC{{Index: 7, Template: 9997, Event: boostup.EventID}, {Index: 8, Template: 9988, Event: 665}}}
	var err error
	w.role.State, err = boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 4}})
	must115(t, err)
	w.state.Position = database.WorldPosition{Town: 222}
	ctx := context.Background()
	_, before, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, "boost-apc-test-ordinary-settings",
		func(role database.Character, profile *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			profile.Data.EliteSelections = map[uint16][3]int64{1: {companion.ID}, 2: {companion.ID}}
			profile.Data.EliteSkillUsage = map[uint16]map[int64][30]int32{1: {companion.ID: {19}}, 2: {companion.ID: {19}}}
			return role.State, json.RawMessage(`{}`), nil
		})
	must115(t, err)
	if _, err = w.loadBoostAPC(); err == nil {
		t.Fatal("unsolicited1811 created an NPC")
	}
	request := make([]byte, 16)
	request[0] = 1
	plan, err := w.requestBoostAPC(ctx, request)
	must115(t, err)
	if len(plan) != 1 || plan[0].ID != 1754 || !observedGameRequest(2333) {
		t.Fatal("native selection/request route missing", plan)
	}
	p := plan[0].Payload
	if len(p) < 532 {
		t.Fatal("selection record missing")
	}
	row := p[len(p)-531:]
	if binary.LittleEndian.Uint16(row[0x0d:]) != 3 || row[0x10] != 1 || binary.LittleEndian.Uint32(row[0x17:]) != 7 {
		t.Fatalf("selection ignored source special index/event binding: wire index=%d, want 7 (AIC template 9997 is not the wire index)", binary.LittleEndian.Uint32(row[0x17:]))
	}
	if p[0] != 2 || binary.LittleEndian.Uint16(p[1+0x0d:]) != 1 || int32(binary.LittleEndian.Uint32(p[1+0x27:])) != slot || binary.LittleEndian.Uint32(p[1+0x33+120:]) != 19 {
		t.Fatal("mode3 update lost mode1 settings or restored the hidden mode2 roster")
	}
	plan, err = w.loadAdventureElite(ctx, []byte{3, 0})
	must115(t, err)
	if len(plan) != 2 || plan[0].ID != 1382 || len(plan[0].Payload) != 4 || plan[0].Payload[3] != 0 || plan[1].ID != 1879 || binary.LittleEndian.Uint16(plan[1].Payload) != 3 {
		t.Fatal("native NPC preparation order/branch", plan)
	}
	after, err := w.prepareAdventure(ctx)
	must115(t, err)
	if !reflect.DeepEqual(after.Data.EliteSelections, before.Data.EliteSelections) || !reflect.DeepEqual(after.Data.EliteSkillUsage, before.Data.EliteSkillUsage) {
		t.Fatal("event preparation changed persisted account settings")
	}
	w.activeDungeon = &dungeon.Session{}
	if _, err = w.requestBoostAPC(ctx, request); err == nil {
		t.Fatal("active dungeon NPC rebuilt")
	}
	if _, err = w.loadBoostAPC(); err == nil {
		t.Fatal("active dungeon mode3 reload accepted")
	}
	w.activeDungeon = nil
	w.state.Position.Town = 1
	if _, err = w.loadBoostAPC(); err == nil {
		t.Fatal("mode3 escaped source event town")
	}
}
