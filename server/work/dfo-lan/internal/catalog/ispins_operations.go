package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

const IspinsOperationsPath = "contents/2022/stolenlandispins/etc/stolenlandispins.etc"

type IspinsOperationDefinition struct {
	Type          uint32
	FixedValue    uint32
	Difficulty    string
	MemberLimit   string
	SelectPool    uint32
	Rewards       []LegionOperationReward
	RandomRanges  []LegionOperationRange
	Probability   uint32
	AllowCoin     *int32
	RecommendFame uint32
}

// Preserve the native party/difficulty dimensions and signed unlimited value.
// These operands do not establish an outcome executor by themselves.
type LegionOperationRange struct {
	Party, Difficulty      string
	Minimum, Maximum, Step int32
}

// The six source columns are preserved independently. Grade/Extra are
// presentation operands; they are never an additional grant multiplier.
type LegionOperationReward struct {
	Difficulty               string
	Template, Amount, Weight uint32
	Grade, Extra             int32
}

// The current source's operation11 is type6 with fixed value4. The native
// selection sends its index as the action2 argument, rather than in Token.
func ParseIspinsOperations(cells []pvf.Token) (map[uint16]IspinsOperationDefinition, error) {
	result := map[uint16]IspinsOperationDefinition{}
	difficulties := map[uint16]string{}
	for _, table := range scriptWarpBlocks(cells, "[operation table]") {
		for _, operation := range scriptWarpBlocks(table, "[operation]") {
			typeCells := sectionCells(operation, "[type]")
			if len(typeCells) != 1 || typeCells[0].Type != 6 || typeCells[0].Text == "" {
				return nil, fmt.Errorf("invalid Ispins operation table type")
			}
			for _, index := range sectionCells(operation, "[index]") {
				if index.Type != 0 || index.Value < 1 || index.Value > 255 || difficulties[uint16(index.Value)] != "" {
					return nil, fmt.Errorf("invalid or duplicate Ispins operation table index")
				}
				difficulties[uint16(index.Value)] = typeCells[0].Text
			}
		}
	}
	for _, block := range scriptWarpBlocks(cells, "[operation data set]") {
		index, kind := sectionCells(block, "[index]"), sectionCells(block, "[type]")
		if len(index) != 1 || index[0].Type != 0 || index[0].Value < 1 || index[0].Value > 255 || len(kind) != 1 || kind[0].Type != 0 || kind[0].Value < 0 {
			return nil, fmt.Errorf("invalid Ispins source operation identity")
		}
		key := uint16(index[0].Value)
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate Ispins source operation%d", key)
		}
		rule := IspinsOperationDefinition{Type: uint32(kind[0].Value), Difficulty: difficulties[key]}
		if member := sectionCells(block, "[member limit]"); len(member) > 0 {
			if len(member) != 1 || member[0].Type != 6 || member[0].Text == "" {
				return nil, fmt.Errorf("invalid Ispins member limit")
			}
			rule.MemberLimit = member[0].Text
		}
		if pool := sectionCells(block, "[select pool]"); len(pool) > 0 {
			if len(pool) != 1 || pool[0].Type != 0 || pool[0].Value < 0 {
				return nil, fmt.Errorf("invalid Ispins select pool")
			}
			rule.SelectPool = uint32(pool[0].Value)
		}
		ranges := sectionCells(block, "[random range value]")
		if len(ranges)%5 != 0 {
			return nil, fmt.Errorf("incomplete Ispins operation random range")
		}
		for i := 0; i < len(ranges); i += 5 {
			if ranges[i].Type != 6 || ranges[i].Text == "" || ranges[i+1].Type != 6 || ranges[i+1].Text == "" || ranges[i+2].Type != 0 || ranges[i+3].Type != 0 || ranges[i+4].Type != 0 || ranges[i+2].Value > ranges[i+3].Value || ranges[i+4].Value < 0 || (ranges[i+2].Value != ranges[i+3].Value && ranges[i+4].Value == 0) {
				return nil, fmt.Errorf("invalid Ispins operation random range")
			}
			for _, previous := range rule.RandomRanges {
				if previous.Party == ranges[i].Text && previous.Difficulty == ranges[i+1].Text {
					return nil, fmt.Errorf("duplicate Ispins operation random range dimension")
				}
			}
			rule.RandomRanges = append(rule.RandomRanges, LegionOperationRange{ranges[i].Text, ranges[i+1].Text, ranges[i+2].Value, ranges[i+3].Value, ranges[i+4].Value})
		}
		for tag, target := range map[string]*uint32{"[probability]": &rule.Probability, "[recommend fame]": &rule.RecommendFame} {
			if values := sectionCells(block, tag); len(values) != 0 {
				if len(values) != 1 || values[0].Type != 0 || values[0].Value < 0 {
					return nil, fmt.Errorf("invalid Ispins operation %s", tag)
				}
				*target = uint32(values[0].Value)
			}
		}
		if values := sectionCells(block, "[allow coin]"); len(values) != 0 {
			if len(values) != 1 || values[0].Type != 0 || values[0].Value < -1 {
				return nil, fmt.Errorf("invalid Ispins operation coin limit")
			}
			value := values[0].Value
			rule.AllowCoin = &value
		}
		rewards := sectionCells(block, "[reward data]")
		if len(rewards)%6 != 0 {
			return nil, fmt.Errorf("incomplete Ispins operation reward row")
		}
		for i := 0; i < len(rewards); i += 6 {
			if rewards[i].Type != 6 || rewards[i].Text == "" {
				return nil, fmt.Errorf("invalid Ispins reward difficulty")
			}
			for j := 1; j < 6; j++ {
				if rewards[i+j].Type != 0 {
					return nil, fmt.Errorf("invalid Ispins reward scalar")
				}
			}
			if rewards[i+1].Value <= 0 || rewards[i+2].Value <= 0 || rewards[i+3].Value <= 0 || rewards[i+4].Value < 0 || rewards[i+5].Value < 0 {
				return nil, fmt.Errorf("invalid Ispins reward template/count/weight")
			}
			rule.Rewards = append(rule.Rewards, LegionOperationReward{rewards[i].Text, uint32(rewards[i+1].Value), uint32(rewards[i+2].Value), uint32(rewards[i+3].Value), rewards[i+4].Value, rewards[i+5].Value})
		}
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
