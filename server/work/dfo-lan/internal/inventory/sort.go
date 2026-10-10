package inventory

import (
	"dfolan/internal/game/protocol"
	"fmt"
	"sort"
)

// Modelled reports whether a slot belongs to a range this server persists.
// The client's own slot space is wider (its sort table spans 380 slots), so
// anything outside these ranges is carried by the client alone and must not be
// touched by a server side rearrangement.
func (r BagRules) Modelled(slot uint16) bool {
	if r.Quick(slot) {
		return true
	}
	if r.EquipmentSlots != [2]uint16{} && slot >= r.EquipmentSlots[0] && slot <= r.EquipmentSlots[1] {
		return true
	}
	for _, v := range r.Slots {
		if v != [2]uint16{} && slot >= v[0] && slot <= v[1] {
			return true
		}
	}
	return false
}

// SortItems adopts the arrangement the client already applied locally (CMD20).
//
// The table maps each old slot to its new one: an item sitting in slot s ends up
// in slot perm[s]. The mapping is a bijection, so no two items can be sent to the
// same slot.
//
// The direction is what the player sees. With the captured request that moved
// 12..18 and 29, applying the table this way collapses a bag holding
// 9,10,11,13..18,29 into the hole-free 9..18; the opposite direction leaves 13
// empty and keeps a lone item out at 29, which is exactly the "one item jumped
// somewhere else" the player reported. Both directions agree on a self-inverse
// table, which is why the first capture (a four cycle) could not tell them apart.
//
// List 0 rearranges both the ordinary items and the equipment bag: the client's
// table addresses one slot space that covers the equipment range (9..64), the
// throw range (65..120) and the material range (121..176) alike.
// [FIX-20261007 特殊容器整理] List 1 = 时装栏（special_equipment[1]），
// List 7 = 宠物栏（special_equipment[7]）：客户端整理这些栏位时同样以 CMD20
// 提交本地排列，此前只支持 List 0，导致时装/徽章相关栏位整理永远落空
// （客户端排好后重进恢复原样）。
func SortItems(b Bag, rules BagRules, r protocol.SortItemRequest) (Bag, error) {
	switch r.List {
	case 0:
		return sortOrdinaryItems(b, rules, r)
	case 1, 7:
		return sortSpecialSpace(b, r)
	default:
		return b, fmt.Errorf("sort item list is not the ordinary inventory")
	}
}

func sortOrdinaryItems(b Bag, rules BagRules, r protocol.SortItemRequest) (Bag, error) {
	if len(r.Slots) == 0 {
		return b, fmt.Errorf("empty sort item table")
	}
	seen := make([]bool, len(r.Slots))
	for _, v := range r.Slots {
		if int(v) >= len(r.Slots) || seen[v] {
			return b, fmt.Errorf("sort item table is not a permutation")
		}
		seen[v] = true
	}
	for idx := range b.Items {
		if e := sortSlot(&b.Items[idx].Slot, r.Slots, rules); e != nil {
			return b, e
		}
	}
	for idx := range b.Equipment {
		if e := sortSlot(&b.Equipment[idx].Slot, r.Slots, rules); e != nil {
			return b, e
		}
	}
	// [FIX-20261007 徽章采纳排列] 徽章区（289..360）不在 BagRules 的分区里，
	// 上面的 sortSlot 因 Modelled() 为假会把整段跳过，于是客户端已经排好的徽章
	// 排列被静默丢弃（客户端整理当场生效、重进恢复乱序）。这里按同一条 CMD20
	// 排列表把徽章区一并落位；随后的 arrangeEmblems 只做去空洞的幂等兜底。
	if e := sortEmblems(&b, r.Slots); e != nil {
		return b, e
	}
	// [FIX-20261007 全局压缩] 客户端分区排序会在各区段间留下空洞（例如
	// 消耗品区 65-107 与材料区 121-153 之间 108-120 全空），服务端应用排列后
	// 把普通物品（消耗品区 65-120 + 材料区 121-176）合并压缩、从 65 起连续
	// 排列，消除空洞；快捷栏（0-8）、装备区（9-64）保持原位，徽章区
	// （289-344）由 arrangeEmblems 单独压缩。
	if e := compactOrdinaryItems(&b, rules); e != nil {
		return b, e
	}
	if e := arrangeEmblems(&b); e != nil {
		return b, e
	}
	return b, nil
}

