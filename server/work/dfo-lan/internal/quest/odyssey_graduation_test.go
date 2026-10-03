package quest_test

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/quest"
	"dfolan/internal/savecontract"
	"dfolan/internal/storage"
	"dfolan/internal/testfixture"
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"testing"
	"time"
)

func graduationFixture(t *testing.T) (*quest.Service, storage.Character) {
	t.Helper()
	g, err := catalog.LoadOdysseyGrowth("../../configs/odyssey-growth-release.json")
	if err != nil {
		t.Fatal(err)
	}
	q, err := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if err != nil {
		t.Fatal(err)
	}
	p, err := catalog.LoadCharacters("../../configs/characters.alljobs-pilot.json")
	if err != nil {
		t.Fatal(err)
	}
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	s := &quest.Service{Catalog: q, Professions: p, Odyssey: g, Progression: &character.ProgressionService{Odyssey: g}}
	r := storage.Character{Profession: 0, Request: req, Name: "GradFixture", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115,"advancement":1,"creation_mode":2,"inventory":{"sentinel":[1,2,3]},"equipment_unlock_mask":7,"unrelated_saved_field":"keep"}`)}
	return s, r
}

func TestGraduationSourcePlanAndPreservedState(t *testing.T) {
	s, role := graduationFixture(t)
	raw, _, ids, err := s.ApplyOdysseyGraduation(role, false)
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(ids, 22826) || slices.Contains(ids, 22987) || !slices.Contains(ids, 22790) {
		t.Fatalf("guide/predecessor selection invalid: %v", ids)
	}
	if len(ids) < 500 {
		t.Fatalf("earlier-level epics missing: %d", len(ids))
	}
	t.Logf("source graduation plan: %d quests", len(ids))
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["inventory"]) != `{"sentinel":[1,2,3]}` || string(doc["unrelated_saved_field"]) != `"keep"` || string(doc["equipment_unlock_mask"]) != `7` {
		t.Fatalf("save fields lost: %s", raw)
	}
	var state character.State
	if err = json.Unmarshal(raw, &state); err != nil {
		t.Fatal(err)
	}
	if !state.OdysseyGraduated || state.OdysseyGraduationVersion != 2 || state.OdysseyGraduationRewardOwed != 10420561 {
		t.Fatalf("graduation state: %+v", state)
	}
	role.State = raw
	t.Setenv("DFO_ODYSSEY_MODE", "1")
	if character.OdysseyRole(role) || character.OdysseyMember(role) || !character.CreatedAsOdyssey(role) {
		t.Fatal("graduation lost source or projected Odyssey mode")
	}
	// Migrate an old paid graduate without owing or granting a duplicate box.
	paid, _, _, err := s.ApplyOdysseyGraduation(role, true)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(paid, &state); err != nil || state.OdysseyGraduationRewardOwed != 0 {
		t.Fatalf("paid receipt ignored: %s %v", paid, err)
	}
}

func TestGraduationEligibilityAndJobFilter(t *testing.T) {
	s, role := graduationFixture(t)
	base, err := s.GraduationQuestPlan(role)
	if err != nil {
		t.Fatal(err)
	}
	id := uint32(22790)
	if !slices.Contains(base, uint16(id)) {
		t.Fatal("fixture epic missing")
	}
	q := s.Catalog.Quests[id]
	q.Jobs = []string{"[unrelated job]"}
	s.Catalog.Quests[id] = q
	ids, err := s.GraduationQuestPlan(role)
	if err != nil || slices.Contains(ids, uint16(id)) {
		t.Fatalf("foreign profession auto-cleared: %v", err)
	}
	q.Jobs = nil // source quests without a [job] restriction apply to every profession
	s.Catalog.Quests[id] = q
	ids, err = s.GraduationQuestPlan(role)
	if err != nil || !slices.Contains(ids, uint16(id)) {
		t.Fatalf("unrestricted epic omitted: %v", err)
	}
	role.State = json.RawMessage(`{"level":114}`)
	if _, applied, err := s.GraduateOdyssey(context.Background(), role); err != nil || applied {
		t.Fatalf("below cap: %v", err)
	}
	if _, err := s.GraduationQuestPlan(role); err == nil {
		t.Fatal("below-cap plan accepted")
	}
	role.Request = nil
	role.State = json.RawMessage(`{"level":115}`)
	if _, applied, err := s.GraduateOdyssey(context.Background(), role); err != nil || applied {
		t.Fatalf("ordinary character: %v", err)
	}
}

func TestOdysseyGraduationAtomicIntegration(t *testing.T) {
	if os.Getenv("ODYSSEY_GRADUATION_INTEGRATION") != "1" {
		t.Skip("requires isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig(os.Getenv("ODYSSEY_GRADUATION_STORAGE"))
	if err != nil {
		t.Fatal(err)
	}
	live, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer live.Close()
	schema := fmt.Sprintf("odyssey_graduation_test_%d", time.Now().UnixNano())
	if _, err = live.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := live.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema = schema
	db, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, migrate := range []func(context.Context) error{db.Migrate, db.MigrateQuests, db.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role := graduationFixture(t)
	s.Store = db
	account, err := db.DevelopmentAccount(ctx, "graduation-test")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID = account
	role, err = db.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := s.GraduationQuestPlan(role)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.DB.Exec(ctx, `INSERT INTO character_quests(character_id,quest_id,status,config_version,progress_model) VALUES($1,$2,'accepted',$4,'prior-active'),($1,$3,'completed',$4,'prior-completed'),($1,22987,'accepted',$4,'future')`, role.ID, ids[0], ids[1], role.ConfigVersion)
	if err != nil {
		t.Fatal(err)
	}
	// Force failure after quest writes. Neither completion rows, state nor
	// receipt may survive the failed character UPDATE.
	if _, err = db.DB.Exec(ctx, `ALTER TABLE characters ADD CONSTRAINT reject_graduation CHECK (NOT (state ? 'odyssey_graduation_version'))`); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.GraduateOdyssey(ctx, role); err == nil {
		t.Fatal("injected storage failure ignored")
	}
	var count int
	if err = db.DB.QueryRow(ctx, `SELECT count(*) FROM character_events`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("receipt survived rollback: %d %v", count, err)
	}
	if err = db.DB.QueryRow(ctx, `SELECT count(*) FROM character_quests`).Scan(&count); err != nil || count != 3 {
		t.Fatalf("quests survived rollback: %d %v", count, err)
	}
	if _, err = db.DB.Exec(ctx, `ALTER TABLE characters DROP CONSTRAINT reject_graduation`); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		got, applied, err := s.GraduateOdyssey(ctx, role) // deliberately replay stale role
		if err != nil || applied != (i == 0) || !character.OdysseyGraduated(got) {
			t.Fatalf("attempt %d: applied=%v err=%v", i, applied, err)
		}
		available, err := s.Available(ctx, got)
		if err != nil || !slices.Contains(available, uint32(22826)) {
			t.Fatalf("112 guide unavailable: %v %v", available, err)
		}
	}
	var model, status string
	if err = db.DB.QueryRow(ctx, `SELECT status,progress_model FROM character_quests WHERE character_id=$1 AND quest_id=$2`, role.ID, ids[1]).Scan(&status, &model); err != nil || model != "prior-completed" {
		t.Fatalf("completed history replaced: %s %v", model, err)
	}
	if err = db.DB.QueryRow(ctx, `SELECT status FROM character_quests WHERE character_id=$1 AND quest_id=22987`, role.ID).Scan(&status); err != nil || status != "accepted" {
		t.Fatalf("115 quest changed: %s %v", status, err)
	}
	if err = db.DB.QueryRow(ctx, `SELECT count(*) FROM character_events`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate receipts: %d %v", count, err)
	}
}
