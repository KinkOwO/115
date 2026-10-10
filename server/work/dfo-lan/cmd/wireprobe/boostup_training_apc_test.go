package main

import (
	"bytes"
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
	"time"
)

func TestBoostTrainingAPCSQLiteHandshakeDoesNotChangeAccounts(t *testing.T) {
	svc, db, role := supportTrainingSQLiteFixture(t, 1)
	ctx := context.Background()
	if e := db.MigrateAdventure(ctx); e != nil {
		t.Fatal(e)
	}
	raw, e := boostup.WriteState(role.State, boostup.State{Version: 1, Activated: true, Variant: 1, Training: boostup.Training{Step: 1}})
	if e != nil {
		t.Fatal(e)
	}
	role.State = raw
	role.Profession = 3
	w := &worldSession{store: db, account: role.AccountID, role: role, boostup: &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1}}, BufferJobs: [][2]byte{{3, 0}}, TrainingAPCs: []boostup.TrainingAPC{{SourceIndex: 19, Template: 8123}}}, state: database.WorldState{Position: database.WorldPosition{Town: 222}}}
	w.characters = svc
	w.fatigue = &character.FatigueService{Location: time.UTC}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["advancement"] = json.RawMessage(`0`)
	w.role.State, _ = json.Marshal(fields)
	before, e := db.AdminCharacter(ctx, role.ID)
	if e != nil {
		t.Fatal(e)
	}
	req := append([]byte{1}, make([]byte, 15)...)
	plan, e := w.requestBoostTrainingAPC(ctx, req)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 1 || plan[0].ID != 1754 {
		t.Fatal("configuration did not wait for native load request")
	}
	plan, e = w.loadAdventureElite(ctx, []byte{3, 0})
	if e != nil {
		t.Fatal(e)
	}
	wantOwner, e := protocol.AdventureEliteCharacterInfo(w.role.WireID, nil, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 2 || plan[0].ID != 1382 || !bytes.Equal(plan[0].Payload, wantOwner) || plan[1].ID != 1879 || !bytes.Equal(plan[1].Payload, []byte{3, 0}) {
		t.Fatal("owned empty elite container must be initialized before source factory completion")
	}
	if len(plan[0].Payload) != 4 || plan[0].Payload[0] != 1 || plan[0].Payload[3] != 0 {
		t.Fatal("training init must not serialize any fake account companion")
	}
	wireID := w.role.WireID
	w.role.WireID = 0
	if invalid, err := w.loadBoostTrainingAPC(); err == nil || len(invalid) != 0 {
		t.Fatal("invalid owner must not publish a partial NPC load")
	}
	w.role.WireID = wireID
	after, e := db.AdminCharacter(ctx, role.ID)
	if e != nil || !bytes.Equal(before.State, after.State) {
		t.Fatal("NPC changed saved player")
	}
	base, _ := protocol.AdventureEliteSelections([]protocol.AdventureEliteSelection{{Mode: 2, Slots: [3]int32{0, -1, -1}}})
	merged, e := w.boostTrainingAPCMerge(base)
	if e != nil || merged[0] != 2 {
		t.Fatal("periodic snapshot lost training NPC")
	}
	if got := binary.LittleEndian.Uint32(merged[1+531+0x17:]); got != 19 {
		t.Fatalf("sent AIC template instead of special table index: %d", got)
	}
	w.boostup.TrainingAPCs[0].SourceIndex = 31
	merged, e = w.boostTrainingAPCMerge(base)
	if e != nil || binary.LittleEndian.Uint32(merged[1+531+0x17:]) != 31 {
		t.Fatal("wire identity did not follow special source index")
	}
	w.activeDungeon = &dungeon.Session{}
	if _, e = w.requestBoostTrainingAPC(ctx, req); e == nil {
		t.Fatal("active NPC can be rebuilt")
	}
}
