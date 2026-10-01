package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

func odysseyRewardFixture(t *testing.T) (storage.Character, *workflow.WearService) {
	t.Helper()
	req := append([]byte{0, 4, 0, 0, 0}, []byte("test")...)
	req = append(req, 0, 0, 0, 0, 0, 0, 255, 0, 1, 0, 2, 0)
	for len(req)%8 != 0 {
		req = append(req, 0)
	}
	full, e := inventory.OpenFullEquipmentCatalog("testdata/odyssey-equipment", odysseySource())
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { full.Close() })
	rules, e := inventory.LoadBagRules("../../configs/inventory.current37.json")
	if e != nil {
		t.Fatal(e)
	}
	wear := &workflow.WearService{WearService: inventory.WearService{Catalog: &inventory.EquipmentCatalog{Full: full}, BagRules: rules}}
	wear.Catalog.Source.Checksum = odysseySource()
	return storage.Character{ID: 9, WireID: 9, Request: req, ConfigVersion: savecontract.Identity(), State: json.RawMessage(`{"level":1,"custom_marker":42}`)}, wear
}

func TestOdysseyArmorSourceAndAtomicGrant(t *testing.T) {
	role, wear := odysseyRewardFixture(t)
	data, e := os.ReadFile("../../docs/evidence/odyssey-rewards-revive-20260917/item-10417790.json")
	if e != nil {
		t.Fatal(e)
	}
	var source catalog.ScriptRecord
	if e = json.Unmarshal(data, &source); e != nil {
		t.Fatal(e)
	}
	var ids []uint32
	inside := false
	for _, c := range source.Cells {
		if c.Text == "[equipment]" {
			inside = true
		}
		if c.Text == "[/equipment]" {
			inside = false
		}
		if inside && c.Type == 0 && c.Value > 100000000 {
			ids = append(ids, uint32(c.Value))
		}
	}
	if fmt.Sprint(ids) != fmt.Sprint(odysseyArmor) {
		t.Fatal("source reward mismatch", ids)
	}
	before := append([]byte(nil), role.State...)
	raw, receipt, e := applyOdysseyArmor(role, wear)
	if e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(raw)
	if e != nil || len(b.Equipment) != 8 {
		t.Fatal(e, len(b.Equipment))
	}
	for i, v := range b.Equipment {
		if v.Template != odysseyArmor[i] || v.Slot != uint16(9+i) {
			t.Fatal(v)
		}
	}
	if !bytes.Contains(raw, []byte(`"custom_marker":42`)) || !bytes.Equal(role.State, before) || !json.Valid(receipt) {
		t.Fatal("state preservation")
	}
	wear.BagRules.EquipmentSlots = [2]uint16{9, 15}
	if out, _, e := applyOdysseyArmor(role, wear); e == nil || out != nil || !bytes.Equal(before, role.State) {
		t.Fatal("partial full-bag award", e)
	}
	role.Request[19] = 0
	if _, _, e := applyOdysseyArmor(role, wear); e == nil {
		t.Fatal("ordinary role awarded")
	}
	t.Log("source eight-item set; existing state retained; full bag atomic; ordinary role rejected")
}

func TestCapturedOdysseyPlayerDeath(t *testing.T) {
	w := &worldSession{role: storage.Character{ID: 9, WireID: 9}, activeDungeon: &dungeon.Session{Loaded: true}}
	for _, raw := range []string{"43020501000000000000000000000000", "a703fd00000000000000000000000000"} {
		p, _ := hex.DecodeString(raw)
		xy, e := protocol.DecodePlayerDeath(p)
		if e != nil || xy[0] != binary.LittleEndian.Uint16(p) {
			t.Fatal(e)
		}
		for n := 0; n < 2; n++ {
			plan, e := w.playerDeath(p)
			if e != nil || len(plan) != 2 || plan[0].ID != 40 || plan[1].ID != 32 || hex.EncodeToString(plan[1].Payload) != "0900000000000000" {
				t.Fatal(plan, e)
			}
		}
		p[4] = 1
		if _, e = w.playerDeath(p); e == nil {
			t.Fatal("foreign mode accepted")
		}
	}
	p := make([]byte, 16)
	w.activeDungeon.Loaded = false
	if _, e := w.playerDeath(p); e == nil {
		t.Fatal("unloaded death accepted")
	}
	w.activeDungeon = nil
	if _, e := w.playerDeath(p); e == nil {
		t.Fatal("town death accepted")
	}
	if _, e := protocol.DecodePlayerDeath(p[:15]); e == nil {
		t.Fatal("truncated death accepted")
	}
	if _, e := protocol.PlayerDeathState(65535); e == nil {
		t.Fatal("invalid actor accepted")
	}
	t.Log("captured CMD40 -> ACK40 01 + NOTI32 0900000000000000; replay has no currency/reward effect")
}

