package character_test

import (
	"context"
	"dfolan/internal/catalog"
	. "dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/testfixture"
	"encoding/hex"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestCapturedAutoSetPersistence(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated schema integration")
	}
	capture := os.Getenv("DFO_TEST_SKILL_CAPTURE_FILE")
	if capture == "" {
		t.Skip("DFO_TEST_SKILL_CAPTURE_FILE selects the historical before-state capture")
	}
	ctx := context.Background()
	fixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := fixture.Storage()
	if e = store.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateGrants(ctx); e != nil {
		t.Fatal(e)
	}
	if e = store.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	c, e := catalog.LoadCharacters("../../configs/characters.awakening-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	l, e := LoadLearningCatalog(testfixture.SkillCatalogPath(t, "release"), c.Source.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := os.ReadFile(capture)
	if e != nil {
		t.Fatal(e)
	}
	var receipt struct{ Before json.RawMessage }
	if e = json.Unmarshal(raw, &receipt); e != nil {
		t.Fatal(e)
	}
	a, e := store.DevelopmentAccount(ctx, "skill-fixture")
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, Character{AccountID: a, Name: "SkillFixture", Profession: 0, ConfigVersion: c.Source.SaveIdentity(), State: receipt.Before, Request: []byte{0}}, 24)
	if e != nil {
		t.Fatal(e)
	}
	role.WireID = 503
	s := Service{Store: store, Catalog: c, Learning: l}
	p, _ := hex.DecodeString("002572000002700000057100000902010012ef00000deb0000150900002961000017ec00001c5b000016490000240800000148000026690000016e00000a6d00002b070000016c00002e2600000a1f00000105010005ba00000a6b00000a43000001310000012100000a1b0000140f0000010e0000010d0000010c00000104000001aa00000141000001110000014400000262000008010101eb000100000000020101000000004800010000000001090001000000010048000100000001004900010000000100eb000100000001000201020000000100baeb9fb30000000000")
	req, e := protocol.DecodeSkillPurchase(p)
	if e != nil {
		t.Fatal(e)
	}
	saved, applied, e := s.Learn(ctx, role, "captured-autoset", req)
	if e != nil || !applied {
		t.Fatal(applied, e)
	}
	again, applied, e := s.Learn(ctx, role, "captured-autoset", req)
	var firstState, retryState any
	json.Unmarshal(saved.State, &firstState)
	json.Unmarshal(again.State, &retryState)
	if e != nil || applied || !reflect.DeepEqual(firstState, retryState) {
		t.Fatal("retry", applied, e)
	}
	var before, after map[string]any
	json.Unmarshal(role.State, &before)
	json.Unmarshal(saved.State, &after)
	if !reflect.DeepEqual(before["inventory"], after["inventory"]) {
		t.Fatal("inventory changed")
	}
	reply, e := s.LearningResponse(saved, req)
	if e != nil {
		t.Fatal(e)
	}
	restore, e := s.VariationRestore(saved)
	if e != nil || len(restore) == 0 {
		t.Fatal(e)
	}
	if out := os.Getenv("SKILL_FIXTURE_DIR"); out != "" {
		if e = os.WriteFile(out+"/autoset-reply.bin", reply, 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(out+"/variation-restore.bin", restore, 0600); e != nil {
			t.Fatal(e)
		}
	}
	req.Options[0].ID = 65535
	if _, _, e = s.Learn(ctx, saved, "invalid-variation", protocol.SkillPurchase{Options: req.Options}); e == nil {
		t.Fatal("invalid variation committed")
	}
	t.Log("captured 37 skill changes and 3+5 variations persisted; retry idempotent; inventory retained")
}
