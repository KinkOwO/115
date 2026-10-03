package character

import (
	"encoding/json"
	"fmt"
	"math"
)

// ApplyGain is pure: monster/quest transaction owners use the same current
// source growth and wire bounds before committing their own durable receipt.
func (s *ProgressionService) ApplyGain(current Character, gain uint64) (Character, GrowthAdvance, error) {
	var state State
	var result GrowthAdvance
	fail := func(e error) (Character, GrowthAdvance, error) { return current, result, e }
	if current.ConfigVersion != s.Catalog.Source.SaveIdentity() || s.Professions.Source.Checksum != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("progression source mismatch"))
	}
	if e := json.Unmarshal(current.State, &state); e != nil {
		return fail(e)
	}
	prof, ok := s.Professions.Professions[current.Profession]
	if !ok || prof.RawSHA256 != state.SourceSHA256 || state.Attributes == nil {
		return fail(fmt.Errorf("missing source character attributes"))
	}
	growth := prof.BaseGrowth
	if state.Advancement != 0 {
		// Normal creation/job changes already persist source-backed branches
		// without pilot flags. Use the same profession/branch as advancement
		// validation, or its first kill cannot finish the death response plan.
		growth = prof.AdvancementGrowth[state.Advancement]
		// Older swordmaster-only catalogs predate AdvancementGrowth. Preserve
		// that saved pilot without letting another job borrow its growth or
		// silently falling back to the unadvanced profession.
		if len(growth) == 0 && current.Profession == 0 && state.Advancement == 1 && state.SwordmasterPilot {
			growth = prof.SwordmasterGrowth
		}
	}
	if len(growth) == 0 {
		return fail(fmt.Errorf("missing source profession growth"))
	}
	var e error
	result, e = AddGrowthExperience(s.Catalog, s.Rules, state.Level, state.Experience, gain)
	if e != nil {
		return fail(e)
	}
	levels := int(result.Level) - int(state.Level)
	if levels > 0 {
		if len(prof.BaseGrowth) == 0 {
			return fail(fmt.Errorf("missing source base profession growth"))
		}
		for name, v := range growth {
			state.Attributes[name] = float32((math.Round(float64(state.Attributes[name])*10) + float64(levels)*math.Round(float64(v)*10)) / 10)
		}
	}
	for i, sp := range state.SkillPoints {
		next := uint32(sp) + result.SkillPointGain
		if next > 65535 {
			return fail(fmt.Errorf("skill point ledger overflow"))
		}
		state.SkillPoints[i] = uint16(next)
	}
	state.Level, state.Experience = result.Level, result.Experience
	// 只统计真实经验增量，不把 GM 设置等级或旧存档总经验反算成奖励。
	if state.AdventureEarnedExperience > math.MaxInt64 || gain > math.MaxInt64-state.AdventureEarnedExperience {
		return fail(fmt.Errorf("冒险团经验累计溢出"))
	}
	state.AdventureEarnedExperience += gain
	updated, e := json.Marshal(state)
	if e != nil {
		return fail(e)
	}
	var previous, fields map[string]json.RawMessage
	if e = json.Unmarshal(current.State, &previous); e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(updated, &fields); e != nil {
		return fail(e)
	}
	for key, value := range fields {
		previous[key] = value
	}
	updated, e = json.Marshal(previous)
	if e != nil {
		return fail(e)
	}
	current.State = updated
	if _, e = (&Service{Catalog: s.Professions}).EntryAddition(current); e != nil {
		return fail(e)
	}
	return current, result, nil
}
