package workflow

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"testing"
)

// nativeCashPilot imports the current inner PVF named by
// DFO_PVF_CORE_TEST_ARCHIVE; the test skips when no archive is configured.
func nativeCashPilot(t *testing.T, release bool) *cashshop.Pilot {
	t.Helper()
	a := catalog.OpenNativeArchive(t)
	c, err := cashshop.ImportPilot(a)
	if err != nil {
		t.Fatal(err)
	}
	p, err := cashshop.NewPilot(c, c.Source.Checksum, release)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

type vaultTestLedger struct {
	state storage.VaultState
	order storage.CashOrder
}

func TestAccountVaultPurchaseUsesSaveIdentity(t *testing.T) {
	p := nativeCashPilot(t, false)
	identity := p.Config.Source.SaveIdentity()
	if identity == p.Config.Source.Checksum {
		t.Fatal("fixture must distinguish archive provenance from save identity")
	}
	const product, template = int32(3999999), int32(2000000000)
	row := make([]pvf.Token, 14)
	row[0].Value, row[1].Value, row[2].Value, row[5].Value = product, template, 1, 100
	row[8] = pvf.Token{Type: 6}
	row[11].Value, row[12], row[13].Value = -1, pvf.Token{Type: 6}, -1
	p.Config.Entries[0].Section = "[item mod or ext]"
	p.Config.Entries[0].Row = row
	p.Config.Policies["[cargo account]"] = []pvf.Token{{Value: product}, {Value: template}, {Value: 1}, {Value: product}}
	rules := inventory.VaultRules{Account: &inventory.AccountVaultRules{RequiredLevel: 1, Upgrades: [][6]int64{{8, 100000, -1, 0, 0, -1}, {16, 100000, -1, 0, 0, int64(template)}}}}
	l := &vaultTestLedger{state: storage.VaultState{Slots: 8, Items: json.RawMessage(`[]`), ConfigVersion: identity}}
	_, applied, err := PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "source-account-vault-0001", []protocol.CeraCartItem{{Product: uint32(product), Quantity: 1}}, func(storage.CashReceipt) error { return nil })
	if err != nil || !applied || l.order.Source != identity || l.order.VaultSpace != 12 || l.state.Slots != 16 || l.state.ConfigVersion != identity {
		t.Fatalf("order=%+v vault=%+v error=%v", l.order, l.state, err)
	}
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
	p := nativeCashPilot(t, false)
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
		_, ok, e := PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "vault-test-000001", cart, prepare)
		if e != nil || !ok || l.state.Slots != u.After || l.order.Lines[0].UnitPrice != u.Price || l.order.Source != p.Config.Source.SaveIdentity() || l.state.ConfigVersion != rules.SourceSHA256 {
			t.Fatal(id, e)
		}
		if _, _, e = PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "vault-test-000002", cart, prepare); e == nil {
			t.Fatal("wrong tier accepted")
		}
		l.state.Slots = u.Before
		if _, _, e = PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "vault-test-000003", cart, func(storage.CashReceipt) error { return fmt.Errorf("encoding failed") }); e == nil || l.state.Slots != u.Before {
			t.Fatal("encoding failure mutated vault")
		}
		cart[0].Quantity = 2
		if _, _, e = PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "vault-test-000004", cart, prepare); e == nil {
			t.Fatal("quantity two accepted")
		}
		cart[0].Quantity = 1
		cart = append(cart, protocol.CeraCartItem{Product: 3000118, Quantity: 1})
		if _, _, e = PurchaseCashVault(p, context.Background(), l, rules, 1, 1, "vault-test-000005", cart, prepare); e == nil {
			t.Fatal("mixed cart accepted")
		}
	}
	if products[3000129].Price != 30 || products[3000130].Price != 60 || products[3000131].Price != 100 {
		t.Fatal("source prices changed")
	}
	t.Log("16 source tiers validated; exact tier and quantity; mixed cart and encoding failure rejected")
}
