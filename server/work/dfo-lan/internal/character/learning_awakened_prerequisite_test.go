package character

import (
	"context"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestBranchlessAwakeningGrantDoesNotDeadlockLearning(t *testing.T) {
	s, c := loadAwakeningGrantFixture(t)
	prof := c.Professions[9]
	raw, err := json.Marshal(State{Level: 115, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills})
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{Profession: 9, ConfigVersion: c.Source.SaveIdentity(), State: raw}
	for stage := byte(1); stage <= 2; stage++ {
		role.State, err = s.ApplyAwakening(role, stage)
		if err != nil {
			t.Fatal(err)
		}
	}
	var state State
	if err = json.Unmarshal(role.State, &state); err != nil {
		t.Fatal(err)
	}
	known, err := s.knownSkills(role, state, 0)
	if err != nil {
		t.Fatal(err)
	}
	if known[255] != 1 || known[81] != 0 {
		t.Fatal("source no longer reproduces the grant's missing prerequisite", known)
	}
	// Dark Cross (64) is an ordinary learnable skill without prerequisites.
	if _, err = s.Learning.index[9][64].costForState(state, 1, known); err != nil {
		t.Fatal(err)
	}
	known[64] = 1
	if err = s.validateLearningPrerequisites(9, known, map[uint16]byte{64: 1}, nil); err != nil {
		t.Fatal("unrelated purchase blocked by source grant 255 -> 81", err)
	}
}

// Use a private PostgreSQL schema to exercise the same awakening event and
// learning transaction as the gateway, without changing any player records.
func TestBranchlessAwakeningAndLearningPersistence(t *testing.T) {
	if os.Getenv("AWAKENING_INTEGRATION") != "1" {
		t.Skip("AWAKENING_INTEGRATION=1 requires local storage")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("branchless_awakening_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, err := admin.DB.Exec(cleanup, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error("temporary schema cleanup", err)
		}
	}()
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateUnifiedOptions, store.MigrateGrants, store.MigrateCharacterEvents, store.MigratePremiums, store.MigrateQuests} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, c := loadAwakeningGrantFixture(t)
	s.Store = store
	account, err := store.DevelopmentAccount(ctx, "branchless-awakening-fixture")
	if err != nil {
		t.Fatal(err)
	}
	for _, job := range []byte{9, 10} {
		prof := c.Professions[job]
		raw, err := json.Marshal(State{Level: 115, AllJobsPilot: true, SourceSHA256: prof.RawSHA256, Attributes: prof.InitialAttributes, InitialSkills: prof.InitialSkills, SkillPoints: [2]uint16{5000, 5000}})
		if err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(raw, &fields); err != nil {
			t.Fatal(err)
		}
		fields["future_save_field"] = json.RawMessage(`{"value":123}`)
		raw, err = json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		role, err := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: fmt.Sprintf("Branchless%d", job), Profession: job, ConfigVersion: c.Source.SaveIdentity(), State: raw, Request: []byte{0}}, 24)
		if err != nil {
			t.Fatal(err)
		}
		for stage := byte(1); stage <= 3; stage++ {
			role, _, err = store.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion, fmt.Sprintf("awakening-v1:%d", stage), "system-awakening-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
				state, err := s.ApplyAwakening(current, stage)
				return state, json.RawMessage(`{"ok":true}`), err
			})
			if err != nil {
				t.Fatalf("job%d stage%d: %v", job, stage, err)
			}
			if job == 9 && stage == 2 {
				req := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 64, Delta: 1}}}
				var applied bool
				role, applied, err = s.Learn(ctx, role, "after-awakening-purchase", req)
				if err != nil || !applied {
					t.Fatal("ordinary purchase after second awakening", applied, err)
				}
				var before, after any
				if err = json.Unmarshal(role.State, &before); err != nil {
					t.Fatal(err)
				}
				role, applied, err = s.Learn(ctx, role, "after-awakening-purchase", req)
				if err != nil || applied {
					t.Fatal("purchase retry changed state or SP", applied, err)
				}
				if err = json.Unmarshal(role.State, &after); err != nil {
					t.Fatal(err)
				}
				// PostgreSQL jsonb normalizes JSON whitespace and object order.
				if !reflect.DeepEqual(before, after) {
					t.Fatal("purchase retry changed saved fields")
				}
			}
		}
	}
	roles, err := store.Characters(ctx, account)
	if err != nil || len(roles) != 2 {
		t.Fatal("saved roles", len(roles), err)
	}
	for _, role := range roles {
		var state State
		if err = json.Unmarshal(role.State, &state); err != nil {
			t.Fatal(err)
		}
		var fields map[string]json.RawMessage
		if err = json.Unmarshal(role.State, &fields); err != nil {
			t.Fatal(err)
		}
		var retained struct{ Value int }
		if err = json.Unmarshal(fields["future_save_field"], &retained); err != nil || retained.Value != 123 {
			t.Fatal("unknown save field lost", err)
		}
		if state.Advancement != 0 || state.Awakening != 3 || state.TechniquePoints[0] != 5 || role.ConfigVersion != c.Source.SaveIdentity() {
			t.Fatal("saved awakening or source changed", role.Profession)
		}
		if role.Profession == 9 && (state.LearnedSkills[0][64] != 1 || state.SkillPoints[0] >= 5000) {
			t.Fatal("ordinary purchase did not persist")
		}
		if _, err = s.EntrySkills(role); err != nil {
			t.Fatal("saved skill entry", err)
		}
		if _, err = s.EntryBasicProbe(role, [2]byte{}); err != nil {
			t.Fatal("saved basic entry", err)
		}
	}
	t.Log("DB PASS: both jobs awakened 1/2/3, job9 ordinary purchase and retry persisted, saved entry and unknown fields preserved")
}

func TestLearningPrerequisitesProtectChangesAndRefunds(t *testing.T) {
	s := Service{Learning: &LearningCatalog{index: map[byte]map[uint16]LearningDefinition{9: {
		255: {Fields: map[string][]pvf.Token{"[pre required skill]": {{Value: 81}, {Value: 1}}}},
		77:  {Fields: map[string][]pvf.Token{"[pre required skill]": {{Value: 8}, {Value: 1}}}},
	}}}}
	for _, tc := range []struct {
		name    string
		known   map[uint16]byte
		changes map[uint16]byte
		reduced map[uint16]bool
		refused bool
	}{
		{"new_skill_missing_prerequisite", map[uint16]byte{77: 1}, map[uint16]byte{77: 1}, nil, true},
		{"source_grant_upgrade_missing_prerequisite", map[uint16]byte{255: 2}, map[uint16]byte{255: 2}, nil, true},
		{"refund_breaks_dependent", map[uint16]byte{255: 1, 81: 0}, map[uint16]byte{81: 0}, map[uint16]bool{81: true}, true},
		{"refund_preserves_dependent", map[uint16]byte{255: 1, 81: 1}, map[uint16]byte{81: 1}, map[uint16]bool{81: true}, false},
		{"unrelated_refund_with_old_gap", map[uint16]byte{255: 1, 64: 0}, map[uint16]byte{64: 0}, map[uint16]bool{64: true}, false},
		{"new_skill_and_prerequisite", map[uint16]byte{77: 1, 8: 1}, map[uint16]byte{77: 1, 8: 1}, nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := s.validateLearningPrerequisites(9, tc.known, tc.changes, tc.reduced)
			if (err != nil) != tc.refused {
				t.Fatalf("refused=%v, want %v: %v", err != nil, tc.refused, err)
			}
		})
	}
}
