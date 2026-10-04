package database

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

// legacySkinCargoDDL is the shape the first version of this feature left in
// existing databases: the cargo was keyed by a PVF-file index the client never
// reads, and the skin id it does read sat unused in `action_param`.
const legacySkinCargoDDL = `CREATE TABLE account_skin_cargo (
 account_id bigint NOT NULL REFERENCES accounts(id),
 source_template bigint NOT NULL,
 skin_index integer NOT NULL,
 action_param integer NOT NULL DEFAULT 0,
 damage_font_index integer NOT NULL DEFAULT 0,
 unlocked_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(account_id,source_template));`

func TestSkinCargoUpgradesLegacyShape(t *testing.T) {
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
	schema := fmt.Sprintf("skin_cargo_%d", time.Now().UnixNano())
	if _, err = admin.db.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.db.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err = s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "skin-cargo-fixture")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, legacySkinCargoDDL); err != nil {
		t.Fatal(err)
	}
	// The unkeyed row is what a partially written registration looks like, and a
	// second account is what the account scoping has to keep out.
	other, err := s.DevelopmentAccount(ctx, "skin-cargo-other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(ctx, `INSERT INTO account_skin_cargo
 (account_id,source_template,skin_index,action_param) VALUES
 ($1,10305398,11,12),($1,10358669,59,59),($1,590700824,18,18),($1,99999,7,0),
 ($2,10305398,11,12)`, account, other); err != nil {
		t.Fatal(err)
	}
	for _, run := range []int{1, 2} {
		if err = s.MigrateSkinCargo(ctx); err != nil {
			t.Fatalf("migration run %d: %v", run, err)
		}
	}
	skins, err := s.ListSkins(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	want := []AccountSkin{{SourceTemplate: 10305398, SkinKey: 12},
		{SourceTemplate: 10358669, SkinKey: 59}, {SourceTemplate: 590700824, SkinKey: 18}}
	for i := range want {
		skins[i].UnlockedAt = time.Time{}
	}
	if !reflect.DeepEqual(skins, want) {
		t.Fatalf("backfilled cargo = %+v want %+v", skins, want)
	}
	// A later registration writes only the current columns, so the legacy NOT NULL
	// column must now carry a default; a replay must not duplicate or re-key.
	if err = s.UnlockSkin(ctx, account, 10399999, 77); err != nil {
		t.Fatal(err)
	}
	if err = s.UnlockSkin(ctx, account, 10305398, 4242); err != nil {
		t.Fatal(err)
	}
	skins, err = s.ListSkins(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if len(skins) != 4 || skins[0].SkinKey != 12 || skins[2].SkinKey != 77 {
		t.Fatalf("cargo after unlock = %+v", skins)
	}
	var rows int
	if err = s.db.QueryRow(ctx, `SELECT count(*) FROM account_skin_cargo WHERE account_id=$1`, account).Scan(&rows); err != nil || rows != 5 {
		t.Fatalf("rows lost by the upgrade: %d %v", rows, err)
	}
	// The applied font is per character, so it lives in its own table and must
	// survive being re-applied.
	if err = s.MigrateSkinSelection(ctx); err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "SkinCargoFixture",
		Profession: 0, ConfigVersion: strings.Repeat("ab", 32),
		State: json.RawMessage(`{"unrelated":123}`), Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	if got, e := s.SelectedSkin(ctx, role.ID, 2); e != nil || got != 0 {
		t.Fatalf("empty selection = %d %v", got, e)
	}
	for _, key := range []uint32{12, 59} {
		if err = s.SelectSkin(ctx, role.ID, 2, key); err != nil {
			t.Fatal(err)
		}
		if got, e := s.SelectedSkin(ctx, role.ID, 2); e != nil || got != key {
			t.Fatalf("selection = %d %v, want %d", got, e, key)
		}
	}
	// The damage-font panel has two tabs, so the two categories have to hold
	// different fonts at the same time, and 解除 (stored as 0) must clear only its
	// own tab.
	if err = s.SelectSkin(ctx, role.ID, 6, 18); err != nil {
		t.Fatal(err)
	}
	if got, e := s.SelectedSkin(ctx, role.ID, 2); e != nil || got != 59 {
		t.Fatalf("category 2 selection = %d %v, want 59", got, e)
	}
	if err = s.SelectSkin(ctx, role.ID, 6, 0); err != nil {
		t.Fatal(err)
	}
	if got, e := s.SelectedSkin(ctx, role.ID, 6); e != nil || got != 0 {
		t.Fatalf("unequipped category 6 = %d %v", got, e)
	}
	if got, e := s.SelectedSkin(ctx, role.ID, 2); e != nil || got != 59 {
		t.Fatalf("unequipping category 6 disturbed category 2: %d %v", got, e)
	}
	var unrelated int
	if err = s.db.QueryRow(ctx, "SELECT (state->>'unrelated')::int FROM characters WHERE id=$1", role.ID).Scan(&unrelated); err != nil || unrelated != 123 {
		t.Fatal("character state changed", err)
	}
	var legacy bool
	if err = s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM information_schema.columns
 WHERE table_schema=current_schema() AND table_name='account_skin_cargo' AND column_name='damage_font_index')`).Scan(&legacy); err != nil || !legacy {
		t.Fatalf("upgrade dropped columns: %v", err)
	}
}
