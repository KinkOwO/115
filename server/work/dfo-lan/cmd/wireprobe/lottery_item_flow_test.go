package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLotteryCatalogMatchesCurrentIndex(t *testing.T) {
	index, err := catalog.LoadBoosterCatalog("", "../../internal/catalog/testdata/item-flow.json")
	if err != nil {
		t.Fatal(err)
	}
	pools, err := loadLotteryItemCatalog("../../configs/lottery-item-pools.json", index.Items)
	if err != nil {
		t.Fatal(err)
	}
	pool := pools.byTemplate[7772]
	if pool.total != 98904 || len(pool.Candidates) != 209 {
		t.Fatalf("unexpected pool: %d candidates, total %d", len(pool.Candidates), pool.total)
	}
	if first, err := pool.pick(0); err != nil || first.Template != 3600 {
		t.Fatalf("first weighted boundary: %+v, %v", first, err)
	}
	if last, err := pool.pick(pool.total - 1); err != nil || last.Template != 3619 {
		t.Fatalf("last weighted boundary: %+v, %v", last, err)
	}
	gold := pools.byTemplate[10306598]
	if gold == nil || gold.total != 10000 || gold.Candidates[0].Template != 0 || gold.Candidates[0].Count != 1000000 {
		t.Fatalf("Gold Refund Pot missing or malformed: %+v", gold)
	}
}

func TestLotteryEquipmentCatalogMatchesCurrentIndex(t *testing.T) {
	index, err := catalog.LoadBoosterCatalog("", "../../internal/catalog/testdata/item-flow.json")
	if err != nil {
		t.Fatal(err)
	}
	pools, err := loadLotteryItemCatalog("../../configs/lottery-item-pools.json", index.Items)
	if err != nil {
		t.Fatal(err)
	}
	count, err := loadLotteryEquipmentPools("../../configs/lottery-equipment-pools.json", index.Items, pools)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2477 || len(pools.byTemplate) != 2753 {
		t.Fatalf("unexpected equipment pools: %d, total %d", count, len(pools.byTemplate))
	}
	if p := pools.byTemplate[7213]; p == nil || len(p.Candidates) != 78 {
		t.Fatalf("Pokin armor pool missing: %+v", p)
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
	pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{7772: pool}}
	index := map[uint32]ItemIndexInfo{3600: {ID: 3600, Kind: "stackable", StackableType: "[material expert job]", StackLimit: 1000}}
	request := []byte{91, 0, 0, 0, 0, 0, 0, 0}
	packets, err := w.openLotteryItem(context.Background(), store, pools, index, request, request)
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
	if _, err := w.openLotteryItem(context.Background(), store, pools, index, request, []byte{1, 2, 3}); err == nil {
		t.Fatal("reopened consumed source")
	}
}

func TestLotteryGoldPotAtomicConsumeGrantAndWire(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: 50, Items: []inventory.BagItem{{Slot: 70, Template: 10306598, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 1, AccountID: 1, State: state}
	store := newMockBoosterStore(role)
	service := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}, BagRules: inventory.BagRules{Source: "test"}}
	w := &worldSession{role: role, loot: service}
	pool := &lotteryItemPool{SourceItem: 10306598, Candidates: []BoosterRewardCandidate{{Template: 0, Weight: 10000, Count: 1000000}}, total: 10000}
	pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{10306598: pool}}
	request := []byte{70, 0, 0, 0, 0, 0, 0, 0}
	packets, err := w.openLotteryItem(context.Background(), store, pools, nil, request, request)
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 2 || len(packets[0].Payload) != 186 || binary.LittleEndian.Uint16(packets[0].Payload[3:]) != 70 || binary.LittleEndian.Uint32(packets[0].Payload[7:]) != 0 || binary.LittleEndian.Uint32(packets[0].Payload[11:]) != 1000000 {
		t.Fatalf("wrong gold pot CMD27 result: %+v", packets)
	}
	after, err := inventory.ReadBag(store.character.State)
	if err != nil {
		t.Fatal(err)
	}
	if after.Gold != 1000050 || len(after.Items) != 0 {
		t.Fatalf("gold pot did not consume and grant atomically: %+v", after)
	}
	var receipt lotteryItemReceipt
	for _, data := range store.receipts {
		if err := json.Unmarshal(data, &receipt); err != nil {
			t.Fatal(err)
		}
	}
	if len(receipt.Updates) != 2 || binary.LittleEndian.Uint32(receipt.Updates[0][6:]) != 1000050 || binary.LittleEndian.Uint32(receipt.Updates[1][2:]) != inventory.DeletedTemplate {
		t.Fatalf("gold and depleted source not reflected in inventory update: %+v", receipt.Updates)
	}
}

