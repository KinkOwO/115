package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strconv"
)

type BakalStageRule struct{ HealthPercent, UnlockGradeBelow int32 }

// PROC orders the phase-shift action while the first boss is still alive.
func loadBakalStage(a *pvf.Archive, r *BakalRaidRules) error {
	ts, e := a.Tokens("contents/2022/bakalraid/monster/bakal/action/proc.act")
	if e != nil {
		return e
	}
	for i := 0; i+3 < len(ts); i++ {
		if ts[i].Text != "[CHECK RAID SYMBOL]" || uint32(ts[i+1].Value) != r.Symbols["[BAKAL HP UNLOCK GRADE]"] || ts[i+2].Text != "[<]" {
			continue
		}
		grade := ts[i+3].Value
		for j := i + 4; j+3 < len(ts) && ts[j].Text != "[/TRIGGER]"; j++ {
			if ts[j].Text == "[FILTER VAR]" && ts[j+1].Type == 10 && a.ResolveString(int(ts[j+1].Value)) == "toint(o:getHpRate())" && ts[j+2].Text == "[<=]" && ts[j+3].Type == 10 {
				hp, e := strconv.ParseInt(a.ResolveString(int(ts[j+3].Value)), 10, 32)
				if e != nil || hp <= 0 || hp >= 100 || grade <= 0 {
					return fmt.Errorf("invalid source phase transition")
				}
				r.Stage = BakalStageRule{HealthPercent: int32(hp), UnlockGradeBelow: grade}
				return nil
			}
		}
	}
	return fmt.Errorf("source phase transition missing")
}
