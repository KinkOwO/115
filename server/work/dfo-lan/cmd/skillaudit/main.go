// skillaudit projects source learning metadata without inventing level/cost rules.
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
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
func learnableSkillFields(cells []pvf.Token) map[string][]pvf.Token {
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

func main() {
	src := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only source")
	out := flag.String("output", "runtime/skill_learning_audit.json", "metadata audit")
	characterFile := flag.String("characters", "configs/characters.next25.json", "profession catalog")
	flag.Parse()
	a, e := pvf.LoadArchive(pvf.Options{Path: *src, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	c, e := catalog.LoadCharacters(*characterFile)
	if e != nil {
		log.Fatal(e)
	}
	type row struct {
		Job          byte
		ID           uint32
		Path, SHA256 string
		Fields       map[string][]pvf.Token
	}
	var result []row
	counts := map[string]int{}
	for job, p := range c.Professions {
		stem := strings.TrimSuffix(path.Base(p.Path), ".chr")
		idx, e := catalog.ResolveScript(a, "skill/"+stem+"skill.lst")
		if e != nil {
			idx, e = catalog.ResolveScript(a, "skill/"+stem+".lst")
		}
		if e != nil {
			log.Fatal(e)
		}
		refs, e := catalog.ParseIndex(idx.Cells)
		if e != nil {
			log.Fatal(e)
		}
		for _, ref := range refs {
			name := ref.Path
			if !strings.HasPrefix(name, "skill/") {
				name = "skill/" + name
			}
			s, e := catalog.ResolveScript(a, name)
			if e != nil {
				counts["unreadable"]++
				continue
			}
			fields := learnableSkillFields(s.Cells)
			result = append(result, row{job, ref.ID, s.Path, s.SHA256, fields})
		}
	}
	b, e := json.MarshalIndent(map[string]any{"source": a.Snapshot(), "rows": result, "counts": counts}, "", "  ")
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("skill metadata=%d counts=%v", len(result), counts)
}
