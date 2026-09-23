package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
)

// OdysseyQuests mirrors the quest tables of aradodyssey.etc that decide which
// story quests an Arad Odyssey character counts as already cleared, and which
// branch quests must stay reachable ([quest clear] / [remove clear quest] /
// [show quest] / [branch quest]).
//
// Source semantics pinned by the manual rebuild (P3 subitems 9/10):
//
//   - [remove clear quest] 12911 sits inside the [quest clear] level 102 table
//     and 12884 is the level 102 branch quest, so the remove table is what
//     keeps the level 102 mainline intact.
//   - Branch quest 22987 (the level 115 abyss guide entry) is absent from every
//     [quest clear] table, so a graduation wipe must never swallow it.
type OdysseyQuests struct {
	// ClearLevels keeps one row per [level] block of [quest clear], in source
	// order. Levels are not required to be sorted by the source.
	ClearLevels []OdysseyQuestClear
	// RemoveClear lists [remove clear quest] ids, subtracted from every
	// ClearedAt result.
	RemoveClear []uint32
	// Show lists [show quest] ids.
	Show []uint32
	// Branches lists one row per (level, job, quest) triplet of
	// [branch quest]; Job -1 means "every profession".
	Branches []OdysseyBranchQuest
}

type OdysseyQuestClear struct {
	Level byte
	Quest []uint32
}

type OdysseyBranchQuest struct {
	Level byte
	// Job is the character script id (catalog.Characters.Professions[*].ID
	// domain); -1 marks an all-profession row.
	Job     int32
	Quest   uint32
	Unknown int32 // trailing cell of each triplet, kept for lossless pinning
}

// OdysseyQuestCounts pins the source table shape (manual: 11 levels / 354
// cleared rows / 10 removals / 2 show rows / branch levels 80, 102, 115).
const (
	OdysseyQuestClearLevels  = 11
	OdysseyQuestClearTotal   = 354
	OdysseyQuestRemoveTotal  = 10
	OdysseyQuestShowTotal    = 2
	OdysseyBranchLevelCount  = 3
	OdysseyGraduationLevelV  = 115
	OdysseyKeepMainlineQuest = 12911 // removed from the cleared set on purpose
	OdysseyAbyssGuideQuest   = 22987 // level 115 branch entry, must survive
)

func findSectionBounds(cells []pvf.Token, name string) (int, int, bool) {
	open := "[" + name[1:]
	close := "[/" + name[1:]
	start, end := -1, -1
	depth := 0
	for i, c := range cells {
		if c.Type != 3 {
			continue
		}
		if depth == 0 && c.Text == open {
			start = i
			depth = 1
			continue
		}
		if depth == 1 && c.Text == close {
			end = i
			return start, end, true
		}
	}
	return start, end, false
}

func sectionInts(cells []pvf.Token, name string) ([]int32, error) {
	start, end, ok := findSectionBounds(cells, name)
	if !ok {
		return nil, fmt.Errorf("missing Odyssey section %s", name)
	}
	var out []int32
	for _, c := range cells[start+1 : end] {
		if c.Type != 0 {
			return nil, fmt.Errorf("non-numeric cell inside %s", name)
		}
		out = append(out, c.Value)
	}
	return out, nil
}

