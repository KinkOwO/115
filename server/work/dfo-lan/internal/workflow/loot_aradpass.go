package workflow

import (
	"context"
	"dfolan/internal/aradpass"
	"dfolan/internal/database"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
)

// aradPassModel 是战令事件在角色事件账本里的模型标记。
// 同一事件键换模型会报 `character event model mismatch`，所以改动语义时**换模型串**。
const aradPassModel = "arad-pass-v1"

// aradPassActivateKey 是"激活皇家通行证"的幂等键（每个角色每赛季一次）。
const aradPassActivateKey = "arad-pass-activate-v1"

// ClaimAradPassReward 领取战令的一档奖励。
//
// 两层分工（§7.2 E13）：纯状态迁移在 `loot.PrepareAradPassReward`，这里负责
// 事务编排 + 幂等回读。奖励**直接落背包/装备栏**（不走系统邮件）—— 战令奖励是
// 玩家用经验换的即时物，与签到/在线时长的"邮件附件"口径不同，源实现也是直接落袋。
//
// 幂等键 `arad-pass-reward:<轨道>:<槽>`：同键重发走收据、不重复发货，也不重跑校验。
// 背包满 → 事务回滚，玩家清理后可再领。
// ClaimAradPassRoyalDaily 领「Royal Pass 每日登录礼包」（CMD680 的 `516, 3`）。
//
// 幂等键 `arad-pass-royal-daily:<游戏日>` ⇒ 同一天重复点只会读回原回执、不再发货；
// 换日（09:00 UTC）后键变了，自然又能领一次。
func (s *LootService) ClaimAradPassRoyalDaily(ctx context.Context, role database.Character, c *aradpass.Catalog,
	day string) (database.Character, loot.AradPassRoyalDailyReceipt, bool, error) {
	var out loot.AradPassRoyalDailyReceipt
	if s == nil || s.Loot == nil || s.Store == nil || c == nil || day == "" {
		return role, out, false, fmt.Errorf("arad pass royal daily storage unavailable")
	}
	key := "arad-pass-royal-daily:" + day
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, aradPassModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			return s.Loot.PrepareAradPassRoyalDaily(LootRole(current), c, day)
		})
	if e != nil {
		return role, out, false, e
	}
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return role, out, false, e
	}
	if out.Day != day || len(out.Items) == 0 {
		return role, out, false, fmt.Errorf("arad pass royal daily receipt mismatch")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

func (s *LootService) ClaimAradPassReward(ctx context.Context, role database.Character, c *aradpass.Catalog,
	track, level int) (database.Character, loot.AradPassRewardReceipt, bool, error) {
	var out loot.AradPassRewardReceipt
	if s == nil || s.Loot == nil || s.Store == nil || c == nil {
		return role, out, false, fmt.Errorf("arad pass reward storage unavailable")
	}
	// `level` = 客户端给的**等级**；事件键仍按 `<轨道>:<槽>`（源表序号）定，
	// 这样既有存档里的键不用变（两轨内槽与等级一一对应）。
	row, ok := c.RewardAtLevel(track, level)
	if !ok {
		return role, out, false, fmt.Errorf("arad pass: no reward for track=%d level=%d", track, level)
	}
	slot := row.Slot
	key := fmt.Sprintf("arad-pass-reward:%d:%d", track, slot)
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), key, aradPassModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, receipt, err := s.Loot.PrepareAradPassReward(LootRole(current), c, track, level)
			if err != nil {
				return nil, nil, err
			}
			b, err := json.Marshal(receipt)
			return raw, b, err
		})
	if e != nil {
		return role, out, false, e
	}
	// 收据表是**唯一**的回执来源：重放路径（applied=false）根本不会执行上面的闭包，
	// 闭包里捕获的 receipt 在那条路上是零值。
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return role, out, false, e
	}
	if out.Track != track || out.Slot != slot || out.Item == 0 || len(out.Slots) == 0 {
		return role, out, false, fmt.Errorf("arad pass reward receipt mismatch")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// ActivateAradPass 激活皇家通行证：置 Royal + 加购票赠送经验 + 发源里的首发奖励
// （`[royalpass first reward]`），**同一笔事务**。
//
// 幂等键 `arad-pass-activate-v1`：重复调用走收据、不重复发货。
// 背包满 → 整笔回滚（不消费激活资格，玩家清理后可重试）。
func (s *LootService) ActivateAradPass(ctx context.Context, role database.Character, c *aradpass.Catalog) (database.Character, loot.AradPassActivationReceipt, bool, error) {
	var out loot.AradPassActivationReceipt
	if s == nil || s.Loot == nil || s.Store == nil || c == nil {
		return role, out, false, fmt.Errorf("arad pass activation storage unavailable")
	}
	saved, applied, e := s.Store.CommitCharacterEvent(ctx, role.AccountID, role.ID,
		s.Loot.Catalog.Source.SaveIdentity(), aradPassActivateKey, aradPassModel,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, receipt, err := s.Loot.PrepareAradPassActivation(LootRole(current), c)
			if err != nil {
				return nil, nil, err
			}
			b, err := json.Marshal(receipt)
			return raw, b, err
		})
	if e != nil {
		return role, out, false, e
	}
	b, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, aradPassActivateKey)
	if e != nil {
		return role, out, false, e
	}
	if e = json.Unmarshal(b, &out); e != nil {
		return role, out, false, e
	}
	// 只有"真的激活了"才算成功：Royal 为假说明状态与收据对不上（例如存档被清过），
	// 让调用方知道这次没有产生任何变更。
	if !out.Royal {
		return role, out, false, fmt.Errorf("arad pass activation produced no royal pass")
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}
