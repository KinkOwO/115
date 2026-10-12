package catalog

import (
	"fmt"
	"math"
)

// NativePhaseRewards binds the owned source boss to the shared transactional
// phase-award service. Base reward difficulty and operation reward difficulty
// are distinct native dimensions; neither is guessed from numeric type IDs.
func NativePhaseRewards(contents *LegionContents, name string, dungeon uint32, operation uint16, baseDifficulty, operationDifficulty string) (LegionPhaseRewards, error) {
	var r LegionPhaseRewards
	if contents == nil || contents.Source.Checksum == "" {
		return r, fmt.Errorf("native legion reward source missing")
	}
	c, ok := contents.Contents[name]
	if !ok || c.NativeOperations == nil || c.PhaseReward <= 0 || c.PhaseReward > math.MaxUint32 || c.ProbabilityRate <= 0 || c.ProbabilityRate > math.MaxUint32 {
		return r, fmt.Errorf("native phase reward limit/source missing")
	}
	owned := false
	sourceIndex := -1
	for i, d := range c.Dungeons {
		if d.Dungeon == dungeon && dungeon != 0 {
			owned = true
			sourceIndex = i
		}
	}
	if !owned {
		return r, fmt.Errorf("reward dungeon does not belong to content")
	}
	d, ok := c.NativeOperations.Definitions[operation]
	if !ok {
		return r, fmt.Errorf("reward operation absent from source")
	}
	r = LegionPhaseRewards{Content: name, Checksum: contents.Source.Checksum, Dungeon: dungeon, WeeklyLimit: uint32(c.PhaseReward), ProbabilityRate: uint32(c.ProbabilityRate)}
	for _, row := range c.Rewards {
		if row.PartyType != "solo" || row.Difficulty != baseDifficulty || row.Kind != "basic" {
			continue
		}
		if len(row.Values) != 3 {
			return r, fmt.Errorf("native basic reward source shape invalid")
		}
		for _, v := range row.Values {
			if v <= 0 || v > math.MaxUint32 {
				return r, fmt.Errorf("invalid native basic reward operand")
			}
		}
		r.Basic = append(r.Basic, LegionOperationReward{Difficulty: row.Difficulty, Weight: uint32(row.Values[0]), Template: uint32(row.Values[1]), Amount: uint32(row.Values[2])})
	}
	for _, row := range d.Rewards {
		if row.Difficulty != operationDifficulty {
			continue
		}
		if row.Template == 0 || row.Amount == 0 || row.Weight == 0 || row.Tier > math.MaxInt32 || row.Account > math.MaxInt32 {
			return r, fmt.Errorf("invalid native operation reward operand")
		}
		r.Additional = append(r.Additional, LegionOperationReward{Difficulty: row.Difficulty, Template: row.Template, Amount: row.Amount, Weight: row.Weight, Grade: int32(row.Tier), Extra: int32(row.Account)})
	}
	// This table is keyed by the source dungeon ordinal, not the run's
	// randomized phase. It has its own lottery, separate from the selected card.
	for _, row := range c.NativeOperations.ClearRewards {
		if row.DungeonIndex != uint32(sourceIndex) {
			continue
		}
		if row.Template == 0 || row.Amount == 0 || row.Weight > r.ProbabilityRate || row.Account > 1 {
			return r, fmt.Errorf("invalid source clear reward")
		}
		r.GlobalClear = append(r.GlobalClear, LegionOperationReward{Template: row.Template, Amount: row.Amount, Weight: row.Weight, Extra: int32(row.Account)})
	}
	if len(r.Basic) == 0 || len(r.Additional) == 0 {
		return r, fmt.Errorf("native reward dimension missing")
	}
	return r, nil
}
