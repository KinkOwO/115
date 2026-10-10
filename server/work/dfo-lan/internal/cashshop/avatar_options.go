package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"fmt"
)

func (c *PilotConfig) loadAvatarAbilityCases(resolve func(string) (catalog.ScriptRecord, error)) error {
	needed := false
	for _, v := range c.Entries {
		if v.Section == "[dont trade avatar]" && len(shopSections(v.Item.Cells)["[ability case index]"]) > 0 {
			needed = true
			break
		}
	}
	if !needed {
		return nil
	}
	s, err := resolve("skill/abilitydatas.dat")
	if err != nil {
		return fmt.Errorf("avatar purchase ability source: %w", err)
	}
	if s.Path != "skill/abilitydatas.dat" || !digestValid(s.SHA256) {
		return fmt.Errorf("invalid avatar purchase ability source")
	}
	c.avatarAbilityCases = map[int32]map[uint16]string{}
	for i, t := range s.Cells {
		if t.Type != 3 || t.Text != "[ability case]" {
			continue
		}
		if i+1 >= len(s.Cells) || s.Cells[i+1].Type != 0 {
			return fmt.Errorf("invalid avatar purchase ability case")
		}
		end := i + 2
		for end < len(s.Cells) && s.Cells[end].Type != 3 {
			end++
		}
		options, err := inventory.AvatarAbilityOptions(s.Cells[i+2 : end])
		if err != nil {
			return err
		}
		id := s.Cells[i+1].Value
		if _, exists := c.avatarAbilityCases[id]; exists {
			return fmt.Errorf("duplicate avatar purchase ability case %d", id)
		}
		c.avatarAbilityCases[id] = options
	}
	return nil
}

func (c PilotConfig) avatarPurchaseOptions(item catalog.ScriptRecord) [4]uint64 {
	fields := shopSections(item.Cells)
	options, err := inventory.AvatarAbilityOptions(fields["[avatar select ability]"])
	if err != nil {
		return [4]uint64{}
	}
	if ref := fields["[ability case index]"]; len(ref) > 0 {
		if len(ref) != 1 || ref[0].Type != 0 {
			return [4]uint64{}
		}
		options = c.avatarAbilityCases[ref[0].Value]
	}
	var out [4]uint64
	for id := range options {
		// Native 14088DFF0 transmits the selected key without adding one;
		// 147184690/147189710 also retain explicit (possibly sparse) keys.
		// Keep 255 reserved by the avatar selection protocol.
		if id < 255 {
			wire := id
			out[wire/64] |= uint64(1) << (wire % 64)
		}
	}
	return out
}
