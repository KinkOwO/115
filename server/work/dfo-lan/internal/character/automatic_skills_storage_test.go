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

func TestAutomaticSkillPersistence(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated schema integration")
	}
	ctx := context.Background()
	cfg, err := storage.LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("automatic_skills_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	store, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateGrants, store.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role, st := AutomaticSkillFixtureForTest(t)
	s.Store = store
	st.SkillPoints[0] = 1000
	st.LearnedSkills[0] = map[uint16]byte{46: 5}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "auto-skills-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID, role.Name, role.State, role.Request = account, "AutoFixture", raw, []byte{0}
	role, err = store.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 503
	known, err := KnownSkillsForTest(s, role, st, 0)
	if err != nil || known[46] != 5 {
		t.Fatal("existing purchased rank overwritten", known, err)
	}
	dependent, cost := DependentSkillForTest(s, st, known)
	if dependent == 0 {
		t.Fatal("no dependent skill fixture")
	}
	req := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: dependent, Delta: 1}}}
	saved, applied, err := s.Learn(ctx, role, "dependent", req)
	if err != nil || !applied {
		t.Fatal("dependent purchase", applied, err)
	}
	again, applied, err := s.Learn(ctx, role, "dependent", req)
	if err != nil || applied {
		t.Fatal("retry", applied, err)
	}
	var a, b State
	json.Unmarshal(saved.State, &a)
	json.Unmarshal(again.State, &b)
	if !reflect.DeepEqual(a, b) || a.SkillPoints[0] != uint16(1000-cost) || a.LearnedSkills[0][46] != 5 {
		t.Fatal("SP/persistence/retry mismatch")
	}
	refund := protocol.SkillPurchase{Entries: []protocol.SkillPurchaseEntry{{ID: 62, Delta: 1, Refund: 1}}}
	if _, _, err = s.Learn(ctx, saved, "refund-free", refund); err == nil {
		t.Fatal("free ranks refunded into SP")
	}
	roles, err := store.Characters(ctx, account)
	if err != nil || len(roles) != 1 {
		t.Fatal(err)
	}
	var loaded State
	if err = json.Unmarshal(roles[0].State, &loaded); err != nil {
		t.Fatal(err)
	}
	known, err = KnownSkillsForTest(s, roles[0], loaded, 0)
	if err != nil || known[62] != 1 || known[dependent] != 1 || loaded.SkillPoints[0] != a.SkillPoints[0] {
		t.Fatal("relogin mismatch", err)
	}
	if _, err = s.EntrySkills(roles[0]); err != nil {
		t.Fatal("relogin payload", err)
	}
	t.Logf("DB PASS: skill%d learned with free62 prerequisite; SP=%d; retry idempotent; refund62 rejected; relog preserved", dependent, a.SkillPoints[0])
}
