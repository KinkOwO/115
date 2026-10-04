package database

import (
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestSQLCFatigueRolloverLedgerAndRecovery(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	for _, migrate := range []func() error{
		func() error { return s.Migrate(ctx) },
		func() error { return s.MigrateFatigue(ctx) },
		func() error { return s.MigrateCharacterEvents(ctx) },
	} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	account, err := s.DevelopmentAccount(ctx, "fatigue")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("b", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Fatigue", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"unknown":{"keep":true},"potions":2}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	run := strings.Repeat("1", 32)
	var wg sync.WaitGroup
	applied := make(chan bool, 2)
	failures := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ok, err := s.ConsumeRoomFatigue(ctx, account, role.ID, "2026-10-04", 10, run, 9, 7)
			applied <- ok
			failures <- err
		}()
	}
	wg.Wait()
	close(applied)
	close(failures)
	successes := 0
	for ok := range applied {
		if ok {
			successes++
		}
	}
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	if successes != 1 {
		t.Fatalf("duplicate room charged %d times", successes)
	}
	state, ok, err := s.ConsumeRoomFatigue(ctx, account, role.ID, "2026-10-04", 10, run, 10, 7)
	if err != nil || !ok || state.Used != 10 || state.UsedMax != 10 {
		t.Fatalf("clamp: %+v %v %v", state, ok, err)
	}
	if _, ok, err := s.ConsumeRoomFatigue(ctx, account, role.ID, "2026-10-04", 10, run, 11, 7); err != nil || !ok {
		t.Fatalf("paid run stranded: %v %v", ok, err)
	}
	if _, _, err := s.ConsumeRoomFatigue(ctx, account, role.ID, "2026-10-04", 10, strings.Repeat("2", 32), 9, 7); !errors.Is(err, ErrFatigueExhausted) {
		t.Fatalf("new run allowed: %v", err)
	}
	charged, rooms, err := s.RunFatigueLedger(ctx, account, role.ID, run)
	if err != nil || charged != 10 || rooms != 3 {
		t.Fatalf("ledger: %d %d %v", charged, rooms, err)
	}
	if paid, err := s.RunPaidFatigue(ctx, role.ID, run); err != nil || !paid {
		t.Fatalf("paid: %v %v", paid, err)
	}
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	recovery := FatigueRecovery{Day: "2026-10-04", Limit: 10, Amount: 4, Template: 100, DailyUses: 2, Cooldown: time.Minute, Now: now}
	boom := errors.New("inventory failure")
	if _, _, err := s.RecoverFatigue(ctx, account, role.ID, version, recovery, func(Character) (json.RawMessage, error) { return nil, boom }); !errors.Is(err, boom) {
		t.Fatal(err)
	}
	var receipts int
	if err := testPool(t, s).QueryRow(ctx, `SELECT count(*) FROM character_fatigue_recovery`).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("failed recovery recorded: %d %v", receipts, err)
	}
	saved, state, err := s.RecoverFatigue(ctx, account, role.ID, version, recovery, func(c Character) (json.RawMessage, error) {
		return json.RawMessage(`{"unknown":{"keep":true},"potions":1}`), nil
	})
	if err != nil || state.Used != 6 || state.UsedMax != 10 || !sameJSON(t, saved.State, json.RawMessage(`{"unknown":{"keep":true},"potions":1}`)) {
		t.Fatalf("recovery: %+v %v", state, err)
	}
	callback := false
	if _, _, err := s.RecoverFatigue(ctx, account, role.ID, version, recovery, func(c Character) (json.RawMessage, error) { callback = true; return c.State, nil }); err == nil || callback {
		t.Fatalf("cooldown bypass: %v %v", callback, err)
	}
	recovery.Now = now.Add(2 * time.Minute)
	if _, state, err = s.RecoverFatigue(ctx, account, role.ID, version, recovery, func(c Character) (json.RawMessage, error) { return c.State, nil }); err != nil || state.Used != 2 || state.UsedMax != 10 {
		t.Fatalf("second recovery: %+v %v", state, err)
	}
	recovery.Now = now.Add(4 * time.Minute)
	if _, _, err := s.RecoverFatigue(ctx, account, role.ID, version, recovery, func(c Character) (json.RawMessage, error) { return c.State, nil }); err == nil {
		t.Fatal("daily quota bypass")
	}
	state, err = s.LoadFatigue(ctx, account, role.ID, "2026-10-03", 20)
	if err != nil || state.Day != "2026-10-04" || state.Limit != 10 || state.Used != 2 {
		t.Fatalf("backwards clock reset: %+v %v", state, err)
	}
	state, err = s.LoadFatigue(ctx, account, role.ID, "2026-10-05", 20)
	if err != nil || state.Used != 0 || state.UsedMax != 0 || state.Limit != 20 {
		t.Fatalf("rollover: %+v %v", state, err)
	}
	if _, err = s.LoadFatigue(ctx, account+100, role.ID, "2026-10-05", 20); !errors.Is(err, ErrNotFound) {
		t.Fatalf("ownership: %v", err)
	}
	if _, err = testPool(t, s).Exec(ctx, `INSERT INTO character_events(character_id,event_key,config_version,model,outcome) VALUES($1,$2,$3,'test','{"gain":4294967296}')`, role.ID, "monster:"+run+":1", version); err != nil {
		t.Fatal(err)
	}
	if total, err := s.RunMonsterExperience(ctx, account, role.ID, run); err != nil || total != 1<<32 {
		t.Fatalf("experience truncated: %d %v", total, err)
	}
	if total, err := s.RunMonsterExperience(ctx, account+100, role.ID, run); err != nil || total != 0 {
		t.Fatalf("experience ownership: %d %v", total, err)
	}
}

