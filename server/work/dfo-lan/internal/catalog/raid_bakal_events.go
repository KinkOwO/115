package catalog

import "dfolan/internal/catalog/pvf"

// Read normal-mode event blocks directly from the canonical raid script.
// Never execute half a behaviour block whose other operations are unsupported.
func parseBakalEnterSymbolRules(ts []pvf.Token) []BakalEnterSymbolRule {
	var out []BakalEnterSymbolRule
	for i := 0; i < len(ts); i++ {
		if ts[i].Text == "[RAID PHASE OF HARDMODE]" {
			break
		}
		if ts[i].Text != "[TRIGGER]" {
			continue
		}
		start := i + 1
		for i++; i < len(ts) && ts[i].Text != "[/TRIGGER]"; i++ {
		}
		if i == len(ts) {
			break
		}
		trigger := ts[start:i]
		if len(trigger) < 2 || trigger[0].Text != "[ON ENTER DUNGEON]" || trigger[1].Type != 0 || trigger[1].Value <= 0 {
			continue
		}
		r := BakalEnterSymbolRule{Dungeon: uint32(trigger[1].Value)}
		valid := true
		for j := 2; j < len(trigger); {
			if j+2 >= len(trigger) || trigger[j].Text != "[IF]" || trigger[j+1].Type != 3 {
				valid = false
				break
			}
			c := BakalSymbolCondition{Name: trigger[j+1].Text, Operator: "="}
			j += 2
			if trigger[j].Text == "[>]" || trigger[j].Text == "[<]" {
				c.Operator = trigger[j].Text
				j++
			}
			if j >= len(trigger) || trigger[j].Type != 0 {
				valid = false
				break
			}
			c.Value = trigger[j].Value
			j++
			r.Conditions = append(r.Conditions, c)
		}
		if !valid || i+1 >= len(ts) || ts[i+1].Text != "[BEHAVIOR]" {
			continue
		}
		start = i + 2
		for i = start; i < len(ts) && ts[i].Text != "[/BEHAVIOR]"; i++ {
		}
		if i == len(ts) {
			break
		}
		body := ts[start:i]
		for j := 0; j < len(body); j += 3 {
			if j+2 >= len(body) || body[j].Text != "[SET]" || body[j+1].Type != 3 || body[j+2].Type != 0 {
				valid = false
				break
			}
			r.Assignments = append(r.Assignments, BakalSymbolAssignment{Name: body[j+1].Text, Value: body[j+2].Value})
		}
		if valid && len(r.Assignments) > 0 {
			out = append(out, r)
		}
	}
	return out
}
