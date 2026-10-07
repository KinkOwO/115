package quest_test

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/quest"
	"dfolan/internal/savecontract"
	"dfolan/internal/testfixture"
	"encoding/json"
	"slices"
	"testing"
	"time"
)

func graduationFixture(t *testing.T) (*quest.Service, database.Character) {
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
	r := database.Character{Profession: 0, Request: req, Name: "GradFixture", ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":115,"advancement":1,"creation_mode":2,"inventory":{"sentinel":[1,2,3]},"equipment_unlock_mask":7,"unrelated_saved_field":"keep"}`)}
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
	if !state.OdysseyGraduated || state.OdysseyGraduationVersion != character.OdysseyGraduationReceiptVersion || state.OdysseyGraduationRewardOwed != 10420561 {
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

// TestGraduationPlanFollowsSourceLevel115Clears pins the source scope of the
// [quest clear] level-115 block: entries whose own catalog minimum level is
// already at graduation are cleared for an Odyssey character as well. A Go-side
// level filter here silently left 52 story quests offered after graduation.
func TestGraduationPlanFollowsSourceLevel115Clears(t *testing.T) {
	s, role := graduationFixture(t)
	ids, err := s.GraduationQuestPlan(role)
	if err != nil {
		t.Fatal(err)
	}
	var atCap []uint16
	for _, id := range ids {
		if d, ok := s.Catalog.Quests[uint32(id)]; ok && d.MinimumLevel >= uint32(character.OdysseyGraduationLevel) {
			atCap = append(atCap, id)
		}
	}
	// Live source 7ef2db59: ClearedAt(115) holds 333 ids, of which 52 carry a
	// graduation-level minimum and none belong to [branch quest].
	if len(atCap) != 52 {
		t.Fatalf("source level-115 clears: want 52 got %d %v", len(atCap), atCap)
	}
	for _, id := range []uint16{22833, 22906, 22957, 22984, 22997, 23028} {
		if !slices.Contains(atCap, id) {
			t.Fatalf("source-cleared quest %d omitted from graduation plan", id)
		}
	}
}

// TestGraduateOdysseyReceiptVersionGate: an older receipt is re-run so a
// source-scope correction reaches live saves, while a current receipt settles.
func TestGraduateOdysseyReceiptVersionGate(t *testing.T) {
	s, role := graduationFixture(t)
	// No store: reaching the service check proves the version gate let this
	// save through for the compensation pass.
	if _, _, err := s.GraduateOdyssey(context.Background(), role); err == nil {
		t.Fatal("legacy save not re-run")
	}
	_, current, _, err := s.ApplyOdysseyGraduation(role, false)
	if err != nil {
		t.Fatal(err)
	}
	role.State = current
	if _, applied, err := s.GraduateOdyssey(context.Background(), role); err != nil || applied {
		t.Fatalf("current receipt re-ran: applied=%v err=%v", applied, err)
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
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	db := fixture.Storage()
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
	for _, seed := range []struct {
		id            uint16
		status, model string
	}{{ids[0], "accepted", "prior-active"}, {ids[1], "completed", "prior-completed"}, {22987, "accepted", "future"}} {
		if err = fixture.SeedQuestRecord(ctx, role.ID, int32(seed.id), seed.status, role.ConfigVersion, seed.model); err != nil {
			t.Fatal(err)
		}
	}
	// Force failure after quest writes. Neither completion rows, state nor
	// receipt may survive the failed character UPDATE.
	if err = fixture.RejectGraduation(ctx, true); err != nil {
		t.Fatal(err)
	}
	if _, _, err = s.GraduateOdyssey(ctx, role); err == nil {
		t.Fatal("injected storage failure ignored")
	}
	count, err := fixture.EventCount(ctx, 0)
	if err != nil || count != 0 {
		t.Fatalf("receipt survived rollback: %d %v", count, err)
	}
	count, err = fixture.QuestCount(ctx, 0)
	if err != nil || count != 3 {
		t.Fatalf("quests survived rollback: %d %v", count, err)
	}
	if err = fixture.RejectGraduation(ctx, false); err != nil {
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
	status, model, err := fixture.QuestRecord(ctx, role.ID, int32(ids[1]))
	if err != nil || model != "prior-completed" {
		t.Fatalf("completed history replaced: %s %v", model, err)
	}
	status, _, err = fixture.QuestRecord(ctx, role.ID, 22987)
	if err != nil || status != "accepted" {
		t.Fatalf("115 quest changed: %s %v", status, err)
	}
	count, err = fixture.EventCount(ctx, 0)
	if err != nil || count != 1 {
		t.Fatalf("duplicate receipts: %d %v", count, err)
	}
}

// TestOdysseyGraduationCompensatesLegacyReceipt drives a save that already
// settled under the previous rule. The new receipt must add only the quest rows
// the save does not carry yet: a quest the player is actively working keeps its
// own hand-in and reward, and the honor box stays a single outstanding debt.
func TestOdysseyGraduationCompensatesLegacyReceipt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	db := fixture.Storage()
	for _, migrate := range []func(context.Context) error{db.Migrate, db.MigrateQuests, db.MigrateCharacterEvents} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	s, role := graduationFixture(t)
	s.Store = db
	account, err := db.DevelopmentAccount(ctx, "graduation-compensation-test")
	if err != nil {
		t.Fatal(err)
	}
	role.AccountID = account
	role, err = db.CreateCharacter(ctx, role, 24)
	if err != nil {
		t.Fatal(err)
	}
	// Replay the previous rule's save: graduation mark at receipt version 2 plus
	// its receipt row, and the quest rows that rule actually wrote.
	legacy := json.RawMessage(`{"level":115,"advancement":1,"creation_mode":2,"odyssey_graduated":true,"odyssey_graduation_version":2,"odyssey_graduation_reward_owed":0,"inventory":{"sentinel":[1,2,3]}}`)
	if err = fixture.SeedCharacterState(ctx, role.ID, legacy); err != nil {
		t.Fatal(err)
	}
	if _, _, err = db.CommitCharacterEvent(ctx, account, role.ID, role.ConfigVersion,
		// The key of the rule this save already settled under, pinned literally so
		// the compensation branch cannot be satisfied by a renamed constant.
		"odyssey-graduation-v2", "odyssey-graduation-v2",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return current.State, json.RawMessage(`{"source":"legacy"}`), nil
		}); err != nil {
		t.Fatal(err)
	}
	role.State = legacy
	plan, err := s.GraduationQuestPlan(role)
	if err != nil {
		t.Fatal(err)
	}
	const active, added = uint16(22835), uint16(23028)
	if !slices.Contains(plan, active) || !slices.Contains(plan, added) {
		t.Fatalf("fixture needs both plan ids: %d %d", active, added)
	}
	if err = fixture.SeedQuestRecord(ctx, role.ID, int32(active), "accepted", role.ConfigVersion, "player-active"); err != nil {
		t.Fatal(err)
	}
	before, err := fixture.QuestCount(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	got, applied, err := s.GraduateOdyssey(ctx, role)
	if err != nil || !applied || !character.OdysseyGraduated(got) {
		t.Fatalf("compensation pass: applied=%v err=%v", applied, err)
	}
	var state character.State
	if err = json.Unmarshal(got.State, &state); err != nil {
		t.Fatal(err)
	}
	if state.OdysseyGraduationVersion != character.OdysseyGraduationReceiptVersion || state.OdysseyGraduationRewardOwed != 10420561 {
		t.Fatalf("compensated state: %+v", state)
	}
	var doc map[string]json.RawMessage
	if err = json.Unmarshal(got.State, &doc); err != nil {
		t.Fatal(err)
	}
	if string(doc["inventory"]) != `{"sentinel":[1,2,3]}` {
		t.Fatalf("inventory lost: %s", got.State)
	}
	if status, model, err := fixture.QuestRecord(ctx, role.ID, int32(active)); err != nil || status != "accepted" || model != "player-active" {
		t.Fatalf("in-progress quest overwritten: %s %s %v", status, model, err)
	}
	if status, model, err := fixture.QuestRecord(ctx, role.ID, int32(added)); err != nil || status != "completed" || model != "odyssey-skip-v1" {
		t.Fatalf("missing source clear not added: %s %s %v", status, model, err)
	}
	after, err := fixture.QuestCount(ctx, role.ID)
	if err != nil || after <= before {
		t.Fatalf("no rows added: before=%d after=%d err=%v", before, after, err)
	}
	// Both receipts remain: the historical one is not rewritten, and a replay of
	// the stale role adds nothing further.
	if count, err := fixture.EventCount(ctx, 0); err != nil || count != 2 {
		t.Fatalf("graduation receipts: %d %v", count, err)
	}
	if _, applied, err := s.GraduateOdyssey(ctx, role); err != nil || applied {
		t.Fatalf("replayed compensation: applied=%v err=%v", applied, err)
	}
}
