package main

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"strings"
	"testing"
)

func testNativeMixedGoldCart(t *testing.T, ctx context.Context, fixture *database.TestFixture, pilot *cashshop.Pilot, keys []byte, account, role int64, balance uint64) {
	t.Helper()
	store := fixture.Storage()
	cart := []protocol.CeraCartItem{{Product: 3400315, Quantity: 1}, {Product: 3400232, Quantity: 1}}
	key := "native-mixed-gold-0001"
	assertUnchanged := func(gold uint32, cera uint64, orders int) {
		t.Helper()
		state, err := fixture.CharacterState(ctx, role)
		if err != nil {
			t.Fatal(err)
		}
		bag, err := inventory.ReadBag(state)
		actual, ce := store.AccountCera(ctx, account)
		count, countErr := fixture.CashOrderCount(ctx, account)
		if countErr != nil {
			t.Fatal(countErr)
		}
		if err != nil || ce != nil || bag.Gold != gold || actual != cera || count != int64(orders) {
			t.Fatalf("mixed transaction: Gold=%d Cera=%d orders=%d error=%v/%v", bag.Gold, actual, count, err, ce)
		}
	}
	setGold := func(value string) {
		t.Helper()
		if err := fixture.SeedWalletGold(ctx, role, json.RawMessage(value)); err != nil {
			t.Fatal(err)
		}
	}
	setCera := func(value uint64) {
		t.Helper()
		if err := fixture.SeedAccountCurrency(ctx, account, int64(value)); err != nil {
			t.Fatal(err)
		}
	}
	ledger := preparedBagLedger{ledger: store, keys: keys, pilot: pilot}
	setCera(0)
	if _, _, err := pilot.Purchase(ctx, ledger, account, role, key, cart); err == nil || !strings.Contains(err.Error(), "insufficient CERA") {
		t.Fatalf("mixed Cera shortage: %v", err)
	}
	assertUnchanged(777, 0, 1)
	setCera(balance)
	setGold("99")
	if _, _, err := pilot.Purchase(ctx, ledger, account, role, key, cart); err == nil || !strings.Contains(err.Error(), "insufficient Gold") {
		t.Fatalf("mixed Gold shortage: %v", err)
	}
	assertUnchanged(99, balance, 1)
	setGold("777")
	bad := ledger
	bad.keys = nil
	if _, _, err := pilot.Purchase(ctx, bad, account, role, key, cart); err == nil || !strings.Contains(err.Error(), "cipher") {
		t.Fatalf("mixed encode failure: %v", err)
	}
	assertUnchanged(777, balance, 1)
	type result struct {
		receipt database.CashReceipt
		applied bool
		err     error
	}
	results := make(chan result, 2)
	for i := 0; i < 2; i++ {
		go func() {
			r, a, e := pilot.Purchase(ctx, ledger, account, role, key, cart)
			results <- result{r, a, e}
		}()
	}
	applied := 0
	for i := 0; i < 2; i++ {
		r := <-results
		if r.err != nil || r.receipt.GoldCharged != 100 || r.receipt.Charged != 7900 {
			t.Fatalf("concurrent mixed purchase: %+v", r)
		}
		if r.applied {
			applied++
		}
	}
	if applied != 1 {
		t.Fatalf("concurrent mixed purchase applied %d times", applied)
	}
	assertUnchanged(677, balance-7900, 2)
	t.Log("native mixed cart: Gold/Cera shortage and encode failures rolled back; simultaneous retry charged both wallets once")
}
