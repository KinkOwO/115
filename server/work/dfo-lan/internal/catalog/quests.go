package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
)

type QuestDefinition struct {
	ID             uint32       `json:"id"`
	Script         ScriptRecord `json:"script"`
	MinimumLevel   uint32       `json:"minimum_level"`
	MaximumLevel   uint32       `json:"maximum_level"`
	Jobs           []string     `json:"jobs"`
	Prerequisites  []uint32     `json:"prerequisites"`
	Kind           string       `json:"kind"`
	ObjectiveCells []pvf.Token  `json:"objective_cells"`
	RewardCells    []pvf.Token  `json:"reward_cells"`
	Pending        []string     `json:"pending,omitempty"`
}
type QuestCatalog struct {
	Source pvf.ArchiveSnapshot        `json:"source"`
	Index  ScriptRecord               `json:"index"`
	Quests map[uint32]QuestDefinition `json:"quests"`
}

func ImportQuests(a *pvf.Archive) (QuestCatalog, error) {
	q := QuestCatalog{Source: a.Snapshot(), Quests: map[uint32]QuestDefinition{}}
	var e error
	q.Index, e = ReadScript(a, "list/quest.lst")
	if e != nil {
		return q, e
	}
	index, e := ParseIndex(q.Index.Cells)
	if e != nil {
		return q, e
	}
	for _, row := range index {
		d := QuestDefinition{ID: row.ID}
		d.Script, e = ResolveScript(a, row.Path)
		if e != nil {
			d.Pending = append(d.Pending, e.Error())
			q.Quests[row.ID] = d
			continue
		}
		level := sectionCells(d.Script.Cells, "[level]")
		if len(level) == 2 && level[0].Type == 0 && level[1].Type == 0 && level[0].Value >= 0 && level[1].Value >= level[0].Value {
			d.MinimumLevel = uint32(level[0].Value)
			d.MaximumLevel = uint32(level[1].Value)
		} else {
			d.Pending = append(d.Pending, "unsupported level condition")
		}
		for _, c := range sectionCells(d.Script.Cells, "[job]") {
			if c.Type == 6 {
				d.Jobs = append(d.Jobs, c.Text)
			} else {
				d.Pending = append(d.Pending, "unsupported job condition")
			}
		}
		for _, c := range sectionCells(d.Script.Cells, "[pre required quest]") {
			if c.Type == 0 && c.Value > 0 {
				d.Prerequisites = append(d.Prerequisites, uint32(c.Value))
			} else {
				d.Pending = append(d.Pending, "unsupported prerequisite condition")
			}
		}
		// The primary objective is the first [type] token; later tokens are
		// sub-conditions. LoadQuests re-derives this for older catalogs.
		for _, c := range sectionCells(d.Script.Cells, "[type]") {
			if c.Type == 6 {
				d.Kind = c.Text
				break
			}
		}
		d.ObjectiveCells = sectionCells(d.Script.Cells, "[int data]")
		d.RewardCells = sectionCells(d.Script.Cells, "[reward int data]")
		q.Quests[row.ID] = d
	}
	return q, nil
}
func LoadQuests(path string) (QuestCatalog, error) {
	var q QuestCatalog
	b, e := os.ReadFile(path)
	if e != nil {
		return q, e
	}
	if e = json.Unmarshal(b, &q); e != nil {
		return q, e
	}
	if len(q.Source.Checksum) != 64 || len(q.Quests) == 0 {
		return q, fmt.Errorf("invalid quest source")
	}
	for id, d := range q.Quests {
		if id != d.ID {
			return q, fmt.Errorf("quest key mismatch")
		}
		// Reproject the current source tag for catalogs generated before this
		// parser correction. Never treat an omitted stale projection as no gate.
		d.Prerequisites = nil
		for _, cell := range sectionCells(d.Script.Cells, "[pre required quest]") {
			if cell.Type != 0 || cell.Value <= 0 {
				d.Pending = append(d.Pending, "unsupported prerequisite condition")
				continue
			}
			d.Prerequisites = append(d.Prerequisites, uint32(cell.Value))
		}
		// A compound [type] lists the primary objective first and appends
		// sub-conditions ("arrive in town", "accept"). Catalogs generated
		// before this correction baked the LAST token as the Kind, which hid
		// the real objective: 34 quests - including the main story's level-22
		// beat (21029, "[look cinematic] arrive in town") - looked like an
		// unimplemented "arrive in town" type when their first token is an
		// objective this build already settles. Re-derive from the first
		// token; the [int data] the decoders read fits that primary objective.
		for _, cell := range sectionCells(d.Script.Cells, "[type]") {
			if cell.Type == 6 {
				d.Kind = cell.Text
				break
			}
		}
		q.Quests[id] = d
	}
	return q, nil
}
