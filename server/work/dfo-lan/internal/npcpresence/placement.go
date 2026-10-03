package npcpresence

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// Placement retains the native signed source fields. Parser state is shared
// across rows and [NPC] sections of one map parser invocation, not across maps.
type Placement struct {
	NPC, Quest              int32
	Completed, DungeonClear bool
	Direction               string
	X, Y, Z                 int32
	Section, Cell           int
	SurvivesLastSection     bool
	Resolved                bool
}

type PlacementProjection struct {
	Rows       []Placement
	Unresolved []string
}

// ProjectPlacements follows1471C2FD0/1471E4790's confirmed grammar. A later
// [NPC] section replaces the earlier vector while optional fields still carry.
// Syntax gaps taint subsequent carried fields rather than guessing defaults.
func ProjectPlacements(cells []pvf.Token) PlacementProjection {
	var result PlacementProjection
	state := Placement{Quest: -1, Resolved: true}
	section := -1
	for start := 0; start < len(cells); start++ {
		if cells[start].Type != 3 || cells[start].Text != "[NPC]" {
			continue
		}
		section++
		end := start + 1
		for end < len(cells) && cells[end].Type != 3 {
			end++
		}
		for i := start + 1; i < end; {
			rowStart := i
			npc := cells[i]
			i++
			if i < end && cells[i].Type == 6 && cells[i].Text == "[visible on dungeon if quest clear]" {
				state.Completed = true
				i++
			}
			if i < end && cells[i].Type == 0 {
				state.Quest = cells[i].Value
				i++
			}
			if i < end && cells[i].Type == 6 && cells[i].Text == "[visible on dungeon clear]" {
				state.DungeonClear = true
				i++
			}
			if npc.Type != 0 || i+4 > end || cells[i].Type != 6 ||
				(cells[i].Text != "[left]" && cells[i].Text != "[right]") ||
				cells[i+1].Type != 0 || cells[i+2].Type != 0 || cells[i+3].Type != 0 {
				result.Unresolved = append(result.Unresolved, fmt.Sprintf("NPC section %d cell %d: unsupported placement syntax", section, rowStart))
				state.Resolved = false
				break
			}
			row := state
			row.NPC, row.Direction = npc.Value, cells[i].Text
			row.X, row.Y, row.Z = cells[i+1].Value, cells[i+2].Value, cells[i+3].Value
			row.Section, row.Cell = section, rowStart
			result.Rows = append(result.Rows, row)
			i += 4
		}
		start = end - 1
	}
	for i := range result.Rows {
		result.Rows[i].SurvivesLastSection = result.Rows[i].Section == section
	}
	return result
}

// QuestSet separates missing input from a verified empty set. Progress is not
// consulted by native placement membership144F54570/144F54990.
type QuestSet struct {
	Known bool
	IDs   map[uint32]bool
}

func (s QuestSet) Has(id uint32) Truth {
	if !s.Known {
		return Unknown
	}
	if s.IDs[id] {
		return True
	}
	return False
}

func (p Placement) Gate(accepted, completed QuestSet) Truth {
	if !p.SurvivesLastSection {
		return False
	}
	if !p.Resolved {
		return Unknown
	}
	if p.Quest < 0 {
		return True
	}
	if p.Completed {
		return completed.Has(uint32(p.Quest))
	}
	return accepted.Has(uint32(p.Quest))
}
