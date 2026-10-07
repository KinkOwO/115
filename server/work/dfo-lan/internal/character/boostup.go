package character

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
)

// baseGrowAwakening mirrors the branchless awakening gate in awakening.go:
// an unadvanced base-grow profession only maps capsule awakening when the
// source carries its own awakening grants.
func baseGrowAwakening(p catalog.Profession) bool {
	return len(p.AdvancementGrowth) == 0 && len(p.AwakeningSkills[0]) > 0
}

// AdminLevel walks the ordinary experience curve up to the target level in a
// single grant; it never lowers a level and never mints anything outside the
// normal growth path.
func (s *ProgressionService) AdminLevel(role Character, target int) (Character, error) {
	if s == nil || target < 2 || target > int(s.Rules.LevelCap) || target-2 >= len(s.Catalog.Thresholds) {
		return role, fmt.Errorf("invalid admin level target")
	}
	var state State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return role, e
	}
	if int(state.Level) >= target {
		return role, nil
	}
	threshold := s.Catalog.Thresholds[target-2]
	if state.Experience >= threshold {
		return role, fmt.Errorf("inconsistent experience ledger")
	}
	next, result, e := s.ApplyGain(role, threshold-state.Experience)
	if e != nil {
		return role, e
	}
	if int(result.Level) != target {
		return role, fmt.Errorf("admin level target mismatch")
	}
	return next, nil
}

// Pure transformation for the caller's existing character-event transaction.
// The observed capsule N19 has both base trees and unspent SP, not the later
// training preset. Reuse normal growth and source awakening grants here.
func (s *Service) BoostLevel(role Character, growth *ProgressionService, target byte) (Character, error) {
	if s == nil || growth == nil {
		return role, fmt.Errorf("boost growth service missing")
	}
	var before State
	if e := json.Unmarshal(role.State, &before); e != nil {
		return role, e
	}
	if before.Advancement == 0 && !baseGrowAwakening(s.Catalog.Professions[role.Profession]) {
		return role, fmt.Errorf("boost source awakening for unadvanced profession is not mapped")
	}
	if target == 0 || before.Level > target {
		return role, fmt.Errorf("capsule cannot lower character level")
	}
	next := role
	var e error
	if before.Level < target {
		next, e = growth.AdminLevel(role, int(target))
		if e != nil {
			return role, e
		}
	}
	for stage := before.Awakening + 1; stage <= 3; stage++ {
		next.State, e = s.ApplyAwakening(next, stage)
		if e != nil {
			return role, e
		}
	}
	return next, nil
}
