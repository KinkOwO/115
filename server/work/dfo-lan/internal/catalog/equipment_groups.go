package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"slices"
)

// EquipmentAbilityGroups is the shared native equipmentgrouping.etc reader.
// A template can belong to several groups; repeated membership is not a second contribution.
func EquipmentAbilityGroups(cells []pvf.Token) (map[uint32][]uint32, error) {
	out := map[uint32][]uint32{}
	var group uint32
	haveGroup, inside := false, false
	field := ""
	for _, c := range cells {
		if c.Type == 3 {
			switch c.Text {
			case "[ability group]":
				inside, haveGroup = true, false
			case "[/ability group]":
				inside = false
			}
			field = c.Text
			continue
		}
		if !inside || (field != "[index]" && field != "[list]") {
			continue
		}
		if c.Type != 0 || c.Value < 0 {
			return nil, fmt.Errorf("invalid equipment ability group cell")
		}
		if field == "[index]" {
			if haveGroup {
				return nil, fmt.Errorf("multiple equipment group indices")
			}
			group, haveGroup = uint32(c.Value), true
		} else {
			if !haveGroup || c.Value == 0 {
				return nil, fmt.Errorf("invalid equipment group member")
			}
			id := uint32(c.Value)
			if !slices.Contains(out[id], group) {
				out[id] = append(out[id], group)
			}
		}
	}
	return out, nil
}
