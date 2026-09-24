package inventory

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"strings"
)

const correctionEquippedLevelKey = "[correction equipped level]"

// WearableBy reports whether a source definition's own requirements admit this
// character: minimum level, usable job, usable grow type. It is the single
// place those three rules live, so wearing a piece and being offered it as a
// drop cannot disagree.
//
// 实机取证 2026-09-23:时装(尤其皮肤 `[skin avatar]`、武器装扮 `[weapon avatar]`)
// 的 .equ 源脚本经常**不带** `[minimum level]`(对比:裤子 502510504 带 `[minimum
// level] 1`,皮肤 502580005 完全没有该段),穿戴时被旧守卫按"unavailable"拒绝。
// 时装家族缺该字段按源语义视为无等级要求;普通装备缺失仍然是不可识别的定义。
func WearableBy(fields map[string][]pvf.Token, kind string, job string, advancement, level byte) error {
	levels := fields["[minimum level]"]
	if len(levels) == 0 {
		if !strings.HasSuffix(kind, " avatar]") {
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
