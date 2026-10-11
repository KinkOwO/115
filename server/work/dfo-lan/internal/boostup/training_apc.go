package boostup

import (
	"dfolan/internal/catalog"
	"fmt"
	"strings"
)

const TrainingAPCPath = "etc/mycharactersapc/mycharacters_special_apc.etc"

type TrainingAPC struct {
	// SourceIndex is [index] in the special APC table, used by N1754.
	SourceIndex uint32
	Template    uint32
	Path        string
}

// BindTrainingAPCs follows the native special-APC event/content binding. The
// client reads the NPC's own AIC equipment, skills and AI; this is not a saved
// account character, and no invented actor, gear or database row is required.
// This adapter is deliberately limited to the source's passive training target
// (warlike=0, no quick skills). It does not invent a 30-skill combat preset or
// truncate the source NPC's learned skills to fit the account preference UI.
func (c *Catalog) BindTrainingAPCs(src TokenSource) error {
	cells, e := src.Tokens(TrainingAPCPath)
	if e != nil {
		return e
	}
	var selected []TrainingAPC
	seen := map[uint32]bool{}
	seenTemplates := map[uint32]bool{}
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 || cells[i].Text != "[character]" {
			continue
		}
		end := i + 1
		for end < len(cells) && (cells[end].Type != 3 || cells[end].Text != "[/character]") {
			end++
		}
		if end == len(cells) {
			return fmt.Errorf("unterminated special APC")
		}
		row := cells[i+1 : end]
		typ := values(row, "[type]")
		if len(typ) == 2 && typ[0].Text == "event" && typ[1].Type == 0 && uint32(typ[1].Value) == EventID {
			contents := values(row, "[usable contents]")
			allowed := false
			for _, v := range contents {
				if v.Text == "boost up event" {
					allowed = true
				}
			}
			if !allowed {
				return fmt.Errorf("event APC is not allowed in boost training")
			}
			sourceIndex, e := number(row, "[index]", 1<<31-1)
			if e != nil || sourceIndex == 0 || seen[sourceIndex] {
				return fmt.Errorf("invalid event special APC index")
			}
			id, e := number(row, "[apc index]", 1<<31-1)
			if e != nil || id == 0 || seenTemplates[id] {
				return fmt.Errorf("invalid event APC template")
			}
			seen[sourceIndex] = true
			seenTemplates[id] = true
			selected = append(selected, TrainingAPC{SourceIndex: sourceIndex, Template: id})
		}
		i = end
	}
	if len(selected) == 0 || len(selected) > 3 {
		return fmt.Errorf("invalid boost training APC count %d", len(selected))
	}
	list, e := src.Tokens("list/aicharacter.lst")
	if e != nil {
		return e
	}
	index, e := catalog.ParseIndex(list)
	if e != nil {
		return e
	}
	paths := map[uint32]string{}
	for _, r := range index {
		paths[r.ID] = r.Path
	}
	var out []TrainingAPC
	for _, entry := range selected {
		id := entry.Template
		path := paths[id]
		if !strings.HasSuffix(path, ".aic") {
			return fmt.Errorf("training APC %d has no AIC source", id)
		}
		aic, e := src.Tokens(path)
		if e != nil {
			return e
		}
		war, e := number(aic, "[warlike]", 1<<31-1)
		if e != nil || war != 0 || len(values(aic, "[quick skill]")) != 0 {
			return fmt.Errorf("training APC %d requires an unverified combat skill adapter", id)
		}
		entry.Path = path
		out = append(out, entry)
	}
	c.TrainingAPCs = out
	return nil
}
