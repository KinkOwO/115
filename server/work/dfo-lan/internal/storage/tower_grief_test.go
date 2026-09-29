package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestTowerGriefServiceDay(t *testing.T) {
	before := time.Date(2026, 9, 27, 8, 59, 59, 0, time.UTC)
	if got := TowerGriefServiceDay(before); got != "2026-09-26" {
		t.Fatalf("before reset: %s", got)
	}
	if got := TowerGriefServiceDay(before.Add(time.Second)); got != "2026-09-27" {
		t.Fatalf("at reset: %s", got)
	}
}

func TestLegacyTowerGriefFloorAcrossAccountCharacters(t *testing.T) {
	var floors [101]uint32
	for floor := 1; floor <= 100; floor++ {
		floors[floor] = uint32(5114 + floor)
	}
	states := []json.RawMessage{
		json.RawMessage(`{"dungeon_best_times":{"5115:normal:solo":15000,"5117:normal:solo":10000},"other":"preserved"}`),
		json.RawMessage(`{"dungeon_best_times":{"5116:normal:solo":12000}}`),
	}
	got, err := legacyTowerGriefFloor(states, floors)
	if err != nil || got != 3 {
		t.Fatalf("imported floor = %d, error = %v", got, err)
	}
	states[1] = json.RawMessage(`{"dungeon_best_times":{"5118:normal:solo":0}}`)
	got, err = legacyTowerGriefFloor(states, floors)
	if err != nil || got != 1 {
		t.Fatalf("non-contiguous/zero-time floor = %d, error = %v", got, err)
	}
}

func TestTowerGriefProgressPostgres(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL schema integration")
	}
	ctx := context.Background()
	cfg, err := LoadConfig("../../runtime/storage/local.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("tower_grief_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer admin.DB.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	store, err := Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTowerGriefProgress(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "tower-fixture")
	if err != nil {
		t.Fatal(err)
	}
	state := json.RawMessage(`{"dungeon_best_times":{"5115:normal:solo":1234},"other":"preserved"}`)
	role, err := store.CreateCharacter(ctx, Character{AccountID: account, Name: "TowerFixture", Profession: 0,
		ConfigVersion: strings.Repeat("ab", 32), State: state, Request: []byte{0}}, 24)
	if err != nil {
		t.Fatal(err)
	}
	var floors [101]uint32
	for floor := 1; floor <= 100; floor++ {
		floors[floor] = uint32(5114 + floor) // isolated fixture only
	}
	progress, err := store.TowerGriefProgress(ctx, account, floors)
	if err != nil || progress.HighestCleared != 1 || progress.ClearedDay != "" {
		t.Fatalf("legacy import: %+v, %v", progress, err)
	}
	var saved json.RawMessage
	if err = store.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, role.ID).Scan(&saved); err != nil {
		t.Fatal(err)
	}
	var want, got any
	if err = json.Unmarshal(state, &want); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(saved, &got); err != nil {
		t.Fatal(err)
	}
	wantBytes, _ := json.Marshal(want)
	gotBytes, _ := json.Marshal(got)
	if !bytes.Equal(wantBytes, gotBytes) {
		t.Fatal("legacy migration modified character state")
	}
	now := time.Now().UTC()
	run := strings.Repeat("a", 32)
	progress, err = store.AdvanceTowerGrief(ctx, account, 2, run, now)
	if err != nil || progress.HighestCleared != 2 {
		t.Fatalf("advance: %+v, %v", progress, err)
	}
	if _, err = store.AdvanceTowerGrief(ctx, account, 2, run, now); err != nil {
		t.Fatalf("idempotent retry: %v", err)
	}
	if _, err = store.AdvanceTowerGrief(ctx, account, 3, strings.Repeat("b", 32), now); err == nil {
		t.Fatal("same-day second floor accepted")
	}
	progress, err = store.AdvanceTowerGrief(ctx, account, 3, strings.Repeat("b", 32), now.Add(24*time.Hour))
	if err != nil || progress.HighestCleared != 3 {
		t.Fatalf("next-day advance: %+v, %v", progress, err)
	}
	if err = store.MigrateTowerProgress(ctx); err != nil {
		t.Fatal(err)
	}
	if err = store.MigrateTowerProgress(ctx); err != nil {
		t.Fatal(err)
	}
	grief, err := store.ReadTowerProgress(ctx, account, TowerGriefPolicy, 1)
	if err != nil || grief.HighestCleared != 3 || grief.EntriesToday != 1 || grief.LastRun != strings.Repeat("b", 32) {
		t.Fatalf("migrated grief progress: %+v, %v", grief, err)
	}
	other := TowerPolicy{Key: "despair", TopFloor: 100, DailyEntries: 3, ResetHourUTC: 9}
	despair, err := store.ReadTowerProgress(ctx, account, other, 0)
	if err != nil || despair.HighestCleared != 0 || despair.EntriesToday != 0 {
		t.Fatalf("independent tower progress: %+v, %v", despair, err)
	}
	day := now.Add(48 * time.Hour)
	for i := 0; i < 3; i++ {
		if _, err = store.ReserveTowerEntry(ctx, account, other, 1, day); err != nil {
			t.Fatalf("other tower entry %d: %v", i+1, err)
		}
	}
	if _, err = store.ReserveTowerEntry(ctx, account, other, 1, day); err != nil {
		t.Fatalf("fourth same-day tower entry should be allowed during testing: %v", err)
	}
	if _, err = store.ReserveTowerEntry(ctx, account, TowerGriefPolicy, 4, day); err != nil {
		t.Fatalf("other tower entries consumed grief admission: %v", err)
	}
	grief, err = store.AdvanceTowerFloor(ctx, account, TowerGriefPolicy, 4, strings.Repeat("c", 32))
	if err != nil || grief.HighestCleared != 4 {
		t.Fatalf("grief floor advance: %+v, %v", grief, err)
	}
	if _, err = store.AdvanceTowerFloor(ctx, account, TowerGriefPolicy, 4, strings.Repeat("c", 32)); err != nil {
		t.Fatalf("idempotent floor advance: %v", err)
	}
	despair, err = store.ReadTowerProgress(ctx, account, other, 0)
	if err != nil || despair.HighestCleared != 0 || despair.EntriesToday != 4 {
		t.Fatalf("grief clear changed other tower: %+v, %v", despair, err)
	}
}
