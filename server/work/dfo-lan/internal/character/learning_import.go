package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"path"
	"sort"
	"strings"
)

// skillType returns the token that declares the skill's own type.
//
// The current PVF repeats [type] inside [vp explain], [preset info] and the
// damage groups to label the default/vp1/vp2 variant a block belongs to, so a
// file's first [type] is regularly a variant label rather than a skill type.
// The skill's own type is the [type] followed by [skill class]; source files
// that omit [skill class] (every passive row) are recognised by the
// [active]/[passive] value itself. Both rules agree on every current file:
// scanning all 4890 skill/*.skl rows finds no disagreement.
func skillType(cells []pvf.Token) (pvf.Token, bool) {
	var byValue pvf.Token
	found := false
	for i := range cells {
		if cells[i].Type != 3 || cells[i].Text != "[type]" {
			continue
		}
		value := pvf.Token{}
		j := i + 1
		for ; j < len(cells) && cells[j].Type != 3; j++ {
			if value.Text == "" {
				value = cells[j]
			}
		}
		next := ""
		if j < len(cells) {
			next = cells[j].Text
		}
		if next == "[skill class]" {
			return value, true
		}
		if !found && (value.Text == "[active]" || value.Text == "[passive]") {
			byValue, found = value, true
		}
	}
	return byValue, found
}

// learnableSkillFields keeps the skill's own learning metadata.
//
// Description containers reuse learning tag names, so they are skipped by
// name rather than by stack order: the current source opens [vp explain] twice
// and closes it once in skill/priest/pandemoniumex.skl, which a strict stack
// would leave unclosed and would then drop every following field.
func LearnableSkillFields(cells []pvf.Token) map[string][]pvf.Token {
	fields := map[string][]pvf.Token{}
	if t, ok := skillType(cells); ok {
		fields["[type]"] = []pvf.Token{t}
	}
	open := map[string]bool{}
	tag := ""
	seen := map[string]bool{}
	enabled := false
	variation := false
	for _, t := range cells {
		if t.Type == 3 && t.Text == "[variation point]" {
			variation = true
			enabled = false
			continue
		}
		if variation {
			if t.Type == 3 && t.Text == "[/variation point]" {
				variation = false
			} else {
				fields["[variation point]"] = append(fields["[variation point]"], t)
			}
			continue
		}
		if t.Type == 3 {
			if strings.HasPrefix(t.Text, "[/") {
				delete(open, "["+t.Text[2:])
				enabled = false
				continue
			}
			tag = t.Text
			enabled = len(open) == 0 && !seen[tag] && tag != "[type]"
			if enabled {
				seen[tag] = true
			}
			if tag == "[vp explain]" || tag == "[explain group]" || tag == "[preset info]" {
				open[tag] = true
				enabled = false
			}
			continue
		}
		if !enabled {
			continue
		}
		switch tag {
		case "[awakening maximum level]", "[awakening]", "[enable by third awakening quest]":
			fields[tag] = append(fields[tag], t)
		case "[name]", "[required level]", "[required level range]", "[maximum level]", "[growtype maximum level]", "[skill fitness growtype]", "[skill fitness second growtype]", "[pre required skill]", "[purchase cost]", "[special purchase cost]", "[feature skill type]", "[fixed level skill]", "[interval level]", "[add level per interval]", "[skill class]":
			fields[tag] = append(fields[tag], t)
		}
	}
	return fields
}

// ImportLearningCatalog uses the profession's own skill list, preserving its
// source order. Unreadable or duplicate skills fail before player storage opens.
func ImportLearningCatalog(a *pvf.Archive, c catalog.Characters) (*LearningCatalog, error) {
	if a == nil || a.Snapshot().Checksum != c.Source.Checksum {
		return nil, fmt.Errorf("learning/PVF source mismatch")
	}
	direct := LearningCatalog{Source: a.Snapshot()}
	jobs := make([]int, 0, len(c.Professions))
	for job := range c.Professions {
		jobs = append(jobs, int(job))
	}
	sort.Ints(jobs)
	for _, job := range jobs {
		p := c.Professions[byte(job)]
		stem := strings.TrimSuffix(path.Base(p.Path), ".chr")
		idx, e := catalog.ResolveScript(a, "skill/"+stem+"skill.lst")
		if e != nil {
			idx, e = catalog.ResolveScript(a, "skill/"+stem+".lst")
		}
		if e != nil {
			return nil, e
		}
		refs, e := catalog.ParseIndex(idx.Cells)
		if e != nil {
			return nil, e
		}
		for _, ref := range refs {
			if ref.ID == 0 || ref.ID > math.MaxUint16 {
				return nil, fmt.Errorf("job %d invalid skill %d", job, ref.ID)
			}
			name := ref.Path
			if !strings.HasPrefix(name, "skill/") {
				name = "skill/" + name
			}
			script, e := catalog.ResolveScript(a, name)
			if e != nil {
				return nil, fmt.Errorf("job %d skill %d: %w", job, ref.ID, e)
			}
			direct.Rows = append(direct.Rows, LearningDefinition{Job: byte(job), ID: uint16(ref.ID), Path: script.Path, SHA256: script.SHA256, Fields: LearnableSkillFields(script.Cells)})
		}
		a.ReleaseReadCaches()
	}
	return newLearningCatalog(direct, c.Source.Checksum)
}
