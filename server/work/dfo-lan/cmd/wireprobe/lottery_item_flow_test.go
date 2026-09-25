package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestLottery7772CatalogMatchesCurrentIndex(t *testing.T) {
	index, err := LoadBoosterCatalog("", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}
	pool, err := loadLotteryItemPool("../../configs/lottery-item-7772.json", index.Items)
	if err != nil {
		t.Fatal(err)
	}
	if pool.total != 98904 || len(pool.Candidates) != 209 {
		t.Fatalf("unexpected pool: %d candidates, total %d", len(pool.Candidates), pool.total)
	}
	if first, err := pool.pick(0); err != nil || first.Template != 3600 {
		t.Fatalf("first weighted boundary: %+v, %v", first, err)
	}
	if last, err := pool.pick(pool.total - 1); err != nil || last.Template != 3619 {
		t.Fatalf("last weighted boundary: %+v, %v", last, err)
	}
}

func TestLotteryItemAtomicConsumeGrantAndWire(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 91, Template: 7772, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 11, AccountID: 1, State: state}
	store := newMockBoosterStore(role)
	service := &loot.Service{
		Catalog:  catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}},
		BagRules: inventory.BagRules{Source: "test", Slots: map[string][2]uint16{"[material expert job]": {121, 176}}, MissingStackLimit: 1000},
	}
	w := &worldSession{role: role, loot: service}
	pool := &lotteryItemPool{SourceItem: 7772, Candidates: []BoosterRewardCandidate{{Template: 3600, Weight: 1, Count: 1}}, total: 1}
	index := map[uint32]ItemIndexInfo{3600: {ID: 3600, Kind: "stackable", StackableType: "[material expert job]", StackLimit: 1000}}
	request := []byte{91, 0, 0, 0, 0, 0, 0, 0}
	packets, err := w.openLotteryItem(context.Background(), store, pool, index, request, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 2 || packets[0].ID != 27 || packets[1].ID != 14 || len(packets[0].Payload) != 186 {
		t.Fatalf("unexpected lottery response: %+v", packets)
	}
	if binary.LittleEndian.Uint16(packets[0].Payload[3:]) != 91 || binary.LittleEndian.Uint32(packets[0].Payload[7:]) != 3600 {
		t.Fatalf("wrong CMD27 result body: %x", packets[0].Payload[:15])
	}
	after, err := inventory.ReadBag(store.character.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Items) != 1 || after.Items[0].Template != 3600 || after.Items[0].Amount != 1 {
		t.Fatalf("consume and reward not atomic: %+v", after.Items)
	}
	if _, err := w.openLotteryItem(context.Background(), store, pool, index, request, []byte{1, 2, 3}); err == nil {
		t.Fatal("reopened consumed source")
	}
}
