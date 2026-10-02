package character_test

import (
	"context"
	. "dfolan/internal/character"

	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

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
	cfg.PostgresSchema = schema
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
	s, c := AwakeningGrantFixtureForTest(t)
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
		role, err := store.CreateCharacter(ctx, Character{AccountID: account, Name: fmt.Sprintf("Branchless%d", job), Profession: job, ConfigVersion: c.Source.SaveIdentity(), State: raw, Request: []byte{0}}, 24)
		if err != nil {
			t.Fatal(err)
		}
		for stage := byte(1); stage <= 3; stage++ {
			role, _, err = store.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion, fmt.Sprintf("awakening-v1:%d", stage), "system-awakening-v1", func(current Character) (json.RawMessage, json.RawMessage, error) {
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
