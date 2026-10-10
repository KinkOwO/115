package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"strings"
)

type AvatarRecastPart struct {
	Grade     int32
	Template  uint32
	Options   map[uint16]string // empty for a stat; otherwise the skill's usable job
	Abilities map[uint16]avatarRecastAbility
	Emblems   map[uint16]map[int32]avatarRecastEmblemPool
}

type AvatarRecastRules struct {
	Source string
	Jobs   map[string]map[string]AvatarRecastPart
	Rolls  map[int32][][3]uint32
}

// Option IDs are explicit source keys, not row offsets. Current native
// 147184690/147189710 read four-cell stat rows and five-cell SKILL_LEVEL rows.
// AvatarAbilityOptions parses the explicit PVF ability keys shared by recasting
// and buying an avatar; it does not infer options from [avatar type select].
func AvatarAbilityOptions(cells []pvf.Token) (map[uint16]string, error) {
	return avatarRecastOptions(cells)
}

func avatarRecastOptions(cells []pvf.Token) (map[uint16]string, error) {
	options := map[uint16]string{}
	for i := 0; i < len(cells); {
		if i+4 > len(cells) || cells[i].Type != 0 || cells[i].Value < 0 || cells[i].Value >= 65535 || cells[i+1].Type != 6 {
			return nil, fmt.Errorf("invalid source avatar ability row")
		}
		id := uint16(cells[i].Value)
		job, size := "", 4
		if cells[i+1].Text == "[SKILL_LEVEL]" {
			size = 5
			if i+size > len(cells) || cells[i+2].Type != 6 || cells[i+3].Type != 0 || cells[i+4].Type != 0 {
				return nil, fmt.Errorf("invalid source avatar skill row")
			}
			job = cells[i+2].Text
		} else if cells[i+2].Type != 6 || (cells[i+3].Type != 0 && cells[i+3].Type != 4) {
			return nil, fmt.Errorf("invalid source avatar stat row")
		}
		if _, exists := options[id]; exists {
			return nil, fmt.Errorf("duplicate source avatar ability %d", id)
		}
		options[id] = job
		i += size
	}
	return options, nil
}

func ImportAvatarRecastRules(a *pvf.Archive, index catalog.ItemIndex) (*AvatarRecastRules, error) {
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return nil, fmt.Errorf("avatar recast source mismatch")
	}
	r := &AvatarRecastRules{Source: a.Snapshot().Checksum, Jobs: map[string]map[string]AvatarRecastPart{}}
	s, e := catalog.ResolveScript(a, "skill/abilitydatas.dat")
	if e != nil {
		return nil, e
	}
	cases := map[int32]map[uint16]string{}
	caseAbilities := map[int32]map[uint16]avatarRecastAbility{}
	for i, t := range s.Cells {
		if t.Type != 3 || t.Text != "[ability case]" {
			continue
		}
		if i+1 >= len(s.Cells) || s.Cells[i+1].Type != 0 {
			return nil, fmt.Errorf("invalid source ability case")
		}
		end := i + 2
		for end < len(s.Cells) && s.Cells[end].Type != 3 {
			end++
		}
		options, e := avatarRecastOptions(s.Cells[i+2 : end])
		if e != nil {
			return nil, e
		}
		id := s.Cells[i+1].Value
		if _, exists := cases[id]; exists {
			return nil, fmt.Errorf("duplicate source ability case")
		}
		cases[id] = options
		caseAbilities[id] = avatarRecastAbilities(s.Cells[i+2 : end])
	}
	s, e = catalog.ResolveScript(a, "etc/conversionavatar.etc")
	if e != nil {
		return nil, e
	}
	jobs, e := catalog.ParseIndex(s.Cells)
	if e != nil {
		return nil, e
	}
	for _, row := range jobs {
		s, e := catalog.ResolveScript(a, path.Join("etc", row.Path))
		if e != nil {
			return nil, e
		}
		job := avatarDisjointSection(s.Cells, "[character type]")
		grade := avatarDisjointSection(s.Cells, "[grade]")
		if len(job) != 1 || job[0].Type != 6 || len(grade) != 1 || grade[0].Type != 0 || grade[0].Value <= 0 {
			return nil, fmt.Errorf("invalid avatar conversion job/grade")
		}
		if _, exists := r.Jobs[job[0].Text]; exists {
			return nil, fmt.Errorf("duplicate avatar conversion job")
		}
		parts := map[string]AvatarRecastPart{}
		for i, t := range s.Cells {
			if t.Type != 3 || t.Text == "[grade]" || i+1 >= len(s.Cells) || s.Cells[i+1].Type != 0 || s.Cells[i+1].Value <= 0 {
				continue
			}
			if !strings.HasSuffix(t.Text, "]") || strings.HasPrefix(t.Text, "[/") {
				continue
			}
			// Native 1473679B0 keeps the first value at offset 296+part*4.
			// 1413F7350 uses precisely that template for the conversion preview.
			id := uint32(s.Cells[i+1].Value)
			item, ok := index.Items[id]
			if !ok || (item.Kind != "equipment" && item.Kind != "avatar") {
				return nil, fmt.Errorf("avatar conversion target %d missing", id)
			}
			defScript, e := catalog.ResolveScript(a, item.Path)
			if e != nil {
				return nil, e
			}
			d := equipmentDefinitionFromScript(id, defScript)
			k := d.Fields["[equipment type]"]
			g := d.Fields["[grade]"]
			// Conversion section names need not equal equipment kind names
			// (neck/belt in this table are breast/waist in equipment scripts).
			if !d.IsAvatar() || !d.IsCloneAvatar() || len(k) == 0 || len(g) != 1 || g[0].Value != grade[0].Value {
				return nil, fmt.Errorf("invalid source avatar conversion target %d", id)
			}
			kind := k[0].Text
			if _, exists := parts[kind]; exists {
				return nil, fmt.Errorf("duplicate avatar conversion equipment kind")
			}
			options, e := avatarRecastOptions(d.Fields["[avatar select ability]"])
			abilities := avatarRecastAbilities(d.Fields["[avatar select ability]"])
			if e != nil {
				return nil, e
			}
			if c := d.Fields["[ability case index]"]; len(c) > 0 {
				if len(c) != 1 || c[0].Type != 0 || len(cases[c[0].Value]) == 0 {
					return nil, fmt.Errorf("missing source avatar ability case")
				}
				options = cases[c[0].Value]
				abilities = caseAbilities[c[0].Value]
			}
			if len(options) == 0 {
				return nil, fmt.Errorf("avatar conversion target has no source options")
			}
			parts[kind] = AvatarRecastPart{Grade: grade[0].Value, Template: id, Options: options, Abilities: abilities}
		}
		if len(parts) == 0 {
			return nil, fmt.Errorf("empty source avatar conversion parts")
		}
		r.Jobs[job[0].Text] = parts
	}
	if len(r.Jobs) == 0 {
		return nil, fmt.Errorf("empty source avatar conversion rules")
	}
	if e := r.prepareEmblems(a, index); e != nil {
		return nil, e
	}
	return r, nil
}
