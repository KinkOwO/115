package adventure

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"strconv"
)

// These retain the existing Python exporter's first-section/closed-block
// semantics. Numeric strings in the experience table are native 64-bit values.
func ruleSection(cells []pvf.Token, tag string) []pvf.Token {
	for i, t := range cells {
		if t.Type != 3 || t.Text != tag {
			continue
		}
		end := i + 1
		for end < len(cells) && cells[end].Type != 3 {
			end++
		}
		return cells[i+1 : end]
	}
	return nil
}

func ruleBlocks(cells []pvf.Token, tag string) [][]pvf.Token {
	var blocks [][]pvf.Token
	start := -1
	closing := "[/" + tag[1:]
	for i, t := range cells {
		if t.Type != 3 {
			continue
		}
		if t.Text == tag {
			start = i + 1
		}
		if start >= 0 && t.Text == closing {
			blocks = append(blocks, cells[start:i])
			start = -1
		}
	}
	return blocks
}

func ruleNumbers(cells []pvf.Token, tag string) ([]int64, error) {
	out := []int64{}
	for _, t := range ruleSection(cells, tag) {
		switch t.Type {
		case 0:
			out = append(out, int64(t.Value))
		case 6:
			v, err := strconv.ParseInt(t.Text, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("%s has invalid numeric string: %w", tag, err)
			}
			out = append(out, v)
		default:
			return nil, fmt.Errorf("%s has unsupported numeric cell type %d", tag, t.Type)
		}
	}
	return out, nil
}

func ruleUint(cells []pvf.Token, tag string) (uint32, error) {
	v, err := ruleNumbers(cells, tag)
	if err != nil {
		return 0, err
	}
	if len(v) != 1 || v[0] < 0 || v[0] > math.MaxUint32 {
		return 0, fmt.Errorf("invalid single %s value", tag)
	}
	return uint32(v[0]), nil
}

func readRuleItem(a *pvf.Archive, index catalog.ItemIndex, id uint32) (Item, error) {
	var out Item
	meta, ok := index.Items[id]
	if !ok || meta.ID != id || meta.Kind != "stackable" || meta.Path == "" {
		return out, fmt.Errorf("adventure reward %d has no native stackable binding", id)
	}
	script, err := catalog.ReadScript(a, meta.Path)
	if err != nil {
		return out, err
	}
	kind := ruleSection(script.Cells, "[stackable type]")
	if len(kind) == 0 || kind[0].Type != 6 {
		return out, fmt.Errorf("adventure reward %d has no type", id)
	}
	out.Path, out.SHA256, out.Type = script.Path, script.SHA256, kind[0].Text
	limit, err := ruleNumbers(script.Cells, "[stack limit]")
	if err != nil {
		return out, err
	}
	if len(limit) > 0 && (limit[0] < 0 || limit[0] > math.MaxUint32) {
		return out, fmt.Errorf("adventure reward %d invalid stack limit", id)
	}
	if len(limit) > 0 {
		out.Limit = uint32(limit[0])
	}
	out.UsagePeriod, err = ruleNumbers(script.Cells, "[usable period]")
	return out, err
}
