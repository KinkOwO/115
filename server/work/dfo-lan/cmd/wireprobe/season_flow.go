package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type seasonRefusal struct {
	code   uint16
	reason string
}

func (e seasonRefusal) Error() string { return e.reason }

func (w *worldSession) refreshSeason(ctx context.Context) ([]outboundPacket, error) {
	if !w.adventureReady || w.characters == nil || w.characters.Store == nil || w.role.ID == 0 {
		return nil, nil
	}
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return nil, err
	}
	// 账号资料是迷雾进度真源；角色JSON只作属性刷新时的快照。
	var raw json.RawMessage
	err = w.characters.Store.DB.QueryRow(ctx, `SELECT jsonb_build_object('season_level',COALESCE(a.data->'season_level','{}'::jsonb))
 FROM characters c LEFT JOIN account_adventures a ON a.account_id=c.account_id
 WHERE c.account_id=$1 AND c.id=$2 AND c.deleted_at IS NULL`, w.account, w.role.ID).Scan(&raw)
	if err != nil {
		return nil, err
	}
	state, err := adventure.ReadSeason(raw)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if state.Season == 0 || state.Week < adventure.SeasonWeek(now) {
		key := fmt.Sprintf("season-week:%d:%s", rules.Season, adventure.SeasonWeek(now))
		saved, _, e := w.characters.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "season-level-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			s, e := adventure.ReadSeason(current.State)
			if e != nil {
				return nil, nil, e
			}
			if e = rules.Normalize(&s, now); e != nil {
				return nil, nil, e
			}
			out, e := adventure.SaveSeason(current.State, s)
			return out, json.RawMessage(`{"weekly_reset":true}`), e
		})
		if e != nil {
			return nil, e
		}
		state, err = adventure.ReadSeason(saved.State)
		if err != nil {
			return nil, err
		}
	}
	if err = rules.Normalize(&state, now); err != nil {
		return nil, err
	}
	w.role.State, err = adventure.SaveSeason(w.role.State, state)
	if err != nil {
		return nil, err
	}
	var specialPackets []outboundPacket
	if _, eligible := rules.SpecialRewardAt(state, now); eligible {
		state, specialPackets, err = w.grantSeasonSpecial(ctx, state, now)
		if err != nil {
			return nil, err
		}
	}
	body := protocol.SeasonLevelHistory(state)
	signature := sha256.Sum256(body)
	oath, err := protocol.SeasonOathHistory(state.Acquisitions)
	if err != nil {
		return nil, err
	}
	oathSignature := sha256.Sum256(oath)
	packets := specialPackets
	if oathSignature != w.seasonOathSnapshot {
		packets = append(packets, outboundPacket{"誓约获取记录同步", 0, 2858, oath})
		w.seasonOathSnapshot = oathSignature
	}
	if signature != w.seasonLevelSnapshot {
		packets = append(packets, outboundPacket{"迷雾誓约进度同步", 0, 2799, body})
		w.seasonLevelSnapshot = signature
	}
	return packets, nil
}

// 源活动期内达到30阶的一次性道具；背包满时保留待发状态，不阻断通关或冒险团窗口。
func (w *worldSession) grantSeasonSpecial(ctx context.Context, previous adventure.SeasonState, now time.Time) (adventure.SeasonState, []outboundPacket, error) {
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return previous, nil, err
	}
	template, eligible := rules.SpecialRewardAt(previous, now)
	if !eligible || w.loot == nil {
		return previous, nil, nil
	}
	definition := rules.Items[template]
	items := catalog.LootCatalog{Source: w.loot.Catalog.Source, Items: map[uint32]catalog.LootItem{template: {ID: template, Kind: "stackable", StackableType: definition.Type, StackLimit: definition.Limit}}}
	key := fmt.Sprintf("season-special:%d:%d", rules.Season, template)
	saved, _, err := w.characters.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "season-level-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var role character.State
		if e := json.Unmarshal(current.State, &role); e != nil {
			return nil, nil, e
		}
		if role.Level < rules.MinimumLevel {
			return nil, nil, fmt.Errorf("角色未达到迷雾活动奖励等级")
		}
		if _, ok := rules.SpecialRewardAt(role.SeasonLevel, now); !ok {
			return current.State, json.RawMessage(`{"already_claimed":true}`), nil
		}
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		bag, e = bag.AddMailItem(items, w.loot.BagRules, nil, inventory.MailItem{Stack: &inventory.BagItem{Template: template, Amount: 1}})
		if e != nil {
			return nil, nil, e
		}
		next, e := inventory.SaveBag(current.State, bag)
		if e != nil {
			return nil, nil, e
		}
		role.SeasonLevel.SpecialClaimed = true
		next, e = adventure.SaveSeason(next, role.SeasonLevel)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"template": template, "count": 1})
		return next, receipt, e
	})
	if errors.Is(err, inventory.ErrMailBagFull) {
		return previous, nil, nil
	}
	if err != nil {
		return previous, nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	state, err := adventure.ReadSeason(saved.State)
	if err != nil {
		return previous, nil, err
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return previous, nil, err
	}
	body, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return previous, nil, err
	}
	return state, []outboundPacket{{"迷雾30阶活动奖励入包", 0, 13, body}}, nil
}

