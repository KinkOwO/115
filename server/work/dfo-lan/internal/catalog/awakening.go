package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

func AwakeningSkillGrants(tokens []pvf.Token) map[byte]map[byte][]int32 {
	result := map[byte]map[byte][]int32{}
	grow, stage, section := 0, 0, ""
	for _, t := range tokens {
		if t.Type == 3 {
			section = strings.ToLower(t.Text)
			if strings.HasPrefix(section, "[growtype ") {
				fmt.Sscanf(section, "[growtype %d]", &grow)
				stage = 0
			}
			if strings.HasPrefix(section, "[awakening ") {
				fmt.Sscanf(section, "[awakening %d]", &stage)
			}
			continue
		}
		// [growtype 1] is growtype 0, the unadvanced base. A profession with no
		// change-of-job branch (demonic swordman, creator mage: [max grow count]
		// 1, the only [growtype N] section present) carries its [awakening 1..3]
		// blocks inside that same section, so growtype 1 must be kept. Ordinary
		// professions keep their awakening blocks under [growtype 2..N] only,
		// and the pre-section grow value 0 stays refused either way.
		if section != "[awakening skill]" || grow < 1 || grow > 16 || stage < 1 || stage > 3 || t.Type != 0 {
			continue
		}
		adv := byte(grow - 1)
		if result[adv] == nil {
			result[adv] = map[byte][]int32{}
		}
		result[adv][byte(stage)] = append(result[adv][byte(stage)], t.Value)
	}
	return result
}
