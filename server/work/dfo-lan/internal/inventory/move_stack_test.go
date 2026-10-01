package inventory

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestBagMoveRoundtripReplayIntegration(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL schema")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	live, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer live.Close()
	schema := fmt.Sprintf("bag_move_%d", time.Now().UnixNano())
	if _, e = live.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer live.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
	cfg.PostgresSchema = schema
	cfg.RedisPrefix = schema + ":"
	s, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	for _, fn := range []func(context.Context) error{s.Migrate, s.MigrateCharacterEvents} {
		if e = fn(ctx); e != nil {
			t.Fatal(e)
		}
	}
	account, e := s.DevelopmentAccount(ctx, "bag-fixture")
	if e != nil {
		t.Fatal(e)
	}
	h := strings.Repeat("a", 64)
	role, e := s.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "BagFixture", Request: []byte{0}, ConfigVersion: h, State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":68,"Template":14,"Amount":5}]}}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	c := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: h}, Items: map[uint32]catalog.LootItem{14: {Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}}}
	rules := BagRules{Source: h, MissingStackLimit: 1000}
	raw, _ := hex.DecodeString("004c0000000000000000000044000e00000000000000ffffffff000000000000")
	out, e := protocol.DecodeItemMove(raw)
	if e != nil {
		t.Fatal(e)
	}
	back := out
	back.SourceSlot, back.DestinationSlot = out.DestinationSlot, out.SourceSlot
	for i, r := range []protocol.ItemMoveRequest{out, back, out, back, out} {
		key := fmt.Sprintf("bagmove:fixture:frame%d", i)
		saved, _, applied, e := MoveStack(ctx, s, role, c, rules, r, key)
		if e != nil || !applied {
			t.Fatal("new frame ignored", i, e, applied)
		}
		b, e := ReadBag(saved.State)
		if e != nil || len(b.Items) != 1 || b.Items[0].Amount != 5 || b.Items[0].Slot != r.SourceSlot {
			t.Fatal("duplicate/lost item", i, b, e)
		}
		replay, _, yes, e := MoveStack(ctx, s, role, c, rules, r, key)
		if e != nil || yes {
			t.Fatal("duplicate frame applied", e, yes)
		}
		if string(replay.State) != string(saved.State) {
			a, _ := ReadBag(replay.State)
			if len(a.Items) != 1 || a.Items[0] != b.Items[0] {
				t.Fatal("replay changed state")
			}
		}
		role = saved
	}
	// Replay the first event after later moves: never reinstall its old state.
	saved, _, yes, e := MoveStack(ctx, s, role, c, rules, out, "bagmove:fixture:frame0")
	if e != nil || yes {
		t.Fatal(e, yes)
	}
	b, _ := ReadBag(saved.State)
	if len(b.Items) != 1 || b.Items[0].Amount != 5 {
		t.Fatal(b)
	}
	stale := out
	stale.SourceItem = 14
	if _, _, _, e = MoveStack(ctx, s, role, c, rules, stale, "bagmove:fixture:stale"); e == nil {
		t.Fatal("phantom source accepted")
	}
	t.Log("PASS five real alternating drags; each new frame applies; exact replay no-op; one stack of5 preserved; stale icon rejected")
}
