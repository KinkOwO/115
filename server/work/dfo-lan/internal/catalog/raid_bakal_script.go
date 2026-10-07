package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type BakalScriptInstruction struct {
	Op   string
	Args []pvf.Token
}
type BakalScriptEvent struct{ Trigger, Behavior []BakalScriptInstruction }
type BakalScriptLocation struct {
	ID, Dungeon        uint32
	Kind               string
	Grid, SpecificGrid [2]byte
	HasSpecificGrid    bool
	Movable            []uint32
	Creatable          []string
}

// The schema describes script syntax, never content IDs, values or routes.
var bakalInstructionSchema = map[string]string{
	"[IF]": "condition", "[ON ENTER DUNGEON]": "n", "[ON GIVEUP DUNGEON]": "n",
	"[ON CHANGE SYMBOL]": "s", "[ON CHANGE DUNGEON STATE]": "n", "[CHECK DUNGEON STATE]": "nt", "[CHECK TIMER END]": "nn",
	"[SET]": "assignment", "[SET TIMER]": "nnn", "[CREATE MONSTER]": "nt", "[RESERVE CREATE MONSTER]": "nt", "[RESERVE DELETE MONSTER]": "nt",
	"[ADD BAKAL RAID BUFF]": "nn", "[AWAKE RANDOM DRAGON]": "n", "[INCREASE BAKAL ANGER]": "nn",
	"[DELETE MONSTER]": "n", "[MOVE MONSTER]": "nn", "[CREATE RANDOM MONSTER]": "ntntn", "[SUB MAX]": "ssss",
	"[SET DUNGEON STATE]": "nt", "[KICK OUT DUNGEON NO PENALTY]": "n", "[MOVE LAST BAKAL DUNGEON]": "", "[CLEAR PHASE]": "", "[FAIL PHASE]": "",
}

func bakalInstructions(ts []pvf.Token) ([]BakalScriptInstruction, error) {
	var out []BakalScriptInstruction
	for i := 0; i < len(ts); {
		op := ts[i].Text
		schema, ok := bakalInstructionSchema[op]
		if !ok || ts[i].Type != 3 {
			return nil, fmt.Errorf("unsupported Bakal instruction %q", op)
		}
		i++
		ins := BakalScriptInstruction{Op: op}
		if schema == "condition" || schema == "assignment" {
			if i >= len(ts) || ts[i].Type != 3 {
				return nil, fmt.Errorf("%s missing symbol", op)
			}
			ins.Args = append(ins.Args, ts[i])
			i++
			if i < len(ts) && ts[i].Type == 3 {
				cmp := ts[i].Text
				if schema == "assignment" && cmp != "[-]" {
					return nil, fmt.Errorf("unsupported source assignment %s", cmp)
				}
				if schema == "condition" && cmp != "[>]" && cmp != "[<]" && cmp != "[=>]" && cmp != "[=<]" && cmp != "[!=]" {
					return nil, fmt.Errorf("unsupported source comparison %s", cmp)
				}
				ins.Args = append(ins.Args, ts[i])
				i++
			}
			schema = "n"
		}
		for _, kind := range schema {
			if i >= len(ts) {
				return nil, fmt.Errorf("%s truncated source operands", op)
			}
			t := ts[i]
			if kind == 'n' && t.Type != 0 || kind == 't' && (t.Type != 6 || t.Text == "") || kind == 's' && t.Type != 3 {
				return nil, fmt.Errorf("%s invalid source operand", op)
			}
			ins.Args = append(ins.Args, t)
			i++
		}
		out = append(out, ins)
	}
	return out, nil
}

