package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
)

type BakalStageRule struct{ HealthPercent, UnlockGradeBelow int32 }

// PROC changes to its phase-shift custom action while alive. The scalar in
// FILTER VAR is a source string-table operand (type10), not a missing value.
func parseBakalStage(a *pvf.Archive, symbols map[string]uint32) (BakalStageRule, error) {
	var r BakalStageRule
	ts, err := a.Tokens("contents/2022/bakalraid/monster/bakal/action/proc.act")
	if err != nil {
		return r, err
	}
	for i := 0; i+3 < len(ts); i++ {
		if ts[i].Text != "[CHECK RAID SYMBOL]" || uint32(ts[i+1].Value) != symbols["[BAKAL HP UNLOCK GRADE]"] || ts[i+2].Text != "[<]" {
			continue
		}
		r.UnlockGradeBelow = ts[i+3].Value
		for j := i + 4; j+3 < len(ts) && ts[j].Text != "[/TRIGGER]"; j++ {
			if ts[j].Text == "[FILTER VAR]" && ts[j+1].Type == 10 && a.ResolveString(int(ts[j+1].Value)) == "toint(o:getHpRate())" && ts[j+2].Text == "[<=]" && ts[j+3].Type == 10 {
				v, err := strconv.ParseInt(a.ResolveString(int(ts[j+3].Value)), 10, 32)
				if err != nil || v <= 0 || v >= 100 || r.UnlockGradeBelow <= 0 {
					return r, fmt.Errorf("invalid native Bakal stage condition")
				}
				r.HealthPercent = int32(v)
				return r, nil
			}
		}
	}
	return r, fmt.Errorf("native Bakal stage condition missing")
}
