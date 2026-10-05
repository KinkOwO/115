package workflow

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBagMoveRoundtripReplayIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	fixture, e := database.OpenTestFixture(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	s := fixture.Storage()
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
	source := pvf.ArchiveSnapshot{Checksum: h}
	role, e := s.CreateCharacter(ctx, database.Character{AccountID: account, Name: "BagFixture", Request: []byte{0}, ConfigVersion: source.SaveIdentity(), State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":68,"Template":14,"Amount":5}]}}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	c := catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{14: {Kind: "stackable", StackableType: "[etc]", StackLimit: 1000}}}
	rules := inventory.BagRules{Source: h, MissingStackLimit: 1000}
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
		b, e := inventory.ReadBag(saved.State)
		if e != nil || len(b.Items) != 1 || b.Items[0].Amount != 5 || b.Items[0].Slot != r.SourceSlot {
			t.Fatal("duplicate/lost item", i, b, e)
		}
		replay, _, yes, e := MoveStack(ctx, s, role, c, rules, r, key)
		if e != nil || yes {
			t.Fatal("duplicate frame applied", e, yes)
		}
		if string(replay.State) != string(saved.State) {
			a, _ := inventory.ReadBag(replay.State)
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
	b, _ := inventory.ReadBag(saved.State)
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