func parseBakalScript(ts []pvf.Token) ([]BakalScriptEvent, map[uint32]BakalScriptLocation, error) {
	var events []BakalScriptEvent
	locations := map[uint32]BakalScriptLocation{}
	phase := false
	for i := 0; i < len(ts); i++ {
		if ts[i].Text == "[RAID PHASE OF HARDMODE]" {
			break
		}
		switch ts[i].Text {
		case "[PHASE]":
			phase = true
		case "[/PHASE]":
			phase = false
		case "[TRIGGER]":
			if !phase {
				return nil, nil, fmt.Errorf("Bakal trigger outside phase")
			}
			start := i + 1
			for i++; i < len(ts) && ts[i].Text != "[/TRIGGER]"; i++ {
			}
			if i == len(ts) {
				return nil, nil, fmt.Errorf("unclosed source trigger")
			}
			t, err := bakalInstructions(ts[start:i])
			if err != nil {
				return nil, nil, err
			}
			if i+1 >= len(ts) || ts[i+1].Text != "[BEHAVIOR]" {
				return nil, nil, fmt.Errorf("source trigger without behavior")
			}
			start = i + 2
			for i = start; i < len(ts) && ts[i].Text != "[/BEHAVIOR]"; i++ {
			}
			if i == len(ts) {
				return nil, nil, fmt.Errorf("unclosed source behavior")
			}
			b, err := bakalInstructions(ts[start:i])
			if err != nil {
				return nil, nil, err
			}
			events = append(events, BakalScriptEvent{t, b})
		case "[LOCATION INFO]":
			loc := BakalScriptLocation{}
			for i++; i < len(ts) && ts[i].Text != "[/LOCATION INFO]"; i++ {
				op := ts[i].Text
				switch op {
				case "[LOCATION INDEX]", "[DUNGEON INDEX]":
					if i+1 >= len(ts) || ts[i+1].Type != 0 || ts[i+1].Value <= 0 {
						return nil, nil, fmt.Errorf("invalid source location index")
					}
					if op == "[LOCATION INDEX]" {
						loc.ID = uint32(ts[i+1].Value)
					} else {
						loc.Dungeon = uint32(ts[i+1].Value)
					}
					i++
				case "[LOCATION XY]", "[LOCATION SPECIFIC XY]":
					if i+2 >= len(ts) || ts[i+1].Type != 0 || ts[i+2].Type != 0 || ts[i+1].Value < 0 || ts[i+1].Value > 255 || ts[i+2].Value < 0 || ts[i+2].Value > 255 {
						return nil, nil, fmt.Errorf("invalid source location grid")
					}
					grid := [2]byte{byte(ts[i+1].Value), byte(ts[i+2].Value)}
					if op == "[LOCATION XY]" {
						loc.Grid = grid
					} else {
						loc.SpecificGrid = grid
						loc.HasSpecificGrid = true
					}
					i += 2
				case "[LOCATION TYPE]":
					if i+1 >= len(ts) || ts[i+1].Type != 6 {
						return nil, nil, fmt.Errorf("missing source location kind")
					}
					loc.Kind = ts[i+1].Text
					i++
				case "[MOVABLE LOCATION]", "[CREATABLE MONSTER]":
					end := "[/MOVABLE LOCATION]"
					if op == "[CREATABLE MONSTER]" {
						end = "[/CREATABLE MONSTER]"
					}
					for i++; i < len(ts) && ts[i].Text != end; i++ {
						if op == "[MOVABLE LOCATION]" && ts[i].Type == 0 && ts[i].Value > 0 {
							loc.Movable = append(loc.Movable, uint32(ts[i].Value))
						} else if op == "[CREATABLE MONSTER]" && ts[i].Type == 6 {
							loc.Creatable = append(loc.Creatable, ts[i].Text)
						} else {
							return nil, nil, fmt.Errorf("invalid source location list")
						}
					}
				}
			}
			if loc.ID == 0 || loc.Dungeon == 0 || loc.Kind == "" || i >= len(ts) {
				return nil, nil, fmt.Errorf("incomplete source location")
			}
			if _, exists := locations[loc.ID]; exists {
				return nil, nil, fmt.Errorf("duplicate source location")
			}
			locations[loc.ID] = loc
		}
	}
	if len(events) == 0 || len(locations) == 0 {
		return nil, nil, fmt.Errorf("missing native phase events or locations")
	}
	return events, locations, nil
}
