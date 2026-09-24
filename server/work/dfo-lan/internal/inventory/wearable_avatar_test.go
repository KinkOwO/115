package inventory

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 实机取证 2026-09-23:皮肤/武器装扮等时装的源脚本不带 [minimum level]
// (对比 fighter裤子 502510504 带 `[minimum level] 1`,皮肤 502580005 没有该段),
// 穿戴被旧守卫按 "unavailable" 拒绝。时装家族缺字段 = 无等级要求;
// 普通装备缺字段仍然是不可识别定义。
func TestWearableByAvatarMissingMinimumLevel(t *testing.T) {
	// Shape copied from equipment/character/fighter/avatar/skin/502580005.equ.
	skin := map[string][]pvf.Token{
		"[usable job]":     {{Type: 6, Text: "[fighter]"}},
		"[equipment type]": {{Type: 6, Text: "[skin avatar]"}},
		"[hit recovery]":   {{Type: 0, Value: 80}},
	}
	if e := WearableBy(skin, "[skin avatar]", "[fighter]", 0, 1); e != nil {
		t.Fatal("avatar without [minimum level] must wear at level 1", e)
	}
	if e := WearableBy(skin, "[skin avatar]", "[gunner]", 0, 1); e == nil {
		t.Fatal("avatar job requirement must still bind")
	}

	// Shape copied from equipment/character/fighter/avatar/pants/502510504.equ:
	// an explicit [minimum level] keeps binding avatars too.
	pants := map[string][]pvf.Token{
		"[minimum level]":  {{Type: 0, Value: 20}},
		"[usable job]":     {{Type: 6, Text: "[all]"}},
		"[equipment type]": {{Type: 6, Text: "[pants avatar]"}},
	}
	if e := WearableBy(pants, "[pants avatar]", "fighter", 0, 19); e == nil {
		t.Fatal("avatar explicit minimum level must bind")
	}
	if e := WearableBy(pants, "[pants avatar]", "fighter", 0, 20); e != nil {
		t.Fatal("avatar explicit minimum level satisfied", e)
	}

	// Ordinary equipment without the field stays an unavailable definition.
	ordinary := map[string][]pvf.Token{
		"[usable job]":     {{Type: 6, Text: "[all]"}},
		"[equipment type]": {{Type: 6, Text: "[weapon]"}},
	}
	if e := WearableBy(ordinary, "[weapon]", "fighter", 0, 115); e == nil {
		t.Fatal("ordinary equipment without [minimum level] must stay unavailable")
	}
}
