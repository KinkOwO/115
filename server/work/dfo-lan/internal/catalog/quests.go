package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"sort"
)

type QuestDefinition struct {
	ID                 uint32       `json:"id"`
	Script             ScriptRecord `json:"script"`
	MinimumLevel       uint32       `json:"minimum_level"`
	MaximumLevel       uint32       `json:"maximum_level"`
	Jobs               []string     `json:"jobs"`
	Prerequisites      []uint32     `json:"prerequisites"`
	PrerequisiteGroups [][]uint32   `json:"prerequisite_groups,omitempty"`
	// Collisions names every [collision quest] peer. Source quests mark the
	// branches of a single choice (Silent City faction, job change, ...) as
	// mutually exclusive: once a character accepts or completes one member,
	// the others must leave the available list and refuse acceptance.
	Collisions     []uint32    `json:"collisions,omitempty"`
	Kind           string      `json:"kind"`
	ObjectiveCells []pvf.Token `json:"objective_cells"`
	RewardCells    []pvf.Token `json:"reward_cells"`
	Pending        []string    `json:"pending,omitempty"`
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
		d.PrerequisiteGroups, d.Prerequisites, e = parsePrerequisiteGroups(d.Script.Cells)
		if e != nil {
			d.Pending = append(d.Pending, e.Error())
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
		d.Collisions = mergeFactionBranchCollisions(row.ID, parseCollisionQuests(d.Script.Cells))
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
		d.PrerequisiteGroups, d.Prerequisites, e = parsePrerequisiteGroups(d.Script.Cells)
		if e != nil {
			d.Pending = append(d.Pending, e.Error())
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
		d.Collisions = mergeFactionBranchCollisions(id, parseCollisionQuests(d.Script.Cells))
		q.Quests[id] = d
	}
	return q, nil
}

// Each repeated [pre required quest] section is an alternative. IDs within a
// section must all be completed. Keep the flat projection for graph audits.
func parsePrerequisiteGroups(cells []pvf.Token) ([][]uint32, []uint32, error) {
	var groups [][]uint32
	var flat []uint32
	var group []uint32
	active := false
	flush := func() error {
		if !active {
			return nil
		}
		if len(group) == 0 {
			// Source files also use an empty section for no prerequisite.
			return nil
		}
		groups = append(groups, group)
		group = nil
		return nil
	}
	for _, cell := range cells {
		if cell.Type == 3 {
			if err := flush(); err != nil {
				return nil, nil, err
			}
			active = cell.Text == "[pre required quest]"
			continue
		}
		if !active {
			continue
		}
		if cell.Type != 0 || cell.Value <= 0 {
			return nil, nil, fmt.Errorf("unsupported prerequisite condition")
		}
		id := uint32(cell.Value)
		group = append(group, id)
		flat = append(flat, id)
	}
	if err := flush(); err != nil {
		return nil, nil, err
	}
	return groups, flat, nil
}

// parseCollisionQuests reads every [collision quest] section: the plain quest
// IDs it names are the peers of a mutually exclusive branch. Unlike the
// prerequisite parser, an empty section is simply no constraint.
func parseCollisionQuests(cells []pvf.Token) []uint32 {
	var ids []uint32
	on := false
	for _, cell := range cells {
		if cell.Type == 3 {
			on = cell.Text == "[collision quest]"
			continue
		}
		if on && cell.Type == 0 && cell.Value > 0 {
			ids = append(ids, uint32(cell.Value))
		}
	}
	return ids
}

// factionBranchGroups models the Silent City (寂静城) faction choice. Source
// quests 3868 / 3869 are [question] quests whose answers gate exactly one
// branch lead-in via [pre required quest answer]; this build does not settle
// question quests yet, so every branch would be offered at once and a
// character could walk all three faction storylines (the reported bug). Until
// the answer mechanism lands, express the same exclusivity with mutual
// [collision quest] edges across the branches:
//
//	{3870}       branch A  — unlocks Luke_01_01 (3923)
//	{3871, 3872} branch B  — unlocks Luke_01_02 (3924); 3872 is the 3869 alt
//	{3873, 3874} branch C  — unlocks Luke_01_03 (3925); 3874 is the 3869 alt
var factionBranchGroups = [][]uint32{
	{3870},
	{3871, 3872},
	{3873, 3874},
}

// mergeFactionBranchCollisions adds every other faction branch to a quest's
// collision set while keeping the parsed source edges, so accepting or
// completing one lead-in hides (and Accept refuses) the siblings of the other
// branches. The result is sorted and duplicate-free.
func mergeFactionBranchCollisions(id uint32, collisions []uint32) []uint32 {
	for i, group := range factionBranchGroups {
		if !containsUint32(group, id) {
			continue
		}
		for j, other := range factionBranchGroups {
			if i == j {
				continue
			}
			collisions = append(collisions, other...)
		}
	}
	seen := make(map[uint32]bool, len(collisions))
	out := collisions[:0]
	for _, c := range collisions {
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func containsUint32(xs []uint32, v uint32) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}
