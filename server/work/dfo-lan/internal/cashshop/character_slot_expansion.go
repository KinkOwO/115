package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// characterSlotSKU / characterSlotTemplate identify the Character Slot
// Extension Kit in the PVF cash shop (etc/(r)cerashop.etc, [item mod or ext] +
// [immediately adaptive product]): product 3000152 sells template 2660239 for
// 290 Cera. The kit is account-level and takes effect at purchase (no bag
// item), matching the official "immediately adaptive product" policy.
const (
	characterSlotSKU      = 3000152
	characterSlotTemplate = 2660239
)

// TryPurchaseCharacterSlotExpansion buys one Character Slot Extension Kit and
// grants the account-level slot bonus inside the same cash transaction. It
// bypasses the ordinary classify path because the shop policy marks the SKU
// immediately adaptive, which the ordinary family refuses without a handler.
func (p *Pilot) TryPurchaseCharacterSlotExpansion(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (CashReceipt, bool, bool, error) {
	if p == nil || len(cart) != 1 || cart[0].Quantity != 1 {
		return CashReceipt{}, false, false, nil
	}
	line := cart[0]
	if line.Product != characterSlotSKU {
		return CashReceipt{}, false, false, nil
	}
	// findEntry permits template fallback; purchase routing must match the requested SKU.
	entry, found := p.findEntry(line.Product, 0)
	if !found || entry.Row[0].Value != characterSlotSKU || entry.Row[1].Value != characterSlotTemplate {
		return CashReceipt{}, false, false, nil
	}
	fail := func(err error) (CashReceipt, bool, bool, error) {
		return CashReceipt{}, false, true, err
	}
	slotLedger, ok := ledger.(CharacterSlotLedger)
	if !ok {
		return fail(fmt.Errorf("character slot ledger missing"))
	}
	if err := p.Config.Validate(); err != nil {
		return fail(err)
	}
	product := Product{ID: uint32(entry.Row[0].Value), Template: uint32(entry.Row[1].Value), Units: uint32(entry.Row[2].Value), Cera: uint32(entry.Row[5].Value), Enabled: true}
	if product.ID != characterSlotSKU || product.Template != characterSlotTemplate || product.Units != 1 || product.Cera == 0 {
		return fail(fmt.Errorf("unsupported character slot product source"))
	}
	quote := Service{Catalog: Catalog{Source: p.Config.Source.SaveIdentity(), Products: map[uint32]Product{product.ID: product}}}
	order, err := quote.Quote(account, character, key, cart, time.Now())
	if err != nil {
		return fail(err)
	}
	receipt, applied, err := slotLedger.PurchaseCashCharacterSlots(ctx, order, 1)
	return receipt, applied, true, err
}
