package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Profession struct {
	ID                byte                   `json:"id"`
	Path              string                 `json:"path"`
	RawSHA256         string                 `json:"raw_sha256"`
	Job               string                 `json:"job"`
	InitialAttributes map[string]float32     `json:"initial_attributes"`
	BaseGrowth        map[string]float32     `json:"base_growth,omitempty"`
	InitialSections   map[string][]pvf.Token `json:"initial_sections"`
	InitialSkills     []int32                `json:"initial_skill_cells"`
	InitialSkillSlots map[uint16]uint16      `json:"initial_skill_slots,omitempty"`
	CreateEquipment   []pvf.Token            `json:"create_equipment_cells"`
	DefaultAppearance []int32                `json:"default_appearance_indices,omitempty"`
}
type Characters struct {
	Source      pvf.ArchiveSnapshot `json:"source"`
	Professions map[byte]Profession `json:"professions"`
}

func ImportCharacters(a *pvf.Archive) (Characters, error) {
	result := Characters{Source: a.Snapshot(), Professions: map[byte]Profession{}}
	list, err := a.Tokens("list/character.lst")
	if err != nil {
		return result, err
	}
	if len(list)%2 != 0 {
		return result, fmt.Errorf("invalid profession index")
	}
	for i := 0; i < len(list); i += 2 {
		if list[i].Type != 0 || list[i].Value < 0 || list[i].Value > 255 || list[i+1].Type != 6 {
			return result, fmt.Errorf("invalid profession pair %d", i/2)
		}
		id := byte(list[i].Value)
		path := strings.ToLower(strings.ReplaceAll(list[i+1].Text, "\\", "/"))
		if _, ok := result.Professions[id]; ok {
			return result, fmt.Errorf("duplicate profession %d", id)
		}
		ts, e := a.Tokens(path)
		if e != nil {
			return result, e
		}
		raw, e := a.ReadRaw(path)
		if e != nil {
			return result, e
		}
		p := Profession{ID: id, Path: path, RawSHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), InitialAttributes: map[string]float32{}, InitialSections: map[string][]pvf.Token{}}
		initial := false
		section := ""
		for _, t := range ts {
			if t.Type == 3 {
				section = strings.ToLower(t.Text)
				if section == "[initial value]" {
					initial = true
				}
				if strings.HasPrefix(section, "[growtype ") {
					initial = false
				}
				continue
			}
			if section == "[job]" && t.Type == 6 {
				p.Job = t.Text
			}
			if section == "[create equipment list]" {
				p.CreateEquipment = append(p.CreateEquipment, t)
			}
			if !initial {
				continue
			}
			if section == "[skill]" {
				if t.Type != 0 {
					return result, fmt.Errorf("unsupported initial skill cell in %s", path)
				}
				p.InitialSkills = append(p.InitialSkills, t.Value)
				continue
			}
			if strings.HasPrefix(section, "[/") {
				continue
			}
			p.InitialSections[section] = append(p.InitialSections[section], t)
		}
		for key, cells := range p.InitialSections {
			if len(cells) == 1 {
				switch cells[0].Type {
				case 0:
					p.InitialAttributes[key] = float32(cells[0].Value)
				case 2:
					p.InitialAttributes[key] = cells[0].Number
				}
			}
		}
		if p.Job == "" || p.InitialAttributes["[hp max]"] <= 0 || p.InitialAttributes["[mp max]"] <= 0 || len(p.InitialSkills)%3 != 0 {
			return result, fmt.Errorf("incomplete initial configuration for %d", id)
		}
		// Unadvanced profession growth is its own source block. Never pull
		// advanced/awakened skills or attributes into the initial profession.
		p.BaseGrowth = map[string]float32{}
		growth := false
		for _, t := range ts {
			if t.Type == 3 {
				section = strings.ToLower(t.Text)
				if strings.HasPrefix(section, "[growtype ") {
					growth = section == "[growtype 1]"
				}
				continue
			}
			if _, known := p.InitialAttributes[section]; !growth || !known {
				continue
			}
			if _, seen := p.BaseGrowth[section]; seen {
				return result, fmt.Errorf("duplicate base growth %s in %s", section, path)
			}
			switch t.Type {
			case 0:
				p.BaseGrowth[section] = float32(t.Value)
			case 2:
				p.BaseGrowth[section] = t.Number
			default:
				return result, fmt.Errorf("unsupported base growth cell in %s", path)
			}
		}
		// Skill availability comes from .chr. A small local hotbar policy puts
		// source-active initial skills in source order; this is not a claim
		// that this ordering reproduces the official default key layout.
		indexPath := "skill/" + strings.TrimSuffix(filepath.Base(path), ".chr") + "skill.lst"
		if _, found := a.FindFile(indexPath); !found {
			// The supplied Knight source uses skill/knight.lst, while older
			// professions use the *skill.lst naming convention. Resolve an
			// existing source file before binding any source initial skill.
			indexPath = "skill/" + strings.TrimSuffix(filepath.Base(path), ".chr") + ".lst"
			if _, found = a.FindFile(indexPath); !found {
				return result, fmt.Errorf("no source skill index for profession %d", id)
			}
		} // skills still learned; no guessed shortcut source
		skillIndex, e := ReadScript(a, indexPath)
		if e != nil {
			return result, e
		}
		rows, e := ParseIndex(skillIndex.Cells)
		if e != nil {
			return result, e
		}
		skillPaths := map[uint32]string{}
		for _, row := range rows {
			skillPaths[row.ID] = row.Path
		}
		p.InitialSkillSlots = map[uint16]uint16{}
		for j := 0; j < len(p.InitialSkills); j += 3 {
			name, ok := skillPaths[uint32(p.InitialSkills[j])]
			if !ok {
				return result, fmt.Errorf("missing initial skill %d", p.InitialSkills[j])
			}
			script, e := ResolveScript(a, "skill/"+name)
			if e != nil {
				return result, e
			}
			if initialShortcutEligible(script.Cells) && len(p.InitialSkillSlots) < 6 {
				p.InitialSkillSlots[uint16(p.InitialSkills[j])] = uint16(len(p.InitialSkillSlots))
			}
		}
		result.Professions[id] = p
	}
	appearance, e := a.Tokens("character/characterinfo.etc")
	if e != nil {
		return result, e
	}
	var row []int32
	active := false
	for _, t := range appearance {
		if t.Type == 3 {
			if t.Text == "[/default equipment index]" && active && len(row) > 0 {
				p, ok := result.Professions[byte(row[0])]
				if ok {
					p.DefaultAppearance = append([]int32{}, row[1:]...)
					result.Professions[p.ID] = p
				}
			}
			active = t.Text == "[default equipment index]"
			row = nil
			continue
		}
		if active {
			if t.Type != 0 {
				return result, fmt.Errorf("non-numeric appearance index")
			}
			row = append(row, t.Value)
		}
	}
	return result, nil
}

