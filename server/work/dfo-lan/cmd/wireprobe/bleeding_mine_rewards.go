package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// 官方奖励袋为账号共有，每周二09:00 UTC刷新；未领取的旧奖励不能清空。
// 规则：https://www.dfoneople.com/news/updates/4603/Bleeding-Mines/Rewards
type bleedingMineRewardState struct {
	Version       int                      `json:"version"`
	Source        string                   `json:"source"`
	Week          string                   `json:"week"`
	Claimed       bool                     `json:"claimed"`
	Selected      [8]uint32                `json:"selected"`
	Cards         []bleedingMineRewardCard `json:"cards"`
	Stages        [3][4]bool               `json:"stages"`
	Completed     [3]bool                  `json:"completed"`
	Members       [3][4]int64              `json:"members"`
	CombineEarned uint32                   `json:"combine_earned"`
	CombineUsed   uint32                   `json:"combine_used"`
	Compositions  map[string]bool          `json:"compositions,omitempty"`
}

type bleedingMineRewardCard struct {
	Template uint32               `json:"template"`
	Group    uint32               `json:"group"`
	Assets   []database.MailAsset `json:"assets"`
}

var errBleedingMineClaimActor = fmt.Errorf("只有参与本周矿区探索的角色可以领取奖励")

func (s *bleedingMineRewardState) hasCards() bool {
	for _, card := range s.Cards {
		if card.Template != 0 {
			return true
		}
	}
	return false
}

func (s *bleedingMineRewardState) addCard(card bleedingMineRewardCard) error {
	for i, old := range s.Cards {
		if old.Template == 0 {
			s.Cards[i] = card
			return nil
		}
	}
	if len(s.Cards) >= 100 {
		return fmt.Errorf("矿区奖励袋容量不足，保留当前阶段待结算")
	}
	s.Cards = append(s.Cards, card)
	return nil
}

func (w *worldSession) updateBleedingMineRewards(ctx context.Context, apply func(*bleedingMineRewardState) ([]database.MailAsset, error)) (*bleedingMineRewardState, error) {
	if w == nil || w.characters == nil || w.store == nil || w.loot == nil || w.loot.BleedingMine == nil || w.role.ConfigVersion != savecontract.Identity() {
		return nil, fmt.Errorf("赤红铁矿原版奖励配置或存储未加载")
	}
	var result bleedingMineRewardState
	_, _, err := w.store.UpdateBleedingMineRewards(ctx, w.account, w.role.ID, w.role.ConfigVersion,
		func(role database.Character, raw json.RawMessage) (json.RawMessage, json.RawMessage, []database.MailAsset, error) {
			if err := json.Unmarshal(raw, &result); err != nil {
				return nil, nil, nil, err
			}
			if result.Version != 0 && result.Version != 1 {
				return nil, nil, nil, fmt.Errorf("矿区奖励存档版本不受支持")
			}
			week := adventure.SeasonWeek(time.Now())
			if result.Version == 0 || result.Week != week && (result.Claimed || !result.hasCards()) {
				result = bleedingMineRewardState{Version: 1, Source: w.role.ConfigVersion, Week: week}
			}
			if result.Source != w.role.ConfigVersion {
				return nil, nil, nil, fmt.Errorf("矿区奖励存档来源已改变，保留旧奖励等待迁移")
			}
			if len(result.Cards) > 100 || result.CombineUsed > result.CombineEarned || result.CombineEarned > 5 {
				return nil, nil, nil, fmt.Errorf("矿区奖励存档超出源边界")
			}
			for _, card := range result.Cards {
				if card.Template != 0 && (card.Group >= 3 || len(card.Assets) != 1 || !json.Valid(card.Assets[0].Item) || w.loot.BleedingMine.Items[card.Template].ID != card.Template) {
					return nil, nil, nil, fmt.Errorf("矿区奖励卡存档无效")
				}
			}
			assets, err := apply(&result)
			if err != nil {
				return nil, nil, nil, err
			}
			encoded, err := json.Marshal(result)
			return role.State, encoded, assets, err
		})
	return &result, err
}

func (w *worldSession) validateBleedingMineRewards(ctx context.Context, group uint32, members [4]int64) error {
	_, err := w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
		if group >= 3 || s.Claimed || s.Completed[group] {
			return nil, fmt.Errorf("该矿区本周已完成，或账号已领取本周奖励")
		}
		if s.Week != adventure.SeasonWeek(time.Now()) {
			return nil, fmt.Errorf("请先领取上一周保留的矿区奖励")
		}
		for _, cleared := range s.Stages[group] {
			if cleared && s.Members[group] != members {
				return nil, fmt.Errorf("本次矿区已有阶段奖励，请先放弃原编队的探索再更换角色")
			}
		}
		for g, ids := range s.Members {
			if !s.Completed[g] {
				continue
			}
			for _, id := range members {
				for _, used := range ids {
					if id != 0 && id == used {
						return nil, fmt.Errorf("角色已绑定本周其他矿区")
					}
				}
			}
		}
		return nil, nil
	})
	return err
}

