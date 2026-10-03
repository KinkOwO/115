package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
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

// Opt in with SHOP_INTEGRATION_CONFIG; all writes use a new temporary schema.
func TestShopQuantityDatabaseAndWire(t *testing.T) {
	path := os.Getenv("SHOP_INTEGRATION_CONFIG")
	if path == "" {
		t.Skip("isolated PostgreSQL integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	cfg, e := storage.LoadConfig(path)
	if e != nil {
		t.Fatal(e)
	}
	admin, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close()
	schema := fmt.Sprintf("shop_sell_test_%d", time.Now().UnixNano())
	if _, e = admin.DB.Exec(ctx, "CREATE SCHEMA "+schema); e != nil {
		t.Fatal(e)
	}
	defer func() {
		if _, e := admin.DB.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); e != nil {
			t.Error(e)
		}
	}()
	cfg.PostgresSchema = schema
	store, e := storage.Open(ctx, cfg)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	var actual string
	if e = store.DB.QueryRow(ctx, "SELECT current_schema()").Scan(&actual); e != nil || actual != schema {
		t.Fatalf("isolation %s %v", actual, e)
	}
	for _, f := range []func(context.Context) error{store.Migrate, store.MigrateCharacterEvents} {
		if e = f(ctx); e != nil {
			t.Fatal(e)
		}
	}
	source := "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"
	price, e := catalog.ShopPriceFromScript([]pvf.Token{{Type: 3, Text: "[price]"}, {Type: 0, Value: 200}})
	if e != nil {
		t.Fatal(e)
	}
	prices := &catalog.ShopPrices{Source: source, Items: map[uint32]catalog.ShopPrice{1150: price}}
	if file := os.Getenv("SHOP_INTEGRATION_PRICES"); file != "" {
		prices, e = catalog.LoadShopPrices(file, source)
		if e != nil {
			t.Fatal(e)
		}
	}
	account, e := store.DevelopmentAccount(ctx, "sale-fixture")
	if e != nil {
		t.Fatal(e)
	}
	state, e := inventory.SaveBag(json.RawMessage(`{"level":1}`), inventory.Bag{Version: "ordinary-bag-v1", Gold: 1998, Items: []inventory.BagItem{{Slot: 124, Template: 1150, Amount: 1000}}})
	if e != nil {
		t.Fatal(e)
	}
	role, e := store.CreateCharacter(ctx, storage.Character{AccountID: account, Name: "SaleFixture", Request: []byte{}, ConfigVersion: source, State: state}, 24)
	if e != nil {
		t.Fatal(e)
	}
	itemMaterials, e := catalog.RestoreItemMaterials(source, []catalog.ItemMaterialEntry{{Template: 999999, Materials: []catalog.ItemMaterialCost{{Template: 1, Count: 1}}}})
	if e != nil {
		t.Fatal(e)
	}
	service := &workflow.ShopService{Store: store, ShopService: inventory.ShopService{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Items: map[uint32]catalog.LootItem{1150: {ID: 1150, Kind: "stackable", StackableType: "[waste]"}}}, EventModel: "reference90-gold-stack-v1", BagRules: inventory.BagRules{MissingStackLimit: 1000, EquipmentSlots: [2]uint16{9, 64}}, Prices: prices, ItemMaterials: itemMaterials}}
	w := worldSession{role: role, shop: service}
	payload, _ := hex.DecodeString("d20100009400000001007c00e8030000c908000000000000")
	binary.LittleEndian.PutUint32(payload[12:], 200)
	binary.LittleEndian.PutUint32(payload[16:], 649)
	packets, e := w.sellItem(payload)
	if e != nil {
		t.Fatal(e)
	}
	if len(packets) != 2 {
		t.Fatalf("packets %d", len(packets))
	}
	if packets[0].Kind != 1 || packets[0].ID != 22 || hex.EncodeToString(packets[0].Payload) != "010e27000001000000007c00c8000000" || packets[1].Kind != 0 || packets[1].ID != 14 {
		t.Fatalf("ACK/inventory mismatch: %+v", packets)
	}
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil || bag.Gold != 9998 || len(bag.Items) != 1 || bag.Items[0].Amount != 800 {
		t.Fatalf("partial sale %+v %v", bag, e)
	}
	// Two concurrent attempts each try to sell 500 of the remaining 800.
	results := make(chan error, 2)
	for range 2 {
		go func() {
			_, _, _, e := service.Sell(ctx, role, protocol.SellItemRequest{Rows: []protocol.SellItemRow{{Slot: 124, Count: 500}}})
			results <- e
		}()
	}
	successes := 0
	for range 2 {
		if <-results == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent oversell: %d successes", successes)
	}
	roles, e := store.Characters(ctx, account)
	if e != nil || len(roles) != 1 {
		t.Fatal(e)
	}
	bag, e = inventory.ReadBag(roles[0].State)
	if e != nil || bag.Gold != 29998 || bag.Items[0].Amount != 300 {
		t.Fatalf("durable concurrency %+v %v", bag, e)
	}
	// Buy uses the same source price, preventing buy-for-1/sell-for-40 arbitrage.
	bought, receipt, applied, e := service.Buy(ctx, role, protocol.BuyItemRequest{Template: 1150, Count: 2, NpcID: 466})
	if e != nil || !applied || receipt.Cost != 400 || receipt.NewGold != 29598 {
		t.Fatalf("buy %+v %v", receipt, e)
	}
	_, sale, applied, e := service.Sell(ctx, bought, protocol.SellItemRequest{Rows: []protocol.SellItemRow{{Slot: receipt.Slot, Count: 2}}})
	if e != nil || !applied || sale.GoldGained != 80 || len(sale.Rows) != 1 || sale.Rows[0].UnitPrice != 40 || sale.NewGold != 29678 {
		t.Fatalf("sell back %+v %v", sale, e)
	}
	other := role
	other.AccountID++
	if _, _, _, e = service.Sell(ctx, other, protocol.SellItemRequest{Rows: []protocol.SellItemRow{{Slot: 124, Count: 1}}}); e == nil {
		t.Fatal("cross-account sale")
	}
	// Stale session state must not resurrect the already consumed stack.
	w.role = role
	binary.LittleEndian.PutUint32(payload[12:], 300)
	binary.LittleEndian.PutUint32(payload[16:], 849)
	if _, e = w.sellItem(payload); e != nil {
		t.Fatal(e)
	}
	bag, e = inventory.ReadBag(w.role.State)
	if e != nil || len(bag.Items) != 0 || bag.Gold != 41678 {
		t.Fatalf("whole stack %+v %v", bag, e)
	}
	if _, e = w.sellItem(payload); e == nil {
		t.Fatal("resold empty slot")
	}
	t.Log("partial/whole stacks, current-source 40 gold, gold purchase 200, concurrent oversell and ownership verified in isolated PostgreSQL")
}
