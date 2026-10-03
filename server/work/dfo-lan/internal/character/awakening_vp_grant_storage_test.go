package character_test

import (
	"context"
	"dfolan/internal/catalog"
	. "dfolan/internal/character"

	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestReconcileTechniquePointsBackfillsLegacyThirdAwakening(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated schema integration")
	}
	ctx := context.Background()
	cfg, e := storage.LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("reconcile_tp_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer admin.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	store, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateCharacterNotices(ctx); e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	a, e := store.DevelopmentAccount(ctx, "reconcile-fixture")
	if e != nil {
		t.Fatal(e)
	}
	legacy := State{Level: 100, Advancement: 1, Awakening: 3, SourceSHA256: "legacy", SkillPoints: [2]uint16{10, 10}, SkillVariations: [2]SkillVariationState{{}}}
	raw, _ := json.Marshal(legacy)
	role, e := store.CreateCharacter(ctx, Character{AccountID: a, Name: "ReconcileFixture", Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw, Request: []byte{0}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Store: store, Catalog: c}
	reconciled, backfilled, e := s.ReconcileTechniquePoints(ctx, role)
	if e != nil || !backfilled {
		t.Fatal(backfilled, e)
	}
	var out State
	if e = json.Unmarshal(reconciled.State, &out); e != nil {
		t.Fatal(e)
	}
	if out.TechniquePoints[0] != 5 {
		t.Fatalf("存量三觉补发后 TP=%d，应为 5", out.TechniquePoints[0])
	}
	if out.Level != 100 || out.SkillPoints[0] != 10 || out.SkillVariations[0].Options != nil {
		t.Fatalf("补发动了无关字段: %+v", out)
	}
	// 已对齐：第二次是 no-op。
	_, again, e := s.ReconcileTechniquePoints(ctx, reconciled)
	if e != nil || again {
		t.Fatal("已对齐仍被补发", again, e)
	}
	// 未三觉：no-op。
	below := State{Level: 115, Advancement: 1, Awakening: 2, SourceSHA256: "legacy", SkillPoints: [2]uint16{10, 10}}
	raw2, _ := json.Marshal(below)
	role2, e := store.CreateCharacter(ctx, Character{AccountID: a, Name: "ReconcileBelow", Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: raw2, Request: []byte{0}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	_, appliedBelow, e := s.ReconcileTechniquePoints(ctx, role2)
	if e != nil || appliedBelow {
		t.Fatal("未三觉被补发", appliedBelow, e)
	}
}
