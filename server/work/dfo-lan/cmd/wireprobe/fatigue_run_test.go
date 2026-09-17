package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/storage"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestFatigueRunLoadingIntegration(t *testing.T) {
	if os.Getenv("FATIGUE_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, err := storage.LoadConfig("../../runtime/swordmaster-pilot-20260916/storage.json")
	if err != nil {
		t.Fatal(err)
	}
	admin, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	schema := fmt.Sprintf("fatigue_run_test_%d", time.Now().UnixNano())
	if _, err = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); err != nil {
			t.Error(err)
		}
	}()
	cfg.PostgresSchema, cfg.RedisPrefix = schema, schema+":"
	s, err := storage.Open(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for _, migrate := range []func(context.Context) error{s.Migrate, s.MigrateFatigue} {
		if err = migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "fatigue-fixture")
	if err != nil {
		t.Fatal(err)
	}
	role, err := s.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "FatigueFixture", Request: []byte{0}, ConfigVersion: strings.Repeat("a", 64), State: []byte(`{}`)}, 24)
	if err != nil {
		t.Fatal(err)
	}
	f := &character.FatigueService{Store: s, Rules: character.FatigueRules{DailyLimit: 1, RoomCost: 1}, Location: time.UTC}
	run := strings.Repeat("a", 32)
	w := &worldSession{account: account, role: role, fatigue: f, activeDungeon: &dungeon.Session{RunID: run, Definition: catalog.DungeonDefinition{ID: 100004941}, Room: catalog.DungeonRoom{Map: 100016050}}}
	// Last point was consumed in the previous room. The captured zeroed CMD37
	// in map100016051 must still receive ACK37 and NOTI30, not a black screen.
	if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	w.activeDungeon.Room.Map = 100016051
	for i := 0; i < 2; i++ {
		plan, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatalf("last-point next room loading refused: %v", err)
		}
		ack, done, fp := false, false, false
		for _, packet := range plan {
			ack = ack || packet.Name == "dungeon_loading_ack"
			done = done || packet.Name == "dungeon_loading_complete"
			fp = fp || packet.Name == "dungeon_fatigue_updated"
		}
		if !ack || !done || !fp {
			t.Fatal("loading handshake incomplete")
		}
	}
	var cost, count int
	if err = s.DB.QueryRow(ctx, `SELECT count(*),sum(cost) FROM character_fatigue_rooms WHERE character_id=$1`, role.ID).Scan(&count, &cost); err != nil || count != 2 || cost != 1 {
		t.Fatal(count, cost, err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, _, err := f.EnterRoom(ctx, account, role.ID, run, uint32(100016052+i%2), false, time.Now()); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	fp, err := f.State(ctx, account, role.ID, time.Now())
	if err != nil || fp.Used != 1 || fp.UsedMax != 1 {
		t.Fatal("counter overflow", fp, err)
	}
	if _, _, err = f.EnterRoom(ctx, account, role.ID, strings.Repeat("b", 32), 1, false, time.Now()); !errors.Is(err, storage.ErrFatigueExhausted) {
		t.Fatal("new exhausted run admitted", err)
	}
	if _, _, err = f.EnterRoom(ctx, account+1, role.ID, run, 1, false, time.Now()); err == nil {
		t.Fatal("wrong owner admitted")
	}
	exempt := strings.Repeat("c", 32)
	if _, _, err = f.EnterRoom(ctx, account, role.ID, exempt, 1, true, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, _, err = f.EnterRoom(ctx, account, role.ID, exempt, 2, false, time.Now()); !errors.Is(err, storage.ErrFatigueExhausted) {
		t.Fatal("zero-cost ledger authorized paid run", err)
	}
	future := time.Now().Add(24 * time.Hour)
	if fp, _, err = f.EnterRoom(ctx, account, role.ID, strings.Repeat("d", 32), 1, false, future); err != nil || fp.Used != 1 {
		t.Fatal("rollover", fp, err)
	}
	if _, _, err = f.EnterRoom(ctx, account, role.ID, strings.Repeat("e", 32), 1, false, time.Now()); !errors.Is(err, storage.ErrFatigueExhausted) {
		t.Fatal("clock rollback granted quota", err)
	}
	t.Log("FATIGUE PASS: last point -> next room ACK37/NOTI30; retries and concurrent rooms clamped; new runs and wrong owner rejected; exempt ledger and rollover checked")
	// Raising today's configured cap must not wait for tomorrow or erase use.
	f.Rules.DailyLimit = 1056
	fp, err = f.State(ctx, account, role.ID, future)
	if err != nil || fp.Limit != 1056 || fp.Used != 1 || fp.UsedMax != 1 {
		t.Fatal("same-day cap migration", fp, err)
	}
	fp, err = f.State(ctx, account, role.ID, future.Add(24*time.Hour))
	if err != nil || fp.Limit != 1056 || fp.Used != 0 || fp.UsedMax != 0 {
		t.Fatal("daily full quota", fp, err)
	}
	f.Rules.DailyLimit = 156
	fp, err = f.State(ctx, account, role.ID, future)
	if err != nil || fp.Limit != 1056 {
		t.Fatal("backward day changed cap", fp, err)
	}
	t.Log("CAP PASS: same-day 1056 preserves use; next day resets; backward day preserves quota")
	testFatigueItems(t, s, role)
}