// loadOdysseyQuests parses the four quest tables out of the aradodyssey.etc
// token stream. Every deviation from the pinned source shape is a hard error:
// the tables drive which quest rows get written, so a silently drifted source
// must never reach the database.
func loadOdysseyQuests(cells []pvf.Token) (*OdysseyQuests, error) {
	q := &OdysseyQuests{}

	// [quest clear]: sequence of [level] N <ids...> [/level] blocks.
	start, end, ok := findSectionBounds(cells, "[quest clear]")
	if !ok {
		return nil, fmt.Errorf("missing Odyssey section [quest clear]")
	}
	total := 0
	for i := start + 1; i < end; {
		c := cells[i]
		if c.Type != 3 || c.Text != "[level]" {
			return nil, fmt.Errorf("unexpected cell in [quest clear] at %d", i)
		}
		i++
		if i >= end || cells[i].Type != 0 {
			return nil, fmt.Errorf("missing [quest clear] level header")
		}
		level := cells[i].Value
		if level <= 0 || level > OdysseyGraduationLevelV {
			return nil, fmt.Errorf("invalid [quest clear] level %d", level)
		}
		i++
		row := OdysseyQuestClear{Level: byte(level)}
		for i < end {
			inner := cells[i]
			if inner.Type == 3 {
				if inner.Text != "[/level]" {
					return nil, fmt.Errorf("unclosed [quest clear] level block")
				}
				i++
				break
			}
			if inner.Type != 0 || inner.Value <= 0 || inner.Value > 65535 {
				return nil, fmt.Errorf("invalid [quest clear] quest id at level %d", level)
			}
			row.Quest = append(row.Quest, uint32(inner.Value))
			i++
		}
		if len(row.Quest) == 0 {
			return nil, fmt.Errorf("empty [quest clear] level %d", level)
		}
		q.ClearLevels = append(q.ClearLevels, row)
	}
	for _, row := range q.ClearLevels {
		total += len(row.Quest)
	}
	if len(q.ClearLevels) != OdysseyQuestClearLevels || total != OdysseyQuestClearTotal {
		return nil, fmt.Errorf("[quest clear] shape drifted: %d levels / %d quests", len(q.ClearLevels), total)
	}

	removes, e := sectionInts(cells, "[remove clear quest]")
	if e != nil {
		return nil, e
	}
	if len(removes) != OdysseyQuestRemoveTotal {
		return nil, fmt.Errorf("[remove clear quest] shape drifted: %d rows", len(removes))
	}
	for _, v := range removes {
		if v <= 0 || v > 65535 {
			return nil, fmt.Errorf("invalid [remove clear quest] id %d", v)
		}
		q.RemoveClear = append(q.RemoveClear, uint32(v))
	}

	shows, e := sectionInts(cells, "[show quest]")
	if e != nil {
		return nil, e
	}
	if len(shows) != OdysseyQuestShowTotal {
		return nil, fmt.Errorf("[show quest] shape drifted: %d rows", len(shows))
	}
	for _, v := range shows {
		if v <= 0 || v > 65535 {
			return nil, fmt.Errorf("invalid [show quest] id %d", v)
		}
		q.Show = append(q.Show, uint32(v))
	}

	// [branch quest]: [level] N ([quest list by job] job quest flag ...
	// [/quest list by job])* blocks. Source truth: these [level] blocks are
	// NOT closed with [/level] - the next [level] (or the section close)
	// implicitly ends the previous one.
	start, end, ok = findSectionBounds(cells, "[branch quest]")
	if !ok {
		return nil, fmt.Errorf("missing Odyssey section [branch quest]")
	}
	branchLevels := map[byte]bool{}
	for i := start + 1; i < end; {
		c := cells[i]
		if c.Type != 3 || c.Text != "[level]" {
			return nil, fmt.Errorf("unexpected cell in [branch quest] at %d", i)
		}
		i++
		if i >= end || cells[i].Type != 0 || cells[i].Value <= 0 || cells[i].Value > OdysseyGraduationLevelV {
			return nil, fmt.Errorf("invalid [branch quest] level header")
		}
		level := byte(cells[i].Value)
		if branchLevels[level] {
			return nil, fmt.Errorf("duplicate [branch quest] level %d", level)
		}
		branchLevels[level] = true
		i++
		rows := 0
		for i < end {
			inner := cells[i]
			if inner.Type == 3 && inner.Text == "[level]" {
				// Implicit close: the next level block starts here.
				break
			}
			if inner.Type != 3 || inner.Text != "[quest list by job]" {
				return nil, fmt.Errorf("unexpected cell in [branch quest] level %d at %d", level, i)
			}
			i++
			// Triplets until the closing [quest list by job] tag.
			var triplet []int32
			for i < end {
				item := cells[i]
				if item.Type == 3 && item.Text == "[/quest list by job]" {
					i++
					break
				}
				if item.Type != 0 {
					return nil, fmt.Errorf("non-numeric [quest list by job] cell at %d", i)
				}
				triplet = append(triplet, item.Value)
				i++
			}
			if len(triplet) == 0 || len(triplet)%3 != 0 {
				return nil, fmt.Errorf("unpaired [quest list by job] triplets at level %d", level)
			}
			for t := 0; t < len(triplet); t += 3 {
				job, quest, extra := triplet[t], triplet[t+1], triplet[t+2]
				if quest <= 0 || quest > 65535 {
					return nil, fmt.Errorf("invalid branch quest id %d at level %d", quest, level)
				}
				q.Branches = append(q.Branches, OdysseyBranchQuest{Level: level, Job: job, Quest: uint32(quest), Unknown: extra})
				rows++
			}
		}
		if rows == 0 {
			return nil, fmt.Errorf("empty [branch quest] level %d", level)
		}
	}
	if len(branchLevels) != OdysseyBranchLevelCount {
		return nil, fmt.Errorf("[branch quest] shape drifted: %d levels", len(branchLevels))
	}
	if _, ok := branchLevels[OdysseyGraduationLevelV]; !ok {
		return nil, fmt.Errorf("[branch quest] missing graduation level %d", OdysseyGraduationLevelV)
	}

	// Pinned source semantics: the removal table exists to rescue the level
	// 102 mainline (12911), and the abyss guide (22987) is branch-only, so it
	// must never appear inside a cleared set.
	if !q.clearedContains(OdysseyKeepMainlineQuest) {
		return nil, fmt.Errorf("[quest clear] lost pinned mainline quest %d", OdysseyKeepMainlineQuest)
	}
	if q.clearedContains(OdysseyAbyssGuideQuest) {
		return nil, fmt.Errorf("[quest clear] must not contain branch quest %d", OdysseyAbyssGuideQuest)
	}
	removed := false
	for _, v := range q.RemoveClear {
		if v == OdysseyKeepMainlineQuest {
			removed = true
		}
	}
	if !removed {
		return nil, fmt.Errorf("[remove clear quest] lost pinned removal %d", OdysseyKeepMainlineQuest)
	}
	return q, nil
}

func (q *OdysseyQuests) clearedContains(id uint32) bool {
	for _, row := range q.ClearLevels {
		for _, v := range row.Quest {
			if v == id {
				return true
			}
		}
	}
	return false
}

// ClearedAt unions every [quest clear] row at or below level, then subtracts
// [remove clear quest]. The result is sorted so callers write deterministic
// quest rows.
func (q *OdysseyQuests) ClearedAt(level byte) []uint32 {
	seen := map[uint32]bool{}
	for _, row := range q.ClearLevels {
		if row.Level > level {
			continue
		}
		for _, id := range row.Quest {
			seen[id] = true
		}
	}
	for _, id := range q.RemoveClear {
		delete(seen, id)
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// BranchQuestsUpTo returns the branch quests at or below level that concern
// profession (rows with Job -1 always apply). Result is sorted and deduped.
func (q *OdysseyQuests) BranchQuestsUpTo(level byte, profession byte) []uint32 {
	seen := map[uint32]bool{}
	for _, row := range q.Branches {
		if row.Level > level || (row.Job != -1 && row.Job != int32(profession)) {
			continue
		}
		seen[row.Quest] = true
	}
	out := make([]uint32, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}
