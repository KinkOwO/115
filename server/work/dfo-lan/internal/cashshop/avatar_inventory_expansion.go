package cashshop

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

// Resolve the action from the native shop item script, including aliases.
// No item ID or shop SKU is used as a substitute for its PVF definition.
func (p *Pilot) AvatarInventoryExpansion(template uint32) (byte, bool, error) {
	if p == nil {
		return 0, false, nil
	}
	entry, found := p.findEntry(0, template)
	if !found {
		return 0, false, nil
	}
	sections := shopSections(entry.Item.Cells)
	action := sections["[action type]"]
	if len(action) == 0 || action[0].Text != "[avatar inventory expansion]" {
		return 0, false, nil
	}
	if entry.ImportError != "" || !digestValid(entry.Item.SHA256) || entry.IndexPath != entry.Item.Path {
		return 0, true, fmt.Errorf("avatar expansion source definition mismatch")
	}
	packet, place := sections["[use action packet]"], sections["[action usable place]"]
	if len(action) != 2 || action[0].Type != 6 || action[1].Type != 0 || action[1].Value <= 0 || action[1].Value > int32(protocol.MaxAvatarInventoryExpansion) || len(packet) != 1 || packet[0].Type != 0 || packet[0].Value != 1 || len(place) != 1 || place[0].Type != 6 || place[0].Text != "[village]" {
		return 0, true, fmt.Errorf("unsupported avatar expansion source action")
	}
	return byte(action[1].Value), true, nil
}

func (p *Pilot) TryPurchaseAvatarInventoryExpansion(ctx context.Context, ledger BagLedger, account, character int64, key string, cart []protocol.CeraCartItem) (CashReceipt, bool, bool, error) {
	for _, line := range cart {
		entry, found := p.findEntry(line.Product, 0)
		if !found {
			continue
		}
		steps, handled, err := p.AvatarInventoryExpansion(uint32(entry.Row[1].Value))
		if !handled && err == nil {
			continue
		}
		fail := func(err error) (CashReceipt, bool, bool, error) {
			return CashReceipt{}, false, true, err
		}
		if err != nil {
			return fail(err)
		}
		if ledger == nil || len(cart) != 1 || line.Quantity != 1 {
			return fail(fmt.Errorf("时装栏扩展券必须单独购买一张"))
		}
		if err := p.Config.Validate(); err != nil {
			return fail(err)
		}
		product, _, err := p.Config.classify(entry)
		if err != nil {
			return fail(err)
		}
		if product.Units != 1 {
			return fail(fmt.Errorf("invalid avatar expansion product count"))
		}
		quote := Service{Catalog: Catalog{Source: p.Config.Source.SaveIdentity(), Products: map[uint32]Product{product.ID: product}}}
		order, err := quote.Quote(account, character, key, cart, time.Now())
		if err != nil {
			return fail(err)
		}
		gold, err := order.GoldTotal()
		if err != nil {
			return fail(err)
		}
		receipt, applied, err := ledger.PurchaseCashToBag(ctx, order, func(raw json.RawMessage) (json.RawMessage, error) {
			bag, err := inventory.ReadBag(raw)
			if err != nil {
				return nil, err
			}
			if int(bag.AvatarExpansion)+int(steps) > int(protocol.MaxAvatarInventoryExpansion) {
				return nil, fmt.Errorf("avatar inventory fully expanded")
			}
			if uint64(bag.Gold) < gold {
				return nil, fmt.Errorf("insufficient Gold")
			}
			bag.Gold -= uint32(gold)
			bag.AvatarExpansion += steps
			return inventory.SaveBag(raw, bag)
		})
		return receipt, applied, true, err
	}
	return CashReceipt{}, false, false, nil
}
