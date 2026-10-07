package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

type CloneAvatarSource struct {
	Slot     uint16 `json:"slot"`
	Template uint32 `json:"template"`
}

// CloneAvatarLook never invents a source object. The native manager looks in
// list1, so ownership in another container cannot satisfy this relationship.
func (b Bag) CloneAvatarLook(item BagEquipment) uint32 {
	if item.CloneSource == nil || item.Slot > 11 {
		return 0
	}
	for _, source := range b.Special[1] {
		if source.Slot == item.CloneSource.Slot && source.Template == item.CloneSource.Template {
			return source.Template
		}
	}
	return 0
}

func CloneAvatarSourcePayloads(state json.RawMessage) ([]byte, []byte, error) {
	b, err := ReadBag(state)
	if err != nil {
		return nil, nil, err
	}
	sources := map[byte]uint16{}
	for _, item := range b.Worn {
		if item.CloneSource != nil && b.CloneAvatarLook(item) != 0 {
			sources[byte(item.Slot)] = item.CloneSource.Slot
		}
	}
	return protocol.CloneAvatarSources(sources)
}

func (s *WearService) cloneSourceFits(clone, look BagEquipment) error {
	c, err := s.Catalog.Definition(clone.Template)
	if err != nil {
		return err
	}
	d, err := s.Catalog.Definition(look.Template)
	if err != nil {
		return err
	}
	ct, dt := c.Fields["[equipment type]"], d.Fields["[equipment type]"]
	if !c.IsCloneAvatar() || !d.IsAvatar() || d.IsCloneAvatar() || len(ct) == 0 || len(dt) == 0 || ct[0].Type != 6 || dt[0].Type != 6 || ct[0].Text != dt[0].Text {
		return fmt.Errorf("Clone source must be a same-part ordinary avatar")
	}
	return nil
}

// NormalizeCloneAvatars plans a lossless upgrade of the old double-Worn
// representation. It mutates only copies and returns an error before any save
// if capacity/definitions are insufficient. Callers commit it atomically.
func (s *WearService) NormalizeCloneAvatars(b Bag) (Bag, bool, error) {
	if s == nil || s.Catalog == nil {
		return b, false, fmt.Errorf("Clone catalog unavailable")
	}
	before := b
	b.Worn = append([]BagEquipment(nil), b.Worn...)
	rows := append([]BagEquipment(nil), b.Special[1]...)
	changed := false
	removed := map[int]bool{}
	occupied := map[uint16]bool{}
	for _, row := range rows {
		occupied[row.Slot] = true
	}
	for i, clone := range b.Worn {
		if clone.Slot > 11 || clone.Group != 0 {
			continue
		}
		definition, err := s.Catalog.Definition(clone.Template)
		if err != nil {
			return before, false, err
		}
		if !definition.IsCloneAvatar() {
			continue
		}
		for j, look := range b.Worn {
			if j == i || look.Slot != clone.Slot || look.Group != 1 {
				continue
			}
			if clone.CloneSource != nil {
				return before, false, fmt.Errorf("Clone has both native source and legacy worn look")
			}
			if err := s.cloneSourceFits(clone, look); err != nil {
				return before, false, err
			}
			var free uint16
			limit := protocol.AvatarInventorySlots(b.AvatarExpansion)
			for free < limit && occupied[free] {
				free++
			}
			if free == limit {
				return before, false, fmt.Errorf("Clone migration requires a free avatar inventory slot")
			}
			occupied[free] = true
			look.Slot, look.Group = free, 0
			look.CloneSource = nil
			rows = append(rows, look)
			b.Worn[i].CloneSource = &CloneAvatarSource{Slot: free, Template: look.Template}
			removed[j], changed = true, true
		}
	}
	if len(removed) > 0 {
		kept := make([]BagEquipment, 0, len(b.Worn)-len(removed))
		for i, row := range b.Worn {
			if !removed[i] {
				kept = append(kept, row)
			}
		}
		b.Worn = kept
		b.Special = copySpecialEquipment(b.Special)
		b.Special[1] = rows
	}
	for i, clone := range b.Worn {
		if clone.CloneSource == nil {
			continue
		}
		found := false
		for _, look := range rows {
			if look.Slot == clone.CloneSource.Slot && look.Template == clone.CloneSource.Template {
				if err := s.cloneSourceFits(clone, look); err != nil {
					return before, false, err
				}
				found = true
				break
			}
		}
		if !found {
			b.Worn[i].CloneSource = nil
			changed = true
		}
	}
	return b, changed, nil
}

