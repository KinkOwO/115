package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
)

type VaultUpgrade struct {
	Product, Template, Price uint32
	Before, After            uint16
}
type VaultLedger interface {
	PurchaseCashVault(context.Context, storage.CashOrder, func(storage.VaultState) (storage.VaultState, error)) (storage.CashReceipt, bool, error)
}

// Native constructor 14008e7b0 defines seventeen capacities, 8..264. Cargo1
// supplies product/template pairs followed by ordinal/product pairs. Keep this
// separate from cargo2 and account mappings, which have different semantics.
func (c PilotConfig) VaultUpgrades() (map[uint32]VaultUpgrade, error) {
	out := map[uint32]VaultUpgrade{}
	cells := c.Policies["[cargo 1]"]
	if len(cells) == 0 {
		return out, nil
	}
	if len(cells) != 64 {
		return nil, fmt.Errorf("unexpected cargo1 mapping width")
	}
	entries := map[int32]OrdinaryProduct{}
	for _, e := range c.Entries {
		if len(e.Row) == 14 {
			entries[e.Row[0].Value] = e
		}
	}
	for i := 0; i < 16; i++ {
		for _, at := range []int{2 * i, 2*i + 1, 32 + 2*i, 33 + 2*i} {
			if cells[at].Type != 0 {
				return nil, fmt.Errorf("invalid cargo1 token")
			}
		}
		id, template := cells[2*i].Value, cells[2*i+1].Value
		if cells[32+2*i].Value != int32(i+1) || cells[33+2*i].Value != id {
			return nil, fmt.Errorf("cargo1 ordinal mismatch")
		}
		e, ok := entries[id]
		if !ok || e.Section != "[item mod or ext]" || e.ImportError != "" {
			return nil, fmt.Errorf("missing cargo1 product definition")
		}
		r := e.Row
		for _, at := range []int{0, 1, 2, 3, 4, 5, 6, 7, 9, 10, 11, 13} {
			if r[at].Type != 0 {
				return nil, fmt.Errorf("invalid cargo price cell")
			}
		}
		if id <= 0 || template <= 0 || r[1].Value != template || r[2].Value != 1 || r[5].Value <= 0 || r[3].Value != 0 || r[4].Value != 0 || r[6].Value != 0 || r[7].Value != 0 || r[9].Value != 0 || r[10].Value != 0 || r[11].Value != -1 || r[12].Type != 6 || r[12].Text != "" || r[13].Value != -1 {
			return nil, fmt.Errorf("unsupported cargo1 price policy")
		}
		if !digestValid(e.Item.SHA256) {
			return nil, fmt.Errorf("missing cargo1 script hash")
		}
		// Limit and reward policies must not be silently discarded.
		for name, width := range map[string]int{"[purchasing limit]": 8, "[specific product mileage]": 2} {
			policy := c.Policies[name]
			for j := 0; j < len(policy); j += width {
				if policy[j].Value == id {
					return nil, fmt.Errorf("cargo1 has additional purchase policy")
				}
			}
		}
		out[uint32(id)] = VaultUpgrade{uint32(id), uint32(template), uint32(r[5].Value), uint16(8 + 16*i), uint16(24 + 16*i)}
	}
	return out, nil
}

func (p *Pilot) PurchaseVault(ctx context.Context, ledger VaultLedger, rules inventory.VaultRules, account, character int64, key string, cart []protocol.CeraCartItem, prepare func(storage.CashReceipt) error) (storage.CashReceipt, bool, error) {
	if p == nil || ledger == nil || prepare == nil || len(cart) != 1 || cart[0].Quantity != 1 {
		return storage.CashReceipt{}, false, fmt.Errorf("vault upgrade requires one unit in a separate order")
	}
	if e := p.Config.validate(); e != nil {
		return storage.CashReceipt{}, false, e
	}
	products, e := p.Config.VaultUpgrades()
	if e != nil {
		return storage.CashReceipt{}, false, e
	}
	u, ok := products[cart[0].Product]
	if !ok {
		return storage.CashReceipt{}, false, fmt.Errorf("unsupported vault upgrade product")
	}
	o := storage.CashOrder{Key: key, Account: account, Character: character, Source: p.Config.Source.Checksum, Lines: []storage.CashOrderLine{{Product: u.Product, Template: u.Template, Quantity: 1, Units: 1, UnitPrice: u.Price}}}
	return ledger.PurchaseCashVault(ctx, o, func(v storage.VaultState) (storage.VaultState, error) {
		if v.ConfigVersion != rules.SourceSHA256 || v.Slots != u.Before {
			return v, fmt.Errorf("vault upgrade requires %d current slots", u.Before)
		}
		allowed := false
		for _, n := range rules.VerifiedSlots {
			if n == u.After {
				allowed = true
			}
		}
		if !allowed {
			return v, fmt.Errorf("vault target capacity not enabled")
		}
		v.Slots = u.After
		if _, e := inventory.VaultPayload(v); e != nil {
			return v, e
		}
		receipt := storage.CashReceipt{Vault: &v, Deliveries: []storage.CashDelivery{{Product: u.Product, Template: u.Template, Amount: 1, Quantity: 1}}}
		if e := prepare(receipt); e != nil {
			return v, e
		}
		return v, nil
	})
}
