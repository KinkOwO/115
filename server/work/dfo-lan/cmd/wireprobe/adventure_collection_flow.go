package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"encoding/json"
	"fmt"
)

// 登记引导装备后才允许完成21651，不能以直接完成任务代替实际登记。
// 原版 adventurecollection.cos 的装备组0只含100261068，且没有奖励。
func (w *worldSession) registerAdventureCollection(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	r, err := protocol.DecodeAdventureCollection(p)
	if err != nil {
		return nil, err
	}
	if w.characters == nil || w.store == nil || w.quests == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("图鉴登记缺少当前账号角色或任务目录")
	}
	if r.Category != 1 {
		return nil, fmt.Errorf("该图鉴分类的登记与奖励尚未接入，未消耗物品")
	}
	template, ok := quest.AdventureCollectionObjective(w.quests.Catalog.Quests[21651])
	if !ok {
		return nil, fmt.Errorf("冒险图鉴引导规则与当前目录不一致")
	}
	done, err := w.quests.Completed(ctx, w.role)
	if err != nil {
		return nil, err
	}
	ready := false
	for _, id := range done {
		if id == 21650 {
			ready = true
		}
	}
	if !ready {
		return nil, fmt.Errorf("尚未完成冒险图鉴前置引导任务")
	}
	if _, err = w.prepareAdventure(ctx); err != nil {
		return nil, err
	}
	key := fmt.Sprintf("adventure-collection-guide:%s:%x", prefix, sha256.Sum256(raw))
	saved, profile, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, key,
		func(role database.Character, profile *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			receipt, e := json.Marshal(map[string]any{"category": r.Category, "slot": r.Slot, "template": template})
			if e != nil {
				return nil, nil, e
			}
			// 原帧重放由事务回执恢复；新的重复登记不能再次扣物，
			// 也不能把其它槽位的登记请求谎报成功。
			if profile.Data.CollectionEquipment[template] {
				return nil, nil, fmt.Errorf("引导装备已经登记，无需重复消耗")
			}
			bag, e := inventory.ReadBag(role.State)
			if e != nil {
				return nil, nil, e
			}
			found := -1
			for i, item := range bag.Equipment {
				if uint32(item.Slot) == r.Slot {
					if item.Template != template || item.Group != 0 || item.Period != 0 && item.Period != protocol.MaxItemPeriod {
						return nil, nil, fmt.Errorf("请选择引导奖励的永久装备；其它收藏组尚未接入，未消耗物品")
					}
					found = i
					break
				}
			}
			if found < 0 {
				return nil, nil, fmt.Errorf("所选背包槽位没有可登记的引导装备")
			}
			bag.Equipment = append(bag.Equipment[:found], bag.Equipment[found+1:]...)
			state, e := inventory.SaveBag(role.State, bag)
			if e != nil {
				return nil, nil, e
			}
			if profile.Data.CollectionEquipment == nil {
				profile.Data.CollectionEquipment = make(map[uint32]bool)
			}
			profile.Data.CollectionEquipment[template] = true
			return state, receipt, nil
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	items, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	active, err := w.quests.Active(ctx, saved)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{
		{"图鉴登记背包同步", 0, 13, items},
		{"图鉴装备登记同步", 0, 2425, protocol.AdventureCollectionGuide(profile.Data.CollectionEquipment)},
	}
	// 只更新真实已接的引导任务。NOTI291会重建全部任务并触发自动寻路，
	// 在图鉴登记过程中可能因此弹出无关的“暂无可用频道”。
	for _, q := range active {
		if q.ID != 21651 {
			continue
		}
		progress, err := protocol.QuestTriggerUpdated(q)
		if err != nil {
			return nil, err
		}
		packets = append(packets, outboundPacket{"图鉴登记任务进度", 1, 33, progress})
		break
	}
	return append(packets, outboundPacket{"图鉴引导登记完成", 1, 2139, protocol.AdventureCollectionResponse(true)}), nil
}
