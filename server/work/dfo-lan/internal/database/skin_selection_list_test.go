package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

// TestSkinSelectionListRoundTrip checks the set semantics the two list families need:
// one write replaces the whole category rather than adding to it, so a deselected skin
// cannot survive into the next entry frame, and an empty write is the 解除 case.
//
// It also pins that the damage-font table is untouched by this feature: the two are
// separate stores on purpose, and a shared table would have to hold both one-id and
// many-id categories.
func TestSkinSelectionListRoundTrip(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated schema integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := loadPostgresTestConfig()
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("skin_selection_list_%d", time.Now().UnixNano())
	if _, err = testPool(t, admin).Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer testPool(t, admin).Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "skin-selection-list-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if e := s.MigrateSkinSelectionList(ctx); e != nil {
		t.Fatal(e)
	}
	// The damage-font store is created by its own migration, and the two coexist: the
	// assertion below is only meaningful once both have run against this schema.
	if e := s.MigrateSkinSelection(ctx); e != nil {
		t.Fatal(e)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "SelectionListFixture",
		Request: []byte{0}, ConfigVersion: "test", State: json.RawMessage(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if keys, e := s.SkinSelectionList(ctx, role.ID, 0); e != nil || len(keys) != 0 {
		t.Fatalf("fresh selection = %v (%v)", keys, e)
	}
	if e := s.SetSkinSelectionList(ctx, role.ID, 0, []uint32{20001, 50002}); e != nil {
		t.Fatal(e)
	}
	keys, e := s.SkinSelectionList(ctx, role.ID, 0)
	if e != nil || !reflect.DeepEqual(keys, []uint32{20001, 50002}) {
		t.Fatalf("selection = %v (%v)", keys, e)
	}
	// Replace, not append: the client sends the slots it wants applied as one list.
	if e := s.SetSkinSelectionList(ctx, role.ID, 0, []uint32{60001}); e != nil {
		t.Fatal(e)
	}
	if keys, e = s.SkinSelectionList(ctx, role.ID, 0); e != nil || !reflect.DeepEqual(keys, []uint32{60001}) {
		t.Fatalf("replaced selection = %v (%v)", keys, e)
	}
	// The categories are independent rows of the same character.
	if e := s.SetSkinSelectionList(ctx, role.ID, 1, []uint32{30001, 100002}); e != nil {
		t.Fatal(e)
	}
	if keys, e = s.SkinSelectionList(ctx, role.ID, 0); e != nil || !reflect.DeepEqual(keys, []uint32{60001}) {
		t.Fatalf("category 0 after a category 1 write = %v (%v)", keys, e)
	}
	// An empty list is the 解除 case and must leave no row behind.
	if e := s.SetSkinSelectionList(ctx, role.ID, 1, nil); e != nil {
		t.Fatal(e)
	}
	if keys, e = s.SkinSelectionList(ctx, role.ID, 1); e != nil || len(keys) != 0 {
		t.Fatalf("cleared selection = %v (%v)", keys, e)
	}
	var kept int
	if e = testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
 WHERE table_schema=current_schema() AND table_name='character_skin_selection'`).Scan(&kept); e != nil {
		t.Fatal(e)
	}
	if kept != 1 {
		t.Fatalf("character_skin_selection exists %d times, the damage-font store must stay as it is", kept)
	}
}
