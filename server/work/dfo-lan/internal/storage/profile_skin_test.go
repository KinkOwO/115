package storage

import (
	"context"
	"dfolan/internal/character"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestProfileSkinPersistence(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated schema integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, err := LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("profile_skin_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, m := range []func(context.Context) error{s.Migrate, s.MigrateProfileSkins, s.MigrateProfileSkins} {
		if err = m(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "profile-skin-fixture")
	if err != nil {
		t.Fatal(err)
	}
	other, err := s.DevelopmentAccount(ctx, "profile-skin-other")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("ab", 32)
	create := func(name string) Character {
		r, e := s.CreateCharacter(ctx, Character{AccountID: account, Name: name, Profession: 0, ConfigVersion: version, State: json.RawMessage(`{"unrelated":123}`), Request: []byte{0}}, 24)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	role := create("SkinFixture")
	role2 := create("SkinFixtureTwo")
	if _, err = s.RestoreProfileSkins(ctx, other, role.ID); err == nil {
		t.Fatal("foreign character accepted")
	}
	var n int
	if err = s.DB.QueryRow(ctx, "SELECT count(*) FROM character_profile_skins").Scan(&n); err != nil || n != 0 {
		t.Fatalf("unauthorized bootstrap: %d %v", n, err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 4)
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state, e := s.RestoreProfileSkins(ctx, account, role.ID)
			if e == nil && !reflect.DeepEqual(state, character.ProfileSkinDefaults()) {
				e = fmt.Errorf("unexpected defaults")
			}
			errs <- e
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		if e != nil {
			t.Fatal(e)
		}
	}
	custom := character.ProfileSkinDefaults()
	custom.Owned = append(custom.Owned, character.ProfileSkinOwned{ID: 60001})
	custom.Selected[2] = 60001
	raw, _ := json.Marshal(custom)
	if _, err = s.DB.Exec(ctx, "UPDATE character_profile_skins SET state=$2 WHERE character_id=$1", role.ID, raw); err != nil {
		t.Fatal(err)
	}
	// A new store connection models a server restart: it must return the row,
	// rather than replacing it with bootstrap constants on each login.
	reopened, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	got, err := reopened.RestoreProfileSkins(ctx, account, role.ID)
	if err != nil || !reflect.DeepEqual(got, custom) {
		t.Fatalf("stored selection reset: %+v %v", got, err)
	}
	got, err = reopened.RestoreProfileSkins(ctx, account, role2.ID)
	if err != nil || !reflect.DeepEqual(got, character.ProfileSkinDefaults()) {
		t.Fatalf("selection leaked across characters: %+v %v", got, err)
	}
	var unrelated int
	if err = s.DB.QueryRow(ctx, "SELECT (state->>'unrelated')::int FROM characters WHERE id=$1", role.ID).Scan(&unrelated); err != nil || unrelated != 123 {
		t.Fatal("character state changed", err)
	}
	if _, err = s.DB.Exec(ctx, "UPDATE character_profile_skins SET state='{}' WHERE character_id=$1", role.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreProfileSkins(ctx, account, role.ID); err == nil {
		t.Fatal("corrupt state silently reset")
	}
	if _, err = s.DB.Exec(ctx, "UPDATE characters SET deleted_at=now() WHERE id=$1", role2.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.RestoreProfileSkins(ctx, account, role2.ID); err == nil {
		t.Fatal("deleted character restored")
	}
}
