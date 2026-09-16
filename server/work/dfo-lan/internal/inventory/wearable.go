package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// WearableBy reports whether a source definition's own requirements admit this
// character: minimum level, usable job, usable grow type. It is the single
// place those three rules live, so wearing a piece and being offered it as a
// drop cannot disagree.
func WearableBy(fields map[string][]pvf.Token, job string, advancement, level byte) error {
	levels := fields["[minimum level]"]
	if len(levels) != 1 || levels[0].Type != 0 || levels[0].Value < 0 || int32(level) < levels[0].Value {
		return fmt.Errorf("equipment minimum level not met or unavailable")
	}
	allowed := false
	for _, j := range fields["[usable job]"] {
		allowed = allowed || j.Text == "[all]" || j.Text == job
	}
	if !allowed {
		return fmt.Errorf("equipment profession requirement not met")
	}
	if grow := fields["[usable grow type]"]; len(grow) > 0 {
		allowed = false
		for _, g := range grow {
			if g.Type == 0 && (g.Value == -1 || g.Value == int32(advancement)) {
				allowed = true
			}
		}
		if !allowed {
			return fmt.Errorf("equipment advancement requirement not met")
		}
	}
	return nil
}
