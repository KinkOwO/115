package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"strconv"
	"strings"
)

// This declares a native last-animation action, not a server timer or a
// fabricated portal. Actual source boss death still authorizes progression.
type ApocalypseBossMove struct {
	Boss                     uint32
	MonsterPath, MonsterHash string
	ActionPath, ActionHash   string
	Operations               []uint32 // nil: unconditional; otherwise the ACT trigger's source operation equalities
}

func (m ApocalypseBossMove) AllowsOperation(operation uint32) bool {
	if operation == 0 {
		return false
	}
	if len(m.Operations) == 0 {
		return true
	}
	for _, allowed := range m.Operations {
		if operation == allowed {
			return true
		}
	}
	return false
}

func importApocalypseBossMoves(a *pvf.Archive) ([]ApocalypseBossMove, error) {
	contents, err := ImportLegionContents(a)
	if err != nil {
		return nil, err
	}
	content, ok := contents.Contents["Apocalypse"]
	if !ok || !content.Complete {
		return nil, fmt.Errorf("apocalypse boss move source unavailable")
	}
	var out []ApocalypseBossMove
	seen := map[uint32]bool{}
	for _, row := range content.Dungeons {
		if row.Boss == 0 || seen[row.Boss] {
			continue
		}
		seen[row.Boss] = true
		mob, err := scriptWarpIndex(a, "list/monster.lst", row.Boss)
		if err != nil {
			return nil, err
		}
		refs := scriptWarpField(mob.Cells, "[last action]")
		if len(refs) == 0 {
			continue
		}
		if len(refs) != 1 || refs[0].Type != 6 {
			return nil, fmt.Errorf("ambiguous apocalypse last action: %s", mob.Path)
		}
		act, err := ReadScript(a, path.Join(path.Dir(mob.Path), strings.ReplaceAll(refs[0].Text, "\\", "/")))
		if err != nil {
			return nil, err
		}
		move, err := apocalypseLastActionMovesNext(act.Cells)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", act.Path, err)
		}
		if move {
			operations, err := apocalypseNextActionOperations(act.Cells, a.ResolveString)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", act.Path, err)
			}
			out = append(out, ApocalypseBossMove{Boss: row.Boss, MonsterPath: mob.Path, MonsterHash: mob.SHA256, ActionPath: act.Path, ActionHash: act.SHA256, Operations: operations})
		}
	}
	return out, nil
}

// Resolve the declaring BEHAVIOR to its actual TRIGGER. Merely finding a tag
// inside an action does not authorize it for every difficulty. Only source
// frame triggers, optionally guarded by operation-index equality, are supported.
func apocalypseNextActionOperations(cells []pvf.Token, resolve func(int) string) ([]uint32, error) {
	behaviors := scriptWarpBlocks(cells, "[BEHAVIOR]")
	index, name := -1, ""
	for i, block := range behaviors {
		found, err := apocalypseLastActionMovesNext(block)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		if index >= 0 {
			return nil, fmt.Errorf("multiple native next-phase behaviors")
		}
		for _, t := range block {
			if t.Type == 3 && (t.Text == "[BEGIN IF]" || t.Text == "[ELSE IF]" || t.Text == "[IF]") {
				return nil, fmt.Errorf("unsupported behavior-level next-phase predicate")
			}
		}
		index = i
		if len(block) > 0 && block[0].Type == 6 {
			name = block[0].Text
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("next-phase action outside behavior")
	}
	if name != "" {
		matches := 0
		for _, block := range behaviors {
			if len(block) > 0 && block[0].Type == 6 && block[0].Text == name {
				matches++
			}
		}
		if matches != 1 {
			return nil, fmt.Errorf("ambiguous next-phase behavior name")
		}
	}
	var operations []uint32
	matched, unconditional := false, false
	for _, block := range scriptWarpBlocks(cells, "[TRIGGER]") {
		uses := false
		for i, t := range block {
			if t.Type != 3 || i+2 >= len(block) || block[i+1].Type != 6 || block[i+1].Text != "ME" {
				continue
			}
			if t.Text == "[DO BEHAVIOR]" && block[i+2].Type == 0 && int64(block[i+2].Value) == int64(index) || t.Text == "[DO BEHAVIOR NAME]" && name != "" && block[i+2].Type == 6 && block[i+2].Text == name {
				uses = true
			}
		}
		if !uses {
			continue
		}
		matched = true
		i := 0
		// Optional native trigger name precedes the first tag.
		if len(block) > 0 && block[0].Type == 6 {
			i++
		}
		if i+1 >= len(block) || block[i].Type != 3 || block[i].Text != "[FRAME]" || block[i+1].Type != 0 || int32(block[i+1].Value) < 0 {
			return nil, fmt.Errorf("unsupported next-phase trigger timing")
		}
		i += 2
		guarded := i < len(block) && block[i].Type == 3 && block[i].Text == "[BEGIN IF]"
		if guarded {
			i++
		}
		for {
			var operation uint32
			if guarded {
				if resolve == nil || i+3 >= len(block) || block[i].Type != 3 || block[i].Text != "[COMPARE VAR]" || block[i+1].Type != 10 || strings.TrimSpace(resolve(int(block[i+1].Value))) != "getLegionOperationIndex()" || block[i+2].Type != 3 || block[i+2].Text != "[==]" || block[i+3].Type != 10 {
					return nil, fmt.Errorf("unsupported next-phase operation predicate")
				}
				v, err := strconv.ParseUint(strings.TrimSpace(resolve(int(block[i+3].Value))), 10, 32)
				if err != nil || v == 0 {
					return nil, fmt.Errorf("nonliteral next-phase operation predicate")
				}
				operation = uint32(v)
				i += 4
			}
			if i+2 >= len(block) || block[i].Type != 3 || block[i+1].Type != 6 || block[i+1].Text != "ME" {
				return nil, fmt.Errorf("unsupported next-phase trigger action")
			}
			t, target := block[i], block[i+2]
			valid := t.Text == "[DO BEHAVIOR]" && target.Type == 0 && int64(target.Value) == int64(index) || t.Text == "[DO BEHAVIOR NAME]" && target.Type == 6 && name != "" && target.Text == name
			if !valid {
				return nil, fmt.Errorf("mixed next-phase trigger actions")
			}
			i += 3
			if guarded {
				operations = append(operations, operation)
			} else {
				unconditional = true
			}
			if !guarded {
				if i != len(block) {
					return nil, fmt.Errorf("unsupported unconditional trigger tail")
				}
				break
			}
			if i < len(block) && block[i].Type == 3 && block[i].Text == "[ELSE IF]" {
				i++
				continue
			}
			if i+1 != len(block) || block[i].Type != 3 || block[i].Text != "[END IF]" {
				return nil, fmt.Errorf("unsupported next-phase condition tail")
			}
			break
		}
	}
	if !matched {
		return nil, fmt.Errorf("orphan next-phase behavior")
	}
	if unconditional {
		return nil, nil
	}
	return operations, nil
}

func apocalypseLastActionMovesNext(cells []pvf.Token) (bool, error) {
	found := false
	for i, t := range cells {
		if t.Type != 3 || t.Text != "[LEGION DUNGEON DIRECT MOVE NEXT PHASE]" {
			continue
		}
		if found || i+1 >= len(cells) || cells[i+1].Type != 6 || cells[i+1].Text != "Apocalypse" {
			return false, fmt.Errorf("ambiguous native next-phase content action")
		}
		if i+2 < len(cells) && cells[i+2].Type != 3 {
			return false, fmt.Errorf("unsupported native next-phase action arguments")
		}
		found = true
	}
	return found, nil
}