func (w *worldSession) bleedingMineCard(id, group uint32) (bleedingMineRewardCard, error) {
	card := bleedingMineRewardCard{Template: id, Group: group}
	awards, err := w.loot.BleedingMine.ResolveSelection(id)
	if err != nil {
		return card, err
	}
	for _, a := range awards {
		var item inventory.MailItem
		meta := w.loot.BleedingMine.Items[a.Template]
		if meta.Kind == "stackable" {
			if _, ok := w.loot.Catalog.Items[a.Template]; !ok {
				return card, fmt.Errorf("矿区奖励%d不在运行物品目录中", a.Template)
			}
			item.Stack = &inventory.BagItem{Template: a.Template, Amount: a.Amount, ExpireTime: inventory.GrantExpireTime}
		} else {
			if a.Amount != 1 || w.loot.Equipment == nil {
				return card, fmt.Errorf("矿区装备奖励数量或目录无效")
			}
			kind, err := w.loot.Equipment.EquipmentKind(a.Template)
			if err != nil {
				return card, err
			}
			if inventory.EquipmentBagSpace(kind) != 0 {
				return card, fmt.Errorf("矿区奖励%d必须保留其专用礼盒", a.Template)
			}
			durability, err := w.loot.Equipment.Reward(a.Template)
			if err != nil {
				return card, err
			}
			item.Equipment = &inventory.BagEquipment{Template: a.Template, Durability: durability, Period: inventory.GrantExpireTime}
		}
		if _, err = item.Row(); err != nil {
			return card, err
		}
		raw, err := json.Marshal(item)
		if err != nil {
			return card, err
		}
		card.Assets = append(card.Assets, database.MailAsset{Item: raw})
	}
	if len(card.Assets) != 1 {
		return card, fmt.Errorf("矿区奖励卡%d未展开为单个有效附件", id)
	}
	return card, nil
}

// 只有服务端确认当前领主死亡之后才生成本阶段奖励，并与去重回执同时落档。
func (w *worldSession) freezeBleedingMineStage(ctx context.Context) error {
	mine := w.bleedingMineStart
	if mine == nil || w.activeDungeon == nil || !w.activeDungeon.Completed() || mine.Stage >= 4 {
		return fmt.Errorf("矿区阶段尚未通关")
	}
	_, err := w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
		if s.Claimed || s.Completed[mine.Group] || s.Week != mine.Week {
			return nil, fmt.Errorf("矿区本次探索的奖励周期已结束")
		}
		if s.Stages[mine.Group][mine.Stage] {
			return nil, nil
		}
		awards, err := w.loot.BleedingMine.Draw(w.loot.BleedingMine.StageBoxes[mine.Group*4+mine.Stage])
		if err != nil {
			return nil, err
		}
		for _, a := range awards {
			if a.Amount > 100 {
				return nil, fmt.Errorf("矿区奖励袋容量不足，保留当前阶段待结算")
			}
			for n := uint32(0); n < a.Amount; n++ {
				card, err := w.bleedingMineCard(a.Template, mine.Group)
				if err != nil {
					return nil, err
				}
				if err = s.addCard(card); err != nil {
					return nil, err
				}
			}
		}
		s.Stages[mine.Group][mine.Stage] = true
		s.Members[mine.Group] = mine.Members
		return nil, nil
	})
	return err
}

func bleedingMineRewardPacket(s *bleedingMineRewardState) (outboundPacket, error) {
	ids := make([]uint32, len(s.Cards))
	for i, c := range s.Cards {
		ids[i] = c.Template
	}
	p, err := protocol.BleedingMineRewardInfo(s.Selected, ids)
	return outboundPacket{"赤红铁矿奖励袋同步", 0, 2708, p}, err
}

