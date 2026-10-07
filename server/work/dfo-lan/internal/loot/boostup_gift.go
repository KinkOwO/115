package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

type BoostGiftReceipt struct {
	Gift  uint16   `json:"gift"`
	Items []uint32 `json:"items"`
	Slots []uint16 `json:"slots"`
}

// PrepareBoostGift 在锁定行上计算「领奖 → 落背包 → 标记已领」的纯状态迁移。
// 提交/幂等回读在 workflow.LootService.ClaimBoostGift（§7.2 E13）。
//
// Entry handlers must first verify the operational event is open and resolve
// g from the loaded PVF. Reuse the established SQL state+receipt transaction.
// Full bags roll back the claim; there is no unverified mail fallback.
func (s *Service) PrepareBoostGift(current Role, g boostup.Gift) (json.RawMessage, BoostGiftReceipt, error) {
	var receipt BoostGiftReceipt
	state, e := boostup.ReadState(current.State)
	if e != nil {
		return nil, receipt, e
	}
	if state.Gifts[g.ID] {
		return nil, receipt, fmt.Errorf("gift already claimed without matching receipt")
	}
	if g.ID == 0 || len(g.Items) == 0 || !g.Direct || g.Trigger != "click button" {
		return nil, receipt, fmt.Errorf("unsupported gift delivery")
	}
	receipt.Gift = g.ID
	// 与训练领奖、自动开盒同一条发放路径（inventory.Awarder），期限哨兵由它统一打。
	awarder := &inventory.Awarder{Catalog: s.Catalog, Rules: s.BagRules, Equipment: s.Equipment}
	raw := current.State
	for _, id := range g.Items {
		var rec inventory.AwardReceipt
		raw, rec, e = awarder.Grant(raw, id, 1)
		if e != nil {
			return nil, BoostGiftReceipt{}, e
		}
		receipt.Items = append(receipt.Items, id)
		receipt.Slots = append(receipt.Slots, rec.Slots...)
	}
	state.Gifts[g.ID] = true
	raw, e = boostup.WriteState(raw, state)
	return raw, receipt, e
}
