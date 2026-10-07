package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrEnchantBeadBlocked = errors.New("enchant beads are blocked during this boost training step")

// Native loader 147776af3..147776b09 reads an integer and stores (value == 1).
// No flag means false; don't infer this from the mission name or step number.
func sourceStepFlag(c []pvf.Token, tag string) (bool, error) {
	for _, t := range c {
		if t.Type != 3 || t.Text != tag {
			continue
		}
		v := values(c, tag)
		if len(v) != 1 || v[0].Type != 0 {
			return false, fmt.Errorf("invalid boost flag %s", tag)
		}
		return v[0].Value == 1, nil
	}
	return false, nil
}

// Call inside the asset transaction, against the freshly locked character.
// Replay receipts are checked by the transaction boundary first, so an older
// successful enchant is not turned into a failure after the next step starts.
func (c *Catalog) CheckEnchantBead(raw json.RawMessage) error {
	if c == nil {
		return nil
	} // event disabled; ordinary enchant remains unchanged
	st, e := ReadState(raw)
	if e != nil {
		return e
	}
	if !st.Activated || st.Training.Finished {
		return nil
	}
	step, e := c.current(st.Training)
	if e != nil {
		return e
	}
	if step.BlockEnchantBead {
		return ErrEnchantBeadBlocked
	}
	return nil
}
