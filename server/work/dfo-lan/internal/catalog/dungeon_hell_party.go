package catalog

import (
	"dfolan/internal/catalog/pvf"
	"slices"
)

// consistentHellPartySection retains one declaration when every occurrence
// agrees. Haze repeats the same Hell room in two ordinary maze blocks; joining
// those cells makes valid scalar/coordinate fields appear malformed. Different
// declarations remain unsupported by the dungeon-wide HellParty model.
func consistentHellPartySection(cells []pvf.Token, name string) []pvf.Token {
	var first []pvf.Token
	found := false
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 || cells[i].Text != name {
			continue
		}
		start := i + 1
		end := start
		for end < len(cells) && cells[end].Type != 3 {
			end++
		}
		block := cells[start:end]
		if found && !slices.Equal(first, block) {
			return nil
		}
		first, found = block, true
		i = end - 1
	}
	return first
}
