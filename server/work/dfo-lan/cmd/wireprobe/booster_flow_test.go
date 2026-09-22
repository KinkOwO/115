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
	premiums  map[uint8]int64
}

func newMockBoosterStore(char storage.Character) *mockBoosterStore {
	return &mockBoosterStore{
		character: char,
		receipts:  make(map[string]json.RawMessage),
		premiums:  make(map[uint8]int64),
	}
}

func (m *mockBoosterStore) ActivatePremium(ctx context.Context, account int64, premiumType uint8, durationSecond int64) (int64, error) {
	oldEnd := m.premiums[premiumType]
	now := int64(1750000000)
	if oldEnd > now {
		now = oldEnd
	}
	newEnd := now + durationSecond
	m.premiums[premiumType] = newEnd
	return newEnd, nil
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

func TestBoosterUseSkinAvatarBoxWithAbilityOption(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	// 50020139: skin_avatar_box.stk
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 65, Template: 50020139, Amount: 1, ExpireTime: 2147483647},
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

	// Client sub_14573DC60 wire request:
	// slot: 65, amount: 1, category: 5 (Gunner F)
	// selection: 505580001 (Copper Brown Skin)
	// avatar_count: 1
	// avatar entry: 505580001, option: 1 (Magical Def 850)
	// trailing zero: 0
	reqBytes := make([]byte, 32)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 5)
	binary.LittleEndian.PutUint32(reqBytes[8:12], 505580001)
	reqBytes[12] = 1 // avatar count
	binary.LittleEndian.PutUint32(reqBytes[13:17], 505580001)
	reqBytes[17] = 1 // option index 1
	reqBytes[18] = 0 // trailing zero

	packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open skin avatar booster failed:", err)
	}

	if len(packets) != 3 {
		t.Fatalf("expected 3 packets, got %d", len(packets))
	}
	if packets[1].Name != "booster_avatar_inventory_updated" {
		t.Fatalf("expected avatar update packet, got %s", packets[1].Name)
	}
	if packets[2].Name != "booster_open_ack" {
		t.Fatalf("expected ack packet, got %s", packets[2].Name)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Special[1]) != 1 {
		t.Fatalf("expected 1 avatar in Special[1], got %d", len(resBag.Special[1]))
	}
	av := resBag.Special[1][0]
	if av.Template != 505580001 {
		t.Fatalf("avatar template mismatch: got %d, want 505580001", av.Template)
	}
	if av.Durability != 1 {
		t.Fatalf("avatar option (durability) mismatch: got %d, want 1", av.Durability)
	}
	if av.Period != MaxExpireTime {
		t.Fatalf("avatar period mismatch: got %d, want %d", av.Period, MaxExpireTime)
	}
	// Box at slot 65 must be consumed
	for _, it := range resBag.Items {
		if it.Slot == 65 {
			t.Fatalf("box at slot 65 was not consumed")
		}
	}
}

func TestBoosterOdysseyModeOpensRegularBooster(t *testing.T) {
	t.Setenv("DFO_ODYSSEY_REWARDS_RELEASE", "1")

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

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open booster in odyssey mode failed:", err)
	}

	if len(packets) < 2 {
		t.Fatalf("expected at least 2 packets, got %d", len(packets))
	}
}

func TestBoosterOpenLifeTokenBox(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	// 10333027: Life Token Box (3)
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Coin:    10,
		Items: []inventory.BagItem{
			{Slot: 76, Template: 10333027, Amount: 1},
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
			Items:  map[uint32]catalog.LootItem{},
		},
		BagRules: inventory.BagRules{
			Source: "test",
			Slots: map[string][2]uint16{
				"[booster]": {65, 120},
				"[etc]":     {65, 120},
			},
			MissingStackLimit: 1000,
		},
	}

	w := &worldSession{
		role: char,
		loot: lootSvc,
	}

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 76)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open life token box failed:", err)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if resBag.Coin != 13 {
		t.Fatalf("expected coin to be 13, got %d", resBag.Coin)
	}
	for _, it := range resBag.Items {
		if it.Slot == 76 {
			t.Fatalf("box at slot 76 was not consumed")
		}
	}
	if len(packets) < 2 {
		t.Fatalf("expected at least 2 packets, got %d", len(packets))
	}
}