func (w *worldSession) claimSeasonReward(ctx context.Context, p []byte) ([]outboundPacket, error) {
	level, err := protocol.DecodeSeasonReward(p)
	if err != nil {
		return nil, err
	}
	if w.characters == nil || w.characters.Store == nil || w.loot == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("迷雾誓约领奖缺少角色或物品目录")
	}
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return nil, err
	}
	var reward adventure.SeasonReward
	for _, row := range rules.Rewards {
		if row.Level == level {
			reward = row
			break
		}
	}
	if reward.Template == 0 {
		return nil, seasonRefusal{19, "不存在此阶段的迷雾誓约奖励"}
	}
	definition := rules.Items[reward.Template]
	items := catalog.LootCatalog{Source: w.loot.Catalog.Source, Items: map[uint32]catalog.LootItem{
		reward.Template: {ID: reward.Template, Kind: "stackable", StackableType: definition.Type, StackLimit: definition.Limit},
	}}
	key := fmt.Sprintf("season-reward:%d:%d", rules.Season, level)
	saved, _, err := w.characters.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "season-level-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state character.State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		if e := rules.Normalize(&state.SeasonLevel, time.Now()); e != nil {
			return nil, nil, e
		}
		if state.Level < rules.MinimumLevel || rules.DisplayLevel(state.SeasonLevel) < reward.Level {
			return nil, nil, seasonRefusal{19, "未达到迷雾誓约领奖阶段"}
		}
		if state.SeasonLevel.RewardMask&reward.Mask != 0 {
			return nil, nil, seasonRefusal{19, "本阶段奖励已领取"}
		}
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		bag, e = bag.AddMailItem(items, w.loot.BagRules, nil, inventory.MailItem{Stack: &inventory.BagItem{Template: reward.Template, Amount: reward.Count}})
		if e != nil {
			return nil, nil, e
		}
		raw, e := inventory.SaveBag(current.State, bag)
		if e != nil {
			return nil, nil, e
		}
		state.SeasonLevel.RewardMask |= reward.Mask
		raw, e = adventure.SaveSeason(raw, state.SeasonLevel)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(reward)
		return raw, receipt, e
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
	body, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	if err != nil {
		return nil, err
	}
	state, err := adventure.ReadSeason(saved.State)
	if err != nil {
		return nil, err
	}
	history := protocol.SeasonLevelHistory(state)
	w.seasonLevelSnapshot = sha256.Sum256(history)
	// CMD2419成功回调0x14052E740不读取额外正文，只根据经理的已领取标记刷新按钮。
	return []outboundPacket{{"迷雾誓约奖励入包", 0, 13, body}, {"迷雾誓约领奖状态", 0, 2799, history}, {"迷雾誓约领奖完成", 1, 2419, []byte{1}}}, nil
}

func (w *worldSession) useSeasonCapsule(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	slot, err := protocol.DecodeSeasonCapsule(p)
	if err != nil {
		return nil, err
	}
	if w.characters == nil || w.characters.Store == nil || w.loot == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("迷雾经验道具缺少角色或背包")
	}
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("season-capsule:%s:%x", prefix, sha256.Sum256(raw))
	var before inventory.Bag
	saved, applied, err := w.characters.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "season-level-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		before = bag
		var template uint32
		now := time.Now()
		for _, row := range bag.Items {
			if row.Slot != slot {
				continue
			}
			if protocol.StoredItemExpired(row.ExpireTime, now.Unix()) {
				return nil, nil, fmt.Errorf("迷雾经验道具已过期")
			}
			template = row.Template
		}
		if _, ok := rules.Capsules[template]; !ok {
			return nil, nil, fmt.Errorf("该槽位不是源定义的迷雾经验道具")
		}
		state, gain, e := adventure.ApplySeasonCapsule(current.State, template, now)
		if e != nil {
			return nil, nil, e
		}
		bag, remaining, e := bag.Consume(w.loot.Catalog, slot, template)
		if e != nil {
			return nil, nil, e
		}
		state, e = inventory.SaveBag(state, bag)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(map[string]any{"template": template, "gain": gain, "remaining": remaining})
		return state, receipt, e
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
	var packets []outboundPacket
	// 原生ACK会自行消耗一件；重传只补权威槽位，避免客户端第二次扣除。
	if applied {
		packets = append(packets, outboundPacket{"迷雾经验道具使用完成", 1, 507, protocol.SeasonCapsuleReply(slot, true)})
	}
	rows := inventory.ChangedItemRows(before, bag)
	if !applied {
		row := protocol.EmptyOrdinaryItem(slot)
		for _, item := range bag.Items {
			if item.Slot == slot {
				row = item.Row()
			}
		}
		rows = [][protocol.CurrentItemRecordSize]byte{row}
	}
	update, err := protocol.InventoryUpdate(rows)
	if err != nil {
		return nil, err
	}
	state, err := adventure.ReadSeason(saved.State)
	if err != nil {
		return nil, err
	}
	history := protocol.SeasonLevelHistory(state)
	w.seasonLevelSnapshot = sha256.Sum256(history)
	packets = append(packets, outboundPacket{"迷雾经验道具库存同步", 0, 14, update}, outboundPacket{"迷雾经验更新", 0, 2799, history})
	return packets, nil
}

