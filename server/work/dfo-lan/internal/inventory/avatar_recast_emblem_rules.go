package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strings"
)

type avatarRecastAbility struct {
	Field string
	Job   string
	Skill int32
}

type avatarRecastEmblemPool struct {
	Match     string
	Templates []uint32
}

// Both avatar ability enums and enchant fields name the same native stats;
// only their delimiters/case differ, e.g. PHYSICAL_ATTACK / physical attack.
func avatarRecastField(s string) string {
	return strings.Map(func(c rune) rune {
		if c == '[' || c == ']' || c == '_' || c == ' ' {
			return -1
		}
		return c
	}, strings.ToLower(s))
}

// The caller first validates these rows with avatarRecastOptions.
func avatarRecastAbilities(cells []pvf.Token) map[uint16]avatarRecastAbility {
	out := map[uint16]avatarRecastAbility{}
	for i := 0; i+4 <= len(cells); {
		ability := avatarRecastAbility{Field: avatarRecastField(cells[i+1].Text)}
		size := 4
		if cells[i+1].Text == "[SKILL_LEVEL]" {
			if i+5 > len(cells) {
				break
			}
			ability.Job, ability.Skill, size = cells[i+2].Text, cells[i+3].Value, 5
		}
		out[uint16(cells[i].Value)] = ability
		i += size
	}
	return out
}

func avatarRecastEmblemAbilities(s catalog.ScriptRecord) map[avatarRecastAbility]bool {
	out := map[avatarRecastAbility]bool{}
	inEnchant, field := false, ""
	for i, t := range s.Cells {
		if t.Type == 3 {
			if t.Text == "[enchant]" {
				inEnchant = true
			}
			if t.Text == "[/enchant]" {
				inEnchant = false
			}
			field = avatarRecastField(t.Text)
			continue
		}
		if !inEnchant {
			continue
		}
		if field == "skilllevelup" {
			if t.Type == 6 && i+2 < len(s.Cells) && s.Cells[i+1].Type == 0 && s.Cells[i+2].Type == 0 && s.Cells[i+2].Value > 0 {
				out[avatarRecastAbility{Field: "skilllevel", Job: t.Text, Skill: s.Cells[i+1].Value}] = true
			}
		} else if (t.Type == 0 && t.Value > 0) || (t.Type == 4 && math.Float32frombits(uint32(t.Value)) > 0) {
			out[avatarRecastAbility{Field: field}] = true
		}
	}
	return out
}

func (r *AvatarRecastRules) prepareEmblems(a *pvf.Archive, index catalog.ItemIndex) error {
	disjoint, e := ImportAvatarDisjointRules(a, index)
	if e != nil {
		return e
	}
	r.Rolls = disjoint.Rolls
	properties := map[uint32]map[avatarRecastAbility]bool{}
	for _, pool := range disjoint.Pools {
		for _, id := range pool {
			if _, ok := properties[id]; ok {
				continue
			}
			s, e := catalog.ResolveScript(a, index.Items[id].Path)
			if e != nil {
				return e
			}
			properties[id] = avatarRecastEmblemAbilities(s)
		}
	}
	for job, parts := range r.Jobs {
		for kind, part := range parts {
			if len(r.Rolls[part.Grade]) == 0 {
				return fmt.Errorf("avatar recast grade has no source reward count")
			}
			part.Emblems = map[uint16]map[int32]avatarRecastEmblemPool{}
			for option, ability := range part.Abilities {
				pools := map[int32]avatarRecastEmblemPool{}
				for grade, sourcePool := range disjoint.Pools {
					pool := avatarRecastEmblemPool{Match: "attribute"}
					for _, id := range sourcePool {
						if properties[id][ability] {
							pool.Templates = append(pool.Templates, id)
						}
					}
					if len(pool.Templates) == 0 {
						pool.Match = "part"
						for _, id := range sourcePool {
							for _, candidate := range part.Abilities {
								if candidate.Job != "" && candidate.Job != "[all]" && candidate.Job != job {
									continue
								}
								if properties[id][candidate] {
									pool.Templates = append(pool.Templates, id)
									break
								}
							}
						}
					}
					if len(pool.Templates) > 0 {
						pools[grade] = pool
					}
				}
				part.Emblems[option] = pools
			}
			parts[kind] = part
		}
	}
	return nil
}
