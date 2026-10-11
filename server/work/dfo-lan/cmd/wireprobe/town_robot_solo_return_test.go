package main

import (
	"bytes"
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/binary"
	"testing"
	"time"
)

func TestTownRobotFleetPersonalBoostGuideReturnDoesNotReportKick(t *testing.T) {
	t.Setenv("DFO_TEST_POSTGRES_DSN", "isolated-sqlite-fixture")
	w := moonWorlds(t, 1)[0]
	w.boostup = &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1}, {Number: 2, Guide: "dungeon", Dungeon: 100004558}}}
	w.professions = w.characters.Catalog
	var err error
	w.role.State, err = boostup.WriteState(w.role.State, boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 2, Phase: 1, Claimed: map[byte]bool{1: true}}})
	must115(t, err)
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{ID: 100004558, Tutorial: true}}
	// Enabled fleet, with no invited companions in the personal guide.
	w.hub = &lanHub{townRobots: &townRobotFleet{}}
	w.characters = nil
	before := bytes.Clone(w.role.State)
	for cycle := 0; cycle < 3; cycle++ {
		solo, err := protocol.SoloPartyInfo(w.role.WireID, [2]byte{3, 86})
		must115(t, err)
		w.soloPartyReady = true
		w.rememberOrdinarySoloParty(solo)
		if w.ordinarySoloParty == nil || !w.isBoostGuideRun() {
			t.Fatal("personal guide bootstrap not tracked")
		}
		identity := w.ordinarySoloParty
		plan, err := w.leaveDungeon()
		must115(t, err)
		gone, err := protocol.OrdinaryPartyGone115(identity.id, identity.context)
		must115(t, err)
		if countPacket(plan, 10) != 0 || countPacket(plan, 9) != 1 || countPacket(plan, 23) != 1 || countPacket(plan, 24) != 1 {
			t.Fatal("training return emitted kick or omitted native cleanup", plan)
		}
		for _, packet := range plan {
			if packet.ID == 9 && !bytes.Equal(packet.Payload, gone) {
				t.Fatal("cleared another party")
			}
			if packet.ID == 23 && binary.LittleEndian.Uint32(packet.Payload[2:]) != 222 {
				t.Fatal("guide did not return to its event town")
			}
		}
		if w.ordinarySoloParty != nil || w.soloPartyReady || !bytes.Equal(before, w.role.State) {
			t.Fatal("ghost party or changed training/reward state")
		}
	}
}

func TestTownRobotFleetRestoredSupportTrainingHandshakeAndReturn(t *testing.T) {
	svc, db, role := supportTrainingSQLiteFixture(t, 1)
	ctx := context.Background()
	must115(t, db.MigrateAdventure(ctx))
	role.Profession = 3
	var err error
	role.State, err = boostup.WriteState(role.State, boostup.State{Version: 1, Activated: true, Variant: 1, Training: boostup.Training{Step: 1}})
	must115(t, err)
	w := &worldSession{store: db, account: role.AccountID, role: role, characters: svc, fatigue: &character.FatigueService{Location: time.UTC}, hub: &lanHub{townRobots: &townRobotFleet{}}, boostup: &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1}}, BufferJobs: [][2]byte{{3, 0}}, TrainingAPCs: []boostup.TrainingAPC{{SourceIndex: 19, Template: 8123}}}, state: database.WorldState{Position: database.WorldPosition{Town: 222}}}
	c, out, _ := newDispatchTestClient()
	c.worldState, c.bootstrapped, c.selectedCharacterID = w, true, role.ID
	before, err := db.AdminCharacter(ctx, role.ID)
	must115(t, err)
	req := &clientRequest{frame: wire.Frame{Type: 1, ID: 2333}, verified: true, plaintext: append([]byte{1}, make([]byte, 15)...)}
	if c.dispatchAccountQueries(req) != dispatchHandled {
		t.Fatal("C2333 still unimplemented")
	}
	packets := lifecyclePackets(t, c, out)
	if len(packets) != 1 || packets[0].ID != 1754 || !w.boostTrainingAPCRequested {
		t.Fatal("missing training settings", packets)
	}
	plan, err := w.loadAdventureElite(ctx, []byte{3, 0})
	must115(t, err)
	if len(plan) != 2 || plan[0].ID != 1382 || len(plan[0].Payload) != 4 || plan[1].ID != 1879 {
		t.Fatal("owner container must precede source NPC", plan)
	}
	solo, err := protocol.SoloPartyInfo(role.WireID, [2]byte{3, 86})
	must115(t, err)
	w.rememberOrdinarySoloParty(solo)
	w.soloPartyReady = true
	w.characters = nil
	w.activeDungeon = &dungeon.Session{Definition: catalog.DungeonDefinition{Tutorial: true}}
	plan, err = w.leaveDungeon()
	must115(t, err)
	if countPacket(plan, 10) != 0 || countPacket(plan, 9) != 1 || w.ordinarySoloParty != nil || w.soloPartyReady {
		t.Fatal("training retained kick notice or ghost party", plan)
	}
	after, err := db.AdminCharacter(ctx, role.ID)
	must115(t, err)
	if !bytes.Equal(before.State, after.State) {
		t.Fatal("NPC handshake changed player archive")
	}
}
