package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"time"
)

func (s *ItemService) ConsumeCharacterSlotTicket(ctx context.Context, role database.Character, r protocol.UseStackableRequest, base int) (database.Character, inventory.ConsumeReceipt, bool, error) {
	var out inventory.ConsumeReceipt
	fail := func(err error) (database.Character, inventory.ConsumeReceipt, bool, error) {
		return role, out, false, err
	}
	rule, found, err := s.Items.Catalog.CharacterSlotTicket(r.Template)
	if err != nil {
		return fail(err)
	}
	if !found || r.List != 0 {
		return fail(fmt.Errorf("invalid character slot ticket use"))
	}
	key, err := s.Items.ConsumeKey(InventoryRole(role), r)
	if err != nil {
		return fail(err)
	}
	// Historical generic-use receipts could have spent this template without
	// expanding. They must never masquerade as an expansion receipt.
	key = "character-slot-expand:" + key
	saved, applied, err := s.Store.CommitCharacterEventTx(ctx, role.AccountID, role.ID, s.Items.Catalog.Source.SaveIdentity(), key, s.Items.Model,
		func(tx *database.Tx, current database.Character) (json.RawMessage, json.RawMessage, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, err
			}
			for _, row := range bag.Items {
				if row.Slot == r.Slot && row.Template == r.Template && protocol.StoredItemExpired(row.ExpireTime, time.Now().Unix()) {
					return nil, nil, fmt.Errorf("角色栏扩展券已过期")
				}
			}
			bag, remaining, err := bag.Consume(s.Items.Catalog, r.Slot, r.Template)
			if err != nil {
				return nil, nil, err
			}
			capacity, err := tx.ExpandCharacterCapacity(ctx, base, rule.Amount, rule.Limit)
			if err != nil {
				return nil, nil, err
			}
			state, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(inventory.ConsumeReceipt{Slot: r.Slot, Template: r.Template, Remaining: remaining, Source: s.Items.Catalog.Source.SaveIdentity(), CharacterCapacity: uint16(capacity)})
			return state, receipt, err
		})
	if err != nil {
		return fail(err)
	}
	receipt, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if err != nil {
		return fail(err)
	}
	if err = json.Unmarshal(receipt, &out); err != nil {
		return fail(err)
	}
	if out.Template != r.Template || out.Slot != r.Slot || out.Source != s.Items.Catalog.Source.SaveIdentity() || out.CharacterCapacity == 0 {
		return fail(fmt.Errorf("character slot ticket receipt conflict"))
	}
	out.EventKey = key
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