func (w *worldSession) openBleedingMineRewards(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if len(p) != 0 || w.channelType != 106 || w.activeDungeon != nil {
		return nil, fmt.Errorf("请在矿区城镇打开奖励袋")
	}
	s, err := w.updateBleedingMineRewards(ctx, func(*bleedingMineRewardState) ([]database.MailAsset, error) { return nil, nil })
	if err != nil {
		return nil, err
	}
	packet, err := bleedingMineRewardPacket(s)
	if err != nil {
		return nil, err
	}
	var packets []outboundPacket
	if w.bleedingMineRoster == nil {
		body, ids, err := w.characters.AllServerRoster(ctx, w.account, w.fatigue, time.Now())
		if err != nil {
			return nil, err
		}
		count := len(ids)
		packets = append(packets, outboundPacket{"赤红铁矿领奖角色数量", 0, 1396, []byte{1, w.characters.ChannelContext[0], byte(count), byte(count >> 8)}}, outboundPacket{"赤红铁矿领奖角色资料", 0, 2, body})
		w.bleedingMineRoster = ids
	}
	if w.bleedingMineRoster != nil {
		profile, err := w.bleedingMineProfile(ctx, w.bleedingMineRoster)
		if err != nil {
			return nil, err
		}
		// 名单必须先到达，再恢复编队索引；奖励袋最后刷新。
		packets = append(packets, profile)
	}
	return append(packets, packet, outboundPacket{"赤红铁矿奖励袋已打开", 1, 2323, []byte{1}}), nil
}

func (w *worldSession) claimBleedingMineRewards(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if w.channelType != 106 || w.activeDungeon != nil || w.bleedingMineStart != nil {
		return nil, fmt.Errorf("请结束探索并返回矿区领取奖励")
	}
	slots, err := protocol.DecodeBleedingMineRewardSelection(p)
	if err != nil {
		return nil, err
	}
	if len(slots) == 0 {
		return nil, fmt.Errorf("至少选择一份矿区奖励")
	}
	s, err := w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
		if s.Claimed {
			return nil, nil
		}
		participant := false
		for _, members := range s.Members {
			for _, id := range members {
				participant = participant || id == w.role.ID
			}
		}
		if !participant {
			return nil, errBleedingMineClaimActor
		}
		var assets []database.MailAsset
		for i, slot := range slots {
			if int(slot) >= len(s.Cards) || s.Cards[slot].Template == 0 {
				return nil, fmt.Errorf("奖励袋格位已变更，请重新打开")
			}
			card := s.Cards[slot]
			// 已落档的通关奖励在掉线后仍可领取；不依赖已销毁的连接会话。
			s.Selected[i] = card.Template
			assets = append(assets, card.Assets...)
		}
		s.Claimed = true
		s.Cards = nil
		return assets, nil
	})
	if err != nil {
		return nil, err
	}
	packet, err := bleedingMineRewardPacket(s)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{packet, {"赤红铁矿奖励已投递邮件", 1, 2322, []byte{1}}}, nil
}

func (w *worldSession) composeBleedingMineRewards(ctx context.Context, p, frame []byte) ([]outboundPacket, error) {
	if w.channelType != 106 || w.activeDungeon != nil || w.bleedingMineStart != nil || w.bleedingMineRoster == nil {
		return nil, fmt.Errorf("请结束探索后合成奖励")
	}
	slots, err := protocol.DecodeBleedingMineCompose(p)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("%x", sha256.Sum256(frame))
	s, err := w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
		if s.Compositions[key] {
			return nil, nil
		}
		if s.Claimed || s.CombineUsed >= s.CombineEarned || s.CombineUsed >= 5 {
			return nil, fmt.Errorf("矿区本周合成次数不足")
		}
		for _, slot := range slots {
			if int(slot) >= len(s.Cards) || s.Cards[slot].Template == 0 || !s.Completed[s.Cards[slot].Group] {
				return nil, fmt.Errorf("矿区合成材料已变更或尚未结算")
			}
		}
		id, err := w.loot.BleedingMine.Compose(s.Cards[slots[0]].Template, s.Cards[slots[1]].Template)
		if err != nil {
			return nil, err
		}
		card, err := w.bleedingMineCard(id, s.Cards[slots[0]].Group)
		if err != nil {
			return nil, err
		}
		s.Cards[slots[0]] = card
		s.Cards[slots[1]] = bleedingMineRewardCard{}
		s.CombineUsed++
		if s.Compositions == nil {
			s.Compositions = map[string]bool{}
		}
		s.Compositions[key] = true
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	packet, err := bleedingMineRewardPacket(s)
	if err != nil {
		return nil, err
	}
	// reader将状态2作为本次合成结果的高亮标记；源材料的空槽仍保留索引。
	packet.Payload[32+int(slots[0])*5+4] = 2
	profile, err := w.bleedingMineProfile(ctx, w.bleedingMineRoster)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{profile, packet, {"赤红铁矿奖励合成完成", 1, 2325, []byte{1}}}, nil
}

func bleedingMineRewardProfile(p []byte, s *bleedingMineRewardState) {
	for g, done := range s.Completed {
		if done {
			binary.LittleEndian.PutUint32(p[g*60+4:], 1)
		}
	}
	binary.LittleEndian.PutUint32(p[185:], s.CombineEarned-s.CombineUsed)
	binary.LittleEndian.PutUint32(p[189:], s.CombineUsed)
	// 193保持当前客户端默认开关，不能把未闭合的该字段误当成领取标记。
}
