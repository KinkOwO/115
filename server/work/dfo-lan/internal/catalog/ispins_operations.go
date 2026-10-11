package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const IspinsOperationsPath = "contents/2022/stolenlandispins/etc/stolenlandispins.etc"

type IspinsOperationDefinition struct {
	Type       uint32
	FixedValue uint32
}

// The current source's operation11 is type6 with fixed value4. The native
// selection sends its index as the action2 argument, rather than in Token.
func ParseIspinsOperations(cells []pvf.Token) (map[uint16]IspinsOperationDefinition, error) {
	result := map[uint16]IspinsOperationDefinition{}
	for _, block := range scriptWarpBlocks(cells, "[operation data set]") {
		index, kind := sectionCells(block, "[index]"), sectionCells(block, "[type]")
		if len(index) != 1 || index[0].Type != 0 || index[0].Value < 1 || index[0].Value > 255 || len(kind) != 1 || kind[0].Type != 0 || kind[0].Value < 0 {
			return nil, fmt.Errorf("invalid Ispins source operation identity")
		}
		key := uint16(index[0].Value)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate Ispins source operation%d", key)
		}
		rule := IspinsOperationDefinition{Type: uint32(kind[0].Value)}
		fixed := sectionCells(block, "[type fixed value]")
		if len(fixed) > 0 {
			if len(fixed) != 1 || fixed[0].Type != 0 || fixed[0].Value < 0 {
				return nil, fmt.Errorf("invalid Ispins operation fixed value")
			}
			rule.FixedValue = uint32(fixed[0].Value)
		}
		if rule.Type == 6 && (rule.FixedValue == 0 || rule.FixedValue > 60) {
			return nil, fmt.Errorf("unsupported Ispins source time limit")
		}
		result[key] = rule
	}
	if len(result) == 0 {
		return nil, fmt.Errorf("Ispins source operation table is empty")
	}
	return result, nil
}