func TestLotteryGoldPotOverflowPreservesSource(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1", Gold: math.MaxUint32, Items: []inventory.BagItem{{Slot: 70, Template: 10306598, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 1, AccountID: 1, State: state}
	store := newMockBoosterStore(role)
	w := &worldSession{role: role, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}, BagRules: inventory.BagRules{Source: "test"}}}
	pool := &lotteryItemPool{SourceItem: 10306598, Candidates: []BoosterRewardCandidate{{Template: 0, Weight: 1, Count: 1}}, total: 1}
	pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{10306598: pool}}
	request := []byte{70, 0, 0, 0, 0, 0, 0, 0}
	if _, err := w.openLotteryItem(context.Background(), store, pools, nil, request, request); err == nil {
		t.Fatal("gold overflow accepted")
	}
	after, err := inventory.ReadBag(store.character.State)
	if err != nil {
		t.Fatal(err)
	}
	if after.Gold != math.MaxUint32 || len(after.Items) != 1 || after.Items[0].Template != 10306598 {
		t.Fatalf("failed lottery mutated saved bag: %+v", after)
	}
}

type idempotentLotteryStore struct {
	*mockBoosterStore
	commitCalled bool
}

func (s *idempotentLotteryStore) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	s.commitCalled = true
	if _, ok := s.receipts[key]; ok {
		return s.character, false, fmt.Errorf("stored receipt must replay before event model check")
	}
	return s.mockBoosterStore.CommitCharacterEvent(ctx, account, id, version, key, model, apply)
}

func TestLotteryLegacyReceiptReplaysWithoutConsumingAnotherPot(t *testing.T) {
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 91, Template: 7772, Amount: 1}, {Slot: 121, Template: 3600, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 11, AccountID: 1, State: state}
	store := idempotentLotteryStore{mockBoosterStore: newMockBoosterStore(role)}
	request := []byte{91, 0, 0, 0, 0, 0, 0, 0}
	key := fmt.Sprintf("lottery-item-7772:%d:%x", role.ID, sha256.Sum256(request))
	store.receipts[key], err = json.Marshal(lotteryItemReceipt{RewardTemplate: 3600, RewardSlot: 121, GrantCount: 1, Updates: [][protocol.CurrentItemRecordSize]byte{protocol.OrdinaryItem(121, 3600, 1)}})
	if err != nil {
		t.Fatal(err)
	}
	w := &worldSession{role: role, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}}}
	pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{7772: {SourceItem: 7772, total: 1}}}
	packets, err := w.openLotteryItem(context.Background(), &store, pools, nil, request, request)
	if err != nil || len(packets) != 2 || binary.LittleEndian.Uint32(packets[0].Payload[7:]) != 3600 {
		t.Fatalf("legacy receipt replay failed: packets=%+v err=%v", packets, err)
	}
	after, err := inventory.ReadBag(store.character.State)
	if err != nil || store.commitCalled || len(after.Items) != 2 || after.Items[0].Template != 7772 {
		t.Fatalf("replay consumed a second pot: %+v err=%v", after, err)
	}
}

