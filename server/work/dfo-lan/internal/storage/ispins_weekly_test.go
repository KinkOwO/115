package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIspinsWeekBoundary(t *testing.T) {
	for _, x := range []struct{ now, want string }{{"2026-10-06T08:59:59Z", "2026-09-29T09:00:00Z"}, {"2026-10-06T09:00:00Z", "2026-10-06T09:00:00Z"}, {"2026-10-13T08:59:59Z", "2026-10-06T09:00:00Z"}} {
		n, _ := time.Parse(time.RFC3339, x.now)
		if got := IspinsWeekStart(n).Format(time.RFC3339); got != x.want {
			t.Fatal(got, x.want)
		}
	}
}
func TestIspinsWeeklyPersistenceIntegration(t *testing.T) {
	if os.Getenv("ISPINS_WEEKLY_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL schema; set ISPINS_WEEKLY_INTEGRATION=1")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg, e := LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	admin, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("ispins_weekly_test_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		c, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if _, e := admin.DB.Exec(c, "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	s, e := Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer func() { s.Close() }()
	if e = s.Migrate(ctx); e != nil {
		t.Fatal(e)
	}
	if e = s.MigrateCharacterEvents(ctx); e != nil {
		t.Fatal(e)
	}
	account, e := s.DevelopmentAccount(ctx, "ispins-weekly-fixture")
	if e != nil {
		t.Fatal(e)
	}
	v := strings.Repeat("a", 64)
	role, e := s.CreateCharacter(ctx, Character{AccountID: account, Name: "WeeklyFixture", WireID: 7, Profession: 0, ConfigVersion: v, Request: []byte{0}, State: json.RawMessage(`{"inventory":{"gold":123,"items":[]},"future":{"keep":true}}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	now, _ := time.Parse(time.RFC3339, "2026-10-03T12:00:00Z")
	next := now.AddDate(0, 0, 7)
	r1, r2 := strings.Repeat("1", 32), strings.Repeat("2", 32)
	var before []byte
	if e = s.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&before); e != nil {
		t.Fatal(e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account, role.ID, now); e != nil || used {
		t.Fatal(used, e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, r1, now, true); e != nil {
		t.Fatal(e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, r1, now, true); e != nil {
		t.Fatal("duplicate", e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, r2, now, true); !errors.Is(e, ErrIspinsWeeklyCleared) {
		t.Fatal("second weekly clear allowed", e)
	}
	s.Close()
	s, e = Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account, role.ID, now); e != nil || !used {
		t.Fatal("reopen lost receipt", used, e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, r2, now, false); e != nil {
		t.Fatal("unlimited", e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account+100, role.ID, now); e != nil || used {
		t.Fatal("other account", used, e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account, role.ID, next); e != nil || used {
		t.Fatal("week reset", used, e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, r1, next, true); e != nil {
		t.Fatal(e)
	}
	if used, e := s.IspinsWeeklyUsed(ctx, account, role.ID, next); e != nil || used {
		t.Fatal("old run replay consumed a new week", used, e)
	}
	if e = s.RecordIspinsWeeklyClear(ctx, account, role.ID, v, strings.Repeat("3", 32), next, true); e != nil {
		t.Fatal(e)
	}
	var after []byte
	s.DB.QueryRow(ctx, "SELECT state FROM characters WHERE id=$1", role.ID).Scan(&after)
	if !bytes.Equal(before, after) {
		t.Fatal("quota metadata rewrote player state")
	}
}
