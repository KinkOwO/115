package inventory

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"sort"
)

// MoveVaultCross moves only the amount that fits at the client's chosen slot.
func MoveVaultCross(primary, secondary Vault, limit uint32, r protocol.ItemMoveRequest) (Vault, Vault, uint32, error) {
	if !((r.SourceList == 2 && r.DestinationList == 45) || (r.SourceList == 45 && r.DestinationList == 2)) {
		return primary, secondary, 0, fmt.Errorf("不是两个个人金库之间的移动")
	}
	if limit == 0 {
		limit = 1000
	}
	primary.Items = append([]VaultItem(nil), primary.Items...)
	secondary.Items = append([]VaultItem(nil), secondary.Items...)
	src, dst := &primary, &secondary
	if r.SourceList == 45 {
		src, dst = &secondary, &primary
	}
	if r.SourceSlot >= src.Slots || r.DestinationSlot >= dst.Slots {
		return primary, secondary, 0, fmt.Errorf("跨库槽位越界")
	}
	from := src.ItemAt(r.SourceSlot)
	if from == nil || from.Template != r.SourceItem {
		return primary, secondary, 0, fmt.Errorf("跨库源物品不存在或已经改变")
	}
	to := dst.ItemAt(r.DestinationSlot)
	if (to == nil && r.DestinationItem != 0) || (to != nil && to.Template != r.DestinationItem) {
		return primary, secondary, 0, fmt.Errorf("跨库目标物品已经改变")
	}
	if from.IsEquip {
		if to != nil {
			return primary, secondary, 0, fmt.Errorf("装备目标槽已占用")
		}
		item := *from
		item.Slot = r.DestinationSlot
		dst.Items = append(dst.Items, item)
		for i := range src.Items {
			if src.Items[i].Slot == r.SourceSlot {
				src.Items = append(src.Items[:i], src.Items[i+1:]...)
				break
			}
		}
		return primary, secondary, 1, nil
	}
	want := r.Count
	if want == 0 || want > from.Amount {
		want = from.Amount
	}
	remaining := want
	merge := func(item *VaultItem) {
		if item == nil || item.IsEquip || item.Template != from.Template || item.ExpireTime != from.ExpireTime || item.Amount >= limit {
			return
		}
		n := limit - item.Amount
		if n > remaining {
			n = remaining
		}
		item.Amount += n
		remaining -= n
	}
	merge(to)
	indices := make([]int, len(dst.Items))
	for i := range indices {
		indices[i] = i
	}
	sort.Slice(indices, func(i, j int) bool { return dst.Items[indices[i]].Slot < dst.Items[indices[j]].Slot })
	for _, i := range indices {
		if remaining == 0 {
			break
		}
		if dst.Items[i].Slot != r.DestinationSlot {
			merge(&dst.Items[i])
		}
	}
	if remaining > 0 && to == nil {
		n := remaining
		if n > limit {
			n = limit
		}
		dst.Items = append(dst.Items, VaultItem{Slot: r.DestinationSlot, Template: from.Template, Amount: n, ExpireTime: from.ExpireTime})
		remaining -= n
	}
	moved := want - remaining
	if moved == 0 {
		return primary, secondary, 0, fmt.Errorf("目标金库已满或堆叠已达上限")
	}
	from.Amount -= moved
	if from.Amount == 0 {
		for i := range src.Items {
			if src.Items[i].Slot == r.SourceSlot {
				src.Items = append(src.Items[:i], src.Items[i+1:]...)
				break
			}
		}
	}
	return primary, secondary, moved, nil
}
