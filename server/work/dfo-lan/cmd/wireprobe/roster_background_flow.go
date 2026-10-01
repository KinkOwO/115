package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/db"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/rosterbg"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

// restoreRosterBackgrounds owns the complete account-level background restore
// operation. The connection loop supplies only its character owner, account,
// output path, and event sink.
func restoreRosterBackgrounds(characters *character.Service, account int64, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if characters == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	state, err := characters.Store.RosterBackgrounds(ctx, account)
	if err != nil {
		event(map[string]any{"kind": "roster_background_restore_error", "error": err.Error()})
		return err
	}
	payload, err := rosterbg.Restore(state)
	if err != nil {
		return err
	}
	if err = send(0, 1759, payload); err != nil {
		return err
	}
	event(map[string]any{"kind": "roster_background_restored", "selected": state.Selected, "owned_count": len(state.Owned)})
	return nil
}

// useRosterBackgroundTicket 将真实背包扣券、账号授权和请求回执放在同一事务中。
func (w *worldSession) useRosterBackgroundTicket(ctx context.Context, p, raw []byte, prefix string, event func(map[string]any)) ([]outboundPacket, error) {
	slot, err := protocol.DecodeRosterBackgroundTicket(p)
	if err != nil {
		return nil, err
	}
	if w == nil || w.characters == nil || w.characters.Store == nil || w.loot == nil ||
		w.role.ID == 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("背景券使用缺少当前账号角色或背包")
	}
	if w.activeDungeon != nil || w.state.Position.Town == 0 {
		return nil, fmt.Errorf("背景券只能在城镇使用")
	}
	if w.role.ConfigVersion != w.loot.Catalog.Source.SaveIdentity() {
		return nil, fmt.Errorf("背景券使用的角色与物品目录版本不一致")
	}
	key := fmt.Sprintf("roster-background-ticket:%s:%x", prefix, sha256.Sum256(raw))
	var receipt struct {
		Template  uint32          `json:"template"`
		Slot      uint16          `json:"slot"`
		Remaining uint32          `json:"remaining"`
		Unlock    rosterbg.Unlock `json:"unlock"`
	}
	store := w.characters.Store
	saved, applied, err := store.CommitCharacterEventTx(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "roster-background-ticket-v1",
		func(tx db.Tx, current storage.Character) (json.RawMessage, json.RawMessage, error) {
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			now := time.Now()
			var template uint32
			for _, item := range bag.Items {
				if item.Slot != slot || item.Amount == 0 {
					continue
				}
				if protocol.StoredItemExpired(item.ExpireTime, now.Unix()) {
					return nil, nil, fmt.Errorf("背景券物品已过期，未消耗道具")
				}
				template = item.Template
				break
			}
			ticket, e := rosterbg.TicketFor(template)
			if e != nil {
				return nil, nil, e
			}
			grant, e := ticket.UnlockAt(now)
			if e != nil {
				return nil, nil, e
			}
			if e = store.UnlockRosterBackground(ctx, tx, current.AccountID, grant, now); e != nil {
				return nil, nil, e
			}
			bag, remaining, e := bag.Consume(w.loot.Catalog, slot, template)
			if e != nil {
				return nil, nil, e
			}
			state, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, nil, e
			}
			receipt.Template, receipt.Slot, receipt.Remaining, receipt.Unlock = template, slot, remaining, grant
			outcome, e := json.Marshal(receipt)
			return state, outcome, e
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	outcome, err := store.CharacterEventReceipt(ctx, w.account, saved.ID, key)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(outcome, &receipt); err != nil {
		return nil, err
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	row := protocol.EmptyOrdinaryItem(slot)
	for _, item := range bag.Items {
		if item.Slot == slot {
			row = protocol.OrdinaryItem(item.Slot, item.Template, item.Amount, item.ExpireTime)
			break
		}
	}
	update, err := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if err != nil {
		return nil, err
	}
	state, err := store.RosterBackgrounds(ctx, w.account)
	if err != nil {
		return nil, err
	}
	backgrounds, err := rosterbg.Restore(state)
	if err != nil {
		return nil, err
	}
	event(map[string]any{"kind": "背景券解锁已保存", "character_id": saved.ID, "template": receipt.Template,
		"slot": slot, "remaining": receipt.Remaining, "category": receipt.Unlock.Category,
		"background_id": receipt.Unlock.ID, "expires_at": receipt.Unlock.ExpiresAt, "applied": applied})
	var packets []outboundPacket
	// 先让原生回执消费旧槽位，再同步绝对数量；重放不重复触发客户端扣除。
	if applied {
		packets = append(packets, outboundPacket{"背景券使用成功", 1, 507, protocol.RosterBackgroundTicketSuccess(slot)})
	}
	return append(packets, outboundPacket{"背景券库存同步", 0, 14, update},
		outboundPacket{"背景授权同步", 0, 1759, backgrounds}), nil
}
