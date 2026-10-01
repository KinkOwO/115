package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mockBoosterStore struct {
	character storage.Character
	receipts  map[string]json.RawMessage
	premiums  map[uint8]int64
	now       int64
}

func newMockBoosterStore(char storage.Character) *mockBoosterStore {
	return &mockBoosterStore{
		character: char,
		receipts:  make(map[string]json.RawMessage),
		premiums:  make(map[uint8]int64),
		now:       time.Now().Unix(),
	}
}

func (m *mockBoosterStore) ActivatePremium(ctx context.Context, account int64, premiumType uint8, durationSecond int64) (int64, error) {
	oldEnd := m.premiums[premiumType]
	now := m.now
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

// 合并适配：上游礼包用例继续验证发奖，模拟存储改为本地原子契约接口。
func (m *mockBoosterStore) CommitCharacterPremiumEvent(ctx context.Context, account, id int64, version, key, model string, apply func(storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error)) (storage.Character, bool, error) {
	if _, ok := m.receipts[key]; ok {
		return m.character, false, nil
	}
	raw, receipt, rewards, err := apply(m.character)
	if err != nil {
		return m.character, false, err
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(receipt, &fields); err != nil {
		return m.character, false, err
	}
	balances := map[uint8]int64{}
	for kind, end := range m.premiums {
		balances[kind] = end
	}
	now := m.now
	var premiums []storage.CashPremium
	for _, reward := range rewards {
		end := max(now, balances[reward.Type]) + reward.DurationSecond
		balances[reward.Type] = end
		premiums = append(premiums, storage.CashPremium{Type: reward.Type, EndTime: end, RemainingSecond: end - now})
	}
	fields["premiums"], _ = json.Marshal(premiums)
	receipt, err = json.Marshal(fields)
	if err != nil {
		return m.character, false, err
	}
	m.character.State, m.receipts[key], m.premiums = raw, receipt, balances
	return m.character, true, nil
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
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "0") // 本用例断言原即时 NOTI66 行为
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
	expectedEnd := store.now + 3*86400
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

// 修复开启（DFO_CONTRACT_PURCHASE_CRASH_FIX!=0，默认即开启）时，booster 开箱
// 命中契约奖不再追加即时 NOTI66，避免客户端闪退；契约仍激活落库。
func TestBoosterOpenMasterContractPackageCrashFix(t *testing.T) {
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "1")
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

	// 修复开启：仅背包刷新 + ACK，不追加即时 NOTI66。
	if len(packets) != 2 {
		t.Fatalf("expected 2 packets (update + ack) with crash fix on, got %d: %+v", len(packets), packets)
	}
	for _, p := range packets {
		if p.ID == 66 {
			t.Fatalf("immediate NOTI66 must not be sent with crash fix on: %+v", p)
		}
	}
	// 契约仍激活落库：4 个契约各 3 天（259200s）。
	expectedEnd := store.now + 3*86400
	for _, pt := range []uint8{22, 27, 92, 79} {
		if store.premiums[pt] != expectedEnd {
			t.Fatalf("expected premium %d end time %d, got %d", pt, expectedEnd, store.premiums[pt])
		}
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
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "0") // 本用例断言原即时 NOTI66 行为
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

// 修复开启（DFO_CONTRACT_PURCHASE_CRASH_FIX!=0，默认即开启）时，直接使用契约
// 道具同样不追加即时 NOTI66；契约仍激活落库。
func TestBoosterDirectContractActivationCrashFix(t *testing.T) {
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "1")
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

	// 修复开启：背包刷新 + ACK 两包，无即时 NOTI66。
	if len(packets) != 2 {
		t.Fatalf("expected 2 packets with crash fix on, got %d: %+v", len(packets), packets)
	}
	for _, p := range packets {
		if p.ID == 66 {
			t.Fatalf("immediate NOTI66 must not be sent with crash fix on: %+v", p)
		}
	}
	if store.premiums[27] <= 1750000000 {
		t.Fatalf("expected tactician contract (27) to be activated, got %d", store.premiums[27])
	}
}

// TestBoosterEquipmentGrantUsesSourceDurability 覆盖 2026-09-22 的玩家报告：
// 开箱拿到的上衣/下装耐久全是 0，穿上不生效。根因是开箱发放常规装备那一路把
// Durability 写死成 0，而任务/GM 发放走 AddEquipment 读的是源 [durability]。
// 两条路必须给出同一个数，否则客户端把新装备当成"耐久 0 / 已损坏"。
// 实机确认（2026-09-23）：开出的上衣耐久 = 源值 60。
func TestBoosterEquipmentGrantUsesSourceDurability(t *testing.T) {
	const (
		boxTpl  uint32 = 70000001
		gearTpl uint32 = 100051398 // 客户端内层 PVF: equipment/character/common/jacket/cloth/100051398.equ, [durability] = 60
	)

	dir := t.TempDir()
	catPath := filepath.Join(dir, "equipment.json")
	body := `{
	  "source": {"format":"test","path":"test","size":1,"checksum":"test","file_count":1,"group_count":1},
	  "rows": [
	    {"ID":100051398,"Path":"equipment/character/common/jacket/cloth/100051398.equ",
	     "SHA256":"0000000000000000000000000000000000000000000000000000000000000000",
	     "Fields":{
	       "[rarity]":[{"type":0,"value":3}],
	       "[equipment type]":[{"type":6,"value":166625925,"text":"[coat]"}],
	       "[durability]":[{"type":0,"value":60}]
	     }}
	  ]
	}`
	if err := os.WriteFile(catPath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	gear, err := inventory.LoadEquipmentCatalog(catPath, "test")
	if err != nil {
		t.Fatal(err)
	}
	want, err := gear.Reward(gearTpl)
	if err != nil {
		t.Fatal(err)
	}
	if want != 60 {
		t.Fatalf("fixture equipment catalog reports durability %d, want 60", want)
	}

	bag := inventory.Bag{
		Version: "ordinary-bag-v1",
		Items:   []inventory.BagItem{{Slot: 65, Template: boxTpl, Amount: 1}},
	}
	state, _ := inventory.SaveBag(json.RawMessage(`{}`), bag)
	store := newMockBoosterStore(storage.Character{ID: 11, AccountID: 1, State: state})

	lootSvc := &loot.Service{
		Catalog: catalog.LootCatalog{
			Source: pvf.ArchiveSnapshot{Checksum: "test"},
			Items: map[uint32]catalog.LootItem{
				boxTpl:  {ID: boxTpl, Kind: "stackable", StackableType: "[booster]", StackLimit: 1000},
				gearTpl: {ID: gearTpl, Kind: "equipment"},
			},
		},
		BagRules: inventory.BagRules{
			Source:            "test",
			Slots:             map[string][2]uint16{"[booster]": {65, 120}},
			MissingStackLimit: 1000,
			EquipmentSlots:    [2]uint16{9, 64},
		},
	}
	boosterCat := &BoosterCatalog{
		Definitions: map[uint32]BoosterDefinition{
			boxTpl: {Template: boxTpl, Type: "[booster]", Pools: []BoosterRewardPool{
				{DrawCount: 1, Candidates: []BoosterRewardCandidate{{Template: gearTpl, Weight: 1000, Count: 1}}},
			}},
		},
	}
	wear := &workflow.WearService{WearService: inventory.WearService{Catalog: gear}}
	w := &worldSession{role: store.character, loot: lootSvc}

	reqBytes := make([]byte, 8)
	binary.LittleEndian.PutUint16(reqBytes[0:2], 65)
	binary.LittleEndian.PutUint32(reqBytes[2:6], 1)
	binary.LittleEndian.PutUint16(reqBytes[6:8], 0)

	packets, err := w.openBoosterItem(context.Background(), store, wear, lootSvc, boosterCat, odysseyWeaponChoices{}, reqBytes, reqBytes)
	if err != nil {
		t.Fatal("open booster failed:", err)
	}

	resBag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		t.Fatal(err)
	}
	if len(resBag.Equipment) != 1 || resBag.Equipment[0].Template != gearTpl {
		t.Fatalf("expected one granted %d, got %+v", gearTpl, resBag.Equipment)
	}
	if resBag.Equipment[0].Durability != want {
		t.Fatalf("granted equipment durability = %d, want %d (source [durability])", resBag.Equipment[0].Durability, want)
	}

	// 线上表现由 NOTI13 行决定：耐久写在行内偏移 11。
	var update []byte
	for _, p := range packets {
		if p.Name == "booster_main_inventory_updated" && p.ID == 14 {
			update = p.Payload
		}
	}
	if len(update) < 3 {
		t.Fatalf("missing inventory update packet, got %d packets", len(packets))
	}
	rows := int(binary.LittleEndian.Uint16(update[1:3]))
	got := -1
	for i := 0; i < rows; i++ {
		off := 3 + i*protocol.CurrentItemRecordSize
		if off+protocol.CurrentItemRecordSize > len(update) {
			break
		}
		row := update[off:]
		if binary.LittleEndian.Uint16(row[0:2]) != resBag.Equipment[0].Slot {
			continue
		}
		got = int(binary.LittleEndian.Uint16(row[11:13]))
	}
	if got != int(want) {
		t.Fatalf("inventory row durability = %d, want %d", got, want)
	}
}
