package character

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

// TagCharacterSnapshot读取已有角色存档，不创建角色、不修改穿戴或技能。
// 调用方负责从同一账号的稳定角色ID解析名单，避免账号排序后错用其他角色。
func (s *Service) TagCharacterSnapshot(role Character) (protocol.TagCharacter, error) {
	var out protocol.TagCharacter
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return out, err
	}
	if role.ID == 0 || role.AccountID == 0 || state.Level == 0 || state.SourceSHA256 == "" {
		return out, fmt.Errorf("队友缺少角色身份或源属性")
	}
	advance, err := state.WireAdvancement()
	if err != nil {
		return out, err
	}
	out.WireID, out.Name = role.WireID, role.Name
	out.Level, out.Profession, out.Advancement = state.Level, role.Profession, advance
	out.Stats, err = entryPackedStats(state)
	if err != nil {
		return out, err
	}
	out.Appearance, err = s.wornAppearance(role.State)
	if err != nil {
		return out, err
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return out, err
	}
	// 与上游当前角色详情一致，使用背包已保存的特殊装备栏开放状态。
	out.ExpandEquipFlags = bag.ExpandEquipFlags
	for _, item := range bag.WornBaseItems() {
		if err := item.ValidateRecord(); err != nil {
			return out, err
		}
		row := protocol.DetailedWorn{Slot: item.Slot, Template: item.Template,
			Durability: item.Durability, Period: item.Period, Record: item.Record,
			AvatarOptions: item.AvatarOptions, AvatarSockets: item.AvatarSockets}
		if item.Slot <= 11 && item.Group == 0 {
			for _, look := range bag.Worn {
				if look.Slot == item.Slot && look.Group == 1 {
					row.HeaderTemplateA = look.Template
					break
				}
			}
		}
		out.Worn = append(out.Worn, row)
	}
	skills, err := s.skillRows(role, state, 0)
	if err != nil {
		return out, err
	}
	for _, skill := range skills {
		if skill.Level != 0 {
			out.Skills = append(out.Skills, skill)
		}
	}
	v := state.SkillVariations[0]
	fillVariationSlots(&v)
	copy(out.Intensions[:], v.Intensions)
	copy(out.Options[:], v.Options)
	return out, nil
}
