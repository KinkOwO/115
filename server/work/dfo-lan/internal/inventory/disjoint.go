package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
)

const ClearCubeFragmentID uint32 = 3037 // 无色小晶块模板ID

type DisjointReward struct {
	Slot     uint16
	Template uint32
	Count    uint32
}

type DisjointResult struct {
	DeletedSlots []uint16
	List         byte
	ToolSlot     uint16
	Rewards      []DisjointReward
}

func (b Bag) Disjoint(
	c catalog.LootCatalog,
	r BagRules,
	eq *EquipmentCatalog,
	requestedSlots []uint16,
	toolSlot uint16,
) (Bag, DisjointResult, error) {
	var zeroResult DisjointResult
	if len(requestedSlots) == 0 {
		return b, zeroResult, fmt.Errorf("empty disjoint slots")
	}
	if r.Source != c.Source.Checksum {
		return b, zeroResult, fmt.Errorf("invalid inventory source")
	}
	if r.EquipmentSlots[0] == 0 || r.EquipmentSlots[0] > r.EquipmentSlots[1] {
		return b, zeroResult, fmt.Errorf("unconfigured equipment slot range")
	}
	materialSlots, ok := r.Slots["[material]"]
	if !ok || materialSlots[0] == 0 || materialSlots[0] > materialSlots[1] {
		return b, zeroResult, fmt.Errorf("unmapped material slot range")
	}

	seenSlots := make(map[uint16]bool, len(requestedSlots))
	for _, reqSlot := range requestedSlots {
		if reqSlot < r.EquipmentSlots[0] || reqSlot > r.EquipmentSlots[1] {
			return b, zeroResult, fmt.Errorf("slot %d outside equipment bag", reqSlot)
		}
		if seenSlots[reqSlot] {
			return b, zeroResult, fmt.Errorf("duplicate disjoint slot %d", reqSlot)
		}
		seenSlots[reqSlot] = true
	}

	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	b.Items = append([]BagItem(nil), b.Items...)

	var deletedSlots []uint16
	accumulatedRewards := make(map[uint32]uint32)
	var rewardOrder []uint32

	// 从 b.Equipment 中逐槽移除命中项并计算产物
	for _, reqSlot := range requestedSlots {
		foundIndex := -1
		for i, eqItem := range b.Equipment {
			if eqItem.Slot == reqSlot {
				foundIndex = i
				break
			}
		}
		if foundIndex == -1 {
			return b, zeroResult, fmt.Errorf("equipment not found at slot %d", reqSlot)
		}

		eqItem := b.Equipment[foundIndex]
		var info DisjointEquipmentInfo
		// [ALIGN-20260930-OATH-DISJOINT] 誓约类分解**不产出通用材料**（官方规则）：
		//   · 光之誓约（誓约核心，`[equipment type] [oath]`）—— 分解后**自动登记装备库**，
		//     但**分解本身不产出材料**，且不可通过商店出售 / 丢弃 / 赠送删除；
		//   · 星蕴石（`[primer]`）—— 分解产出的是**对应套装的星蕴石碎片**（角色绑定，
		//     1000 个→自选史诗星蕴石 / 1500 个→自选太初星蕴石），属于**另一套产物**，
		//     不走这里的 rarity 材料表；
		//   · 从装备库生成的誓约装备同样无材料产出。
		// 所以这两类跳过 `CalculateDisjointRewards`。（碎片系统待实现。）
		oathLike := false
		if eq != nil {
			if def, err := eq.Definition(eqItem.Template); err == nil {
				info = ExtractDisjointEquipmentInfo(def)
				if info.Impossible {
					return b, zeroResult, fmt.Errorf("item at slot %d cannot be disassembled", reqSlot)
				}
				switch equipmentTypeKind(def) {
				case "[oath]", "[primer]":
					oathLike = true
				}
			} else {
				info = DisjointEquipmentInfo{
					Template:     eqItem.Template,
					Rarity:       0,
					MinimumLevel: 1,
					Value:        1000,
				}
			}
		} else {
			info = DisjointEquipmentInfo{
				Template:     eqItem.Template,
				Rarity:       0,
				MinimumLevel: 1,
				Value:        1000,
			}
		}

		if !oathLike {
			itemRewards := CalculateDisjointRewards(info, nil)
			for _, rw := range itemRewards {
				if rw.Count == 0 {
					continue
				}
				if accumulatedRewards[rw.Template] == 0 {
					rewardOrder = append(rewardOrder, rw.Template)
				}
				accumulatedRewards[rw.Template] += rw.Count
			}
		}

		deletedSlots = append(deletedSlots, reqSlot)
		b.Equipment = append(b.Equipment[:foundIndex], b.Equipment[foundIndex+1:]...)
	}

	occupied := map[uint16]bool{}
	for _, eqItem := range b.Equipment {
		occupied[eqItem.Slot] = true
	}
	for _, it := range b.Items {
		occupied[it.Slot] = true
	}

	var rewards []DisjointReward

	// 发放产物
	for _, template := range rewardOrder {
		remaining := accumulatedRewards[template]
		accountSpace, fixedSlot, isAccount := AccountMaterialTarget(template)

		// 落位与堆叠上限必须和 AddItem / addStackable 用**同一套**规则，否则分解产物
		// 会出现在"客户端认为不对"的那一栏。旧实现把非账号材料一律塞进材料栏，
		// 于是 [unlimited waste] 的奥德赛金币/银币（客户端钉在消耗品栏 65..120）
		// 落到了材料栏 121..176 —— 客户端的分解产物列表根本拿不到它。2026-09-23 修。
		target := materialSlots
		limit := stackLimitFor(r, "", 0)
		if item, ok := c.Items[template]; ok {
			target = stackableSlotRange(r, item.StackableType)
			limit = stackLimitFor(r, item.StackableType, item.StackLimit)
		}
		if isAccount {
			// 账号共享材料仍优先入材料栏槽位（随后 sweep 会迁入 list35）；
			// 只有材料栏满了才直接用它自己的固定存储槽。**此处行为与改动前一致。**
			target = materialSlots
		}

		// 1. 优先堆叠到背包已有堆
		for i := range b.Items {
			it := &b.Items[i]
			if it.Template == template {
				if it.Amount < limit {
					space := limit - it.Amount
					toAdd := remaining
					if toAdd > space {
						toAdd = space
					}
					it.Amount += toAdd
					remaining -= toAdd
					rewards = append(rewards, DisjointReward{
						Slot:     it.Slot,
						Template: template,
						Count:    toAdd,
					})
					if remaining == 0 {
						break
					}
				}
			}
		}

		// 2. 剩余部分新建堆
		if remaining > 0 {
			if isAccount {
				// 账号共享材料优先入材料栏槽位（后续 sweep 会迁入 list35）；若材料栏满则直接分配其固定存储槽
				var chosenSlot uint16
				for n := uint32(materialSlots[0]); n <= uint32(materialSlots[1]); n++ {
					slot := uint16(n)
					if !occupied[slot] {
						chosenSlot = slot
						occupied[slot] = true
						break
					}
				}
				if chosenSlot == 0 {
					if accountSpace != AccountMaterialSpace {
						return b, zeroResult, fmt.Errorf("material inventory is full")
					}
					chosenSlot = fixedSlot
					occupied[chosenSlot] = true
				}
				toAdd := remaining
				b.Items = append(b.Items, BagItem{
					Slot:     chosenSlot,
					Template: template,
					Amount:   toAdd,
				})
				remaining = 0
				rewards = append(rewards, DisjointReward{
					Slot:     chosenSlot,
					Template: template,
					Count:    toAdd,
				})
			} else {
				// 非账号材料按它自己的 [stackable type] 落栏：元素结晶/灵魂等是
				// [material]（仍落材料栏，与改动前一致），奥德赛货币是
				// [unlimited waste]（落消耗品栏 65..120）。
				for n := uint32(target[0]); n <= uint32(target[1]); n++ {
					slot := uint16(n)
					if !occupied[slot] {
						toAdd := remaining
						if toAdd > limit {
							toAdd = limit
						}
						b.Items = append(b.Items, BagItem{
							Slot:     slot,
							Template: template,
							Amount:   toAdd,
						})
						occupied[slot] = true
						remaining -= toAdd
						rewards = append(rewards, DisjointReward{
							Slot:     slot,
							Template: template,
							Count:    toAdd,
						})
						if remaining == 0 {
							break
						}
					}
				}
			}
		}

		if remaining > 0 {
			return b, zeroResult, fmt.Errorf("material inventory is full")
		}
	}

	res := DisjointResult{
		DeletedSlots: deletedSlots,
		List:         0,
		ToolSlot:     toolSlot,
		Rewards:      rewards,
	}
	return b, res, nil
}
