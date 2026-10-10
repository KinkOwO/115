package character

import (
	"dfolan/internal/inventory"
	"encoding/json"
	"sort"
)

// CompanionReadiness is a read-only equipment summary, not a damage estimate.
// A missing weapon is advisory: native AI can supply a default visual weapon.
type CompanionReadiness struct {
	CharacterID         int64    `json:"character_id"`
	Name                string   `json:"name"`
	Level               byte     `json:"level"`
	WornCount           int      `json:"worn_count"`
	Weapon              uint32   `json:"weapon"`
	WeaponSlotKnown     bool     `json:"weapon_slot_known"`
	WeaponRequiredLevel *int32   `json:"weapon_required_level,omitempty"`
	Warnings            []string `json:"warnings,omitempty"`
}

func (s *Service) CompanionReadiness(role Character) (CompanionReadiness, error) {
	r := CompanionReadiness{CharacterID: role.ID, Name: role.Name}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return r, err
	}
	r.Level = state.Level
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return r, err
	}
	slot, known := s.WearRules.Slots["[weapon]"]
	r.WeaponSlotKnown = known
	for _, item := range bag.WornBaseItems() {
		if item.Template == 0 {
			continue
		}
		r.WornCount++
		if known && item.Slot == slot {
			r.Weapon = item.Template
		}
	}
	if known && r.Weapon == 0 {
		r.Warnings = append(r.Warnings, "未穿戴武器；原生默认武器外观不等于已有武器属性")
	}
	if known && r.Weapon != 0 && s.Equipment != nil {
		if d, e := s.Equipment.Definition(r.Weapon); e == nil {
			if cells := d.Fields["[minimum level]"]; len(cells) > 0 && cells[0].Value >= 0 {
				value := cells[0].Value
				r.WeaponRequiredLevel = &value
			}
		}
	}
	sort.Strings(r.Warnings)
	return r, nil
}