func (w *worldSession) acquireSeasonOath(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	choice, err := protocol.DecodeSeasonOath(p)
	if err != nil {
		return nil, err
	}
	if w.characters == nil || w.characters.Store == nil || w.loot == nil || w.loot.Equipment == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("誓约装备获取缺少角色或装备目录")
	}
	rules, err := adventure.CurrentSeason()
	if err != nil {
		return nil, err
	}
	if int(choice) >= len(rules.OathEquipment) {
		return nil, seasonRefusal{1, "誓约装备选择超出源列表"}
	}
	template := rules.OathEquipment[choice]
	key := fmt.Sprintf("season-oath:%s:%x", prefix, sha256.Sum256(raw))
	saved, _, err := w.characters.Store.CommitCharacterEvent(ctx, w.account, w.role.ID, w.role.ConfigVersion, key, "season-level-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var state character.State
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		now := time.Now()
		if e := rules.Normalize(&state.SeasonLevel, now); e != nil {
			return nil, nil, e
		}
		var allRewards uint32
		for _, reward := range rules.Rewards {
			allRewards |= reward.Mask
		}
		if state.Level < rules.MinimumLevel || state.SeasonLevel.RewardMask&allRewards != allRewards {
			return nil, nil, seasonRefusal{14, "必须达到115级并领取全部迷雾阶段奖励"}
		}
		count := uint32(len(state.SeasonLevel.Acquisitions))
		if count >= rules.MaxAcquisitions || rules.Level(state.SeasonLevel) <= rules.DisplayMaxLevel+count {
			return nil, nil, seasonRefusal{19, "未达到下一次誓约装备获取所需经验"}
		}
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, nil, e
		}
		if bag.Gold < rules.OathCost.Gold {
			return nil, nil, fmt.Errorf("誓约装备获取金币不足")
		}
		var materials []inventory.MaterialCost
		for _, cost := range rules.OathCost.Materials {
			materials = append(materials, inventory.MaterialCost{Template: cost.Template, Count: cost.Count})
		}
		bag, e = bag.PayMaterials(materials, 1)
		if e != nil {
			return nil, nil, e
		}
		bag.Gold -= rules.OathCost.Gold
		bag, _, e = bag.AddEquipment(w.loot.Equipment, w.loot.BagRules.EquipmentSlots, template, 1)
		if e != nil {
			return nil, nil, e
		}
		entry := adventure.OathAcquisition{Template: template, Time: now.Unix(), Name: current.Name}
		state.SeasonLevel.Acquisitions = append(state.SeasonLevel.Acquisitions, entry)
		// 先校验原生历史编码，防止入包成功后才发现记录不能同步。
		if _, e = protocol.SeasonOathHistory(state.SeasonLevel.Acquisitions); e != nil {
			return nil, nil, e
		}
		next, e := inventory.SaveBag(current.State, bag)
		if e != nil {
			return nil, nil, e
		}
		next, e = adventure.SaveSeason(next, state.SeasonLevel)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(entry)
		return next, receipt, e
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
	state, err := adventure.ReadSeason(saved.State)
	if err != nil {
		return nil, err
	}
	oath, err := protocol.SeasonOathHistory(state.Acquisitions)
	if err != nil {
		return nil, err
	}
	history := protocol.SeasonLevelHistory(state)
	w.seasonOathSnapshot = sha256.Sum256(oath)
	w.seasonLevelSnapshot = sha256.Sum256(history)
	return []outboundPacket{{"誓约装备及费用同步", 0, 13, items}, {"誓约获取记录同步", 0, 2858, oath}, {"迷雾誓约进度同步", 0, 2799, history}, {"誓约装备获取完成", 1, 2405, []byte{1}}}, nil
}
