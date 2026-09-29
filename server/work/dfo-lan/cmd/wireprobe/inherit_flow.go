package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// CMD 1722 = 装备继承（itemCarving / Inherit）确认。
//
// ★ 2026-09-28：这条分派以前**整条缺失**。客户端按下确认后直发 1722，
// 服务端既不改状态也不回包，客户端就一直停在等待态 —— 玩家看到的就是
// 「按下继承毫无效果」。与 CHANGELOG 里 CMD205（增幅书）那次是同一类缺陷。
//
// 流程：解码 → 事务落库（支持多条记录）→ **只发 kind=0 的 NOTI14 行刷新**
// （更新两件装备的数值）。**不发任何 1722 出站包** —— CMD 1722 在客户端双向都没有
// 继承结果的接收通道：kind=1 是客户端自己发的命令（没有接收 handler），
// kind=0 那一侧的 handler 是小游戏道具计数通知，都与继承结果无关。
// 取证见 internal/game/protocol/inherit.go 末尾的注释块。
func (w *worldSession) inherit(service *inventory.WearService, p, raw []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 || w.activeDungeon != nil {
		return nil, fmt.Errorf("装备继承需要已选角色且位于城镇")
	}
	r, err := protocol.DecodeInheritRequest(p)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("inherit:%x", sha256.Sum256(raw))
	saved, receipts, err := service.ApplyInherit(ctx, w.role, key, r)
	if err != nil {
		return nil, err
	}
	w.role = saved
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}

	plan := []outboundPacket{}

	// ★ 刷新只走 id14 增量行，**不发 id13 全量重建**：继承窗口确认后仍持有两件
	// 装备的对象引用，整包重建会打断这些引用 —— 强化 / 增幅路径早有同一条结论
	// （reinforcement_flow.go、amplify_flow.go 的注释）。每条记录的 base + material
	// 槽位都要发：基础件带新等级 / 新附魔，材料件已清零。
	seen := map[uint32]bool{}
	for _, out := range receipts {
		for _, loc := range []struct {
			space byte
			slot  uint16
		}{{out.BaseSpace, out.BaseSlot}, {out.MaterialSpace, out.MaterialSlot}} {
			// 同一容器同一槽位只发一次；轮换继承时一件装备可能被多条记录碰过，去重后只发最终态。
			key := uint32(loc.space)<<16 | uint32(loc.slot)
			if seen[key] {
				continue
			}
			items := bag.Equipment
			if loc.space == 3 {
				items = bag.WornBaseItems()
			}
			var rows []inventory.BagEquipment
			for _, it := range items {
				if it.Slot == loc.slot {
					rows = append(rows, it)
					break
				}
			}
			if len(rows) == 0 {
				continue
			}
			body, e := inventory.EquipmentPayload(loc.space, rows, false)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"inherit_slots_updated", 0, 14, body})
			seen[key] = true
		}
	}
	// 穿戴窗口整窗刷新一次：穿在身上的装备等级/附魔变了要同步到角色面板。
	if wornBody, e := inventory.WornSpaceUpdate(saved.State); e == nil && len(wornBody) > 0 {
		plan = append(plan, outboundPacket{"inherit_worn_window_refreshed", 0, 14, wornBody})
	}

	for _, out := range receipts {
		event(map[string]any{"kind": "inherit_committed", "character_id": saved.ID,
			"base_slot": out.BaseSlot, "base_space": out.BaseSpace, "base_template": out.BaseTemplate,
			"material_slot": out.MaterialSlot, "material_space": out.MaterialSpace, "material_template": out.MaterialTemplate,
			"before_level": out.BeforeLevel, "after_level": out.AfterLevel, "material_level": out.MaterialLevel,
			"amplify_type": out.AmplifyType, "amplify_value": out.AmplifyValue,
			"refine": out.Refine, "enchant_card": out.EnchantCard})
	}
	return plan, nil
}
