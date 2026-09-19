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

type mockBoosterStore struct {
	character storage.Character
	receipts  map[string]json.RawMessage
}

func newMockBoosterStore(char storage.Character) *mockBoosterStore {
	return &mockBoosterStore{
		character: char,
		receipts:  make(map[string]json.RawMessage),
	}
}

func (m *mockBoosterStore) CommitCharacterEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error) {
	newState, receipt, err := apply(m.character)
	if err != nil {
		return m.character, false, err
	}
	m.character.State = newState
	m.receipts[key] = receipt
	return m.character, true, nil
}

func (m *mockBoosterStore) CharacterEventReceipt(ctx context.Context, account, id int64, key string) (json.RawMessage, error) {
	return m.receipts[key], nil
}

func TestBoosterUseTitleBox(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 65, Template: 590722923, Amount: 1, ExpireTime: 2147483647},
		},
	}
	state, _ := inventory.SaveBag(json.RawMessage(`{}`), bag)
	char := storage.Character{
		ID:        10,
		AccountID: 1,
		State:     state,
	}
	store := newMockBoosterStore(char)

	lootSvc := &loot.Service{
		Catalog: catalog.LootCatalog{
			Source: pvf.ArchiveSnapshot{Checksum: "test"},
			Items: map[uint32]catalog.LootItem{
				590722923: {ID: 590722923, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000},
				590722924: {ID: 590722924, Kind: "stackable", StackableType: "[booster selection]", StackLimit: 1000},
				590722925: {ID: 590722925, Kind: "stackable", StackableType: "[booster selection]", StackLimit: 1000},
			},
		},
		BagRules: inventory.BagRules{
			Source: "test",
			Slots: map[string][2]uint16{
				"[booster]":           {65, 120},
				"[booster selection]": {65, 120},
			},
			MissingStackLimit: 1000,
		},
	}

	w := &worldSession{
		role: char,
		loot: lootSvc,
	}

	// Request: slot 65, amount 1, category 0 (8 bytes)
	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open booster failed:", err)
	}

	if len(packets) < 2 {
		t.Fatalf("expected at least 2 packets (inv update + ack), got %d", len(packets))
	}
	last := packets[len(packets)-1]
	if last.Name != "booster_open_ack" || last.ID != 160 {
		t.Fatalf("unexpected last packet (expected booster_open_ack): %+v", last)
	}

	// Verify bag after opening
	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	// Original box in slot 65 should be gone
	for _, it := range resBag.Items {
		if it.Template == 590722923 {
			t.Fatal("box 590722923 was not consumed")
		}
	}
	// Should have gained 590722924 and 590722925
	gained := map[uint32]bool{}
	for _, it := range resBag.Items {
		gained[it.Template] = true
		if it.ExpireTime != MaxExpireTime {
			t.Fatalf("item %d missing expiration time: %d", it.Template, it.ExpireTime)
		}
	}
	if !gained[590722924] || !gained[590722925] {
		t.Fatalf("expected 590722924 and 590722925 in bag, got %+v", resBag.Items)
	}
}

func TestBoosterUseAvatarBoxSelection(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 65, Template: 590722922, Amount: 1, ExpireTime: 2147483647},
		},
	}
	state, _ := inventory.SaveBag(json.RawMessage(`{}`), bag)
	char := storage.Character{
		ID:        10,
		AccountID: 1,
		State:     state,
	}
	store := newMockBoosterStore(char)

	w := &worldSession{
		role: char,
	}

	// 8 avatars selected:
	avatars := []uint32{
		517552707, 517562666, 517572669, 517522680,
		517502703, 517512687, 517532669, 517542683,
	}
	reqBytes := make([]byte, 8+len(avatars)*4)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 16)
	for i, av := range avatars {
		binary.LittleEndian.PutUint32(reqBytes[8+i*4:12+i*4], av)
	}

	packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open avatar booster failed:", err)
	}

	// Packets: main inv update (14), avatar inv update (14 space 1), ack (160)
	if len(packets) != 3 {
		t.Fatalf("expected 3 packets, got %d", len(packets))
	}
	if packets[1].Name != "booster_avatar_inventory_updated" {
		t.Fatalf("expected avatar update packet at index 1, got %s", packets[1].Name)
	}
	if packets[2].Name != "booster_open_ack" {
		t.Fatalf("expected ack packet at index 2, got %s", packets[2].Name)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Special[1]) != 8 {
		t.Fatalf("expected 8 avatars in space 1, got %d", len(resBag.Special[1]))
	}
	for i, av := range resBag.Special[1] {
		if av.Template != avatars[i] {
			t.Fatalf("avatar %d mismatch: got %d want %d", i, av.Template, avatars[i])
		}
		if av.Period != MaxExpireTime {
			t.Fatalf("avatar %d period mismatch: %d", i, av.Period)
		}
	}
}

func TestBoosterUseCreatureBoxSelection(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 70, Template: 590722926, Amount: 1, ExpireTime: 2147483647},
		},
	}
	state, _ := inventory.SaveBag(json.RawMessage(`{}`), bag)
	char := storage.Character{
		ID:        10,
		AccountID: 1,
		State:     state,
	}
	store := newMockBoosterStore(char)

	w := &worldSession{
		role: char,
	}

	// 1 creature selected (500331099 - in items.index.json it's a title in equipment/character/common/title/)
	reqBytes := make([]byte, 16)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 70)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)
	binary.LittleEndian.PutUint32(reqBytes[8:12], 500331099)

	packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open creature booster failed:", err)
	}

	if len(packets) < 2 {
		t.Fatalf("expected at least 2 packets, got %d", len(packets))
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Equipment) != 1 {
		t.Fatalf("expected 1 equipment in b.Equipment, got %d", len(resBag.Equipment))
	}
	if resBag.Equipment[0].Template != 500331099 {
		t.Fatalf("equipment template mismatch: got %d", resBag.Equipment[0].Template)
	}
}
