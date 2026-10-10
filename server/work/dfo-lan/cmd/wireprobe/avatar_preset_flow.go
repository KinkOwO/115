package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// avatarPresetTicketUsePackets 构造装扮预设扩展券使用后的回包。
//
// 与幻化栏券（走 USERINFO1 解锁字节）不同，预设栏位数**只**由 noti1585
// AVATAR_PRESET_LIST 携带（没有独立的解锁位），所以落地就是重发一帧新的列表。
// 顺带把用掉的那一格用绝对行回给客户端，否则客户端会留着最后一张券的图标
// （与疲劳药水/幻化栏券同一范式）。
func avatarPresetTicketUsePackets(state json.RawMessage, slot uint16, applied bool) ([]outboundPacket, error) {
	bag, err := inventory.ReadBag(state)
	if err != nil {
		return nil, err
	}
	pages := protocol.DefaultAvatarPresetPages() + bag.AvatarPresetPages
	if pages > protocol.MaxAvatarPresetPages {
		pages = protocol.MaxAvatarPresetPages
	}
	list, err := protocol.AvatarPresetList(pages)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"avatar_preset_list", 0, 1585, list}}
	if applied {
		row := protocol.EmptyOrdinaryItem(slot)
		for _, item := range bag.Items {
			if item.Slot == slot {
				row = protocol.OrdinaryItem(slot, item.Template, item.Amount)
			}
		}
		update, err := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"avatar_preset_ticket_spent", 0, 14, update})
	}
	return packets, nil
}

// avatarPresetTicketSlot 判断这条 CMD507 是不是在点名一个装着装扮预设扩展券的槽位。
//
// 客户端对同一个 CMD507 复用多种动作（54 药水 / 73 时装栏 / 101+197 幻化栏 /
// 169 皮肤仓 / 206 飞艇 / 337 直升胶囊…），而装扮预设券的动作号在 PVF 里查不到，
// 所以这一路**按槽位内容识别**，不参与动作号白名单。
func (w *worldSession) avatarPresetTicketSlot(p []byte) (uint16, bool) {
	if w == nil || w.role.ID == 0 || len(p) < 2 {
		return 0, false
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 {
		return 0, false
	}
	bag, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return 0, false
	}
	if !inventory.HoldsAvatarPresetTicket(bag, slot) {
		return 0, false
	}
	return slot, true
}

// useAvatarPresetTicket 处理「右键使用一张 Avatar Preset Expansion Ticket」。
//
// 语义：把页签（栏位）数 +1 —— 默认 1 页，用一张券 +1 页；并在同一个
// 事务里消耗掉那张券。事务 key 沿用本项目口径 `<feature>:<会话前缀>:<帧哈希>`，
// 所以同一条请求重放只结算一次，而连续用第二张券（帧不同）是新事件。
func (w *worldSession) useAvatarPresetTicket(ctx context.Context, request, raw []byte, prefix string, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.role.ID <= 0 || w.role.AccountID <= 0 || w.store == nil || w.loot == nil || prefix == "" || len(raw) < 13 {
		return nil, fmt.Errorf("avatar preset expansion requires a loaded character")
	}
	if len(request) < 2 {
		return nil, fmt.Errorf("avatar preset expansion request too short")
	}
	slot := binary.LittleEndian.Uint16(request)
	key := fmt.Sprintf("avatar-preset-expand:%s:%x", prefix, sha256.Sum256(raw))
	saved, applied, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "avatar-preset-expand-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, err
			}
			index := -1
			for i, item := range bag.Items {
				if item.Slot == slot && item.Amount > 0 {
					index = i
					break
				}
			}
			if index < 0 {
				return nil, nil, fmt.Errorf("slot %d is empty", slot)
			}
			if bag.Items[index].Template != inventory.AvatarPresetTicketTemplate {
				return nil, nil, fmt.Errorf("slot %d does not hold an avatar preset expansion ticket", slot)
			}
			if protocol.StoredItemExpired(bag.Items[index].ExpireTime, time.Now().Unix()) {
				return nil, nil, fmt.Errorf("avatar preset expansion ticket expired")
			}
			if bag.AvatarPresetPages >= protocol.MaxAvatarPresetPages-1 {
				return nil, nil, fmt.Errorf("avatar preset fully expanded")
			}
			template := bag.Items[index].Template
			next, _, err := bag.Consume(w.loot.Catalog, slot, template)
			if err != nil {
				return nil, nil, err
			}
			next.AvatarPresetPages = bag.AvatarPresetPages + 1
			state, err := inventory.SaveBag(current.State, next)
			if err != nil {
				return nil, nil, err
			}
			if _, err := avatarPresetTicketUsePackets(state, slot, true); err != nil {
				return nil, nil, err
			}
			receipt, err := json.Marshal(map[string]any{"template": template, "slot": slot, "pages": next.AvatarPresetPages + 1})
			return state, receipt, err
		})
	if err != nil {
		return nil, err
	}
	packets, err := avatarPresetTicketUsePackets(saved.State, slot, applied)
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	if event != nil {
		pages := uint16(0)
		if bag, bagErr := inventory.ReadBag(saved.State); bagErr == nil {
			pages = uint16(protocol.DefaultAvatarPresetPages() + bag.AvatarPresetPages)
		}
		event(map[string]any{"kind": "avatar_preset_expansion_saved", "character_id": saved.ID, "slot": slot, "pages": uint16(pages), "applied": applied})
	}
	return packets, nil
}
