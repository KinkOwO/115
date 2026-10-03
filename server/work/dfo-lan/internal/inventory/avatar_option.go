package inventory

import (
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
)

func (s *WearService) SelectAvatarOption(role Role, r protocol.AvatarOptionRequest) (json.RawMessage, error) {
	if s == nil || s.Catalog == nil || !s.Rules.Special || s.Catalog.Source.SaveIdentity() != role.ConfigVersion || r.Location != 2 || r.Option == 0 || r.Option == 255 {
		return nil, fmt.Errorf("invalid avatar selection context")
	}
	b, e := ReadBag(role.State)
	if e != nil {
		return nil, e
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
