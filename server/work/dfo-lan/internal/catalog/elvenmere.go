package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
	"strings"
)

const ElvenmereRulesPath = "contents/2022/elvenmere/etc/elvenmere.cos"

type ElvenmereItemReward struct{ Template, Count uint32 }
type ElvenmereFloorRule struct {
	Weekly, Season ElvenmereItemReward
	Experience     uint64
}
type ElvenmereRules struct {
	Areas        []ElvenmereArea
	Path, SHA256 string
	MaxFloor     byte
	Floors       map[byte]ElvenmereFloorRule
}

func ImportElvenmereRules(a *pvf.Archive) (*ElvenmereRules, error) {
	text, err := a.ReadText(ElvenmereRulesPath)
	if err != nil {
		return nil, err
	}
	raw, err := a.ReadRaw(ElvenmereRulesPath)
	if err != nil {
		return nil, err
	}
	r, err := parseElvenmereRules(text)
	if err != nil {
		return nil, err
	}
	if len(r.Areas) == 0 {
		return nil, fmt.Errorf("elvenmere source area routing missing")
	}
	r.Path, r.SHA256 = ElvenmereRulesPath, fmt.Sprintf("%x", sha256.Sum256(raw))
	return r, nil
}

// Read the verified numeric source sections; unrelated presentation and map
// sections are outside this reward adapter. Missing/duplicate rows fail closed.
func parseElvenmereRules(text string) (*ElvenmereRules, error) {
	sections := map[string][]string{}
	seen := map[string]bool{}
	active := ""
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") {
			end := strings.IndexByte(line, ']')
			if end < 0 {
				return nil, fmt.Errorf("elvenmere malformed section")
			}
			name := line[1:end]
			if strings.HasPrefix(name, "/") {
				if active != "" && name != "/"+active {
					return nil, fmt.Errorf("elvenmere mismatched section")
				}
				active = ""
				continue
			}
			if active != "" {
				return nil, fmt.Errorf("elvenmere unclosed %s", active)
			}
			if name != "max floor" && name != "weekly reward" && name != "season reward" && name != "floor clear exp" {
				continue
			}
			if seen[name] {
				return nil, fmt.Errorf("elvenmere duplicate %s", name)
			}
			seen[name] = true
			sections[name] = strings.Fields(line[end+1:])
			if name != "max floor" {
				active = name
			}
		} else if active != "" {
			sections[active] = append(sections[active], strings.Fields(line)...)
		}
	}
	if active != "" || len(sections["max floor"]) != 1 {
		return nil, fmt.Errorf("elvenmere missing max floor or close")
	}
	max, err := strconv.ParseUint(sections["max floor"][0], 10, 8)
	// The existing native notification has two fixed 100-byte floor arrays.
	if err != nil || max == 0 || max > 100 {
		return nil, fmt.Errorf("elvenmere unsupported max floor")
	}
	r := &ElvenmereRules{MaxFloor: byte(max), Floors: map[byte]ElvenmereFloorRule{}}
	for name, width := range map[string]int{"weekly reward": 3, "season reward": 3, "floor clear exp": 2} {
		cells := sections[name]
		if !seen[name] || len(cells)%width != 0 {
			return nil, fmt.Errorf("elvenmere malformed %s", name)
		}
		rows := map[byte]bool{}
		for i := 0; i < len(cells); i += width {
			floor, e := strconv.ParseUint(cells[i], 10, 8)
			if e != nil || floor == 0 || floor > max || rows[byte(floor)] {
				return nil, fmt.Errorf("elvenmere invalid %s floor", name)
			}
			rows[byte(floor)] = true
			v, e := strconv.ParseUint(cells[i+1], 10, 32)
			if e != nil {
				return nil, fmt.Errorf("elvenmere invalid %s value", name)
			}
			row := r.Floors[byte(floor)]
			if width == 2 {
				row.Experience = v
			} else {
				count, e := strconv.ParseUint(cells[i+2], 10, 32)
				if e != nil || v == 0 || count == 0 {
					return nil, fmt.Errorf("elvenmere invalid %s reward", name)
				}
				reward := ElvenmereItemReward{uint32(v), uint32(count)}
				if name == "weekly reward" {
					row.Weekly = reward
				} else {
					row.Season = reward
				}
			}
			r.Floors[byte(floor)] = row
		}
		if name != "season reward" && len(rows) != int(max) {
			return nil, fmt.Errorf("elvenmere incomplete %s", name)
		}
	}
	r.Areas, err = parseElvenmereAreas(text, r.MaxFloor)
	return r, err
}