// Skill metadata precedes its nested dungeon/PvP damage sections. Repeated
// [type] inside damage groups describes damage, not the skill's active type.
// Initial shortcuts use command-capable skills; utility actions with an
// explicitly disabled command editor remain learned and use native controls.
// This is a local initial layout, not a claim about official saved hotkeys.
func initialShortcutEligible(cells []pvf.Token) bool {
	first := func(name string) []pvf.Token {
		for i, c := range cells {
			if c.Type != 3 || c.Text != name {
				continue
			}
			end := i + 1
			for end < len(cells) && cells[end].Type != 3 {
				end++
			}
			return cells[i+1 : end]
		}
		return nil
	}
	kind, custom := first("[type]"), first("[command customizing]")
	return len(kind) == 1 && kind[0].Text == "[active]" &&
		len(first("[command]")) > 0 &&
		!(len(custom) == 1 && custom[0].Type == 0 && custom[0].Value == 0)
}
func LoadCharacters(path string) (Characters, error) {
	var c Characters
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	e = json.Unmarshal(b, &c)
	if e != nil {
		return c, e
	}
	if len(c.Professions) == 0 || len(c.Source.Checksum) != 64 {
		return c, fmt.Errorf("invalid character catalog")
	}
	for id, p := range c.Professions {
		if id != p.ID || p.InitialAttributes["[hp max]"] <= 0 || p.RawSHA256 == "" {
			return c, fmt.Errorf("invalid profession %d", id)
		}
	}
	return c, nil
}
