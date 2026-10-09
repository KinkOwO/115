package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// skillTreeSKU / skillTreeTemplate identify the Skill Type Extension Ticket
// (美服 "Dual Skill Build License") in the PVF cash shop (etc/(r)cerashop.etc,
// [item mod or ext]): product 3000150 sells template 821 for 390 Cera. The
// ticket unlocks the character's second skill type, takes effect at purchase
// (86JP SkillTreeExpansionService.TryUnlock runs inside the cash transaction,
// "购买后立即生效，不进入背包"), and [not stackable buy] limits it to one per
// character.
const (
	skillTreeSKU      = 3000150
	skillTreeTemplate = 821
)

// SkillTreeLedger unlocks the character's second skill type inside the same
// cash transaction (Skill Type Extension Ticket, 购买即生效).
type SkillTreeLedger interface {
	PurchaseCashSkillTreeExpansion(context.Context, CashOrder) (CashReceipt, bool, error)
}

// TryPurchaseSkillTreeExpansion buys one Skill Type Extension Ticket and flips
// the character's skill-tree selection from locked (0) to "skill type 1" in the
// same cash transaction. It bypasses the ordinary classify path because
// [item mod or ext] products are reserved for capacity/refresh handlers the
// ordinary family refuses without.
func (p *Pilot) TryPurchaseSkillTreeExpansion(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (CashReceipt, bool, bool, error) {
	if p == nil || len(cart) != 1 || cart[0].Quantity != 1 {
		return CashReceipt{}, false, false, nil
	}
	line := cart[0]
	entry, found := p.findEntry(line.Product, skillTreeTemplate)
	if !found || entry.Row[0].Value != skillTreeSKU || entry.Row[1].Value != skillTreeTemplate {
		return CashReceipt{}, false, false, nil
	}
	fail := func(err error) (CashReceipt, bool, bool, error) {
		return CashReceipt{}, false, true, err
	}
	skillLedger, ok := ledger.(SkillTreeLedger)
	if !ok {
		return fail(fmt.Errorf("skill tree ledger missing"))
	}
	if err := p.Config.Validate(); err != nil {
		return fail(err)
	}
	product := Product{ID: uint32(entry.Row[0].Value), Template: uint32(entry.Row[1].Value), Units: uint32(entry.Row[2].Value), Cera: uint32(entry.Row[5].Value), Enabled: true}
	if product.ID != skillTreeSKU || product.Template != skillTreeTemplate || product.Units != 1 || product.Cera == 0 {
		return fail(fmt.Errorf("unsupported skill tree product source"))
	}
	quote := Service{Catalog: Catalog{Source: p.Config.Source.SaveIdentity(), Products: map[uint32]Product{product.ID: product}}}
	order, err := quote.Quote(account, character, key, cart, time.Now())
	if err != nil {
		return fail(err)
	}
	receipt, applied, err := skillLedger.PurchaseCashSkillTreeExpansion(ctx, order)
	return receipt, applied, true, err
}
