package character

import (
	"dfolan/internal/inventory"
	"dfolan/internal/social"
	"encoding/json"
)

// SocialCharacterInfo takes its look from the same saved worn projection as
// town entry, including clone avatars. It never changes the supplied save.
func (s *Service) SocialCharacterInfo(role Character) (social.CharacterInfo, error) {
	var info social.CharacterInfo
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return info, err
	}
	growth, err := state.WireAdvancement()
	if err != nil {
		return info, err
	}
	info.CharacterName, info.Job, info.GrowthType, info.Level = role.Name, role.Profession, growth, state.Level
	info.ServerGroup = s.ChannelContext[0]
	info.Fame, err = s.EquipmentFame(role.State)
	if err != nil {
		return info, err
	}
	rows, err := s.wornAppearance(role.State)
	if err != nil {
		return info, err
	}
	for _, row := range rows {
		if row.Slot < 12 {
			info.AvatarIndex[row.Slot] = row.Model
		}
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return info, err
	}
	for _, item := range bag.WornBaseItems() {
		if item.Slot == inventory.WeaponSlot {
			info.WeaponIndex = item.Template
		}
	}
	info.WeaponSkinIndex = bag.WeaponSkin
	// No saved dye/expert/reputation projection exists yet. Their required
	// native fields remain zero; don't invent values from fame or level.
	return info, nil
}
