package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
)

const ClearCubeFragmentID uint32 = 3037    // 无色小晶块模板ID
const DefaultDisjointCubeYield uint32 = 20 // 每件装备固定产出数量

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

	// 穿戴防拆保护：若请求槽位命中 b.Worn 中的任一穿戴装备，返回错误
	for _, reqSlot := range requestedSlots {
		for _, worn := range b.Worn {
			if reqSlot == worn.Slot {
				return b, zeroResult, fmt.Errorf("cannot disjoint worn equipment at slot %d", reqSlot)
			}
		}
	}

	b.Equipment = append([]BagEquipment(nil), b.Equipment...)
	b.Items = append([]BagItem(nil), b.Items...)

	var deletedSlots []uint16
	var totalCubes uint32

	// 从 b.Equipment 中逐槽移除命中项
	for _, reqSlot := range requestedSlots {
		foundIndex := -1
		for i, eq := range b.Equipment {
			if eq.Slot == reqSlot {
				foundIndex = i
				break
			}
		}
		if foundIndex == -1 {
			return b, zeroResult, fmt.Errorf("equipment not found at slot %d", reqSlot)
		}
		deletedSlots = append(deletedSlots, reqSlot)
		totalCubes += DefaultDisjointCubeYield
		b.Equipment = append(b.Equipment[:foundIndex], b.Equipment[foundIndex+1:]...)
	}

	// 晶块堆叠上限
	limit := r.MissingStackLimit
	if item, ok := c.Items[ClearCubeFragmentID]; ok && item.StackLimit != 0 {
		limit = item.StackLimit
	}
	if limit == 0 {
		limit = 1000
	}

	occupied := map[uint16]bool{}
	for _, eq := range b.Equipment {
		occupied[eq.Slot] = true
	}
	for _, it := range b.Items {
		occupied[it.Slot] = true
	}

	remaining := totalCubes
	var rewards []DisjointReward

	// 优先堆叠到材料槽范围内已存在的 3037 堆
	for i := range b.Items {
		it := &b.Items[i]
		if it.Template == ClearCubeFragmentID && it.Slot >= materialSlots[0] && it.Slot <= materialSlots[1] {
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
					Template: ClearCubeFragmentID,
					Count:    toAdd,
				})
				if remaining == 0 {
					break
				}
			}
		}
	}

	// 剩余部分找材料槽范围内第一个空槽新建 3037 堆
	if remaining > 0 {
		for n := uint32(materialSlots[0]); n <= uint32(materialSlots[1]); n++ {
			slot := uint16(n)
			if !occupied[slot] {
				toAdd := remaining
				if toAdd > limit {
					toAdd = limit
				}
				b.Items = append(b.Items, BagItem{
					Slot:     slot,
					Template: ClearCubeFragmentID,
					Amount:   toAdd,
				})
				occupied[slot] = true
				remaining -= toAdd
				rewards = append(rewards, DisjointReward{
					Slot:     slot,
					Template: ClearCubeFragmentID,
					Count:    toAdd,
				})
				if remaining == 0 {
					break
				}
			}
		}
	}

	if remaining > 0 {
		return b, zeroResult, fmt.Errorf("material inventory is full")
	}

	res := DisjointResult{
		DeletedSlots: deletedSlots,
		List:         0,
		ToolSlot:     toolSlot,
		Rewards:      rewards,
	}
	return b, res, nil
}
