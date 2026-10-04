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

// TestSkinSelectionSlotsRoundTripKeepsPositions checks the shape the 表情 bar needs: the
// client's reader consumes four words in order and forwards the resulting vector to the
// chat channel unchanged, so a stored selection has to come back with the same cell index
// — including holes, and including the same skin in two cells.
func TestSkinSelectionSlotsRoundTripKeepsPositions(t *testing.T) {
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
	schema := fmt.Sprintf("skin_selection_slot_%d", time.Now().UnixNano())
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
	if e := s.MigrateSkinSelectionList(ctx); e != nil {
		t.Fatal(e)
	}
	account, err := s.DevelopmentAccount(ctx, "skin-selection-slot-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "SelectionSlotFixture",
		Request: []byte{0}, ConfigVersion: "test", State: json.RawMessage(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	const width = 4
	if slots, e := s.SkinSelectionSlots(ctx, role.ID, 3, width); e != nil ||
		!reflect.DeepEqual(slots, []uint32{0, 0, 0, 0}) {
		t.Fatalf("fresh slots = %v (%v)", slots, e)
	}
	// Cell 1 empty and the same skin in cells 0 and 3: neither may collapse.
	if e := s.SetSkinSelectionSlots(ctx, role.ID, 3, []uint32{40001, 0, 40002, 40001}); e != nil {
		t.Fatal(e)
	}
	slots, e := s.SkinSelectionSlots(ctx, role.ID, 3, width)
	if e != nil || !reflect.DeepEqual(slots, []uint32{40001, 0, 40002, 40001}) {
		t.Fatalf("slots = %v (%v)", slots, e)
	}
	// Replace, not append, and a zero word leaves no row.
	if e := s.SetSkinSelectionSlots(ctx, role.ID, 3, []uint32{0, 40003, 0, 0}); e != nil {
		t.Fatal(e)
	}
	if slots, e = s.SkinSelectionSlots(ctx, role.ID, 3, width); e != nil ||
		!reflect.DeepEqual(slots, []uint32{0, 40003, 0, 0}) {
		t.Fatalf("replaced slots = %v (%v)", slots, e)
	}
	// A narrower read keeps the cells that still fit and drops the rest, so a frame never
	// grows past the width the client's reader consumes.
	if slots, e = s.SkinSelectionSlots(ctx, role.ID, 3, 2); e != nil ||
		!reflect.DeepEqual(slots, []uint32{0, 40003}) {
		t.Fatalf("two-cell read = %v (%v)", slots, e)
	}
	// The positional table and the set table are separate rows: the 涂鸦 category keeps
	// its own selection.
	if e := s.SetSkinSelectionList(ctx, role.ID, 7, []uint32{90001}); e != nil {
		t.Fatal(e)
	}
	if keys, e := s.SkinSelectionList(ctx, role.ID, 7); e != nil ||
		!reflect.DeepEqual(keys, []uint32{90001}) {
		t.Fatalf("spray selection = %v (%v)", keys, e)
	}
	if slots, e = s.SkinSelectionSlots(ctx, role.ID, 7, width); e != nil ||
		!reflect.DeepEqual(slots, []uint32{0, 0, 0, 0}) {
		t.Fatalf("category 7 slots = %v (%v), the positional table must stay empty", slots, e)
	}
}

// TestSkinFavoriteStoreEnforcesPerPageCap checks the two things the NOTI2641 frame depends
// on: the groups are per page, and an add past the client's own limit of ten is refused
// rather than stored — the client refuses an eleventh star with message 101037008 of its
// own, so a server that stored it would push a row the panel will not show.
func TestSkinFavoriteStoreEnforcesPerPageCap(t *testing.T) {
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
	schema := fmt.Sprintf("skin_favorite_%d", time.Now().UnixNano())
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
	if e := s.MigrateSkinSelectionList(ctx); e != nil {
		t.Fatal(e)
	}
	account, err := s.DevelopmentAccount(ctx, "skin-favorite-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "FavoriteFixture",
		Request: []byte{0}, ConfigVersion: "test", State: json.RawMessage(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	const groups, limit = 10, 3
	pages, e := s.SkinFavorites(ctx, role.ID, groups)
	if e != nil || len(pages) != groups {
		t.Fatalf("fresh favourites = %v (%v), want %d empty groups", pages, e, groups)
	}
	for i := 1; i <= limit; i++ {
		ok, e := s.SetSkinFavorite(ctx, role.ID, 0, uint32(20000+i), true, limit)
		if e != nil || !ok {
			t.Fatalf("star %d stored ok=%v (%v)", i, ok, e)
		}
	}
	// The cap is per page and the add is refused, not queued.
	if ok, e := s.SetSkinFavorite(ctx, role.ID, 0, 20999, true, limit); e != nil || ok {
		t.Fatalf("eleventh star accepted (ok=%v, %v)", ok, e)
	}
	if pages, e = s.SkinFavorites(ctx, role.ID, groups); e != nil ||
		len(pages[0]) != limit {
		t.Fatalf("page 0 after a refused add = %v (%v)", pages[0], e)
	}
	// Another page has its own budget.
	if ok, e := s.SetSkinFavorite(ctx, role.ID, 1, 30001, true, limit); e != nil || !ok {
		t.Fatalf("page 1 star refused (ok=%v, %v)", ok, e)
	}
	// Re-starred is not a second row.
	if ok, e := s.SetSkinFavorite(ctx, role.ID, 1, 30001, true, limit); e != nil || !ok {
		t.Fatalf("re-star failed (ok=%v, %v)", ok, e)
	}
	if pages, e = s.SkinFavorites(ctx, role.ID, groups); e != nil || len(pages[1]) != 1 {
		t.Fatalf("page 1 = %v (%v)", pages[1], e)
	}
	// Removal clears the row, so the absolute frame the server pushes afterwards can take
	// the star away from a client whose table still holds it.
	if ok, e := s.SetSkinFavorite(ctx, role.ID, 1, 30001, false, limit); e != nil || !ok {
		t.Fatalf("un-star failed (ok=%v, %v)", ok, e)
	}
	if pages, e = s.SkinFavorites(ctx, role.ID, groups); e != nil || len(pages[1]) != 0 {
		t.Fatalf("page 1 after removal = %v (%v)", pages[1], e)
	}
	var kept int
	if e = testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM information_schema.tables
 WHERE table_schema=current_schema() AND table_name='character_skin_selection_list'`).Scan(&kept); e != nil {
		t.Fatal(e)
	}
	if kept != 1 {
		t.Fatalf("character_skin_selection_list exists %d times, the set table must stay as it is", kept)
	}
}