func TestSQLCBirthAndWorldPreserveExistingProgress(t *testing.T) {
	s, ctx := sqlcTestStore(t)
	if err := s.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := s.DevelopmentAccount(ctx, "world")
	if err != nil {
		t.Fatal(err)
	}
	version := strings.Repeat("c", 64)
	role, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Legacy", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{"unknown":true}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	for _, migrate := range []func() error{func() error { return s.MigrateBirth(ctx) }, func() error { return s.MigrateWorld(ctx) }} {
		if err := migrate(); err != nil {
			t.Fatal(err)
		}
	}
	if stage, dungeon, err := s.BirthStage(ctx, account, role.ID); err != nil || stage != BirthComplete || dungeon != 0 {
		t.Fatalf("legacy birth reset: %d %d %v", stage, dungeon, err)
	}
	fresh, err := s.CreateCharacter(ctx, Character{AccountID: account, Name: "Fresh", Request: []byte{1}, ConfigVersion: version, State: json.RawMessage(`{}`)}, 4)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StartBirth(ctx, account, fresh.ID); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.AdvanceBirth(ctx, account, fresh.ID, BirthEntered, 1<<31); err != nil || !ok {
		t.Fatalf("birth advance: %v %v", ok, err)
	}
	if err := s.MigrateBirth(ctx); err != nil {
		t.Fatal(err)
	}
	if stage, dungeon, err := s.BirthStage(ctx, account, fresh.ID); err != nil || stage != BirthEntered || dungeon != 1<<31 {
		t.Fatalf("recorded birth overwritten: %d %d %v", stage, dungeon, err)
	}
	if ok, err := s.AdvanceBirth(ctx, account, fresh.ID, BirthPending, 0); err != nil || ok {
		t.Fatalf("birth reversed: %v %v", ok, err)
	}
	if _, _, err := s.BirthStage(ctx, account+100, fresh.ID); err == nil {
		t.Fatal("birth ownership bypass")
	}
	initial := WorldPosition{Town: 1, Area: 2, X: 3, Y: 4, Return: &WorldReturn{Town: 5, X: 6}}
	state, err := s.LoadWorld(ctx, account, role.ID, 0, initial, version)
	if err != nil || state.Revision != 1 {
		t.Fatalf("world init: %+v %v", state, err)
	}
	next := initial
	next.X = 100
	changed, err := s.SaveWorld(ctx, account, role.ID, 0, state, next)
	if err != nil || changed.Revision != 2 {
		t.Fatalf("world save: %+v %v", changed, err)
	}
	if _, err := s.SaveWorld(ctx, account, role.ID, 0, state, initial); !errors.Is(err, ErrWorldConflict) {
		t.Fatalf("stale revision accepted: %v", err)
	}
	restored, err := s.LoadWorld(ctx, account, role.ID, 0, initial, strings.Repeat("d", 64))
	if err != nil || restored.Position.X != 100 || restored.ConfigVersion != version {
		t.Fatalf("ordinary world reset: %+v %v", restored, err)
	}
	special := WorldPosition{Town: 239, X: 1}
	entry, err := s.LoadWorld(ctx, account, role.ID, 3, special, version)
	if err != nil {
		t.Fatal(err)
	}
	session := special
	session.X = 70
	if _, err := s.SaveWorld(ctx, account, role.ID, 3, entry, session); err != nil {
		t.Fatal(err)
	}
	special.X = 9
	reentry, err := s.LoadWorld(ctx, account, role.ID, 3, special, version)
	if err != nil || reentry.Position.X != 9 || reentry.Revision != 1 {
		t.Fatalf("special position restored: %+v %v", reentry, err)
	}
	if restored, err := s.LoadWorld(ctx, account, role.ID, 0, initial, version); err != nil || restored.Position.X != 100 {
		t.Fatalf("channel overwrote ordinary: %+v %v", restored, err)
	}
	if _, err := s.LoadWorld(ctx, account+100, role.ID, 0, initial, version); !errors.Is(err, ErrNotFound) {
		t.Fatalf("world ownership: %v", err)
	}
	if n, err := s.ScrubPollutedWorldPositions(ctx, []uint32{239}); err != nil || n != 0 {
		t.Fatalf("clean world scrubbed: %d %v", n, err)
	}
	changed.Position.Town = 239
	if _, err := s.SaveWorld(ctx, account, role.ID, 0, changed, changed.Position); err != nil {
		t.Fatal(err)
	}
	if n, err := s.ScrubPollutedWorldPositions(ctx, []uint32{239}); err != nil || n != 1 {
		t.Fatalf("polluted world kept: %d %v", n, err)
	}
}
