package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
	"strings"
)

// .chr growtype numbering starts at one (the unadvanced profession).
// Keep the source triples separate from initial and awakening grants.
func AdvancementSkillGrants(tokens []pvf.Token) (map[byte][]int32, error) {
	out := map[byte][]int32{}
	grow, section := 0, ""
	for _, t := range tokens {
		if t.Type == 3 {
			section = strings.ToLower(t.Text)
			if strings.HasPrefix(section, "[growtype ") {
				n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(section, "[growtype "), "]"))
				if err != nil {
					continue
				}
				grow = n
				if grow < 1 || grow > 16 {
					return nil, fmt.Errorf("invalid source growtype")
				}
			}
			continue
		}
		if section != "[skill]" || grow < 2 {
			continue
		}
		if t.Type != 0 {
			return nil, fmt.Errorf("non-integer advancement skill")
		}
		out[byte(grow-1)] = append(out[byte(grow-1)], t.Value)
	}
	for _, row := range out {
		if len(row)%3 != 0 {
			return nil, fmt.Errorf("incomplete advancement skill triple")
		}
		seen := map[int32]bool{}
		for i := 0; i < len(row); i += 3 {
			if row[i] < 1 || row[i] > 65535 || row[i+1] < 1 || row[i+1] > 255 || row[i+2] < 1 || row[i+2] > 255 || seen[row[i]] {
				return nil, fmt.Errorf("invalid advancement skill triple")
			}
			seen[row[i]] = true
		}
	}
	return out, nil
}
