package main

import (
	"context"
	"dfolan/internal/cashshop"
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
}

func (l *vaultPilotLedger) PurchaseCashVault(_ context.Context, o storage.CashOrder, fn func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error) {
	if l.calls > 0 && l.order.Key == o.Key {
		return storage.CashReceipt{Vault: &l.vault}, false, nil
	}
	next, e := fn(l.vault)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.vault = next
	l.order = o
	l.calls++
	return storage.CashReceipt{Vault: &l.vault, After: 70, Deliveries: []storage.CashDelivery{{Product: o.Lines[0].Product, Quantity: 1}}}, true, nil
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
	rules.VerifiedSlots = append(rules.VerifiedSlots, 24)
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
	if e != nil || len(packets) != 3 || packets[0].ID != 13 || packets[0].Payload[0] != 2 || binary.LittleEndian.Uint16(packets[0].Payload[1:]) != 24 || packets[1].ID != 53 || packets[2].ID != 64 {
		t.Fatal("vault purchase packets", e)
	}
	if _, e = preparePackets(s.keys, packets); e != nil {
		t.Fatal(e)
	}
	r, applied, e = s.purchase(context.Background(), p, l, 1, 1, body, frame)
	if e != nil || applied || l.calls != 1 {
		t.Fatal("duplicate request applied", e)
	}
	packets, e = shopPilotPackets(r, 70, false)
	if e != nil || len(packets) != 2 {
		t.Fatal("replay repeated success", e)
	}
	t.Log("PASS vault NOTI13 kind2 / balance53 / ACK64; encrypted encoding; replay no extra ACK; no bag NOTI14")
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
	return storage.CashReceipt{CharacterState: state, Before: 1000, After: 955, Charged: 45, Deliveries: []storage.CashDelivery{{Product: o.Lines[0].Product, Quantity: o.Lines[0].Quantity}}}, true, nil
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
	role, e := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "ShopWireTest", Request: []byte{0}, ConfigVersion: p.Config.Source.Checksum, State: json.RawMessage(`{}`)}, 24)
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