func TestBoosterOpenMasterContractPackage(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	// 10333726: Master Contract Package 3 Days
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 77, Template: 10333726, Amount: 1},
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
			Items:  map[uint32]catalog.LootItem{},
		},
		BagRules: inventory.BagRules{
			Source: "test",
			Slots: map[string][2]uint16{
				"[booster]":  {65, 120},
				"[etc]":      {65, 120},
				"[contract]": {65, 120},
			},
			MissingStackLimit: 1000,
		},
	}

	w := &worldSession{
		role: char,
		loot: lootSvc,
	}

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 77)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open master contract package failed:", err)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Items) != 0 {
		t.Fatalf("expected 0 items in bag (contracts directly activated), got %d: %+v", len(resBag.Items), resBag.Items)
	}
	for _, it := range resBag.Items {
		if it.Slot == 77 {
			t.Fatalf("box at slot 77 was not consumed")
		}
	}
	// Check that 4 premiums (22: Conqueror, 27: Tactician, 92: Cube, 79: Growth) were activated for 3 days (259200s)
	expectedEnd := int64(1750000000) + 3*86400
	for _, pt := range []uint8{22, 27, 92, 79} {
		if store.premiums[pt] != expectedEnd {
			t.Fatalf("expected premium %d end time %d, got %d", pt, expectedEnd, store.premiums[pt])
		}
	}

	// Packets: inventory update, 4x NOTI 66, ACK 160 -> total 6 packets
	if len(packets) != 6 {
		t.Fatalf("expected 6 packets (update + 4x noti66 + ack), got %d", len(packets))
	}
	noti66Count := 0
	for _, p := range packets {
		if p.ID == 66 && p.Name == "booster_special_item_noti" {
			noti66Count++
		}
	}
	if noti66Count != 4 {
		t.Fatalf("expected 4 NOTI 66 packets, got %d", noti66Count)
	}
}

func TestBoosterOpenRemySparklingTouchBox(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	// 10333023: Remy's Sparkling Touch 30 Box
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 82, Template: 10333023, Amount: 1},
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
			Items:  map[uint32]catalog.LootItem{},
		},
		BagRules: inventory.BagRules{
			Source: "test",
			Slots: map[string][2]uint16{
				"[booster]": {65, 120},
				"[waste]":   {65, 120},
			},
			MissingStackLimit: 1000,
		},
	}

	w := &worldSession{
		role: char,
		loot: lootSvc,
	}

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 82)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, lootSvc, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open remy box failed:", err)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Items) != 1 || resBag.Items[0].Template != 2660671 || resBag.Items[0].Amount != 30 {
		t.Fatalf("expected 30 Remy's Touch (2660671), got %+v", resBag.Items)
	}
	if len(packets) < 2 {
		t.Fatalf("expected at least 2 packets, got %d", len(packets))
	}
}

func TestBoosterDirectContractActivation(t *testing.T) {
	cat, err := LoadBoosterCatalog("../../configs/booster-catalog.json", "../../configs/items.index.json")
	if err != nil {
		t.Fatal(err)
	}

	// 44: Tactician's Contract 3D
	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items: []inventory.BagItem{
			{Slot: 65, Template: 44, Amount: 1},
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

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, nil, nil, cat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("direct contract activation failed:", err)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Items) != 0 {
		t.Fatalf("contract item at slot 65 was not consumed")
	}

	// Packets: inventory update, NOTI 66, ACK 160
	if len(packets) != 3 {
		t.Fatalf("expected 3 packets, got %d", len(packets))
	}
	if packets[0].Name != "contract_inventory_updated" || packets[0].ID != 14 {
		t.Fatalf("expected packet 0 to be inventory updated, got %+v", packets[0])
	}
	if packets[1].Name != "contract_special_item_noti" || packets[1].ID != 66 {
		t.Fatalf("expected packet 1 to be NOTI 66, got %+v", packets[1])
	}
	if packets[2].Name != "contract_use_ack" || packets[2].ID != 160 {
		t.Fatalf("expected packet 2 to be ACK 160, got %+v", packets[2])
	}
	if store.premiums[27] <= 1750000000 {
		t.Fatalf("expected tactician contract (27) to be activated, got %d", store.premiums[27])
	}
}
