package loot

import (
	"crypto/sha256"
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// BoostAutoGrant 是自动开盒展开出来的一行发放：模板与件数。
type BoostAutoGrant struct {
	Template uint32 `json:"template"`
	Amount   uint32 `json:"amount"`
}

// 胶囊自动开盒的事务编排在 workflow.LootService.boostAutoStep；这里只做
// 纯展开与纯落账计算。

// BoostAutoGrants 把毕业一步发下来的外层包装**只开一层**。
//
// 展开复用掉落线同一个 `RewardBoxSource`（cmd 侧 boosterBoxSource 适配器，
// 数据源就是 cashshop.BoosterCatalog），不再自建一份礼包目录视图：源里
// 「[booster] 开一层出什么」只有一处解释，事件线和掉落线必须给同一个答案。
//
// 只开一层不是偷懒，是源的意思：第九关的 590015951 开出来是一件装备，
// 第十一关的 590015965 开出来是 590015966×1000、3037×1000 和
// 590015964×1 —— 最后那个是 `[booster selection]` 选择箱，玩家自己点开挑，
// 官方把它**作为未开封的箱子**放进奖励里。递归展开会替玩家做选择，
// 发下去的东西就不是那份奖励了。
//
// 每一池必须只有一个候选（直接型）。事件奖励的内容是确定的，若这里出现
// 带权重的池，说明这份包装其实要玩家自己掷，服务端替它掷就等于把定值奖励
// 当成抽奖券；宁可拒绝也不改变奖励本身。
func (s *Service) BoostAutoGrants(role Role, step byte, boxes []boostup.Reward) ([]BoostAutoGrant, error) {
	if s.RewardBoxes == nil {
		return nil, fmt.Errorf("boost auto-box source missing")
	}
	seed := sha256.Sum256([]byte(fmt.Sprintf("boost-auto:%d:%d:%d", role.AccountID, role.ID, step)))
	rng := RNG{binary.LittleEndian.Uint32(seed[:4])}
	var grants []BoostAutoGrant
	for _, box := range boxes {
		if box.Count == 0 || box.Count > 64 {
			return nil, fmt.Errorf("invalid boost outer-box count")
		}
		def, ok := s.RewardBoxes.RewardBox(box.Item)
		if !ok || len(def.Pools) == 0 {
			return nil, fmt.Errorf("boost auto-box %d has no direct reward pool", box.Item)
		}
		for _, p := range def.Pools {
			if len(p.Candidates) != 1 {
				return nil, fmt.Errorf("boost auto-box %d pool is not a direct reward", box.Item)
			}
		}
		for i := uint32(0); i < box.Count; i++ {
			for _, p := range def.Pools {
				for d := uint32(0); d < p.draws(); d++ {
					c, ok := p.pick(&rng)
					if !ok {
						continue
					}
					amount := c.Count
					if amount == 0 {
						amount = 1
					}
					// 目录里没有这个模板 = 源留的空槽，不能凭空发明成物品；
					// 目录认得的包装（含选择箱）原样发下去，由玩家自己开。
					if !s.RewardBoxes.Item(c.Template) {
						return nil, fmt.Errorf("boost auto-box %d pays unknown template %d", box.Item, c.Template)
					}
					grants = append(grants, BoostAutoGrant{Template: c.Template, Amount: amount})
				}
			}
		}
	}
	if len(grants) == 0 || len(grants) > 256 {
		return nil, fmt.Errorf("invalid boost auto rewards")
	}
	return grants, nil
}

// PrepareBoostAutoStep 在锁定的角色状态与账户材料快照上完成一步自动开盒的
// 纯计算：训练推进、展开、共享材料落地与背包发放，并回写三处状态。
//
// 背包发放复用 inventory.Awarder —— 服务端发放物品的那一条路（任务奖励、
// GM 发放、副本结算都走它）：可堆叠物按堆叠上限分槽并打永不过期哨兵，装备
// 走 EquipmentCatalog.Reward 取源 [durability]，宠物装另走 AddPetGear。
// 自动开盒没有理由自己再写一遍这套规则。
//
// 背包放不下时整步回滚，不发邮寄：与 PrepareBoostGift 同一条约定（没有
// 「未核实的邮寄兜底」）。领奖标记与发放同事务，回滚后玩家清包再点一次
// 即可，不会既丢了这一步又拿不到东西。
//
// 提交与幂等在 workflow.LootService.boostAutoStep（§7.2 E13）。
func (s *Service) PrepareBoostAutoStep(current Role, c *boostup.Catalog, step byte, counts json.RawMessage) (json.RawMessage, json.RawMessage, BoostStepReceipt, error) {
	st, e := boostup.ReadState(current.State)
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	if !st.Activated || st.Training.Claimed[step] {
		return nil, nil, BoostStepReceipt{}, fmt.Errorf("invalid auto reward state")
	}
	var boxes []boostup.Reward
	st.Training, boxes, e = c.Claim(st.Training, step, st.Variant == 1)
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	grants, e := s.BoostAutoGrants(current, step, boxes)
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	materials, e := inventory.ReadAccountMaterials(counts)
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	state := current.State
	out := BoostStepReceipt{Step: step, Claim: true, AutoOpened: true, Rewards: boxes}
	for _, g := range grants {
		if slot, shared := inventory.AccountMaterialSlot(g.Template); shared {
			if g.Amount == 0 {
				return nil, nil, BoostStepReceipt{}, fmt.Errorf("boost auto reward has no amount")
			}
			materials, _, e = materials.Add(g.Template, g.Amount)
			if e != nil {
				return nil, nil, BoostStepReceipt{}, e
			}
			out.Shared = append(out.Shared, g.Template)
			out.Granted = append(out.Granted, g)
			out.Slots = append(out.Slots, slot)
			continue
		}
		var r inventory.AwardReceipt
		state, r, e = awarder.Grant(state, g.Template, g.Amount)
		if e != nil {
			return nil, nil, BoostStepReceipt{}, e
		}
		out.Granted = append(out.Granted, g)
		out.Slots = append(out.Slots, r.Slots...)
	}
	state, e = boostup.WriteState(state, st)
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	materialState, e := materials.Save()
	if e != nil {
		return nil, nil, BoostStepReceipt{}, e
	}
	return state, materialState, out, nil
}
