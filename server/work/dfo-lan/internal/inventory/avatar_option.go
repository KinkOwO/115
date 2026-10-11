package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

const rareAvatarOptionBoxTemplate uint32 = 2660452

func (s *WearService) SelectAvatarOption(role Role, r protocol.AvatarOptionRequest, consumables catalog.LootCatalog) (json.RawMessage, error) {
	if s == nil || s.Catalog == nil || !s.Rules.Special || s.Catalog.Source.SaveIdentity() != role.ConfigVersion || r.Option == 0 || r.Option == 255 {
		return nil, fmt.Errorf("invalid avatar selection context")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	if r.Paid {
		if r.Location != 0 || r.ConsumableSlot < 2 || r.ConsumableTemplate != rareAvatarOptionBoxTemplate {
			return nil, fmt.Errorf("unsupported rare avatar option consumable")
		}
		for n, item := range b.Worn {
			if item.Slot != r.Slot {
				continue
			}
			if item.Template != r.Template {
				return nil, fmt.Errorf("stale avatar selection")
			}
			if item.Durability == 0 {
				return nil, fmt.Errorf("paid avatar reselection requires an existing option")
			}
			d, err := s.Catalog.Definition(item.Template)
			if err != nil {
				return nil, err
			}
			rarity := d.Fields["[rarity]"]
			if !d.IsAvatar() || !d.IsCloneAvatar() || len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value != 2 || !hasAvatarSelectAbility(d, r.Option) {
				return nil, fmt.Errorf("target is not a rare selectable Clone avatar")
			}
			next, _, err := b.Consume(consumables, r.ConsumableSlot, r.ConsumableTemplate)
			if err != nil {
				return nil, err
			}
			next.Worn = append([]BagEquipment(nil), next.Worn...)
			next.Worn[n].Durability = uint16(r.Option)
			return SaveBag(role.State, next)
		}
		return nil, fmt.Errorf("avatar selection target missing")
	}
	if r.Location != 2 {
		return nil, fmt.Errorf("invalid avatar selection location")
	}
	for n, item := range b.Special[1] {
		if item.Slot != r.Slot {
			continue
		}
		if item.Template != r.Template {
			return nil, fmt.Errorf("stale avatar selection")
		}
		if item.Durability != 0 {
			return nil, fmt.Errorf("avatar option already selected")
		}
		d, e := s.Catalog.Definition(item.Template)
		if e != nil {
			return nil, e
		}
		k := d.Fields["[equipment type]"]
		if len(k) == 0 || EquipmentBagSpace(k[0].Text) != 1 || len(d.Fields["[avatar type select]"]) == 0 {
			return nil, fmt.Errorf("avatar has no selectable source options")
		}
		// Native14576da96 maps wire u16+11 to item field58;145aadc40
		// writes the chosen option to field58. This is durability for other kinds.
		b.Special[1][n].Durability = uint16(r.Option)
		return SaveBag(role.State, b)
	}
	return nil, fmt.Errorf("avatar selection target missing")
}

func hasAvatarSelectAbility(d EquipmentDefinition, option byte) bool {
	cells := d.Fields["[avatar select ability]"]
	for i := 0; i+3 < len(cells); i++ {
		code, ability, plus, value := cells[i], cells[i+1], cells[i+2], cells[i+3]
		if code.Type != 0 || code.Value != int32(option) || (ability.Type != 6 && ability.Type != 8) || ability.Text == "" && ability.Reference == "" || (plus.Type != 3 && plus.Type != 6) || plus.Text != "+" || value.Type != 0 && value.Type != 2 {
			continue
		}
		return true
	}
	return false
}
