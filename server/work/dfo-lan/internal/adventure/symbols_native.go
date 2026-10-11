package adventure

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// The source index is also the achievement key used by 142F9FB10.
// Test policy unlocks selection, never synthesizes achievement completion.
func readSymbols(cells []pvf.Token) (map[uint32]bool, error) {
	result := map[uint32]bool{}
	for _, block := range ruleBlocks(cells, "[adv symbol]") {
		id, err := ruleUint(block, "[index]")
		if err != nil {
			return nil, err
		}
		if result[id] {
			return nil, fmt.Errorf("duplicate adventure symbol %d", id)
		}
		result[id] = true
	}
	if !result[0] {
		return nil, fmt.Errorf("adventure symbols lack source default")
	}
	return result, nil
}
