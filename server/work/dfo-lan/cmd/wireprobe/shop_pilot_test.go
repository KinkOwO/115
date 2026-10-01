package main

import (
	"bytes"
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

type pilotLedger struct {
	calls int
	order storage.CashOrder
	state json.RawMessage
}

type vaultPilotLedger struct {
	pilotLedger
	vault storage.VaultState
	space byte
}

func (l *vaultPilotLedger) VaultPurchaseSpace(context.Context, int64, int64, string) (byte, error) {
	return l.space, nil
}

func (l *vaultPilotLedger) PurchaseCashVault(_ context.Context, o storage.CashOrder, fn func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error) {
	if o.VaultSpace != l.space {
		return storage.CashReceipt{}, false, fmt.Errorf("扩容目标金库不一致")
	}
	if l.calls > 0 && l.order.Key == o.Key {
		return storage.CashReceipt{Vault: &l.vault, VaultSpace: o.VaultSpace}, false, nil
	}
	next, e := fn(l.vault)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.vault = next
	l.order = o
	l.calls++
	return storage.CashReceipt{Vault: &l.vault, VaultSpace: o.VaultSpace, After: 70, Deliveries: []storage.CashDelivery{{Product: o.Lines[0].Product, Quantity: 1}}}, true, nil
}
func TestVaultPurchasePackets(t *testing.T) {
	p, e := cashshop.LoadPilot("../../configs/shop-special-candidate.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := inventory.LoadVaultRules("../../configs/vault.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for slots := uint16(24); slots <= 264; slots += 16 {
		rules.VerifiedSlots = append(rules.VerifiedSlots, slots)
	}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	s.vaultRules = &rules
	l := &vaultPilotLedger{vault: storage.VaultState{Slots: 8, Items: []byte(`[]`), ConfigVersion: rules.SourceSHA256}}
	body := make([]byte, 16)
	body[2] = 1
	binary.LittleEndian.PutUint32(body[5:], 3000129)
	binary.LittleEndian.PutUint32(body[9:], 1)
	frame := append(make([]byte, 13), body...)
	r, applied, e := s.purchase(context.Background(), p, l, 1, 1, body, frame)
	if e != nil || !applied {
		t.Fatal(e)
	}
	packets, e := shopPilotPackets(r, 70, true)
	if e != nil {
		t.Fatal("金库购买回包失败", e)
	}
	if len(packets) != 4 {
		t.Fatalf("金库购买应返回解锁通知、快照、余额和成功应答，实际 %d 包", len(packets))
	}
	// 原生刷新仅在容量增大时执行，NOTI66 必须先于改写容量的 NOTI13。
	if packets[0].Kind != 0 || packets[0].ID != 66 || !bytes.Equal(packets[0].Payload, []byte{1, 0, 24, 0}) {
		t.Fatalf("首包应为个人金库 24 格即时解锁通知：%+v", packets[0])
	}
	if packets[1].Kind != 0 || packets[1].ID != 13 || !bytes.Equal(packets[1].Payload, []byte{2, 24, 0, 0, 0, 0}) {
		t.Fatalf("解锁后应同步个人金库容量和物品快照：%+v", packets[1])
	}
	if packets[2].Kind != 0 || packets[2].ID != 53 || packets[3].Kind != 1 || packets[3].ID != 64 {
		t.Fatal("金库快照后应同步余额并返回购买成功")
	}
	if _, e = preparePackets(s.keys, packets); e != nil {
		t.Fatal(e)
	}
	firstPackets := packets
	r, applied, e = s.purchase(context.Background(), p, l, 1, 1, body, frame)
	if e != nil || applied || l.calls != 1 {
		t.Fatal("duplicate request applied", e)
	}
	packets, e = shopPilotPackets(r, 70, false)
	if e != nil {
		t.Fatal("金库购买重放回包失败", e)
	}
	if len(packets) != 3 {
		t.Fatalf("重放只能同步解锁、快照和余额，不应重复购买成功应答，实际 %d 包", len(packets))
	}
	for i, packet := range packets {
		if packet.Kind != firstPackets[i].Kind || packet.ID != firstPackets[i].ID || !bytes.Equal(packet.Payload, firstPackets[i].Payload) {
			t.Fatalf("重放第 %d 包未保持金库同步内容和顺序：%+v", i, packet)
		}
	}
	t.Log("通过：NOTI66 即时解锁先于 NOTI13 金库快照、余额和购买成功应答；重复请求仅处理一次且不重复成功应答")
	secondary, err := p.Config.VaultUpgrades(45)
	if err != nil || len(secondary) != 16 || secondary[3000129].Before != 8 || secondary[3000129].Price != 30 || secondary[3001202].Before != 24 || secondary[3001216].After != 264 {
		t.Fatalf("第二金库的源商品、首档或末档无效：%v", err)
	}
	for id, upgrade := range secondary {
		second := &vaultPilotLedger{space: 45, vault: storage.VaultState{Slots: upgrade.Before, Items: []byte(`[]`), ConfigVersion: rules.SourceSHA256}}
		binary.LittleEndian.PutUint32(body[5:], id)
		request := append(make([]byte, 13), body...)
		r, applied, err := s.purchase(context.Background(), p, second, 1, 1, body, request)
		if err != nil || !applied || second.vault.Slots != upgrade.After || second.order.VaultSpace != 45 || second.order.Lines[0].UnitPrice != upgrade.Price || l.vault.Slots != 24 {
			t.Fatalf("第二金库商品 %d 未独立升级或价格不符：%v", id, err)
		}
		packets, err := shopPilotPackets(r, 70, applied)
		if err != nil || len(packets) != 4 {
			t.Fatalf("第二金库商品 %d 回包无效：%v", id, err)
		}
		wantNotice := []byte{22, 0, byte(upgrade.After), byte(upgrade.After >> 8)}
		wantVault := []byte{45, byte(upgrade.After), byte(upgrade.After >> 8), 0, 0}
		if packets[0].ID != 66 || !bytes.Equal(packets[0].Payload, wantNotice) || packets[1].ID != 13 || !bytes.Equal(packets[1].Payload, wantVault) {
			t.Fatalf("第二金库商品 %d 的通知类型、容量或容器编号不符", id)
		}
		if _, err = preparePackets(s.keys, packets); err != nil {
			t.Fatal(err)
		}
		r, applied, err = s.purchase(context.Background(), p, second, 1, 1, body, request)
		if err != nil || applied || second.calls != 1 || r.VaultSpace != 45 {
			t.Fatalf("第二金库商品 %d 重放丢失金库目标或重复升级：%v", id, err)
		}
		if _, _, err = s.purchase(context.Background(), p, second, 1, 1, body, append(request, 1)); err == nil {
			t.Fatalf("第二金库商品 %d 重复购买旧档位未被拒绝", id)
		}
	}
}

func (f *pilotLedger) PurchaseCashToBag(_ context.Context, o storage.CashOrder, deliver func(json.RawMessage) (json.RawMessage, error)) (storage.CashReceipt, bool, error) {
	if f.calls > 0 && o.Key == f.order.Key {
		return storage.CashReceipt{CharacterState: f.state, After: 955}, false, nil
	}
	state, e := deliver(json.RawMessage(`{}`))
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	f.calls++
	f.order = o
	f.state = state
	return storage.CashReceipt{CharacterState: state, Before: 1000, After: 955, Charged: 45, Deliveries: []storage.CashDelivery{{Product: o.Lines[0].Product, Template: o.Lines[0].Template, Amount: o.Lines[0].Quantity * o.Lines[0].Units, Quantity: o.Lines[0].Quantity}}}, true, nil
}
func TestShopPilotRequestToPackets(t *testing.T) {
	p, e := cashshop.LoadPilot("../../configs/shop-purchase-pilot.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	f := &pilotLedger{}
	body := make([]byte, 16)
	body[2] = 1
	binary.LittleEndian.PutUint32(body[5:], 3000118)
	binary.LittleEndian.PutUint32(body[9:], 1)
	frame := append(make([]byte, 13), body...)
	r, applied, e := s.purchase(context.Background(), p, f, 1, 1, body, frame)
	if e != nil || !applied || f.calls != 1 || f.order.Lines[0].UnitPrice != 45 {
		t.Fatal(e)
	}
	packets, e := shopPilotPackets(r, r.After, applied)
	if e != nil || len(packets) != 3 || packets[0].ID != 14 || packets[1].ID != 53 || packets[2].ID != 64 || len(packets[2].Payload) != 49 {
		t.Fatalf("%+v %v", packets, e)
	}
	_, applied, e = s.purchase(context.Background(), p, f, 1, 1, body, frame)
	if e != nil || applied || f.calls != 1 {
		t.Fatal("replay paid", e)
	}
	frame[8] = 1
	if _, _, e = s.purchase(context.Background(), p, f, 1, 1, body, frame); e != nil || f.calls != 2 {
		t.Fatal("second legitimate order rejected", e)
	}
	packets, e = shopPilotPackets(r, r.After, false)
	if e != nil || len(packets) != 2 {
		t.Fatal("duplicate purchase UI acknowledgement", e)
	}
}

func TestShopPilotCreatureEggPackets(t *testing.T) {
	p, e := cashshop.LoadPilot("../../configs/shop-vault-release.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	f := &pilotLedger{}
	body := make([]byte, 16)
	body[2] = 1
	binary.LittleEndian.PutUint32(body[5:], 3300000)
	binary.LittleEndian.PutUint32(body[9:], 1)
	frame := append(make([]byte, 13), body...)
	r, applied, e := s.purchase(context.Background(), p, f, 1, 1, body, frame)
	if e != nil || !applied || f.calls != 1 || f.order.Lines[0].UnitPrice != 500 || f.order.Lines[0].Template != 63006 {
		t.Fatalf("purchase error: %v applied: %v", e, applied)
	}
	packets, e := shopPilotPackets(r, r.After, applied)
	if e != nil {
		t.Fatal(e)
	}
	if len(packets) != 4 {
		t.Fatalf("expected 4 packets, got %d: %+v", len(packets), packets)
	}
	if packets[0].ID != 14 || packets[1].ID != 14 || packets[2].ID != 53 || packets[3].ID != 64 {
		t.Fatalf("unexpected packet IDs: %+v", packets)
	}
	if packets[1].Payload[0] != 7 || packets[1].Name != "cera_purchase_creature_inventory" {
		t.Fatalf("creature packet mismatch: %+v", packets[1])
	}
}

func TestShopPilotLifeTokenPackets(t *testing.T) {
	p, e := cashshop.LoadPilot("../../configs/shop-purchase-pilot.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	f := &pilotLedger{}
	body := make([]byte, 16)
	body[2] = 1
	binary.LittleEndian.PutUint32(body[5:], 3000110) // Life Token 10 EA
	binary.LittleEndian.PutUint32(body[9:], 1)
	frame := append(make([]byte, 13), body...)
	r, applied, e := s.purchase(context.Background(), p, f, 1, 1, body, frame)
	if e != nil || !applied || f.calls != 1 || f.order.Lines[0].UnitPrice != 140 || f.order.Lines[0].Template != 1 {
		t.Fatalf("purchase error: %v applied: %v", e, applied)
	}
	packets, e := shopPilotPackets(r, r.After, applied)
	if e != nil {
		t.Fatal(e)
	}
	if len(packets) != 4 {
		t.Fatalf("expected 4 packets, got %d: %+v", len(packets), packets)
	}
	if packets[0].ID != 14 || packets[1].ID != 53 || packets[2].ID != 64 || packets[3].ID != 13 {
		t.Fatalf("unexpected packet IDs: %+v", packets)
	}
	// Packet 0 is NOTI 14 (filtered inventory update, slots <= 1 omitted so client never crashes on toast)
	if packets[0].Payload[0] != 0 {
		t.Fatalf("expected space 0, got %d", packets[0].Payload[0])
	}
	// Packet 3 is NOTI 13 (space 0) with slot 1 (Coin)
	payload := packets[3].Payload
	if len(payload) < 5 {
		t.Fatalf("short payload: %d", len(payload))
	}
	space := payload[0]
	count := binary.LittleEndian.Uint16(payload[3:5])
	if space != 0 || count != 2 { // slot 0 (gold) + slot 1 (coin)
		t.Fatalf("expected space 0, count 2, got space=%d count=%d", space, count)
	}
	// Check slot 1 in row 1 (row 0 starts at 5, row 1 starts at 5 + CurrentItemRecordSize)
	row1 := payload[5+protocol.CurrentItemRecordSize:]
	slot1 := binary.LittleEndian.Uint16(row1[0:2])
	tpl1 := binary.LittleEndian.Uint32(row1[2:6])
	cnt1 := binary.LittleEndian.Uint32(row1[6:10])
	if slot1 != 1 || tpl1 != 1 || cnt1 != 10 {
		t.Fatalf("slot 1 item mismatch: slot=%d tpl=%d cnt=%d", slot1, tpl1, cnt1)
	}
}

func TestShopPilotDatabasePurchase(t *testing.T) {
	if os.Getenv("CASH_INTEGRATION") != "1" {
		t.Skip("isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig("../../runtime/storage/local.json")
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("shop_wire_test_%d", time.Now().UnixNano())
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
	for _, fn := range []func(context.Context) error{store.Migrate, store.MigrateGrants, store.MigrateCashShop} {
		if e = fn(ctx); e != nil {
			t.Fatal(e)
		}
	}
	p, e := cashshop.LoadPilot("../../configs/shop-purchase-pilot.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	account, e := store.DevelopmentAccount(ctx, "wire-fixture")
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "ShopWireTest", Request: []byte{0}, ConfigVersion: p.Config.Source.SaveIdentity(), State: json.RawMessage(`{}`)}, 24)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = store.DB.Exec(ctx, `INSERT INTO account_currency(account_id,cera) VALUES($1,1000)`, account); e != nil {
		t.Fatal(e)
	}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	body := make([]byte, 16)
	body[2] = 1
	binary.LittleEndian.PutUint32(body[5:], 3000118)
	binary.LittleEndian.PutUint32(body[9:], 1)
	frame := append(make([]byte, 13), body...)
	// Missing response cipher is detected inside the transaction before debit.
	if _, _, e = s.purchase(ctx, p, store, account, role.ID, body, frame); e == nil {
		t.Fatal("missing cipher accepted")
	}
	balance, e := store.AccountCera(ctx, account)
	if e != nil || balance != 1000 {
		t.Fatal("failed encoding charged", e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	r, applied, e := s.purchase(ctx, p, store, account, role.ID, body, frame)
	if e != nil || !applied || r.After != 955 {
		t.Fatalf("%+v %v", r, e)
	}
	packets, e := shopPilotPackets(r, r.After, applied)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := preparePackets(s.keys, packets)
	if e != nil || len(encoded) != 3 {
		t.Fatal("wire encode", e)
	}
	for _, packet := range encoded {
		if e = wire.ValidateServer(packet.Raw); e != nil {
			t.Fatal(e)
		}
	}
	_, applied, e = s.purchase(ctx, p, store, account, role.ID, body, frame)
	if e != nil || applied {
		t.Fatal("duplicate charged", e)
	}
	var saved json.RawMessage
	if e = store.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, role.ID).Scan(&saved); e != nil {
		t.Fatal(e)
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(saved, &document) != nil || document["inventory"] == nil {
		t.Fatal("inventory absent")
	}
	var orders, claimed int
	if e = store.DB.QueryRow(ctx, `SELECT count(*) FROM cash_orders`).Scan(&orders); e != nil || orders != 1 {
		t.Fatal("order audit", e)
	}
	if e = store.DB.QueryRow(ctx, `SELECT count(*) FROM cash_inventory WHERE claimed_at IS NOT NULL AND template=15 AND amount=1`).Scan(&claimed); e != nil || claimed != 1 {
		t.Fatal("delivery audit", e)
	}
	t.Log("PASS CMD64 pilot: server price45; Cera1000->955; template15x1 in saved bag; NOTI14+NOTI53+CMD64 encrypted; one audit; retry no debit; encoding failure rolled back")
	// Two different cart products commit once, with one completion per product.
	mixed := make([]byte, 27)
	mixed[2] = 2
	binary.LittleEndian.PutUint32(mixed[5:], 3000118)
	binary.LittleEndian.PutUint32(mixed[9:], 1)
	binary.LittleEndian.PutUint32(mixed[17:], 3000673)
	binary.LittleEndian.PutUint32(mixed[21:], 1)
	mixedFrame := append(make([]byte, 13), mixed...)
	r, applied, e = s.purchase(ctx, p, store, account, role.ID, mixed, mixedFrame)
	if e != nil || !applied || r.Charged != 145 || r.After != 810 || len(r.Deliveries) != 2 {
		t.Fatalf("mixed receipt %+v %v", r, e)
	}
	packets, e = shopPilotPackets(r, r.After, applied)
	if e != nil || len(packets) != 4 {
		t.Fatal("mixed packets", e)
	}
	for i, id := range []uint32{3000118, 3000673} {
		if packets[i+2].ID != 64 || binary.LittleEndian.Uint32(packets[i+2].Payload[6:]) != id {
			t.Fatal("wrong cart completion")
		}
	}
	encoded, e = preparePackets(s.keys, packets)
	if e != nil {
		t.Fatal(e)
	}
	for _, packet := range encoded {
		if e = wire.ValidateServer(packet.Raw); e != nil {
			t.Fatal(e)
		}
	}
	_, applied, e = s.purchase(ctx, p, store, account, role.ID, mixed, mixedFrame)
	if e != nil || applied {
		t.Fatal("mixed replay charged", e)
	}
	balance, e = store.AccountCera(ctx, account)
	if e != nil || balance != 810 {
		t.Fatal("mixed balance", e)
	}
	if e = store.DB.QueryRow(ctx, `SELECT count(*) FROM cash_inventory WHERE claimed_at IS NOT NULL`).Scan(&claimed); e != nil || claimed != 3 {
		t.Fatal("mixed audit", e)
	}
	t.Log("PASS mixed CMD64: two products, debit145 once, balance810, two ACKs, three total claimed lines, replay no debit")
	// Material and consumable in one transaction use independent inventory ranges.
	binary.LittleEndian.PutUint32(mixed[5:], 3002398)
	binary.LittleEndian.PutUint32(mixed[17:], 3000393)
	mixedFrame = append(make([]byte, 13), mixed...)
	r, applied, e = s.purchase(ctx, p, store, account, role.ID, mixed, mixedFrame)
	if e != nil || !applied || r.Charged != 240 || r.After != 570 {
		t.Fatalf("material cart %+v %v", r, e)
	}
	if e = store.DB.QueryRow(ctx, `SELECT state FROM characters WHERE id=$1`, role.ID).Scan(&saved); e != nil {
		t.Fatal(e)
	}
	bag, e := inventory.ReadBag(saved)
	if e != nil {
		t.Fatal(e)
	}
	found := 0
	for _, row := range bag.Items {
		if row.Template == 10308836 {
			if row.Slot < 121 || row.Slot > 176 {
				t.Fatal("material slot")
			}
			found++
		}
		if row.Template == 2660452 {
			if row.Slot < 65 || row.Slot > 120 {
				t.Fatal("consumable slot")
			}
			found++
		}
	}
	if found != 2 {
		t.Fatal("persisted new products missing")
	}
	packets, e = shopPilotPackets(r, r.After, applied)
	if e != nil || len(packets) != 4 {
		t.Fatal("material packets", e)
	}
	if _, e = preparePackets(s.keys, packets); e != nil {
		t.Fatal(e)
	}
	_, applied, e = s.purchase(ctx, p, store, account, role.ID, mixed, mixedFrame)
	if e != nil || applied {
		t.Fatal("material replay charged", e)
	}
	balance, e = store.AccountCera(ctx, account)
	if e != nil || balance != 570 {
		t.Fatal("material balance", e)
	}
	t.Log("PASS category cart: debit240 once; balance570; material121..176 and consumable65..120 persisted; ACKs encoded; replay no debit")
}

// 契约激活回执在 ACK64 之后追加 NOTI66,客户端即时刷新权益状态。
func TestShopPilotPremiumActivationNotice(t *testing.T) {
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "0")
	r := storage.CashReceipt{
		CharacterState: json.RawMessage(`{}`),
		Deliveries:     []storage.CashDelivery{{Product: 3500001, Template: 45, Amount: 1, Quantity: 1}},
		Premiums:       []storage.CashPremium{{Type: 27, EndTime: 1800000000}},
	}
	packets, e := shopPilotPackets(r, 70, true)
	if e != nil {
		t.Fatal(e)
	}
	last := packets[len(packets)-1]
	if last.Name != "cera_purchase_premium_activated" || last.ID != 66 {
		t.Fatalf("expected premium notice after acks, got %+v", last)
	}
	if last.Payload[2] != 27 {
		t.Fatalf("premium type mismatch: %+v", last.Payload)
	}
}

func TestShopPilotContractPurchaseCrashFix(t *testing.T) {
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "1")
	r := storage.CashReceipt{
		CharacterState: json.RawMessage(`{}`),
		Deliveries:     []storage.CashDelivery{{Product: 3500009, Template: 33, Amount: 1, Quantity: 1}},
		Premiums:       []storage.CashPremium{{Type: 22, EndTime: time.Now().Add(24 * time.Hour).Unix()}},
	}
	packets, err := shopPilotPackets(r, 70, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(packets) != 3 {
		t.Fatalf("want inventory, balance, ACK only; got %+v", packets)
	}
	if packets[0].ID != 14 || packets[1].ID != 53 || packets[2].Kind != 1 || packets[2].ID != 64 {
		t.Fatalf("unexpected contract purchase packet order: %+v", packets)
	}
}

type contractCartPilotLedger struct {
	pilotLedger
}

func (l *contractCartPilotLedger) PurchaseCashMixed(_ context.Context, o storage.CashOrder, fn func(json.RawMessage) (json.RawMessage, error), premiums map[int]storage.CashPremiumActivation) (storage.CashReceipt, bool, error) {
	state, e := fn(l.state)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.state = state
	l.order = o
	l.calls++
	var charged uint64
	var out []storage.CashPremium
	var deliveries []storage.CashDelivery
	for _, line := range o.Lines {
		charged += uint64(line.UnitPrice) * uint64(line.Quantity)
		deliveries = append(deliveries, storage.CashDelivery{Product: line.Product, Template: line.Template, Amount: line.Quantity * line.Units, Quantity: line.Quantity})
	}
	for _, act := range premiums {
		out = append(out, storage.CashPremium{Type: act.Type, EndTime: time.Now().Unix() + act.DurationSecond})
	}
	return storage.CashReceipt{Order: o.Key, Charged: charged, CharacterState: json.RawMessage(`{}`), Deliveries: deliveries, Premiums: out}, true, nil
}

// 实机 2026-09-23:合并购买契约曾被 "premium contracts require a separate
// order" 整单拒绝;契约行现在逐行激活并可与普通商品同单。
func TestShopPilotContractCartPurchase(t *testing.T) {
	t.Setenv("DFO_CONTRACT_PURCHASE_CRASH_FIX", "0")
	p, e := cashshop.LoadPilot("../../configs/shop-vault-release.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	ledger := &contractCartPilotLedger{pilotLedger{state: json.RawMessage(`{}`)}}
	s, e := newShopPilotSession()
	if e != nil {
		t.Fatal(e)
	}
	s.keys = make([]byte, wire.SessionKeyBytes)
	body := make([]byte, 40)
	body[2] = 3
	binary.LittleEndian.PutUint32(body[5:], 3500001)
	binary.LittleEndian.PutUint32(body[9:], 1)
	binary.LittleEndian.PutUint32(body[17:], 3500009)
	binary.LittleEndian.PutUint32(body[21:], 1)
	binary.LittleEndian.PutUint32(body[29:], 3500016)
	binary.LittleEndian.PutUint32(body[33:], 1)
	frame := append(make([]byte, 13), body...)
	r, applied, e := s.purchase(context.Background(), p, ledger, 1, 1, body, frame)
	if e != nil || !applied {
		t.Fatalf("merged contract cart failed: %v applied: %v", e, applied)
	}
	if len(r.Premiums) != 3 {
		t.Fatalf("expected three activations, got %+v", r.Premiums)
	}
	types := map[uint8]bool{}
	for _, pr := range r.Premiums {
		types[pr.Type] = true
	}
	for _, want := range []uint8{27, 22, 92} {
		if !types[want] {
			t.Fatalf("missing premium activation %d: %+v", want, r.Premiums)
		}
	}
	packets, e := shopPilotSpaces(p, r, r.After, applied)
	if e != nil {
		t.Fatal(e)
	}
	last := packets[len(packets)-1]
	if last.Name != "cera_purchase_premium_activated" || last.ID != 66 {
		t.Fatalf("expected trailing premium notice, got %+v", last)
	}
}
