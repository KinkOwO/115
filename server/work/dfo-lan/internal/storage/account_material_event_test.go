package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Uses an isolated schema so existing characters and material counts are untouched.
func TestAccountMaterialEventIntegration(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("CASH_INTEGRATION=1 requires local PostgreSQL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
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
	schema := fmt.Sprintf("account_material_test_%d", time.Now().UnixNano())
	if _, err := admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	cfg.MaxConnections = 8
	s, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents, s.MigrateAccountMaterials} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "account-material-fixture")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("a", 64)
	roles := make([]Character, 2)
	for i := range roles {
		roles[i], err = s.CreateCharacter(ctx, Character{AccountID: account, Name: fmt.Sprintf("Material%d", i), Request: []byte{0}, ConfigVersion: version, State: json.RawMessage(`{}`)}, 24)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO account_material_storage(account_id,counts) VALUES($1,$2)`, account, `{"version":"account-materials-v1","counts":{"367":10}}`); err != nil {
		t.Fatal(err)
	}
	type materialCounts struct {
		Version string            `json:"version"`
		Counts  map[string]uint32 `json:"counts"`
	}
	var applied atomic.Int32
	spend := func(role Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, error) {
		var m materialCounts
		if err := json.Unmarshal(raw, &m); err != nil {
			return nil, nil, err
		}
		if m.Counts["367"] < 7 {
			return nil, nil, fmt.Errorf("insufficient account material")
		}
		m.Counts["367"] -= 7
		updated, err := json.Marshal(m)
		if err == nil {
			applied.Add(1)
		}
		return role.State, updated, err
	}
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i, role := range roles {
		wg.Add(1)
		go func(i int, role Character) {
			defer wg.Done()
			_, _, yes, err := s.CommitAccountMaterialEvent(ctx, account, role.ID, version, fmt.Sprintf("spend-%d", i), "skill-material-v1", spend)
			if err == nil && yes {
				successes.Add(1)
			}
		}(i, role)
	}
	wg.Wait()
	if successes.Load() != 1 || applied.Load() != 1 {
		t.Fatalf("shared balance was deducted more than once: commits=%d callbacks=%d", successes.Load(), applied.Load())
	}
	var count materialCounts
	raw, err := s.AccountMaterials(ctx, account)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &count); err != nil || count.Counts["367"] != 3 {
		t.Fatalf("unexpected remaining count: %s err=%v", raw, err)
	}
	for i, role := range roles {
		_, _, replayed, err := s.CommitAccountMaterialEvent(ctx, account, role.ID, version, fmt.Sprintf("spend-%d", i), "skill-material-v1", spend)
		if err == nil {
			if replayed {
				t.Fatal("replay deducted the balance")
			}
			break
		}
	}
	if applied.Load() != 1 {
		t.Fatal("replay ran spend callback")
	}
}
