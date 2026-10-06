package main

import (
	"bytes"
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

// This regression requires an explicitly selected disposable database. It
// reproduces the live CMD64 rejection with the current native shop, without
// editing player saves or loading a historical JSON content catalog.
func TestShopPilotNativeSaveIdentityPurchase(t *testing.T) {
	runNativeShopPurchase(t, false)
}
func TestShopPilotNativeGoldPurchase(t *testing.T) {
	runNativeShopPurchase(t, true)
}
func runNativeShopPurchase(t *testing.T, goldPurchase bool) {
	// Only the native archive is required now: SQLite is the only storage engine and the
	// fixture opens its own database file, so there is no test database to name.
	archive := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if archive == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for isolated native purchase regression")
	}
	source, err := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: archive, ExpectedChecksum: os.Getenv("DFO_PVF_CORE_TEST_SHA256")})
	if err != nil {
		t.Fatal(err)
	}
	defer source.Close()
	raw, err := source.CashShop()
	if err != nil {
		t.Fatal(err)
	}
	pilot, err := cashshop.NewPilot(raw, source.Snapshot().Checksum, true)
	if err != nil {
		t.Fatal(err)
	}
	if pilot.Config.Source.Checksum == pilot.Config.Source.SaveIdentity() {
		t.Fatal("native source must differ from the save contract")
	}
	products, err := pilot.ProductSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	sku := uint32(3400232)
	if goldPurchase {
		sku = 3400315
	}
	product, found := products[sku]
	if goldPurchase && (!found || product.Gold != 100 || product.Cera != 0 || product.Template != 590715403 || product.Units != 1) {
		t.Fatalf("native live Gold row mismatch: %+v", product)
	}
	if !found {
		t.Fatal("live rejected SKU absent from native catalog")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	fixture, err := database.OpenTestFixture(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := fixture.Storage()
	for _, migrate := range []func(context.Context) error{store.Migrate, store.MigrateGrants, store.MigrateCashShop} {
		if err := migrate(ctx); err != nil {
			t.Fatal(err)
		}
	}
	account, err := store.DevelopmentAccount(ctx, "native-cash-fixture")
	if err != nil {
		t.Fatal(err)
	}
	// Match a normalized existing character, including unrelated saved data.
	original := json.RawMessage(`{"unrelated":{"keep":true},"inventory":{"version":"ordinary-bag-v1","gold":1000,"items":[{"slot":65,"Template":14,"Amount":3}]}}`)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "NativeCashFixture", Request: []byte{0}, ConfigVersion: pilot.Config.Source.SaveIdentity(), State: original}, 24)
	if err != nil {
		t.Fatal(err)
	}
	initialBalance := uint64(product.Cera) * 2
	if goldPurchase {
		initialBalance = 20000
	}
	if err = fixture.SeedAccountCurrency(ctx, account, int64(initialBalance)); err != nil {
		t.Fatal(err)
	}
	bodyHex := "000001000028e2330001000000000000"
	if goldPurchase {
		bodyHex = "00000100007be2330001000000000000"
	}
	body, err := hex.DecodeString(bodyHex)
	if err != nil {
		t.Fatal(err)
	}
	frame := append(make([]byte, 13), body...)
	session, err := newShopPilotSession()
	if err != nil {
		t.Fatal(err)
	}
	session.keys = make([]byte, wire.SessionKeyBytes)
	// The old archive-hash order still fails the unchanged ledger guard.
	old := database.CashOrder{Key: "old-native-source-0001", Account: account, Character: role.ID, Source: pilot.Config.Source.Checksum, Lines: []database.CashOrderLine{{Product: product.ID, Template: product.Template, Quantity: 1, Units: product.Units, UnitPrice: product.Cera, GoldUnitPrice: product.Gold}}}
	if _, _, err = store.PurchaseCashToBag(ctx, old, func(raw json.RawMessage) (json.RawMessage, error) {
		t.Fatal("wrong-source delivery executed")
		return nil, nil
	}); err == nil || !strings.Contains(err.Error(), "does not match character source") {
		t.Fatalf("old order: %v", err)
	}
	if goldPurchase {
		if err := fixture.SeedWalletGold(ctx, role.ID, json.RawMessage(`99`)); err != nil {
			t.Fatal(err)
		}
		if _, _, err := session.purchase(ctx, pilot, store, account, role.ID, body, frame); err == nil || !strings.Contains(err.Error(), "insufficient Gold") {
			t.Fatalf("insufficient Gold: %v", err)
		}
		if err := fixture.SeedWalletGold(ctx, role.ID, json.RawMessage(`1000`)); err != nil {
			t.Fatal(err)
		}
	}
	// Encode failure must also leave balance and state unchanged.
	badSession := *session
	badSession.keys = nil
	if _, _, err = badSession.purchase(ctx, pilot, store, account, role.ID, body, frame); err == nil || !strings.Contains(err.Error(), "cipher") {
		t.Fatalf("encode failure: %v", err)
	}
	failedState, err := fixture.CharacterState(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	failedBag, err := inventory.ReadBag(failedState)
	if err != nil || failedBag.Gold != 1000 || len(failedBag.Items) != 1 {
		t.Fatalf("failed request changed bag: %+v %v", failedBag, err)
	}
	balance, err := store.AccountCera(ctx, account)
	if err != nil || balance != initialBalance {
		t.Fatalf("failed requests charged: %d %v", balance, err)
	}
	receipt, applied, err := session.purchase(ctx, pilot, store, account, role.ID, body, frame)
	if err != nil || !applied || receipt.Charged != uint64(product.Cera) || receipt.After != initialBalance-uint64(product.Cera) || receipt.GoldCharged != uint64(product.Gold) {
		t.Fatalf("native receipt=%+v applied=%t error=%v", receipt, applied, err)
	}
	bag, bagErr := inventory.ReadBag(receipt.CharacterState)
	var delivered uint32
	var retained bool
	for _, item := range bag.Items {
		if item.Template == product.Template {
			delivered += item.Amount
		}
		if item.Slot == 65 && item.Template == 14 && item.Amount == 3 {
			retained = true
		}
	}
	if bagErr != nil || !retained || delivered != product.Units || bag.Gold != 1000-product.Gold {
		t.Fatalf("native bag=%+v error=%v", bag, bagErr)
	}
	if len(receipt.Deliveries) != 1 || receipt.Deliveries[0].Template != product.Template || receipt.Deliveries[0].Amount != product.Units {
		t.Fatalf("wrong native delivery: %+v", receipt.Deliveries)
	}
	packets, err := shopPilotSpaces(pilot, receipt, receipt.After, true)
	expectedPackets := 3
	if goldPurchase {
		expectedPackets = 4
	}
	if err != nil || len(packets) != expectedPackets {
		t.Fatalf("native packets=%+v error=%v", packets, err)
	}
	if goldPurchase {
		restore, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		last := packets[len(packets)-1]
		if err != nil || last.Kind != 0 || last.ID != 13 || !bytes.Equal(last.Payload, restore) {
			t.Fatalf("Gold balance snapshot after ACK: %+v %v", last, err)
		}
	}
	if _, err = preparePackets(session.keys, packets); err != nil {
		t.Fatal(err)
	}
	state, version, err := fixture.CharacterSnapshot(ctx, role.ID)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if json.Unmarshal(state, &document) != nil || !bytes.Contains(document["unrelated"], []byte("true")) || version != role.ConfigVersion {
		t.Fatalf("unrelated save data or identity changed: %s", state)
	}
	orderSource, err := fixture.CashOrderSource(ctx, account, receipt.Order)
	if err != nil || orderSource != role.ConfigVersion {
		t.Fatalf("audit identity=%s error=%v", orderSource, err)
	}
	// A retry must not debit twice or overwrite a newer saved state.
	if goldPurchase {
		if err := fixture.SeedWalletGold(ctx, role.ID, json.RawMessage(`777`)); err != nil {
			t.Fatal(err)
		}
	}
	if err = fixture.MergeCharacterFields(ctx, role.ID, json.RawMessage(`{"newer":true}`)); err != nil {
		t.Fatal(err)
	}
	replay, applied, err := session.purchase(ctx, pilot, store, account, role.ID, body, frame)
	if err != nil || applied || !bytes.Contains(replay.CharacterState, []byte("newer")) {
		t.Fatalf("native replay: applied=%t error=%v", applied, err)
	}
	if goldPurchase {
		replayBag, err := inventory.ReadBag(replay.CharacterState)
		if err != nil || replayBag.Gold != 777 {
			t.Fatalf("replay overwrote newer Gold: %+v %v", replayBag, err)
		}
	}
	balance, err = store.AccountCera(ctx, account)
	if err != nil || balance != receipt.After {
		t.Fatalf("retry charged: %d %v", balance, err)
	}
	var audits, pending int64
	if audits, err = fixture.CashOrderCount(ctx, account); err != nil || audits != 1 {
		t.Fatalf("audits=%d error=%v", audits, err)
	}
	if pending, err = fixture.CashInventoryCount(ctx, account, 1, 0, 0); err != nil || pending != 0 {
		t.Fatalf("duplicate pending delivery=%d error=%v", pending, err)
	}
	if goldPurchase {
		testNativeMixedGoldCart(t, ctx, fixture, pilot, session.keys, account, role.ID, initialBalance)
	}
	t.Logf("PASS current native SKU=%d template=%d units=%d Cera=%d Gold=%d; wrong source/encoding failures rolled back, debit+delivery+audit committed once, replay preserved newer state", product.ID, product.Template, product.Units, product.Cera, product.Gold)
}
