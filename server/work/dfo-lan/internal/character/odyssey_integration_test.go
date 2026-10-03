package character_test

import (
	"context"
	"dfolan/internal/catalog"
	. "dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestOdysseyGrowthDatabaseReplay(t *testing.T) {
	equalState := func(a, b json.RawMessage) bool {
		var x, y map[string]any
		if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
			return false
		}
		return reflect.DeepEqual(x, y)
	}
	if os.Getenv("ODYSSEY_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("../../runtime/swordmaster-pilot-20260916/storage.json")
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("odyssey_growth_test_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	store, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	for _, f := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if e = f(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := store.DevelopmentAccount(ctx, "odyssey-growth-fixture")
	if e != nil {
		t.Fatal(e)
	}
	s, role := OdysseyGrowthFixtureForTest(t)
	s.Store = store
	role.AccountID = account
	role.WireID = 0
	role, e = store.CreateCharacter(ctx, role, 24)
	if e != nil {
		t.Fatal(e)
	}
	run := &dungeon.Session{RunID: "0123456789abcdef0123456789abcdef", Loaded: true, Definition: catalog.DungeonDefinition{ID: 100004934, Odyssey: true}, Room: catalog.DungeonRoom{Boss: true}, Monsters: []protocol.DungeonMonster{{Entity: 4096, Rank: 3}}, Dead: map[uint16]bool{}, Unowned: map[uint16]bool{}}
	if _, _, e = s.OdysseyClear(ctx, role, run); e == nil {
		t.Fatal("unfinished run granted growth")
	}
	if _, e = run.ConfirmDeath(4096, role.WireID, role.WireID); e != nil {
		t.Fatal(e)
	}
	if e = run.BossCheck(protocol.BossCheckRequest{Actor: role.WireID, Target: 4096}, role.WireID); e != nil {
		t.Fatal(e)
	}
	next, applied, e := s.OdysseyClear(ctx, role, run)
	if e != nil || !applied {
		t.Fatal(applied, e)
	}
	replay, applied, e := s.OdysseyClear(ctx, role, run)
	if e != nil || applied || !equalState(replay.State, next.State) {
		t.Fatal("clear replay", e)
	}
	var state State
	json.Unmarshal(next.State, &state)
	if state.Level != 15 {
		t.Fatal(state.Level)
	}
	journal, e := s.OdysseyProgressPayload(replay)
	if e != nil || len(journal) != 280 || binary.LittleEndian.Uint32(journal) != 100004934 || binary.LittleEndian.Uint32(journal[4:]) != 0 {
		t.Fatal("committed journal/replay", journal, e)
	}
	// Advance the disposable fixture only, then retry its three independent gifts.
	next, _, e = store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "test-level90", "fixture", func(r Character) (json.RawMessage, json.RawMessage, error) {
		n, e := s.ApplyOdysseyTarget(r, 90)
		return n.State, json.RawMessage(`{}`), e
	})
	if e != nil {
		t.Fatal(e)
	}
	next, applied, pending := s.OdysseyGifts(ctx, next)
	if !applied || len(pending) != 0 {
		t.Fatal(applied, pending)
	}
	replay, applied, pending = s.OdysseyGifts(ctx, next)
	if applied || len(pending) != 0 || !equalState(replay.State, next.State) {
		t.Fatal("gift replay", pending)
	}
	b, e := inventory.ReadBag(next.State)
	if e != nil || len(b.Items) != 3 {
		t.Fatal("milestone inventory", b, e)
	}
	t.Log("DB PASS first clear ->15; stale replay unchanged; level90 grants three gifts exactly once; gameplay roles untouched")
}
