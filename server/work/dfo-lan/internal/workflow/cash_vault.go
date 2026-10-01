package workflow

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
)

type VaultLedger interface {
	PurchaseCashVault(context.Context, storage.CashOrder, func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error)
}

func PurchaseCashVault(p *cashshop.Pilot, ctx context.Context, ledger VaultLedger, rules inventory.VaultRules, account, character int64, key string, cart []protocol.CeraCartItem, prepare func(storage.CashReceipt) error) (storage.CashReceipt, bool, error) {
	if p == nil || ledger == nil || prepare == nil || len(cart) != 1 || cart[0].Quantity != 1 {
		return storage.CashReceipt{}, false, fmt.Errorf("vault upgrade requires one unit in a separate order")
	}
	if e := p.Config.Validate(); e != nil {
		return storage.CashReceipt{}, false, e
	}
	products, e := p.Config.VaultUpgrades()
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	u, ok := products[cart[0].Product]
	secondary, e := p.Config.VaultUpgrades(45)
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	if second, found := secondary[cart[0].Product]; found {
		if !ok {
			u, ok = second, true
		} else if selector, supports := ledger.(interface {
			VaultPurchaseSpace(context.Context, int64, int64, string) (byte, error)
		}); supports {
			space, err := selector.VaultPurchaseSpace(ctx, account, character, key)
			if err != nil {
				return storage.CashReceipt{}, false, err
			}
			if space == 45 {
				u = second
			} else if space != 0 {
				return storage.CashReceipt{}, false, fmt.Errorf("共用扩容商品的金库目标无效")
			}
		}
	}
	if !ok {
		accountProducts, err := p.Config.AccountVaultUpgrades(rules.Account)
		if err != nil {
			return storage.CashReceipt{}, false, err
		}
		u, ok = accountProducts[cart[0].Product]
		if !ok {
			return storage.CashReceipt{}, false, fmt.Errorf("unsupported vault upgrade product")
		}
	}
	o := storage.CashOrder{Key: key, Account: account, Character: character, Source: p.Config.Source.SaveIdentity(), Lines: []storage.CashOrderLine{{Product: u.Product, Template: u.Template, Quantity: 1, Units: 1, UnitPrice: u.Price}}}
	o.VaultSpace = u.Space
	return ledger.PurchaseCashVault(ctx, o, func(v storage.VaultState) (storage.VaultState, error) {
		source := rules.SourceSHA256
		if u.Space == 12 {
			source = p.Config.Source.SaveIdentity()
		}
		if v.ConfigVersion != source || v.Slots != u.Before {
			return v, fmt.Errorf("vault upgrade requires %d current slots", u.Before)
		}
		allowed := u.Space == 12
		for _, n := range rules.VerifiedSlots {
			if n == u.After {
				allowed = true
			}
		}
		if !allowed {
			return v, fmt.Errorf("vault target capacity not enabled")
		}
		v.Slots = u.After
		if u.Space == 12 {
			if _, e := inventory.AccountVaultPayload(storage.AccountVaultState{Slots: v.Slots, Items: v.Items}, *rules.Account); e != nil {
				return v, e
			}
		} else if _, e := inventory.VaultPayload(v); e != nil {
			return v, e
		}
		receipt := storage.CashReceipt{Vault: &v, VaultSpace: u.Space, Deliveries: []storage.CashDelivery{{Product: u.Product, Template: u.Template, Amount: 1, Quantity: 1}}}
		if e := prepare(receipt); e != nil {
			return v, e
		}
		return v, nil
	})
}
