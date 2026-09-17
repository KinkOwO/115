package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
	"testing"
)

type vaultTestLedger struct {
	state storage.VaultState
	order storage.CashOrder
}

func (l *vaultTestLedger) PurchaseCashVault(_ context.Context, o storage.CashOrder, fn func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error) {
	next, e := fn(l.state)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	l.state = next
	l.order = o
	return storage.CashReceipt{Vault: &next}, true, nil
}
func TestVaultSourcePurchase(t *testing.T) {
	p, e := LoadPilot("../../configs/shop-special-candidate.json", "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80")
	if e != nil {
		t.Fatal(e)
	}
	products, e := p.Config.VaultUpgrades()
	if e != nil || len(products) != 16 {
		t.Fatal(products, e)
	}
	rules, e := inventory.LoadVaultRules("../../configs/vault.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for n := uint16(24); n <= 264; n += 16 {
		rules.VerifiedSlots = append(rules.VerifiedSlots, n)
	}
	for id, u := range products {
		l := &vaultTestLedger{state: storage.VaultState{Slots: u.Before, Items: []byte(`[{"slot":0,"template":14,"amount":5}]`), ConfigVersion: rules.SourceSHA256}}
		cart := []protocol.CeraCartItem{{Product: id, Quantity: 1}}
		prepare := func(r storage.CashReceipt) error { _, e := inventory.VaultPayload(*r.Vault); return e }
		_, ok, e := p.PurchaseVault(context.Background(), l, rules, 1, 1, "vault-test-000001", cart, prepare)
		if e != nil || !ok || l.state.Slots != u.After || l.order.Lines[0].UnitPrice != u.Price {
			t.Fatal(id, e)
		}
		if _, _, e = p.PurchaseVault(context.Background(), l, rules, 1, 1, "vault-test-000002", cart, prepare); e == nil {
			t.Fatal("wrong tier accepted")
		}
		l.state.Slots = u.Before
		if _, _, e = p.PurchaseVault(context.Background(), l, rules, 1, 1, "vault-test-000003", cart, func(storage.CashReceipt) error { return fmt.Errorf("encoding failed") }); e == nil || l.state.Slots != u.Before {
			t.Fatal("encoding failure mutated vault")
		}
		cart[0].Quantity = 2
		if _, _, e = p.PurchaseVault(context.Background(), l, rules, 1, 1, "vault-test-000004", cart, prepare); e == nil {
			t.Fatal("quantity two accepted")
		}
		cart[0].Quantity = 1
		cart = append(cart, protocol.CeraCartItem{Product: 3000118, Quantity: 1})
		if _, _, e = p.PurchaseVault(context.Background(), l, rules, 1, 1, "vault-test-000005", cart, prepare); e == nil {
			t.Fatal("mixed cart accepted")
		}
	}
	if products[3000129].Price != 30 || products[3000130].Price != 60 || products[3000131].Price != 100 {
		t.Fatal("source prices changed")
	}
	t.Log("16 source tiers validated; exact tier and quantity; mixed cart and encoding failure rejected")
}
