package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"fmt"
	"sort"
	"time"
)

// radiantBoxOpens maps the window's mode word to how many boxes it opens. The
// sender 141aa9240 stores the mode in the window and picks between its two
// material buttons with it, so the mode is the only count the request carries.
// The mapping is logged with every open so a live run can confirm it.
func radiantBoxOpens(mode uint32) (uint32, error) {
	switch mode {
	case 0:
		return 1, nil
	case 1:
		return 10, nil
	}
	return 0, fmt.Errorf("radiant box mode %d is not implemented", mode)
}

// radiantBoxHeld reports which imported box the player is holding. The event
// request names no template, so the bag decides; the transactional open re-reads
// the same bag before it spends anything.
func radiantBoxHeld(service *loot.Service, role storage.Character) (uint32, error) {
	box, ok, e := radiantBoxInBag(service, role)
	if e != nil {
		return 0, e
	}
	if !ok {
		return 0, fmt.Errorf("no imported radiant box is owned")
	}
	return box, nil
}

func radiantBoxInBag(service *loot.Service, role storage.Character) (uint32, bool, error) {
	if service == nil || service.Boxes == nil {
		return 0, false, fmt.Errorf("box catalog is not loaded")
	}
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return 0, false, e
	}
	ids := make([]string, 0, len(service.Boxes.Tables))
	for id := range service.Boxes.Tables {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		var box uint32
		if _, scanErr := fmt.Sscanf(id, "%d", &box); scanErr != nil {
			continue
		}
		if _, held, ok := loot.BoxOpenMaterial(bag, box); ok && held > 0 {
			return box, true, nil
		}
	}
	return 0, false, nil
}

// radiantDeviceWindowState answers both shop and bag-side state requests.
func radiantDeviceWindowState(service *loot.Service, role storage.Character) ([]byte, error) {
	box, held, e := radiantBoxInBag(service, role)
	if e != nil {
		return nil, e
	}
	if !held {
		return protocol.CeraShopDeviceState(0, 0)
	}
	bonus, section, e := service.BoxWindowCounters(role.State, box)
	if e != nil {
		return nil, e
	}
	return protocol.CeraShopDeviceState(bonus, section)
}

// openRadiantBox answers the radiant treasure box window's EVENT_REQUEST. It
// spends the material, rolls every open, and reports the result through
// NOTI2551.
func (w *worldSession) openRadiantBox(ctx context.Context, box, count uint32) ([]outboundPacket, error) {
	if w == nil || w.loot == nil {
		return nil, fmt.Errorf("box open before the loot service is ready")
	}
	before, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	saved, receipt, _, e := w.loot.OpenBoxes(ctx, w.role, box, count)
	if e != nil {
		return nil, e
	}
	table, ok := w.loot.Boxes.Table(box)
	if !ok {
		return nil, fmt.Errorf("box %d has no imported content table", box)
	}
	entries := loot.BoxNoticeEntries(table, receipt.Points, receipt.Granted)
	list := make([]protocol.RadiantBoxEntry, 0, len(entries))
	for _, row := range entries {
		list = append(list, protocol.RadiantBoxEntry{Key: row.Key, Value: row.Value})
	}
	if uint32(len(receipt.Results)) != count {
		return nil, fmt.Errorf("box %d receipt has %d main results for %d opens", box, len(receipt.Results), count)
	}
	state := table.BoxStateRows(receipt.Points)
	state = append(state, loot.BoxMainResultRows(receipt.Results)...)
	state = append(state, loot.BoxBonusResultRows(receipt.Bonus)...)
	rows := make([]protocol.RadiantBoxRow, 0, len(state))
	for _, row := range state {
		rows = append(rows, protocol.RadiantBoxRow{
			Key: row.Key, Flag: row.Flag,
			Template: row.Template, Count: row.Count, Threshold: row.Threshold,
		})
	}
	notice, e := protocol.RadiantBoxNotice(list, rows)
	if e != nil {
		return nil, e
	}
	bag, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	// 增量包不会删除未列出的槽位；比较开罐前后状态，显式清除耗尽的罐子。
	// 若奖励复用了原槽位，差异行会直接同步新物品，避免误删奖励。
	update, e := protocol.InventoryUpdate(inventory.ChangedItemRows(before, bag))
	if e != nil {
		return nil, e
	}
	w.role = saved
	plan := make([]outboundPacket, 0, 2+len(receipt.Premiums))
	// 兑换遗留契约时可能删除多个槽位，使用完整还原清除客户端残留图标。
	if len(receipt.Premiums) > 0 {
		restore, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
		if err != nil {
			return nil, err
		}
		plan = append(plan,
			outboundPacket{"radiant_box_inventory_restored", 0, 13, restore},
			outboundPacket{"radiant_box_notice", 0, 2551, notice},
		)
	} else {
		plan = append(plan,
			outboundPacket{"radiant_box_inventory_updated", 0, 14, update},
			outboundPacket{"radiant_box_notice", 0, 2551, notice},
		)
	}
	for _, premium := range receipt.Premiums {
		remaining := premium.EndTime - time.Now().Unix()
		if remaining <= 0 {
			continue
		}
		payload, err := protocol.PremiumActivationNotice(premium.Type, remaining)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"radiant_box_contract_noti", 0, 66, payload})
	}
	return plan, nil
}
