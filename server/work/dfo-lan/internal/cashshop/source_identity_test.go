package cashshop

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"testing"
)

func TestCashPurchaseSaveIdentity(t *testing.T) {
	config := contractCartCatalog(t)
	identity := config.Source.SaveIdentity()
	if identity == config.Source.Checksum {
		t.Fatal("fixture must distinguish archive provenance from save identity")
	}
	t.Run("ordinary", func(t *testing.T) {
		ledger := &packLedger{state: json.RawMessage(`{}`)}
		_, _, err := (&Pilot{Config: config}).Purchase(context.Background(), ledger, 1, 1, "source-ordinary-0001", []protocol.CeraCartItem{{Product: 3000999, Quantity: 1}})
		if err != nil || ledger.order.Source != identity {
			t.Fatalf("order source=%s error=%v", ledger.order.Source, err)
		}
	})
	t.Run("contract cart", func(t *testing.T) {
		ledger := &contractCartLedger{state: json.RawMessage(`{}`)}
		_, _, err := (&Pilot{Config: config}).Purchase(context.Background(), ledger, 1, 1, "source-contract-0001", []protocol.CeraCartItem{{Product: 3500001, Quantity: 1}, {Product: 3000999, Quantity: 1}})
		if err != nil || ledger.mixedOrder.Source != identity {
			t.Fatalf("order source=%s error=%v", ledger.mixedOrder.Source, err)
		}
	})
	expansion := config.Entries[len(config.Entries)-1]
	expansion.Row = append([]pvf.Token(nil), expansion.Row...)
	expansion.Row[0].Value = 3999999
	expansion.Row[1].Value = 2660296
	expansion.Row[9].Value = 0
	expansion.Section = "[item mod or ext]"
	expansion.IndexPath = "stackable/cash/inven_upgradekit1.stk"
	expansion.Item = catalog.ScriptRecord{Path: expansion.IndexPath, SHA256: config.Source.Checksum, Cells: []pvf.Token{{Type: 3, Text: "[stackable type]"}, {Type: 6, Text: "[etc]"}}}
	config.Entries = append(append([]OrdinaryProduct(nil), config.Entries...), expansion)
	t.Run("inventory expansion", func(t *testing.T) {
		ledger := &packLedger{state: json.RawMessage(`{}`)}
		_, _, err := (&Pilot{Config: config}).Purchase(context.Background(), ledger, 1, 1, "source-expansion-0001", []protocol.CeraCartItem{{Product: 3999999, Quantity: 1}})
		bag, bagErr := inventory.ReadBag(ledger.state)
		if err != nil || bagErr != nil || bag.Expansion != 1 || ledger.order.Source != identity {
			t.Fatalf("order=%+v bag=%+v error=%v/%v", ledger.order, bag, err, bagErr)
		}
	})
	t.Run("account vault", func(t *testing.T) {
		accountConfig := config
		accountConfig.Policies = make(map[string][]pvf.Token, len(config.Policies)+1)
		for name, cells := range config.Policies {
			accountConfig.Policies[name] = cells
		}
		accountConfig.Policies["[cargo account]"] = []pvf.Token{{Value: 3999999}, {Value: 2660296}, {Value: 1}, {Value: 3999999}}
		rules := inventory.VaultRules{Account: &inventory.AccountVaultRules{RequiredLevel: 1, Upgrades: [][6]int64{{8, 100000, -1, 0, 0, -1}, {16, 100000, -1, 0, 0, 2660296}}}}
		ledger := &vaultTestLedger{state: storage.VaultState{Slots: 8, Items: json.RawMessage(`[]`), ConfigVersion: identity}}
		_, applied, err := (&Pilot{Config: accountConfig}).PurchaseVault(context.Background(), ledger, rules, 1, 1, "source-account-vault-0001", []protocol.CeraCartItem{{Product: 3999999, Quantity: 1}}, func(storage.CashReceipt) error { return nil })
		if err != nil || !applied || ledger.order.Source != identity || ledger.order.VaultSpace != 12 || ledger.state.Slots != 16 || ledger.state.ConfigVersion != identity {
			t.Fatalf("order=%+v vault=%+v error=%v", ledger.order, ledger.state, err)
		}
	})
}