func copySpecialEquipment(in map[byte][]BagEquipment) map[byte][]BagEquipment {
	out := make(map[byte][]BagEquipment, len(in)+1)
	for space, rows := range in {
		out[space] = append([]BagEquipment(nil), rows...)
	}
	return out
}

// CloneMoveAckMode identifies the writer's relation-only revocation request.
// 145ADF2B0 emits list1/source-slot/0 -> list3/worn-slot/copied-template,
// Flags[2]=1. 145283750 passes ACK mode1 to 145ACBC90 to clear the cover.
func CloneMoveAckMode(state json.RawMessage, r protocol.ItemMoveRequest) byte {
	b, err := ReadBag(state)
	if err != nil || r.SourceList != 1 || r.DestinationList != 3 || r.SourceItem != 0 || r.Flags[2] != 1 {
		return 0
	}
	for _, clone := range b.Worn {
		link := clone.CloneSource
		if clone.Slot == r.DestinationSlot && link != nil && link.Slot == r.SourceSlot && link.Template == r.DestinationItem && b.CloneAvatarLook(clone) != 0 {
			return 1
		}
	}
	return 0
}

func (s *WearService) moveNativeClone(role Role, b Bag, r protocol.ItemMoveRequest) (json.RawMessage, bool, error) {
	if r.SourceList != 1 && r.SourceList != 3 || r.DestinationList != 1 && r.DestinationList != 3 {
		return nil, false, nil
	}
	find := func(list byte, slot uint16) *BagEquipment {
		rows := b.Special[1]
		if list == 3 {
			rows = b.WornBaseItems()
		}
		for _, row := range rows {
			if row.Slot == slot {
				copy := row
				return &copy
			}
		}
		return nil
	}
	a, z := find(r.SourceList, r.SourceSlot), find(r.DestinationList, r.DestinationSlot)
	isClone := func(item *BagEquipment) bool {
		if item == nil {
			return false
		}
		d, err := s.Catalog.Definition(item.Template)
		return err == nil && d.IsCloneAvatar()
	}
	aClone, zClone := isClone(a), isClone(z)
	if (aClone || zClone) && r.Count != 1 {
		return nil, true, fmt.Errorf("Clone move requires count one")
	}
	if r.SourceList == 1 && r.DestinationList == 3 && zClone {
		link := z.CloneSource
		revoke := r.Flags[2] == 1 && r.SourceItem == 0 && link != nil && r.SourceSlot == link.Slot && r.DestinationItem == link.Template
		if revoke || a != nil && !aClone {
			if revoke {
				if a == nil || a.Template != link.Template {
					return nil, true, fmt.Errorf("stale Clone source identity")
				}
			} else if r.SourceItem != a.Template || r.DestinationItem != z.Template && r.DestinationItem != b.CloneAvatarLook(*z) {
				return nil, true, fmt.Errorf("stale Clone binding identity")
			}
			if err := s.cloneSourceFits(*z, *a); err != nil {
				return nil, true, err
			}
			var source *CloneAvatarSource
			if !revoke {
				if err := s.wearable(role, *a, z.Slot); err != nil {
					return nil, true, err
				}
				source = &CloneAvatarSource{Slot: a.Slot, Template: a.Template}
			}
			for i := range b.Worn {
				if b.Worn[i].Slot == z.Slot && b.Worn[i].Template == z.Template {
					b.Worn[i].CloneSource = source
				}
			}
			raw, err := SaveBag(role.State, b)
			return raw, true, err
		}
	}
	if !aClone && !zClone {
		return nil, false, nil
	}
	if r.SourceList == r.DestinationList && r.SourceList == 3 {
		return nil, true, fmt.Errorf("unsupported worn Clone swap")
	}
	if r.SourceItem != 0 && (a == nil || r.SourceItem != a.Template) || r.DestinationItem != 0 && (z == nil || r.DestinationItem != z.Template) {
		return nil, true, fmt.Errorf("stale Clone move identity")
	}
	for _, side := range []struct {
		item *BagEquipment
		list byte
		slot uint16
	}{{a, r.DestinationList, r.DestinationSlot}, {z, r.SourceList, r.SourceSlot}} {
		if side.item == nil {
			continue
		}
		if side.list == 3 {
			if err := s.wearable(role, *side.item, side.slot); err != nil {
				return nil, true, err
			}
		} else {
			if side.slot >= protocol.AvatarInventorySlots(b.AvatarExpansion) {
				return nil, true, fmt.Errorf("destination outside avatar inventory capacity")
			}
			d, err := s.Catalog.Definition(side.item.Template)
			if err != nil || !d.IsAvatar() {
				return nil, true, fmt.Errorf("Clone move requires avatar inventory")
			}
		}
	}
	b.Special = copySpecialEquipment(b.Special)
	// When a Clone replaces a worn look, the normal swap returns that entire
	// look instance to the Clone's former bag slot; that slot is its source.
	if r.SourceList == 1 && r.DestinationList == 3 && aClone && z != nil {
		if zClone {
			a.CloneSource = z.CloneSource
		} else {
			if err := s.cloneSourceFits(*a, *z); err != nil {
				return nil, true, err
			}
			a.CloneSource = &CloneAvatarSource{Slot: r.SourceSlot, Template: z.Template}
		}
	}
	set := func(list byte, slot uint16, item *BagEquipment) {
		rows := b.Worn
		if list == 1 {
			rows = b.Special[1]
		}
		kept := make([]BagEquipment, 0, len(rows)+1)
		for _, row := range rows {
			if row.Slot != slot {
				kept = append(kept, row)
			}
		}
		if item != nil {
			copy := *item
			copy.Slot = slot
			if list == 1 {
				copy.Group = 0
				copy.CloneSource = nil
			} else {
				copy.Group = s.itemGroup(&copy, 0)
			}
			kept = append(kept, copy)
		}
		if list == 1 {
			b.Special[1] = kept
		} else {
			b.Worn = kept
		}
	}
	set(r.SourceList, r.SourceSlot, z)
	set(r.DestinationList, r.DestinationSlot, a)
	b.RebaseCloneAvatarSources(r)
	raw, err := SaveBag(role.State, b)
	return raw, true, err
}

// The physical source is followed across a bag swap, never replaced by a
// different instance with the same template. Moving it out clears the link.
func (b *Bag) RebaseCloneAvatarSources(r protocol.ItemMoveRequest) {
	for i, item := range b.Worn {
		if item.CloneSource == nil {
			continue
		}
		link := *item.CloneSource
		if r.SourceList == 1 && link.Slot == r.SourceSlot && r.SourceItem == link.Template {
			if r.DestinationList != 1 {
				b.Worn[i].CloneSource = nil
				continue
			}
			link.Slot = r.DestinationSlot
		} else if r.DestinationList == 1 && link.Slot == r.DestinationSlot && r.DestinationItem == link.Template {
			if r.SourceList != 1 {
				b.Worn[i].CloneSource = nil
				continue
			}
			link.Slot = r.SourceSlot
		}
		b.Worn[i].CloneSource = &link
	}
}
