package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

type BakalMode uint8

const (
	BakalNormal BakalMode = iota
	BakalHard
)

// Select only the requested phase tree. The shared header remains source
// owned; renaming the hard block delimiters changes syntax, never operands.
func bakalPhaseTokens(ts []pvf.Token, mode BakalMode) ([]pvf.Token, error) {
	if mode != BakalNormal && mode != BakalHard {
		return nil, fmt.Errorf("invalid Bakal mode %d", mode)
	}
	startTag, endTag := "[PHASE]", "[/PHASE]"
	if mode == BakalHard {
		startTag, endTag = "[RAID PHASE OF HARDMODE]", "[/RAID PHASE OF HARDMODE]"
	}
	headerEnd := len(ts)
	start, end := -1, -1
	for i, t := range ts {
		if t.Text == "[PHASE]" || t.Text == "[RAID PHASE OF HARDMODE]" {
			if i < headerEnd {
				headerEnd = i
			}
		}
		if t.Text == startTag {
			if start != -1 {
				return nil, fmt.Errorf("duplicate Bakal mode phase")
			}
			start = i
		}
		if t.Text == endTag && start >= 0 {
			end = i
			break
		}
	}
	if start < 0 || end < start {
		return nil, fmt.Errorf("missing or unclosed Bakal mode %d phase", mode)
	}
	out := append([]pvf.Token(nil), ts[:headerEnd]...)
	out = append(out, pvf.Token{Type: 3, Text: "[PHASE]"})
	out = append(out, ts[start+1:end]...)
	out = append(out, pvf.Token{Type: 3, Text: "[/PHASE]"})
	// The canonical script declares one shared battlefield after the normal
	// phase, outside both difficulty trees. Include only that explicit block.
	for i, t := range ts {
		if t.Text != "[LOCATIONS]" {
			continue
		}
		last := i + 1
		for last < len(ts) && ts[last].Text != "[/LOCATIONS]" {
			last++
		}
		if last == len(ts) {
			return nil, fmt.Errorf("unclosed shared Bakal battlefield")
		}
		out = append(out, ts[i:last+1]...)
	}
	return out, nil
}

func importBakalRaidMode(ts []pvf.Token, mode BakalMode) (*BakalRaidRules, error) {
	selected, err := bakalPhaseTokens(ts, mode)
	if err != nil {
		return nil, err
	}
	r, err := importBakalRaidRules(selected)
	if err != nil {
		return nil, err
	}
	r.Mode = mode
	return r, nil
}
