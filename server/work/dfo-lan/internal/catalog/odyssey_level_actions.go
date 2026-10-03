package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// ParseOdysseyLevelActions retains source levels and ordered action names. The
// character executor maps supported action names to existing operations.
func ParseOdysseyLevelActions(cells []pvf.Token) (map[byte][]string, error) {
	start, end, ok := findSectionBounds(cells, "[level action]")
	if !ok {
		return nil, fmt.Errorf("missing Odyssey level actions")
	}
	out := map[byte][]string{}
	for i := start + 1; i < end; {
		token := cells[i]
		// Milestone rewards share this section, but have their own parser.
		if token.Type == 3 && token.Text == "[reward info]" {
			_, stop, ok := findSectionBounds(cells[i:end], "[reward info]")
			if !ok {
				return nil, fmt.Errorf("invalid Odyssey level reward section")
			}
			i += stop + 1
			continue
		}
		if token.Type != 3 || token.Text != "[action data]" || i+1 >= end || cells[i+1].Type != 0 || cells[i+1].Value <= 0 || cells[i+1].Value > 255 {
			return nil, fmt.Errorf("invalid Odyssey action header")
		}
		level := byte(cells[i+1].Value)
		if _, duplicate := out[level]; duplicate {
			return nil, fmt.Errorf("duplicate Odyssey action level %d", level)
		}
		i += 2
		var actions []string
		seen := map[string]bool{}
		for i < end && !(cells[i].Type == 3 && cells[i].Text == "[/action data]") {
			c := cells[i]
			if c.Type != 6 || c.Text == "" || seen[c.Text] {
				return nil, fmt.Errorf("invalid Odyssey action at level %d", level)
			}
			seen[c.Text] = true
			actions = append(actions, c.Text)
			i++
		}
		if i >= end || len(actions) == 0 {
			return nil, fmt.Errorf("unclosed/empty Odyssey actions")
		}
		out[level] = actions
		i++
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("empty Odyssey level actions")
	}
	return out, nil
}
