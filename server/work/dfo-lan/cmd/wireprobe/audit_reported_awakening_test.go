package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestAuditOdysseyDemonSlayerFirstAwakening50(t *testing.T) {
	svc, db, role := awakeningSQLiteFixture(t, 4)
	// Profession 11 = swordman female, growtype 2 = demon slayer.
	p, ok := svc.Catalog.Professions[11]
	if !ok {
		t.Fatal("profession missing")
	}
	st := character.State{Level: 50, Advancement: 2, AllJobsPilot: true, SourcePath: p.Path, SourceSHA256: p.RawSHA256, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills}
	raw, e := json.Marshal(st)
	if e != nil {
		t.Fatal(e)
	}
	var f map[string]json.RawMessage
	json.Unmarshal(raw, &f)
	f["audit_mode"] = json.RawMessage(`"odyssey-entry-request"`)
	raw, _ = json.Marshal(f)
	role, e = db.CreateCharacter(context.Background(), database.Character{AccountID: role.AccountID, Name: "Demon50", Profession: 11, ConfigVersion: svc.Catalog.Source.SaveIdentity(), State: raw, Request: []byte{11}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	w := &worldSession{role: role}
	body, _ := hex.DecodeString("0456d610000000005831d710000100000000000000000000")
	packets, e := awakenCharacter(svc, w, body, make([]byte, wire.SessionKeyBytes))
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, p := range packets {
		if p.Name == "awakening_completed" {
			found = true
		}
	}
	if !found {
		t.Fatal("success response missing")
	}
	saved, e := db.AdminCharacter(context.Background(), role.ID)
	if e != nil {
		t.Fatal(e)
	}
	var state character.State
	json.Unmarshal(saved.State, &state)
	if state.Level != 50 || state.Advancement != 2 || state.Awakening != 1 {
		t.Fatalf("incorrect state %+v", state)
	}
	t.Logf("reported profession %s level50 advancement2: command success, saved awakening1; client popup/odyssey progression not exercised", p.Path)
}
