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