// compactOrdinaryItems compacts the ordinary bag per segment, following the
// 90US sortSegment model: each of the BagRules' throw/material ranges is
// compacted from its own first slot and stable-sorted by template ID within the
// segment. Segments are NOT merged (an earlier merge pushed material items into
// the consumable range, and the client renders those ranges as separate bag
// tabs, so profession materials ended up displayed in the consumable tab).
// Quick slots, the equipment range and the emblem zone are left untouched
// (emblems are compacted separately by arrangeEmblems).
func compactOrdinaryItems(b *Bag, rules BagRules) error {
	const zoneStart, zoneEnd = 289, 360
	type cand struct {
		item *BagItem
		slot uint16
	}
	occupied := map[uint16]bool{}
	for idx := range b.Items {
		if s := b.Items[idx].Slot; s >= zoneStart && s <= zoneEnd {
			occupied[s] = true
		}
	}
	// [FIX-20261010 重复段去重] 配置里 [throw] 与 [etc] 可能指向同一槽位段
	// （如均 [65,120]）。若同一段被压缩两次，第二次的 cands 已是紧凑后的物品，
	// occupied 从段首起全满，next 会越过整段物品找到段内靠后的空位，把刚压好的
	// 段整体推后（实测：18 件从 65-82 被推回 83-100，重进恢复"跳位"）。这里按
	// 段范围去重，同一 [start,end] 只压缩一次。
	seenSeg := map[uint32]bool{}
	for _, seg := range rules.Slots {
		if seg == [2]uint16{} {
			continue
		}
		segKey := uint32(seg[0])<<16 | uint32(seg[1])
		if seenSeg[segKey] {
			continue
		}
		seenSeg[segKey] = true
		var cands []cand
		for idx := range b.Items {
			item := &b.Items[idx]
			s := item.Slot
			if s < seg[0] || s > seg[1] {
				continue
			}
			cands = append(cands, cand{item: item, slot: s})
		}
		sort.SliceStable(cands, func(i, j int) bool {
			if cands[i].item.Template != cands[j].item.Template {
				return cands[i].item.Template < cands[j].item.Template
			}
			return cands[i].slot < cands[j].slot
		})
		next := seg[0]
		for _, c := range cands {
			for occupied[next] && next <= seg[1] {
				next++
			}
			if next > seg[1] {
				break // segment full; keep the rest where they are
			}
			c.item.Slot = next
			occupied[next] = true
			next++
		}
	}
	return nil
}

// EmblemZoneFirst/Last 是徽章（avatar emblem）在普通物品空间里的槽位区间，
// 与 stack_request.go 的 `[avatar emblem]` home 一致。
const (
	EmblemZoneFirst uint16 = 289
	EmblemZoneLast  uint16 = 360
)

// sortEmblems adopts the client's CMD20 arrangement for the emblem zone
// (289..360). Those slots are absent from BagRules, so sortSlot skips them and
// the player's emblem ordering used to be thrown away. Direction matches
// sortSlot: the item in slot s ends up in perm[s]. A move that would leave the
// zone is refused rather than silently kept, since the table is a bijection and
// a mismatched one would corrupt the container.
func sortEmblems(b *Bag, perm []uint16) error {
	for idx := range b.Items {
		old := int(b.Items[idx].Slot)
		if old < int(EmblemZoneFirst) || old > int(EmblemZoneLast) || old >= len(perm) {
			continue
		}
		next := int(perm[old])
		if next == old {
			continue
		}
		if next < int(EmblemZoneFirst) || next > int(EmblemZoneLast) {
			return fmt.Errorf("emblem sort would leave the emblem zone")
		}
		b.Items[idx].Slot = uint16(next)
	}
	return nil
}