func lotteryEquipmentWear(t *testing.T) *workflow.WearService {
	t.Helper()
	data := []byte(`{"source":{"checksum":"test"},"rows":[{"ID":10858,"Path":"equipment/test.equ","SHA256":"0000000000000000000000000000000000000000000000000000000000000000","Fields":{"[rarity]":[{"type":0,"value":3}],"[equipment type]":[{"type":6,"text":"[coat]"},{"type":0,"value":18}],"[durability]":[{"type":0,"value":60}]}}]}`)
	path := filepath.Join(t.TempDir(), "equipment.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog(path, "test")
	if err != nil {
		t.Fatal(err)
	}
	return &workflow.WearService{WearService: inventory.WearService{Catalog: gear, BagRules: inventory.BagRules{EquipmentSlots: [2]uint16{9, 9}}}}
}

func TestLotteryEquipmentGrantAndFullBagRollback(t *testing.T) {
	wear := lotteryEquipmentWear(t)
	for _, full := range []bool{false, true} {
		bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 70, Template: 7213, Amount: 1}}}
		if full {
			bag.Equipment = []inventory.BagEquipment{{Slot: 9, Template: 10858, Durability: 60}}
		}
		state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
		if err != nil {
			t.Fatal(err)
		}
		role := storage.Character{ID: 11, AccountID: 1, State: state}
		store := newMockBoosterStore(role)
		w := &worldSession{role: role, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}}}
		pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{7213: {SourceItem: 7213, Candidates: []BoosterRewardCandidate{{Template: 10858, Weight: 1, Count: 1}}, total: 1}}}
		index := map[uint32]ItemIndexInfo{10858: {ID: 10858, Kind: "equipment"}}
		request := []byte{70, 0, 0, 0, 0, 0, 0, 0}
		packets, err := w.openLotteryItem(context.Background(), store, pools, index, request, request, wear)
		if full {
			if err == nil {
				t.Fatal("full equipment bag accepted")
			}
			after, readErr := inventory.ReadBag(store.character.State)
			if readErr != nil || len(after.Items) != 1 || len(after.Equipment) != 1 {
				t.Fatalf("failed grant changed bag: %+v %v", after, readErr)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(packets) != 2 || len(packets[0].Payload) != 186 || binary.LittleEndian.Uint16(packets[0].Payload[5:]) != 9 || binary.LittleEndian.Uint32(packets[0].Payload[7:]) != 10858 || binary.LittleEndian.Uint16(packets[0].Payload[16:]) != 60 {
			t.Fatalf("wrong armor result: %+v", packets)
		}
		after, readErr := inventory.ReadBag(store.character.State)
		if readErr != nil || len(after.Items) != 0 || len(after.Equipment) != 1 || after.Equipment[0].Durability != 60 {
			t.Fatalf("armor not committed: %+v %v", after, readErr)
		}
	}
}

func TestLotteryAvatarGrantAndReplay(t *testing.T) {
	wear := lotteryEquipmentWear(t)
	bag := inventory.Bag{Version: "ordinary-bag-v1", Items: []inventory.BagItem{{Slot: 70, Template: 7213, Amount: 1}}}
	state, err := inventory.SaveBag(json.RawMessage(`{}`), bag)
	if err != nil {
		t.Fatal(err)
	}
	role := storage.Character{ID: 11, AccountID: 1, State: state}
	store := newMockBoosterStore(role)
	w := &worldSession{role: role, loot: &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "test"}}}}
	pools := &lotteryItemCatalog{byTemplate: map[uint32]*lotteryItemPool{7213: {SourceItem: 7213, Candidates: []BoosterRewardCandidate{{Template: 48357, Weight: 1, Count: 1}}, total: 1}}}
	index := map[uint32]ItemIndexInfo{48357: {ID: 48357, Kind: "avatar"}}
	request := []byte{70, 0, 0, 0, 0, 0, 0, 0}
	packets, err := w.openLotteryItem(context.Background(), store, pools, index, request, request, wear)
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 3 || len(packets[0].Payload) != 190 || binary.LittleEndian.Uint32(packets[0].Payload[7:]) != 48357 || packets[2].Payload[0] != 1 {
		t.Fatalf("wrong avatar result: %+v", packets)
	}
	after, err := inventory.ReadBag(store.character.State)
	if err != nil || len(after.Items) != 0 || len(after.Special[1]) != 1 || after.Special[1][0].Template != 48357 {
		t.Fatalf("avatar not committed: %+v %v", after, err)
	}
	replay, err := w.openLotteryItem(context.Background(), store, pools, index, request, request, wear)
	if err != nil || len(replay) != 3 || string(replay[0].Payload) != string(packets[0].Payload) {
		t.Fatalf("avatar replay changed: %+v %v", replay, err)
	}
}
