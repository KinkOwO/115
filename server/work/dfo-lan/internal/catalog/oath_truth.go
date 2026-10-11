package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

const OathTruthPath = "etc/115lvability2/oathsystemscript.cos"

type TruthCost struct{ Template, Amount uint32 }
type TruthOption struct {
	AbilityType                              int
	Ability, Buff, SpecialDamage, CombatRate float64
}
type TruthStage struct {
	Options []TruthOption
	Costs   map[int][]TruthCost
}
type OathTruthRules struct {
	Source, RawSHA256 string
	MaxLevel          int
	Stages            map[int]TruthStage
	Alternatives      map[uint32][]uint32
	MaterialSources   map[string]string
}

func ImportOathTruth(a *pvf.Archive, index ItemIndex) (*OathTruthRules, error) {
	text, err := a.ReadText(OathTruthPath)
	if err != nil {
		return nil, err
	}
	r, err := ParseOathTruth(text)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", OathTruthPath, err)
	}
	raw, err := a.ReadRaw(OathTruthPath)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	r.RawSHA256 = hex.EncodeToString(sum[:])
	r.Source = a.Snapshot().Checksum
	if index.Source.Checksum != r.Source {
		return nil, fmt.Errorf("truth material index source mismatch")
	}
	r.Alternatives = map[uint32][]uint32{}
	r.MaterialSources = map[string]string{}
	for _, stage := range r.Stages {
		for _, costs := range stage.Costs {
			for _, cost := range costs {
				if cost.Template == 0 {
					continue
				}
				if _, seen := r.Alternatives[cost.Template]; seen {
					continue
				}
				item, ok := index.Items[cost.Template]
				if !ok || item.Kind != "stackable" {
					return nil, fmt.Errorf("truth material missing from source index")
				}
				script, err := ResolveScript(a, item.Path)
				if err != nil {
					return nil, err
				}
				r.MaterialSources[script.Path] = script.SHA256
				r.Alternatives[cost.Template] = []uint32{}
				for _, token := range sectionCells(script.Cells, "[alternative materials]") {
					if token.Type != 0 || token.Value <= 0 {
						return nil, fmt.Errorf("invalid truth alternative material")
					}
					id := uint32(token.Value)
					if alt, ok := index.Items[id]; !ok || alt.Kind != "stackable" {
						return nil, fmt.Errorf("truth alternative missing from source index")
					}
					r.Alternatives[cost.Template] = append(r.Alternatives[cost.Template], id)
				}
			}
		}
	}
	return r, nil
}

// Only the source's reinforce section is consumed. Slots beyond the source's
// available maximum remain locked even though the UI has ten option bytes.
func ParseOathTruth(text string) (*OathTruthRules, error) {
	if !strings.Contains(text, "[/oath reinforce]") {
		return nil, fmt.Errorf("truth section is not closed")
	}
	r := &OathTruthRules{Stages: map[int]TruthStage{}}
	index, group := -1, 0
	var stage TruthStage
	var option *TruthOption
	for _, line := range sectionLines(strings.TrimRight(text, "\x00"), "[oath reinforce]", "[/oath reinforce]") {
		if line == "" {
			continue
		}
		switch line {
		case "[level slot]":
			index = -1
			stage = TruthStage{Costs: map[int][]TruthCost{}}
		case "[/level slot]":
			if index < 0 {
				return nil, fmt.Errorf("truth slot missing index")
			}
			if _, ok := r.Stages[index]; ok {
				return nil, fmt.Errorf("duplicate truth slot")
			}
			r.Stages[index] = stage
		case "[option slot]":
			stage.Options = append(stage.Options, TruthOption{})
			option = &stage.Options[len(stage.Options)-1]
		case "[/option slot]":
			option = nil
		case "[/group]":
			group = 0
		default:
			if strings.HasPrefix(line, "[") {
				end := strings.IndexByte(line, ']')
				if end < 0 {
					return nil, fmt.Errorf("invalid truth tag")
				}
				tag, value := line[:end+1], strings.TrimSpace(line[end+1:])
				switch tag {
				case "[max level available]", "[index]", "[group]":
					n, err := strconv.Atoi(value)
					if err != nil {
						return nil, err
					}
					switch tag {
					case "[max level available]":
						r.MaxLevel = n
					case "[index]":
						index = n
					case "[group]":
						if n < 1 || group != 0 || len(stage.Costs[n]) != 0 {
							return nil, fmt.Errorf("invalid truth cost group")
						}
						group = n
					}
				case "[ability type]", "[ability value]", "[buff value]", "[special object damage rate]", "[combat rate]":
					if option == nil {
						return nil, fmt.Errorf("truth value outside option")
					}
					v, err := strconv.ParseFloat(value, 64)
					if err != nil || v < 0 || v > 1000000 {
						return nil, fmt.Errorf("invalid truth value")
					}
					switch tag {
					case "[ability type]":
						if v != 1 && v != 2 {
							return nil, fmt.Errorf("unsupported truth ability")
						}
						option.AbilityType = int(v)
					case "[ability value]":
						option.Ability = v
					case "[buff value]":
						option.Buff = v
					case "[special object damage rate]":
						option.SpecialDamage = v
					case "[combat rate]":
						option.CombatRate = v
					}
				}
			} else if group != 0 {
				fields := strings.Fields(line)
				if len(fields) != 2 {
					return nil, fmt.Errorf("invalid truth cost row")
				}
				id, e1 := strconv.ParseUint(fields[0], 10, 32)
				amount, e2 := strconv.ParseUint(fields[1], 10, 32)
				if e1 != nil || e2 != nil || amount == 0 {
					return nil, fmt.Errorf("invalid truth cost")
				}
				for _, prior := range stage.Costs[group] {
					if prior.Template == uint32(id) {
						return nil, fmt.Errorf("duplicate truth material")
					}
				}
				stage.Costs[group] = append(stage.Costs[group], TruthCost{uint32(id), uint32(amount)})
			}
		}
	}
	if r.MaxLevel < 1 || r.MaxLevel > 9 {
		return nil, fmt.Errorf("invalid truth maximum")
	}
	for n := 1; n <= r.MaxLevel; n++ {
		s, ok := r.Stages[n]
		if !ok || len(s.Options) < 1 || len(s.Options) > 256 || len(s.Costs) == 0 {
			return nil, fmt.Errorf("incomplete truth stage %d", n)
		}
		for i := 1; i <= len(s.Costs); i++ {
			if len(s.Costs[i]) == 0 {
				return nil, fmt.Errorf("non-contiguous truth payment groups")
			}
		}
		for _, o := range s.Options {
			if o.AbilityType == 0 {
				return nil, fmt.Errorf("truth option missing ability")
			}
		}
	}
	return r, nil
}
