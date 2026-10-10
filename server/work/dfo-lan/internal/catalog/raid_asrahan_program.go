package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// The instruction grammar is structural. All identities, operands, mode
// branches and ordering remain in the selected native source.
type AsrahanInstruction struct {
	Op   string
	Args []pvf.Token
}
type AsrahanScriptEvent struct {
	Trigger, Behavior []AsrahanInstruction
}

var asrahanTriggerGrammar = map[string]string{
	"[IF]": "if", "[ON CHANGE SYMBOL]": "s", "[CHECK TIMER END]": "nn",
}
var asrahanBehaviorGrammar = map[string]string{
	"[SET]": "set", "[ASSIGN]": "ss", "[SUM]": "sss", "[SUB]": "ss", "[SUB MAX]": "ssss",
	"[SET DUNGEON STATE]": "nt", "[SET TIMER]": "nnn", "[RESET TIMER]": "nn", "[SEND RAID SET TIMER]": "nnnn",
	"[ADD REMAIN TIME]": "n", "[ADD COIN LIMIT]": "n", "[CLEAR PHASE]": "",
	"[INIT ASRAHAN BUFF GROUP]": "", "[INIT DIVINE BEAST]": "", "[SET FLOOR START TIME]": "",
	"[INIT BUFF THUMB UP]": "", "[SET MEMORY OVERLOAD]": "s", "[SET MEMORY OVERLOAD2]": "ss",
	"[RESET REVIVAL TIME]": "", "[RESET RETRY DUNGEON TIME]": "", "[INIT FLOOR1 ACTIVE BUFF COUNT]": "s",
	"[INIT ASRAHAN TIME LINE]": "", "[APPLY FLOOR1 PASSIVE BUFF]": "", "[LOG CLEAR RAID FLOOR]": "",
	"[RESET ASRAHAN TIME LINE]": "", "[INIT FLOOR2 BUFF AMP]": "", "[INIT FLOOR2 DUNGEON TIMER]": "",
	"[SET FLOOR2 REVELATION]": "s", "[RESET USE COIN COUNT]": "", "[RESET STACKABLE LIMIT COUNT]": "",
	"[RESET ASRAHAN DEBUFF]": "", "[INIT ASRAHAN MU DUNGEON]": "", "[INIT ASRAHAN MU HP LOCK]": "",
	"[INIT FLOOR3 RANDOM PATTERN]": "", "[RESET ASRAHAN TERMINUS TIME LINE]": "", "[RETURN TO VILLAGE]": "",
	"[UPDATE DIVINE BEAST]": "sssn", "[CHARGE DIVINE BEAST]": "ssn", "[SPAWN ASRAHAN MONSTER]": "n",
	"[RETURN TO VILLAGE FROM DUNGEON]": "n", "[RESET FLOOR2 DUNGEON TIMER]": "",
	"[ADD ASRAHAN DEBUFF RANDOM]": "n", "[FINISH TERMINUS RECOVERY STATE]": "sn", "[CHECK MU PHASE2]": "",
	"[SPAWN ASRAHAN MU MONSTER]": "n", "[RETURN TO VILLAGE FROM MU DUNGEON]": "",
	"[ASRAHAN MU RECOVERY]": "n", "[RESUME ASRAHAN TIME LINE]": "",
}

func asrahanOperand(symbols map[string]uint32, t pvf.Token, kind byte) bool {
	switch kind {
	case 's':
		return t.Type == 3 && symbols[t.Text] != 0
	case 'n':
		return t.Type == 0
	case 't':
		return t.Type == 6 && t.Text != ""
	}
	return false
}

func asrahanInstructions(ts []pvf.Token, symbols map[string]uint32, grammar map[string]string) ([]AsrahanInstruction, error) {
	var out []AsrahanInstruction
	for i := 0; i < len(ts); {
		op := ts[i].Text
		shape, ok := grammar[op]
		if ts[i].Type != 3 || !ok {
			return nil, fmt.Errorf("unbound Asrahan instruction %q at cell %d", op, i)
		}
		i++
		if shape == "if" || shape == "set" {
			if i >= len(ts) || !asrahanOperand(symbols, ts[i], 's') {
				return nil, fmt.Errorf("%s requires declared target symbol", op)
			}
			shape = "sn"
			if i+1 < len(ts) && ts[i+1].Type == 3 {
				operator := ts[i+1].Text
				if op == "[SET]" {
					if operator != "[+]" {
						return nil, fmt.Errorf("unsupported Asrahan assignment %s", operator)
					}
					shape = "sos"
				} else {
					switch operator {
					case "[!=]", "[>]", "[<]", "[>=]", "[<=]", "[==]":
						shape = "son"
					default:
						return nil, fmt.Errorf("unsupported Asrahan comparison %s", operator)
					}
				}
			}
		}
		if i+len(shape) > len(ts) {
			return nil, fmt.Errorf("truncated Asrahan %s", op)
		}
		args := append([]pvf.Token(nil), ts[i:i+len(shape)]...)
		for j, kind := range []byte(shape) {
			if kind != 'o' && !asrahanOperand(symbols, args[j], kind) {
				return nil, fmt.Errorf("invalid Asrahan %s operand %d", op, j)
			}
		}
		i += len(shape)
		out = append(out, AsrahanInstruction{op, args})
	}
	return out, nil
}

// Compile validates every complete event, including actions whose runtime
// binding has not yet been installed. It never drops an unfamiliar action.
func CompileAsrahanPhase(r *AsrahanRaidRules, p AsrahanRaidPhase) ([]AsrahanScriptEvent, error) {
	if r == nil || len(r.Symbols) == 0 || len(p.Events) == 0 {
		return nil, fmt.Errorf("Asrahan source program missing")
	}
	program := make([]AsrahanScriptEvent, 0, len(p.Events))
	for row, event := range p.Events {
		t, err := asrahanInstructions(event.Trigger, r.Symbols, asrahanTriggerGrammar)
		if err != nil {
			return nil, fmt.Errorf("%s event %d trigger: %w", r.Path, row, err)
		}
		b, err := asrahanInstructions(event.Behavior, r.Symbols, asrahanBehaviorGrammar)
		if err != nil {
			return nil, fmt.Errorf("%s event %d behavior: %w", r.Path, row, err)
		}
		if len(t) == 0 || len(b) == 0 {
			return nil, fmt.Errorf("empty Asrahan event %d", row)
		}
		for _, instruction := range b {
			if instruction.Op == "[SET DUNGEON STATE]" {
				if _, ok := p.Dungeons[uint32(instruction.Args[0].Value)]; !ok {
					return nil, fmt.Errorf("event %d references foreign Asrahan dungeon", row)
				}
			}
		}
		program = append(program, AsrahanScriptEvent{t, b})
	}
	return program, nil
}
