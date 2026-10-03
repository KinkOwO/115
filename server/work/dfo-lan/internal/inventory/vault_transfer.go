package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
)

// TransferStacks follows the live 20260916T130209 vault CMD19: Source is the
// dragged item, Destination is its target. It supports split/merge, not implicit
// swaps with other templates or gear. Both saved states are copied first.
func (s *VaultService) TransferStacks(role Role, v VaultState, r protocol.ItemMoveRequest) (json.RawMessage, json.RawMessage, error) {
	fail := func(reason string) (json.RawMessage, json.RawMessage, error) {
		return nil, nil, fmt.Errorf("%s", reason)
	}
	if s.Catalog.Source.SaveIdentity() != role.ConfigVersion || v.ConfigVersion != s.Rules.SourceSHA256 || !s.Rules.allows(v.Slots) {
		return fail("vault transfer source/capacity mismatch")
	}
	if !((r.DestinationList == 2 && (r.SourceList == 0 || r.SourceList == 2)) || (r.DestinationList == 0 && r.SourceList == 2)) || r.Extra != 0 || r.Selection != 0xffffffff || r.Flags != [3]byte{} {
		return fail("unsupported vault transfer request")
	}
	if r.DestinationList == r.SourceList && r.DestinationSlot == r.SourceSlot {
		return fail("identical vault locations")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, nil, e
	}
	vault, e := ReadVaultBagItems(v)
	if e != nil {
		return nil, nil, e
	}
	b.Items = append([]BagItem{}, b.Items...)
	rows := func(space byte) *[]BagItem {
		if space == 2 {
			return &vault
		}
		return &b.Items
	}
	find := func(space byte, slot uint16) *BagItem {
		for _, x := range *rows(space) {
			if x.Slot == slot {
				v := x
				return &v
			}
		}
		return nil
	}
	from := find(r.SourceList, r.SourceSlot)
	to := find(r.DestinationList, r.DestinationSlot)
	// Live 131224: a zero-count warehouse reposition names the empty slot
	// first and the occupied slot second. Validate both identities before
	// normalizing this symmetric operation; positive-count transfers retain
	// their observed source-to-destination direction.
	if r.Count == 0 && r.SourceList == 2 && r.DestinationList == 2 && from == nil && to != nil && r.SourceItem == 0 && r.DestinationItem == to.Template {
		r.SourceSlot, r.DestinationSlot = r.DestinationSlot, r.SourceSlot
		r.SourceItem, r.DestinationItem = r.DestinationItem, r.SourceItem
		from, to = to, nil
	}
	if from == nil || r.SourceItem != from.Template || r.Count > from.Amount {
		return fail("stale vault source or quantity")
	}
	if (to == nil && r.DestinationItem != 0) || (to != nil && (r.DestinationItem != to.Template || to.Template != from.Template)) {
		return fail("stale or incompatible vault target")
	}
	if to != nil && to.ExpireTime != from.ExpireTime {
		return fail("不同期限的金库物品不能合并")
	}
	// Live 130633/130704 uses count zero for a whole-stack warehouse merge.
	// Do not generalize that sentinel to unobserved cross-container requests.
	if r.Count == 0 {
		if r.SourceList != 2 || r.DestinationList != 2 {
			return fail("zero count requires a warehouse move")
		}
		r.Count = from.Amount
	}
	item, ok := s.Catalog.Items[from.Template]
	if !ok || item.Kind != "stackable" {
		if from != nil {
			item = catalog.LootItem{ID: from.Template, Kind: "stackable", StackableType: "[waste]", StackLimit: s.BagRules.MissingStackLimit}
			ok = true
		} else {
			return fail("vault transfer requires a known stackable")
		}
	}
	// These source fields require metadata the ordinary BagItem does not hold.
	contents := false
	for _, t := range item.Script.Cells {
		if t.Type == 3 {
			contents = t.Text == "[impossible contents]"
			switch t.Text {
			case "[expiration date]", "[period]", "[package data]", "[selection]", "[booster info]", "[creature]", "[cannot store]", "[cannot put in cargo]":
				return fail("item requires special storage policy")
			}
		} else if contents && (t.Type != 6 || t.Text != "gift") {
			return fail("unimplemented storage restriction")
		}
	}
	home, ok := s.BagRules.Slots[item.StackableType]
	if !ok {
		switch item.StackableType {
		case "[etc]", "[waste]", "[hp]", "[mp]", "[hp mp]", "[expert town potion]":
			home = [2]uint16{65, 120}
		default:
			switch item.StackableType {
			case "[material]", "[upgrade limit cube]":
				home = [2]uint16{121, 176}
			case "[quest]", "[quest receive]":
				home = [2]uint16{177, 232}
			case "[material expert job]":
				home = [2]uint16{233, 288}
			case "[avatar emblem]", "[rune]":
				home = [2]uint16{289, 344}
			default:
				home = [2]uint16{65, 120}
			}
		}
	}
	legal := func(space byte, slot uint16) bool {
		if space == 2 {
			return slot < v.Slots
		}
		// Slot zero is not accepted by the current persistent bag model.
		if slot == 0 || !((item.StackableType != "[material]" && s.BagRules.Quick(slot)) || (slot >= home[0] && slot <= home[1])) {
			return false
		}
		for _, gear := range b.Equipment {
			if gear.Slot == slot {
				return false
			}
		}
		return true
	}
	if !legal(r.DestinationList, r.DestinationSlot) || !legal(r.SourceList, r.SourceSlot) {
		return fail("vault or bag slot outside usable capacity")
	}
	limit := item.StackLimit
	if limit == 0 {
		limit = s.BagRules.MissingStackLimit
	}
	targetAmount := uint64(r.Count)
	if to != nil {
		targetAmount += uint64(to.Amount)
	}
	if limit == 0 || from.Amount > limit || targetAmount > uint64(limit) {
		return fail("vault transfer exceeds stack limit")
	}
	replace := func(space byte, slot uint16, template, amount uint32) {
		p := rows(space)
		kept := make([]BagItem, 0, len(*p)+1)
		for _, x := range *p {
			if x.Slot != slot {
				kept = append(kept, x)
			}
		}
		if amount > 0 {
			kept = append(kept, BagItem{Slot: slot, Template: template, Amount: amount, ExpireTime: from.ExpireTime})
		}
		*p = kept
	}
	replace(r.SourceList, r.SourceSlot, from.Template, from.Amount-r.Count)
	if r.SourceList == 0 && r.DestinationList == 2 && to == nil {
		// A deposit naming an empty slot first fills compatible warehouse
		// stacks. Keep the requested slot for any remainder; never move gear.
		sort.Slice(vault, func(i, j int) bool { return vault[i].Slot < vault[j].Slot })
		left := r.Count
		for i := range vault {
			if vault[i].Template != from.Template || vault[i].ExpireTime != from.ExpireTime || vault[i].Amount >= limit {
				continue
			}
			n := limit - vault[i].Amount
			if n > left {
				n = left
			}
			vault[i].Amount += n
			left -= n
			if left == 0 {
				break
			}
		}
		if left != 0 {
			replace(2, r.DestinationSlot, from.Template, left)
		}
	} else {
		replace(r.DestinationList, r.DestinationSlot, from.Template, uint32(targetAmount))
	}
	state, e := SaveBag(role.State, b)
	if e != nil {
		return nil, nil, e
	}
	items, e := json.Marshal(vault)
	if e != nil {
		return nil, nil, e
	}
	v.Items = items
	if _, e = VaultPayload(v); e != nil {
		return nil, nil, e
	}
	if _, e = protocol.InventoryRestore(b.Rows(), b.Expansion); e != nil {
		return nil, nil, e
	}
	return state, items, nil
}
