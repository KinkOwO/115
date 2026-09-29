package npcpresence

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

type QuestProjection struct {
	ID           uint32
	Path, SHA256 string
	Blocks       []VisibilityBlock
	ShowOverride *int32
	CancelRevert bool
	Gaps         []string
}

func sourceSections(cells []pvf.Token, name string) [][]pvf.Token {
	var out [][]pvf.Token
	for i, c := range cells {
		if c.Type != 3 || c.Text != name {
			continue
		}
		end := i + 1
		for end < len(cells) && cells[end].Type != 3 {
			end++
		}
		out = append(out, cells[i+1:end])
	}
	return out
}

func sourceScalar(cells []pvf.Token, name string) (pvf.Token, bool) {
	r := sourceSections(cells, name)
	if len(r) != 1 || len(r[0]) != 1 {
		return pvf.Token{}, false
	}
	return r[0][0], true
}

// ProjectQuest projects source blocks, not an accepted-quest snapshot into
// native operations. Handler guards and native quest-ID rewrites must first
// establish which condition and script execute.
func ProjectQuest(d catalog.QuestDefinition) QuestProjection {
	p := QuestProjection{ID: d.ID, Path: d.Script.Path, SHA256: d.Script.SHA256, CancelRevert: true}
	for _, c := range d.Script.Cells {
		if c.Type == 3 && c.Text == "[npc visivility not revert]" {
			p.CancelRevert = false
		}
	}
	if len(sourceSections(d.Script.Cells, "[visible npc]")) != 0 {
		if v, ok := sourceScalar(d.Script.Cells, "[visible npc]"); ok && v.Type == 0 {
			n := v.Value
			p.ShowOverride = &n
		} else {
			p.Gaps = append(p.Gaps, "unresolved [visible npc] source")
		}
	}
	conditions := map[string]int32{"[accept]": 0, "[clear]": 1, "[clearable]": 2, "[clearing]": 3}
	for i := 0; i < len(d.Script.Cells); i++ {
		if d.Script.Cells[i].Type != 3 || d.Script.Cells[i].Text != "[npc visibility]" {
			continue
		}
		end := i + 1
		for end < len(d.Script.Cells) && !(d.Script.Cells[end].Type == 3 && d.Script.Cells[end].Text == "[/npc visibility]") {
			end++
		}
		if end == len(d.Script.Cells) {
			p.Gaps = append(p.Gaps, "unclosed NPC visibility source block")
			break
		}
		cells := d.Script.Cells[i+1 : end]
		b := VisibilityBlock{Revert: true}
		valid := true
		v, ok := sourceScalar(cells, "[condition]")
		cond, known := conditions[v.Text]
		valid = valid && ok && v.Type == 6 && known
		b.Condition = cond
		v, ok = sourceScalar(cells, "[visibility]")
		valid = valid && ok && v.Type == 6
		switch v.Text {
		case "[show]":
			b.Show = true
		case "[hide]":
		case "[delete]":
			b.Protect = true
		default:
			valid = false
		}
		if len(sourceSections(cells, "[revert]")) != 0 {
			v, ok = sourceScalar(cells, "[revert]")
			valid = valid && ok && v.Type == 6
			switch v.Text {
			case "[true]":
			case "[false]":
				b.Revert = false
			default:
				valid = false
			}
		}
		for _, c := range cells {
			if c.Type == 3 && c.Text != "[condition]" && c.Text != "[visibility]" && c.Text != "[npc]" && c.Text != "[/npc]" && c.Text != "[revert]" {
				valid = false
			}
		}
		npcSections := sourceSections(cells, "[npc]")
		if len(npcSections) != 1 {
			valid = false
		}
		for _, r := range npcSections {
			for _, c := range r {
				// Native1470A2910 only returns numeric kinds0/1/2 or
				// string kinds3/4. Kind7 is skipped by its cursor loop,
				// regardless of payload; it is not an NPC ID or a gate.
				if c.Type == 7 {
					continue
				}
				if c.Type != 0 || c.Value < 0 {
					valid = false
					continue
				}
				b.NPCs = append(b.NPCs, uint32(c.Value))
			}
		}
		if valid {
			p.Blocks = append(p.Blocks, b)
		} else {
			p.Gaps = append(p.Gaps, fmt.Sprintf("NPC visibility source block %d is unsupported", i))
		}
		i = end
	}
	return p
}

type Index struct {
	Source    string
	Areas     map[string]catalog.WorldArea
	Quests    map[uint32]QuestProjection
	TownRules map[uint32][]PhaseRule
	TownGaps  map[uint32][]string
}

