package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestIspinsPotionPostgresAtomicLimitAndReplay(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("isolated PostgreSQL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer fixture.Close()
	runIspinsPotionAtomicLimitAndReplay(t, ctx, fixture.Storage())
}

func runIspinsPotionAtomicLimitAndReplay(t *testing.T, ctx context.Context, store *database.Store) {
	t.Helper()
	if err := store.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateCharacterEvents(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigrateGrants(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.MigratePremiums(ctx); err != nil {
		t.Fatal(err)
	}
	account, err := store.DevelopmentAccount(ctx, "ispins-potion-guard")
	if err != nil {
		t.Fatal(err)
	}
	source := pvf.ArchiveSnapshot{Checksum: strings.Repeat("a", 64)}
	state, err := inventory.SaveBag(json.RawMessage(`{"opaque":{"future":17}}`), inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 65, Template: 700, Amount: 4}, {Slot: 66, Template: 701, Amount: 2}}})
	if err != nil {
		t.Fatal(err)
	}
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "IspinsPotionGuard", ConfigVersion: source.SaveIdentity(), Request: []byte{0, 255}, State: state}, 24)
	if err != nil {
		t.Fatal(err)
	}
	role.WireID = 7
	w := &worldSession{store: store, role: role, items: &inventory.ItemService{Model: "ispins-potion-test", Catalog: catalog.LootCatalog{Source: source, Items: map[uint32]catalog.LootItem{700: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}, 701: {Kind: "stackable", StackableType: "[etc]", StackLimit: 100}}}}, activeDungeon: &dungeon.Session{Loaded: true}, ispins: &ispinsRun{orderOwned: true, potionLimit: 2}, dungeons: &catalog.DungeonCatalog{StackableLimits: &catalog.StackableDungeonLimits{Potions: map[uint32]bool{700: true}}}}
	request := func(slot uint16, id, instance uint32) []byte {
		p := make([]byte, 16)
		binary.LittleEndian.PutUint16(p, slot)
		binary.LittleEndian.PutUint32(p[3:], instance)
		binary.LittleEndian.PutUint32(p[7:], id)
		return p
	}
	use := func(slot uint16, id, instance uint32) error {
		_, err := w.useStackable(request(slot, id, instance), func(map[string]any) {})
		return err
	}
	amount := func(id uint32) uint32 {
		t.Helper()
		b, err := inventory.ReadBag(w.role.State)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range b.Items {
			if r.Template == id {
				return r.Amount
			}
		}
		return 0
	}
	// A bad slot is rejected within the transaction and does not use quota.
	if err := use(99, 700, 1); err == nil || w.ispins.potionUsed != 0 {
		t.Fatal("failed use counted", err)
	}
	for _, instance := range []uint32{10, 11} {
		if err := use(65, 700, instance); err != nil {
			t.Fatal(err)
		}
	}
	if w.ispins.potionUsed != 2 || amount(700) != 2 {
		t.Fatal("successful uses did not count once")
	}
	// Replays at the cap must still return their committed receipt.
	for _, instance := range []uint32{10, 11} {
		if err := use(65, 700, instance); err != nil {
			t.Fatal("receipt blocked by cap", err)
		}
	}
	if w.ispins.potionUsed != 2 || amount(700) != 2 {
		t.Fatal("replay spent item or quota")
	}
	before := string(w.role.State)
	if err := use(65, 700, 12); err == nil || string(w.role.State) != before {
		t.Fatal("over-limit use committed", err)
	}
	if err := use(66, 701, 13); err != nil || w.ispins.potionUsed != 2 || amount(701) != 1 {
		t.Fatal("unlisted item counted as potion", err)
	}
	if !strings.Contains(string(w.role.State), `"future":17`) {
		t.Fatal("opaque legacy state lost")
	}
	// A new attempt has its own cap; a previous committed request remains a replay.
	w.ispins.potionUsed = 0
	if err := use(65, 700, 11); err != nil || w.ispins.potionUsed != 0 || amount(700) != 2 {
		t.Fatal("previous receipt counted on new attempt", err)
	}
	if err := use(65, 700, 14); err != nil || w.ispins.potionUsed != 1 || amount(700) != 1 {
		t.Fatal("fresh attempt quota failed", err)
	}
}

func TestIspinsCoinSourceCapBeforeChargeAndReplay(t *testing.T) {
	w, store, p := ceraReviveFixture(t, 0)
	store.balance = 90
	cap := int32(2)
	w.ispins = &ispinsRun{operationIndex: 1}
	w.dungeons.IspinsOperations = map[uint16]catalog.IspinsOperationDefinition{1: {AllowCoin: &cap}}
	for i := byte(1); i <= 2; i++ {
		w.pilotDeath.Dead = true
		w.pilotDeath.Sequence = uint32(i)
		frame := []byte{41, i}
		if _, err := w.useCoinRevive(context.Background(), store, p, frame, false); err != nil {
			t.Fatal(err)
		}
		if _, err := w.useCoinRevive(context.Background(), store, p, frame, false); err != nil {
			t.Fatal("replay refused", err)
		}
	}
	if w.ispins.coinsUsed != 2 || store.balance != 60 || len(store.grants) != 2 {
		t.Fatal("coin replay counted twice")
	}
	w.pilotDeath.Dead = true
	w.pilotDeath.Sequence = 3
	if _, err := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false); err == nil || !w.pilotDeath.Dead || store.balance != 60 || len(store.grants) != 2 {
		t.Fatal("source cap charged account", err)
	}
}
