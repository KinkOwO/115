package main

import (
	"dfolan/internal/quest"
	"fmt"
)

func jobAllowed(e *quest.Entry, job string) bool {
	if len(e.Jobs) == 0 {
		return true
	}
	for _, j := range e.Jobs {
		if j == "[all]" || j == job {
			return true
		}
	}
	return false
}

func prereqMet(e *quest.Entry, completed map[uint32]bool) bool {
	if len(e.PrerequisiteGroups) == 0 {
		return true
	}
	for _, group := range e.PrerequisiteGroups {
		if len(group) == 0 {
			continue
		}
		all := true
		for _, p := range group {
			if !completed[p] {
				all = false
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// whyBlocked lists every reason this quest is not offered, ignoring job and
// prerequisites (the caller has already filtered on those).
func whyBlocked(e *quest.Entry, level, adv byte) []string {
	var out []string
	if !e.Implemented {
		out = append(out, "objective type not implemented")
	}
	if !e.RewardUsable {
		out = append(out, "reward type not settleable")
	}
	if !e.GrowUsable {
		out = append(out, "grow-type gate unreadable")
	}
	if uint32(level) < e.MinimumLevel {
		out = append(out, fmt.Sprintf("below min level %d", e.MinimumLevel))
	}
	if uint32(level) > e.MaximumLevel {
		out = append(out, fmt.Sprintf("above max level %d", e.MaximumLevel))
	}
	for _, g := range e.GrowTypes {
		if g >= 0 && g != int32(adv) {
			out = append(out, fmt.Sprintf("needs advancement %d", g))
		}
	}
	return out
}

// classify returns the ids offered now, and per-id block reasons for the rest.
func classify(x *quest.Index, completed map[uint32]bool, level, adv byte, job string) ([]uint32, map[uint32][]string) {
	var offered []uint32
	blocked := map[uint32][]string{}
	for _, id := range x.Ordered {
		if completed[id] {
			continue
		}
		e := x.Entries[id]
		var reasons []string
		if !jobAllowed(e, job) {
			reasons = append(reasons, "wrong job")
		}
		if !prereqMet(e, completed) {
			reasons = append(reasons, "prerequisite incomplete")
		}
		reasons = append(reasons, whyBlocked(e, level, adv)...)
		if len(reasons) == 0 {
			offered = append(offered, id)
		} else {
			blocked[id] = reasons
		}
	}
	return offered, blocked
}

// successors are quests that name a completed quest as a prerequisite: the
// story's immediate next steps.
func successors(x *quest.Index, completed map[uint32]bool) []uint32 {
	var out []uint32
	seen := map[uint32]bool{}
	for id, e := range x.Entries {
		for _, p := range e.Prerequisites {
			if completed[p] && !seen[id] {
				seen[id] = true
				out = append(out, id)
			}
		}
	}
	return out
}