func NewIndex(world catalog.WorldCatalog, quests catalog.QuestCatalog) (*Index, error) {
	if len(world.Source.Checksum) != 64 || world.Source.Checksum != quests.Source.Checksum {
		return nil, fmt.Errorf("NPC catalogs do not share a verified PVF source identity")
	}
	index := &Index{Source: world.Source.Checksum, Areas: world.Areas, Quests: make(map[uint32]QuestProjection), TownRules: make(map[uint32][]PhaseRule), TownGaps: make(map[uint32][]string)}
	bindings, err := catalog.ParseIndex(world.TownIndex.Cells)
	if err != nil {
		return nil, err
	}
	pathIDs := make(map[string][]uint32)
	for _, b := range bindings {
		pathIDs[normalPath(b.Path)] = append(pathIDs[normalPath(b.Path)], b.ID)
	}
	seen := make(map[uint32]bool)
	for _, town := range world.Towns {
		ids := pathIDs[normalPath(town.Path)]
		for _, id := range ids {
			if len(town.SHA256) != 64 {
				index.TownGaps[id] = append(index.TownGaps[id], "town source identity is missing")
			}
			if seen[id] || len(ids) != 1 {
				index.TownGaps[id] = append(index.TownGaps[id], "town source binding is ambiguous")
			}
			seen[id] = true
			rules, gaps := ProjectPhaseRules(town.Cells)
			index.TownRules[id] = append(index.TownRules[id], rules...)
			index.TownGaps[id] = append(index.TownGaps[id], gaps...)
		}
	}
	for id, d := range quests.Quests {
		p := ProjectQuest(d)
		if id != d.ID || d.Script.Path == "" || len(d.Script.SHA256) != 64 {
			p.Gaps = append(p.Gaps, "quest source identity is absent or inconsistent")
		}
		index.Quests[id] = p
	}
	return index, nil
}

func normalPath(s string) string { return strings.ToLower(strings.ReplaceAll(s, "\\", "/")) }

type Query struct {
	Town, Area, NPC     uint32
	Phase               *int32
	FinalRoot           string
	Instances           map[uint32]Truth
	Accepted, Completed QuestSet
	Visibility          *VisibilityReplay
}

// ResolveNPC is the single composition entry. Supplying only persistence
// snapshots leaves cache, final map and instance knowledge unknown. FinalRoot
// must identify this generation's actual selected resource, not a source
// candidate. A different observed resource stays outside the projected scope.
func (i *Index) ResolveNPC(q Query) Result {
	a, found := i.Areas[catalog.AreaKey(q.Town, q.Area)]
	if !found {
		return Result{NPC: q.NPC, State: StateUnknown, Reasons: []string{"source area is absent"}}
	}
	m := ProjectMap(a, q.Phase)
	if q.FinalRoot != "" && m.RootPath != "" && normalPath(q.FinalRoot) == normalPath(m.RootPath) {
		m.Selected = True
		m.Instances = q.Instances
		// Remove only the missing override-order input, not source syntax gaps.
		gaps := m.Gaps[:0]
		for _, g := range m.Gaps {
			if g != "final map override order is not supplied" {
				gaps = append(gaps, g)
			}
		}
		m.Gaps = gaps
	}
	return Resolve(q.NPC, m, q.Accepted, q.Completed, q.Visibility)
}

func (i *Index) ApplyCondition(r *VisibilityReplay, sourceQuest, nativeQuest uint32, condition int32, apply bool) error {
	p, ok := i.Quests[sourceQuest]
	if !ok || len(p.Gaps) != 0 || condition < 0 || condition > 2 {
		r.Forget()
		return fmt.Errorf("quest condition/native consumer is outside the closed projection scope")
	}
	r.ApplyCondition(nativeQuest, p.Blocks, condition, apply)
	return nil
}

type RootCandidate struct {
	Phase      int32             `json:"phase_index"`
	RootPath   string            `json:"root_path"`
	Placements []PlacementResult `json:"placements"`
	Gaps       []string          `json:"gaps"`
}

// Candidates explains source alternatives within one area, preserving root
// ownership. These alternatives are never passed as a selected-map union.
func (i *Index) Candidates(town, area, npc uint32, accepted, completed QuestSet) []RootCandidate {
	a, ok := i.Areas[catalog.AreaKey(town, area)]
	if !ok {
		return nil
	}
	phases := []int32{-1}
	for _, slot := range a.PhaseMaps {
		phases = append(phases, slot.Index)
	}
	var out []RootCandidate
	for _, phase := range phases {
		m := ProjectMap(a, &phase)
		c := RootCandidate{Phase: phase, RootPath: m.RootPath, Gaps: m.Gaps}
		for _, r := range m.Placements {
			if r.NPC == int32(npc) {
				c.Placements = append(c.Placements, PlacementResult{r, r.Gate(accepted, completed)})
			}
		}
		if len(c.Placements) != 0 {
			out = append(out, c)
		}
	}
	return out
}

type RuleReference struct {
	Quest        uint32            `json:"quest"`
	Path         string            `json:"source_path"`
	SHA256       string            `json:"source_sha256"`
	Blocks       []VisibilityBlock `json:"blocks,omitempty"`
	ShowOverride bool              `json:"show_override"`
	Gaps         []string          `json:"gaps,omitempty"`
}

func (i *Index) VisibilitySources(npc uint32) []RuleReference {
	var out []RuleReference
	for _, p := range i.Quests {
		ref := RuleReference{Quest: p.ID, Path: p.Path, SHA256: p.SHA256, Gaps: p.Gaps}
		ref.ShowOverride = p.ShowOverride != nil && *p.ShowOverride >= 0 && uint32(*p.ShowOverride) == npc
		for _, b := range p.Blocks {
			for _, n := range b.NPCs {
				if n == npc {
					ref.Blocks = append(ref.Blocks, b)
					break
				}
			}
		}
		if ref.ShowOverride || len(ref.Blocks) != 0 {
			out = append(out, ref)
		}
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Quest < out[b].Quest })
	return out
}
