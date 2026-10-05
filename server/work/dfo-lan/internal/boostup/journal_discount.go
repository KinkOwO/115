package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
)

type JournalDiscount struct {
	Step, Phase               byte
	Condition, Gold, Material uint32
}

func parseJournalDiscounts(cells []pvf.Token) ([]JournalDiscount, error) {
	roots, e := sections(cells, "[discount cost]", "[/discount cost]")
	if e != nil {
		return nil, e
	}
	if len(roots) == 0 {
		return nil, nil
	}
	if len(roots) != 1 {
		return nil, fmt.Errorf("duplicate boost discount root")
	}
	rows, e := sections(roots[0], "[step]", "[/step]")
	if e != nil {
		return nil, e
	}
	var out []JournalDiscount
	seen := map[[3]uint32]bool{}
	for _, row := range rows {
		n, e := number(row, "[no]", 254)
		if e != nil || n == 0 {
			return nil, fmt.Errorf("invalid discount step")
		}
		phase, e := number(row, "[state]", 2)
		if e != nil {
			return nil, e
		}
		condition, e := number(row, "[condition]", ^uint32(0))
		if e != nil {
			return nil, e
		}
		gold, e := number(row, "[gold]", 100)
		if e != nil {
			return nil, e
		}
		material, e := number(row, "[material]", 100)
		if e != nil {
			return nil, e
		}
		key := [3]uint32{n, phase, condition}
		if seen[key] {
			return nil, fmt.Errorf("duplicate boost discount condition")
		}
		seen[key] = true
		out = append(out, JournalDiscount{byte(n), byte(phase), condition, gold, material})
	}
	return out, nil
}

// The current captured normal equipment conversion uses source condition 0.
// Caller must read this from the locked character, not request parameters.
func (c *Catalog) JournalTransformDiscount(raw json.RawMessage) (uint32, uint32, error) {
	if c == nil {
		return 0, 0, nil
	}
	st, e := ReadState(raw)
	if e != nil {
		return 0, 0, e
	}
	if !st.Activated || st.Training.Finished {
		return 0, 0, nil
	}
	if _, e = c.current(st.Training); e != nil {
		return 0, 0, e
	}
	for _, d := range c.JournalDiscounts {
		if d.Step != st.Training.Step || d.Phase != st.Training.Phase {
			continue
		}
		if d.Condition != 0 {
			return 0, 0, fmt.Errorf("unmapped boost journal discount condition %d", d.Condition)
		}
		if d.Phase == 2 && !st.Training.Claimed[d.Step] {
			return 0, 0, fmt.Errorf("discount requires claimed training step")
		}
		return d.Gold, d.Material, nil
	}
	return 0, 0, nil
}