func TestOdysseyWeaponBoxKeepsOriginalSelection(t *testing.T) {
	role, _ := odysseyRewardFixture(t)
	raw, _, e := applyOdysseyWeaponBox(role)
	if e != nil {
		t.Fatal(e)
	}
	b, e := inventory.ReadBag(raw)
	if e != nil || len(b.Items) != 1 || b.Items[0].Template != 10417789 || b.Items[0].Amount != 1 || len(b.Equipment) != 0 {
		t.Fatal(b, e)
	}
	for n := uint16(66); n <= 120; n++ {
		b.Items = append(b.Items, inventory.BagItem{Slot: n, Template: 10417789, Amount: 1})
	}
	role.State, e = inventory.SaveBag(role.State, b)
	if e != nil {
		t.Fatal(e)
	}
	if raw, _, e = applyOdysseyWeaponBox(role); e == nil || raw != nil {
		t.Fatal("full bag partially granted", e)
	}
	t.Log("original10417789 box only; no automatic weapon choice; full consumable bag rejected")
}

func TestOdysseyArmorDatabaseReplay(t *testing.T) {
	if os.Getenv("ODYSSEY_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// 配置路径可覆盖：默认指向的历史目录（swordmaster-pilot-20260916）已不在仓库里，
	// 所以这个集成测试一直是 skip 状态。设 ODYSSEY_INTEGRATION_CONFIG 就能用任意本地
	// PG 配置跑（测试自建独立 schema 并 DROP，不碰真实存档）。
	cfgPath := os.Getenv("ODYSSEY_INTEGRATION_CONFIG")
	if cfgPath == "" {
		cfgPath = "../../runtime/swordmaster-pilot-20260916/storage.json"
	}
	cfg, e := storage.LoadConfig(cfgPath)
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("odyssey_reward_test_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	cfg.RedisPrefix = schema + ":"
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
	account, e := store.DevelopmentAccount(ctx, "odyssey-fixture")
	if e != nil {
		t.Fatal(e)
	}
	role, wear := odysseyRewardFixture(t)
	role.ID = 0
	role.WireID = 0
	role.AccountID = account
	role.Name = "OdysseyFixture"
	role, e = store.CreateCharacter(ctx, role, 24)
	if e != nil {
		t.Fatal(e)
	}
	updated, applied, e := grantOdysseyArmor(ctx, store, wear, role)
	if e != nil || !applied {
		t.Fatal(e, applied)
	}
	replay, applied, e := grantOdysseyArmor(ctx, store, wear, role)
	if e != nil || applied {
		t.Fatal(e, applied)
	}
	var a, b any
	json.Unmarshal(updated.State, &a)
	json.Unmarshal(replay.State, &b)
	aa, _ := json.Marshal(a)
	bb, _ := json.Marshal(b)
	if !bytes.Equal(aa, bb) {
		t.Fatal("replay changed inventory")
	}
	receipt, e := store.CharacterEventReceipt(ctx, account, role.ID, odysseyArmorEvent)
	if e != nil || !bytes.Contains(receipt, []byte("10417790")) {
		t.Fatal(e)
	}
	t.Log("temporary-schema transaction: first grant eight, stale replay zero, persisted receipt, real characters unchanged")
	updated, applied, e = grantOdysseyWeaponBox(ctx, store, replay)
	if e != nil || !applied {
		t.Fatal(e, applied)
	}
	replay, applied, e = grantOdysseyWeaponBox(ctx, store, replay)
	if e != nil || applied {
		t.Fatal(e, applied)
	}
	bag, e := inventory.ReadBag(replay.State)
	if e != nil || len(bag.Items) != 1 || len(bag.Equipment) != 8 {
		t.Fatal(e, bag)
	}
	t.Log("weapon box persisted exactly once; selection deliberately unsettled")
	// 创建奖励第三行：药水 x30，独立事件键，重放必须被幂等拦下。
	potLoot := odysseyCreatePotionCatalog()
	updated, applied, e = grantOdysseyCreatePotion(ctx, store, potLoot, wear.BagRules, replay)
	if e != nil || !applied {
		t.Fatal(e, applied)
	}
	replay, applied, e = grantOdysseyCreatePotion(ctx, store, potLoot, wear.BagRules, updated)
	if e != nil || applied {
		t.Fatal("药水事件重放没有被幂等拦下", e)
	}
	potBag, e := inventory.ReadBag(replay.State)
	if e != nil {
		t.Fatal(e)
	}
	var potTotal uint32
	for _, it := range potBag.Items {
		if it.Template == odysseyCreatePotion {
			potTotal += it.Amount
		}
	}
	if potTotal != odysseyCreatePotionCount {
		t.Fatalf("药水合计 %d，期望 %d（重放不应再加）", potTotal, odysseyCreatePotionCount)
	}
	potReceipt, e := store.CharacterEventReceipt(ctx, account, role.ID, odysseyCreatePotionEvent)
	if e != nil || !bytes.Contains(potReceipt, []byte("10418028")) {
		t.Fatal("药水收据缺失", e)
	}
	t.Log("create potion x30 settled exactly once with its own event key")
	choices, e := loadOdysseyWeaponChoices("../../configs/odyssey-weapon-box-candidate.json")
	if e != nil {
		t.Fatal(e)
	}
	selection := protocol.WeaponBoxSelection{Slot: bag.Items[0].Slot, Category: [2]byte{0, 0}, Template: 101040856}
	selected, packets, e := selectOdysseyWeapon(ctx, store, wear, replay, choices, selection)
	if e != nil || len(packets) != 2 {
		t.Fatal(e, packets)
	}
	_, packets, e = selectOdysseyWeapon(ctx, store, wear, replay, choices, selection)
	if e != nil || len(packets) != 1 {
		t.Fatal("duplicate weapon delta", e)
	}
	other := selection
	other.Template = 101001229
	if _, _, e = selectOdysseyWeapon(ctx, store, wear, replay, choices, other); e == nil {
		t.Fatal("box reused for different selection")
	}
	selected, applied, e = grantOdysseyCredits(ctx, store, selected)
	if e != nil || !applied {
		t.Fatal(e)
	}
	w := &worldSession{role: selected, activeDungeon: &dungeon.Session{RunID: "integration-run", Loaded: true, Definition: catalog.DungeonDefinition{Odyssey: true}, Room: catalog.DungeonRoom{Map: 1}}, dungeons: &catalog.DungeonCatalog{Maps: map[uint32]catalog.ScriptRecord{1: {}}}}
	coin := make([]byte, 8)
	binary.LittleEndian.PutUint16(coin, selected.WireID)
	for i := 0; i < 10; i++ {
		frame := []byte{40, byte(i)}
		if _, e = w.playerDeath(make([]byte, 16), frame); e != nil {
			t.Fatal(e)
		}
		if i > 0 {
			if plan, e := w.pilotRevive(ctx, store, coin, []byte{41, 0}); e != nil || len(plan) != 0 || !w.pilotDeath.Dead {
				t.Fatal("old revive changed new death", e)
			}
		}
		plan, e := w.pilotRevive(ctx, store, coin, []byte{41, byte(i)})
		if e != nil || len(plan) != 2 || plan[1].Payload[2] != 1 {
			t.Fatal(e, plan)
		}
		if _, e = w.playerDeath(make([]byte, 16), frame); e != nil || w.pilotDeath.Dead {
			t.Fatal("old death replay killed revived actor", e)
		}
	}
	if _, e = w.playerDeath(make([]byte, 16), []byte{40, 11}); e != nil {
		t.Fatal(e)
	}
	if _, e = w.pilotRevive(ctx, store, coin, []byte{41, 11}); e == nil || !w.pilotDeath.Dead {
		t.Fatal("exhausted credit revived actor")
	}
	final, applied, e := grantOdysseyCredits(ctx, store, w.role)
	if e != nil || applied {
		t.Fatal("relogin refilled credits", e)
	}
	var credit map[string]json.RawMessage
	json.Unmarshal(final.State, &credit)
	if string(credit[odysseyCreditField]) != "0" {
		t.Fatal("wrong remaining credit")
	}
	t.Log("one chosen weapon persisted; replay emits no inventory delta; test credits10->0; old packets and relogin do not refill/spend/revive")
}
