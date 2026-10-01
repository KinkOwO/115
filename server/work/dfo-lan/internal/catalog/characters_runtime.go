package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"maps"
)

// Default shortcuts and enabling source command/default-branch projections
// are server behavior. Profession fields and skill grants remain PVF data.
type CharacterRuntimePolicy struct {
	Version                    int                        `json:"version"`
	SourceChecksum             string                     `json:"source_checksum"`
	InitialSkillSlots          map[byte]map[uint16]uint16 `json:"initial_skill_slots"`
	EnableAdvancementShortcuts bool                       `json:"enable_advancement_shortcuts"`
	EnableSourceCommands       bool                       `json:"enable_source_commands"`
}

func characterCloneMapSlices[K comparable, V any](src map[K][]V) map[K][]V {
	out := make(map[K][]V, len(src))
	for k, v := range src {
		out[k] = append([]V(nil), v...)
	}
	return out
}
func characterCloneNested[K, L comparable, V any](src map[K]map[L]V) map[K]map[L]V {
	out := make(map[K]map[L]V, len(src))
	for k, v := range src {
		out[k] = maps.Clone(v)
	}
	return out
}

// ProjectCharacterRuntime keeps the existing gateway's effective view. The
// full ImportCharacters result separately retains the source growtype arrays.
// Those three duplicate/diagnostic arrays have no current gateway consumer;
// advancement grants/growth are already represented by their dedicated fields.
func ProjectCharacterRuntime(source Characters, policy CharacterRuntimePolicy) (Characters, error) {
	var out Characters
	// source_checksum 为空 = 接受内层归档的实际哈希（自动派生模式，见 next142）。
	// 非空则必须是合法的 32 字节 SHA256 且与源一致 —— 保留"钉死某一版"的能力。
	if policy.SourceChecksum != "" {
		b, err := hex.DecodeString(policy.SourceChecksum)
		if err != nil || len(b) != 32 || policy.SourceChecksum != source.Source.Checksum {
			return out, fmt.Errorf("character runtime policy/source mismatch")
		}
	}
	if policy.Version != 1 || len(policy.InitialSkillSlots) != len(source.Professions) || len(source.Professions) == 0 {
		return out, fmt.Errorf("character runtime policy/source mismatch")
	}
	out.Source = source.Source
	out.Professions = make(map[byte]Profession, len(source.Professions))
	for id, p := range source.Professions {
		layout, ok := policy.InitialSkillSlots[id]
		if !ok || len(layout) == 0 {
			return out, fmt.Errorf("missing profession %d shortcut policy", id)
		}
		grants := map[uint16]bool{}
		if len(p.InitialSkills)%3 != 0 {
			return out, fmt.Errorf("invalid native initial skill grants")
		}
		for i := 0; i < len(p.InitialSkills); i += 3 {
			if p.InitialSkills[i] > 0 && p.InitialSkills[i] <= 65535 && p.InitialSkills[i+1] > 0 && p.InitialSkills[i+2] == 1 {
				grants[uint16(p.InitialSkills[i])] = true
			}
		}
		used := map[uint16]bool{}
		for skill, slot := range layout {
			if !grants[skill] || slot >= 14 || used[slot] {
				return out, fmt.Errorf("shortcut %d/%d is not an initial source grant or has invalid slot", id, skill)
			}
			used[slot] = true
		}
		p.InitialAttributes = maps.Clone(p.InitialAttributes)
		p.BaseGrowth = maps.Clone(p.BaseGrowth)
		p.SwordmasterGrowth = maps.Clone(p.SwordmasterGrowth)
		p.InitialSections = characterCloneMapSlices[string, pvf.Token](p.InitialSections)
		p.InitialSkills = append([]int32(nil), p.InitialSkills...)
		p.AdvancementSkills = characterCloneMapSlices[byte, int32](p.AdvancementSkills)
		p.AdvancementGrowth = characterCloneNested(p.AdvancementGrowth)
		awakening := make(map[byte]map[byte][]int32, len(p.AwakeningSkills))
		for key, rows := range p.AwakeningSkills {
			awakening[key] = characterCloneMapSlices[byte, int32](rows)
		}
		p.AwakeningSkills = awakening
		p.CreateEquipment = append([]pvf.Token(nil), p.CreateEquipment...)
		p.CreateEquipmentOrder = append([]string(nil), p.CreateEquipmentOrder...)
		p.CreateEquipmentBySlot = characterCloneNested(p.CreateEquipmentBySlot)
		p.DefaultAppearance = append([]int32(nil), p.DefaultAppearance...)
		p.InitialSkillSlots = maps.Clone(layout)
		if policy.EnableAdvancementShortcuts {
			p.AdvancementSkillSlots = characterCloneNested(p.AdvancementSkillSlots)
		} else {
			p.AdvancementSkillSlots = nil
		}
		if policy.EnableSourceCommands {
			p.SkillCommands = characterCloneMapSlices[uint16, uint32](p.SkillCommands)
		} else {
			p.SkillCommands = nil
		}
		p.Growth, p.PresetSkills, p.SlotSkills = nil, nil, nil
		out.Professions[id] = p
	}
	return out, nil
}
