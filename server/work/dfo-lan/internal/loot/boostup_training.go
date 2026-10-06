package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

type BoostStepReceipt struct {
	Granted    []BoostAutoGrant
	Shared     []uint32
	Step       byte
	Claim      bool
	AutoOpened bool
	Rewards    []boostup.Reward
	Slots      []uint16
}

// PrepareBoostStep 在锁定行上计算「看攻略推进 / 领奖落包」的纯状态迁移。
// 幂等提交与回执回读在 workflow.LootService.BoostStepRequest（§7.2 E13）。
func (s *Service) PrepareBoostStep(role Role, c *boostup.Catalog, step byte, claim bool) (json.RawMessage, BoostStepReceipt, error) {
	out := BoostStepReceipt{Step: step, Claim: claim}
	if c == nil || step == 0 || int(step) > len(c.Steps) {
		return nil, out, fmt.Errorf("invalid boost step")
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, out, e
	}
	if !st.Activated {
		return nil, out, fmt.Errorf("role is not activated")
	}
	raw := role.State
	if claim {
		if st.Training.Claimed[step] {
			return nil, out, fmt.Errorf("claimed boost step lacks durable reward receipt")
		}
		st.Training, out.Rewards, e = c.Claim(st.Training, step, st.Variant == 1)
		if e != nil {
			return nil, out, e
		}
		if len(out.Rewards) == 0 {
			return nil, out, fmt.Errorf("boost reward source empty")
		}
		// 发放走 inventory.Awarder —— 服务端发物品只有一条路（任务奖励、GM 发放、
		// 自动开盒同源）：可堆叠物按类型槽位段落位并打永不过期哨兵，装备走源
		// [durability]。自己调 Bag.Add 会漏掉期限那一格，客户端把活动奖励渲染成
		// 「剩余期限已过」（2026-09-27 的银增幅书就是这样）。
		awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
		for _, r := range out.Rewards {
			var rec inventory.AwardReceipt
			raw, rec, e = awarder.Grant(raw, r.Item, r.Count)
			if e != nil {
				return nil, out, e
			}
			out.Slots = append(out.Slots, rec.Slots...)
		}
	} else {
		st.Training, e = c.GuideViewed(st.Training, step)
		if e != nil {
			return nil, out, e
		}
	}
	raw, e = boostup.WriteState(raw, st)
	return raw, out, e
}