// arrangeEmblems compacts the emblem zone (289..360) in place: items already
// living in the zone are re-slotted contiguously from 289, filling holes, and
// nothing outside the zone is ever moved (unlike an earlier all-2xxxxxxx
// variant, a rename ticket or any other emblem-like item in the ordinary bag is
// never pulled in). It runs AFTER sortEmblems and is idempotent: a
// hole-free zone keeps its arrangement.
//
// ⚠️ 不要把候选自己的槽预先标进 occupied —— 那会把自己的位置当成障碍跳过，
// 使"压缩"退化成整段平移。2026-10-07 实测：连续 289..320 会被推成 321..352，
// 下一次又弹回 289..320，玩家每点一次整理徽章区就整体跳 32 格。
func arrangeEmblems(b *Bag) error {
	const zoneStart, zoneEnd = EmblemZoneFirst, EmblemZoneLast
	var cands []*BagItem
	for idx := range b.Items {
		item := &b.Items[idx]
		if item.Slot < zoneStart || item.Slot > zoneEnd {
			continue
		}
		cands = append(cands, item)
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Slot < cands[j].Slot })
	for i, c := range cands {
		c.Slot = uint16(int(zoneStart) + i)
	}
	return nil
}

// sortSpecialSpace applies the client's CMD20 permutation to one special
// equipment container (space 1 = avatar wardrobe, space 7 = creatures). The
// table is a bijection over the client's slot space, so moving rows through it
// cannot collide; rows whose slot falls outside the table are refused rather
// than silently kept (a mismatched table would corrupt the container).
// [FIX-20261008 宠物消耗品整理] List 7 宠物栏的排列表同时覆盖宠物消耗品
// （pet_items，slot 376-431，即客户端 Creature 栏 Use 子栏）。此前只应用排列
// 到 Special[7]，pet_items 不动，客户端本地排好但服务端存档未变，重进恢复
// 原样（Use 子栏"没法整理"）。这里把同一排列应用到 pet_items，且目标 slot
// 限定在 376-431 区内，防止映射窜入 Special[7] 区导致下次校验存档失败。
func sortSpecialSpace(b Bag, r protocol.SortItemRequest) (Bag, error) {
	if len(r.Slots) == 0 {
		return b, fmt.Errorf("empty sort item table")
	}
	seen := make([]bool, len(r.Slots))
	for _, v := range r.Slots {
		if int(v) >= len(r.Slots) || seen[v] {
			return b, fmt.Errorf("sort item table is not a permutation")
		}
		seen[v] = true
	}
	rows := b.Special[r.List]
	for idx := range rows {
		old := int(rows[idx].Slot)
		if old >= len(r.Slots) {
			return b, fmt.Errorf("special sort slot out of table: %d", old)
		}
		if next := int(r.Slots[old]); next != old {
			rows[idx].Slot = uint16(next)
		}
	}
	if r.List == 7 {
		const petFirst, petLast = 376, 431
		for idx := range b.PetItems {
			old := int(b.PetItems[idx].Slot)
			if old >= len(r.Slots) {
				continue
			}
			if next := int(r.Slots[old]); next != old && next >= petFirst && next <= petLast {
				b.PetItems[idx].Slot = uint16(next)
			}
		}
	}
	return b, nil
}

// sortSlot maps one occupied slot through the client's arrangement. Slots this
// server does not model are carried by the client alone, and nothing crosses the
// modelled boundary in either direction.
func sortSlot(slot *uint16, perm []uint16, rules BagRules) error {
	old := int(*slot)
	if old >= len(perm) || !rules.Modelled(uint16(old)) {
		return nil
	}
	next := int(perm[old])
	if next == old {
		return nil
	}
	if !rules.Modelled(uint16(next)) {
		return fmt.Errorf("sort item would leave the modelled slots")
	}
	*slot = uint16(next)
	return nil
}
