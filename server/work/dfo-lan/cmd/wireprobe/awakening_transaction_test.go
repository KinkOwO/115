package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/testfixture"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func awakeningSQLiteFixture(t *testing.T, connections int) (*character.Service, *database.Store, database.Character) {
	t.Helper()
	ctx := context.Background()
	db, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "awakening.db"), MaxConnections: int32(connections), SQLiteBusyTimeoutMS: 100})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(db.Close)
	if err = db.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	c, err := catalog.LoadCharacters("../../configs/characters.skycastle-release.json")
	if err != nil {
		t.Fatal(err)
	}
	l, err := character.LoadLearningCatalog(testfixture.SkillCatalogPath(t, "next27"), c.Source.Checksum)
	if err != nil {
		t.Fatal(err)
	}
	account, err := db.DevelopmentAccount(ctx, "awake-tx-test")
	if err != nil {
		t.Fatal(err)
	}
	p := c.Professions[0]
	st := character.State{Level: 115, Advancement: 1, AllJobsPilot: true, SourcePath: p.Path, SourceSHA256: p.RawSHA256, Attributes: p.InitialAttributes, InitialSkills: p.InitialSkills}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	json.Unmarshal(raw, &fields)
	fields["inventory"] = json.RawMessage(`{"version":"ordinary-bag-v1","gold":12345}`)
	fields["future_field"] = json.RawMessage(`{"keep":true}`)
	raw, err = json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	role, err := db.CreateCharacter(ctx, database.Character{AccountID: account, Name: "AwakeTxn", Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	entries := map[uint32]catalog.SkinStorageEntry{10420562: {Template: 10420562, SkinID: 20078, SkinType: "party frame"}}
	if err = db.UnlockSkin(ctx, account, 10420562, 20078); err != nil {
		t.Fatal(err)
	}
	if err = db.SetSkinSelectionList(ctx, role.ID, 0, []uint32{20078, 50000, 60000}); err != nil {
		t.Fatal(err)
	}
	svc := &character.Service{Store: db, Catalog: c, Learning: l, ProfileFrames: func(ctx context.Context, r character.Character) ([2]uint32, error) {
		return loadProfileFrames(ctx, db, r, entries)
	}}
	return svc, db, role
}

func TestAwakeningSQLiteProjectionDoesNotNestTransactions(t *testing.T) {
	for _, connections := range []int{4, 1} {
		t.Run(fmt.Sprintf("pool%d", connections), func(t *testing.T) {
			svc, db, role := awakeningSQLiteFixture(t, connections)
			w := &worldSession{role: role}
			keys := make([]byte, wire.SessionKeyBytes)
			for stage := byte(1); stage <= 3; stage++ {
				// Exact first-awakening body captured in the player's 00:03:48
				// request. Retain its opaque prefix and transport padding.
				body, err := hex.DecodeString("0456d610000000005831d710000100000000000000000000")
				if err != nil {
					t.Fatal(err)
				}
				binary.LittleEndian.PutUint32(body[13:], uint32(stage))
				start := time.Now()
				packets, err := awakenCharacter(svc, w, body, keys)
				if err != nil {
					t.Fatalf("stage%d refused after %s: %v", stage, time.Since(start), err)
				}
				if time.Since(start) > 2*time.Second {
					t.Fatal("awakening still waits on a nested database operation")
				}
				found := false
				for _, p := range packets {
					if p.Name == "awakening_character_updated" {
						if binary.LittleEndian.Uint32(p.Payload[5+0x2c:]) != 20078 {
							t.Fatal("selected party frame lost")
						}
					}
					if p.Name == "awakening_completed" {
						found = true
					}
				}
				if !found {
					t.Fatal("missing success response")
				}
				before := string(w.role.State)
				if _, err = awakenCharacter(svc, w, body, keys); err != nil || string(w.role.State) != before {
					t.Fatal("duplicate awakening changed save", err)
				}
			}
			saved, err := db.AdminCharacter(context.Background(), role.ID)
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			json.Unmarshal(saved.State, &fields)
			if string(fields["awakening"]) != "3" || string(fields["inventory"]) != `{"version":"ordinary-bag-v1","gold":12345}` || string(fields["future_field"]) != `{"keep":true}` {
				t.Fatalf("save fields changed: %s", saved.State)
			}
		})
	}
}

func TestAdvancementSQLiteProjectionDoesNotNestTransactions(t *testing.T) {
	for _, connections := range []int{4, 1} {
		t.Run(fmt.Sprintf("pool%d", connections), func(t *testing.T) {
			svc, db, role := awakeningSQLiteFixture(t, connections)
			w := &worldSession{role: role}
			body := make([]byte, 14)
			body[13] = 2
			packets, err := changeGrowType(svc, w, body, make([]byte, wire.SessionKeyBytes), 1881)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, p := range packets {
				if p.Name == "advancement_completed" {
					found = true
				}
			}
			if !found {
				t.Fatal("missing advancement success")
			}
			saved, err := db.AdminCharacter(context.Background(), role.ID)
			if err != nil {
				t.Fatal(err)
			}
			var state character.State
			if err = json.Unmarshal(saved.State, &state); err != nil || state.Advancement != 2 || state.Awakening != 0 {
				t.Fatal("advancement not saved", err)
			}
		})
	}
}

func TestEntryProjectionSnapshotIsRequestLocalAndRejectsForeignRoles(t *testing.T) {
	svc, db, role := awakeningSQLiteFixture(t, 1)
	view, err := svc.WithEntryProjectionSnapshot(role)
	if err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*database.Character){func(r *database.Character) { r.ID++ }, func(r *database.Character) { r.AccountID++ }, func(r *database.Character) { r.ConfigVersion = "foreign" }} {
		foreign := role
		mutate(&foreign)
		if _, err = view.EntryBasicProbe(foreign, [2]byte{}); err == nil {
			t.Fatal("foreign identity used cached presentation")
		}
	}
	if err = db.SetSkinSelectionList(context.Background(), role.ID, 0, []uint32{20000, 50000, 60000}); err != nil {
		t.Fatal(err)
	}
	next, err := svc.WithEntryProjectionSnapshot(role)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		svc   *character.Service
		frame uint32
	}{{view, 20078}, {next, 20000}, {svc, 20000}} {
		body, err := tc.svc.EntryBasicProbe(role, [2]byte{})
		if err != nil || binary.LittleEndian.Uint32(body[5+0x2c:]) != tc.frame {
			t.Fatal("snapshot leaked into shared service", err)
		}
	}
}

func TestAwakeningEncodeFailureRollsBackBeforeCommit(t *testing.T) {
	svc, db, role := awakeningSQLiteFixture(t, 4)
	w := &worldSession{role: role}
	body := make([]byte, 17)
	binary.LittleEndian.PutUint32(body[13:], 1)
	if _, err := awakenCharacter(svc, w, body, nil); err == nil {
		t.Fatal("invalid transport keys accepted")
	}
	saved, err := db.AdminCharacter(context.Background(), role.ID)
	if err != nil {
		t.Fatal(err)
	}
	if string(saved.State) != string(role.State) {
		t.Fatal("failed encoding committed awakening")
	}
	if _, err := db.CharacterEventReceipt(context.Background(), role.AccountID, role.ID, "awakening-v1:1"); err == nil {
		t.Fatal("failed encoding left a receipt")
	}
}
