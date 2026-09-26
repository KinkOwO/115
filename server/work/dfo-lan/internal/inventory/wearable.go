package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

const correctionEquippedLevelKey = "[correction equipped level]"

// WearableBy reports whether a source definition's own requirements admit this
// character: minimum level (including its [correction equipped level]
// adjustment), usable job, usable grow type. It is the single place those three
// rules live, so wearing a piece and being offered it as a drop cannot disagree.
//
// kind is the source [equipment type] text. The avatar family - skins
// ([skin avatar]), weapon avatars ([weapon avatar]) and the rest of the
// [* avatar] kinds - routinely ships .equ scripts with no [minimum level]
// section at all: trousers 502510504 carry "[minimum level] 1" while skin
// 502580005 has no such section. The old guard read a missing field as an
// unrecognisable definition and refused the piece, which the client renders as
// "背包已满". For those kinds a missing section means "no level requirement";
// for ordinary equipment it still means unavailable.
func WearableBy(fields map[string][]pvf.Token, kind string, job string, advancement, level byte) error {
	levels := fields["[minimum level]"]
	if len(levels) == 0 {
		if !strings.HasSuffix(kind, " avatar]") && !IsPetGear(kind) && EquipmentBagSpace(kind) != 7 {
			return fmt.Errorf("equipment minimum level not met or unavailable")
		}
		levels = []pvf.Token{{Type: 0, Value: 0}}
	} else if len(levels) != 1 || levels[0].Type != 0 || levels[0].Value < 0 {
		return fmt.Errorf("equipment minimum level not met or unavailable")
	}
	required := levels[0].Value
	if corr := fields[correctionEquippedLevelKey]; len(corr) == 1 && corr[0].Type == 0 {
		required += corr[0].Value
	}
	if required < 0 {
		required = 0
	}
	if int32(level) < required {
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
