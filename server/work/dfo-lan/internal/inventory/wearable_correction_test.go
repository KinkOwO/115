package inventory

import (
	"testing"

	"dfolan/internal/catalog/pvf"
)

// 装备可以自带 [correction equipped level]（使用等级修正，客户端显示为「装备等级限制-5」）。
// 有效需求 = [minimum level] + 修正。
//
// 2026-09-23 实机：100051394 的 [minimum level]=50、修正=-5（有效 45），
// 角色 38 级 + 霸王 10 = 有效 48 ≥ 45，本该能穿却被拒 —— 因为旧代码只读 [minimum level]。
func TestWearableByAppliesCorrectionEquippedLevel(t *testing.T) {
	fields := map[string][]pvf.Token{
		"[minimum level]":             {{Type: 0, Value: 50}},
		"[correction equipped level]": {{Type: 0, Value: -5}},
		"[usable job]":                {{Type: 6, Text: "[all]"}},
	}
	// 有效需求 45 ⇒ 48 级应当放行。
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 48); err != nil {
		t.Fatalf("expected level 48 to wear an item whose effective requirement is 45, got %v", err)
	}
	// 44 < 45 ⇒ 仍应拒绝。
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 44); err == nil {
		t.Fatal("expected level 44 to be refused: effective requirement is 45")
	}
	// 边界：正好等于有效需求。
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 45); err != nil {
		t.Fatalf("expected level 45 to be accepted exactly, got %v", err)
	}
}

// 修正字段缺失时必须保持旧行为（只用 [minimum level]）。
func TestWearableByWithoutCorrectionKeepsOldBehaviour(t *testing.T) {
	fields := map[string][]pvf.Token{
		"[minimum level]": {{Type: 0, Value: 50}},
		"[usable job]":    {{Type: 6, Text: "[all]"}},
	}
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 48); err == nil {
		t.Fatal("expected level 48 to be refused when only [minimum level] is present")
	}
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 50); err != nil {
		t.Fatalf("expected level 50 to be accepted, got %v", err)
	}
}

// 修正字段形状异常（token 数不为 1、或类型非 0）时，按"没有该字段"处理 ——
// 不得因此放宽，也不得引入新的拒绝理由。
func TestWearableByIgnoresMalformedCorrection(t *testing.T) {
	cases := map[string][]pvf.Token{
		"two tokens": {{Type: 0, Value: -5}, {Type: 0, Value: -5}},
		"text type":  {{Type: 6, Text: "-5"}},
	}
	for name, corr := range cases {
		t.Run(name, func(t *testing.T) {
			fields := map[string][]pvf.Token{
				"[minimum level]":             {{Type: 0, Value: 50}},
				"[correction equipped level]": corr,
				"[usable job]":                {{Type: 6, Text: "[all]"}},
			}
			if err := WearableBy(fields, "[weapon]", "swordman", 0, 50); err != nil {
				t.Fatalf("malformed correction must not change behaviour, got %v", err)
			}
			if err := WearableBy(fields, "[weapon]", "swordman", 0, 49); err == nil {
				t.Fatal("level 49 must still be refused: malformed correction is ignored")
			}
		})
	}
}

// 修正把需求压到负数时钳到 0（0 级即可穿），不得出现反向放行或异常。
func TestWearableByClampsNegativeRequirement(t *testing.T) {
	fields := map[string][]pvf.Token{
		"[minimum level]":             {{Type: 0, Value: 1}},
		"[correction equipped level]": {{Type: 0, Value: -5}},
		"[usable job]":                {{Type: 6, Text: "[all]"}},
	}
	if err := WearableBy(fields, "[weapon]", "swordman", 0, 0); err != nil {
		t.Fatalf("expected level 0 to be accepted after clamping, got %v", err)
	}
}

// 等级以外的两条规则不受本次改动影响。
func TestWearableByCorrectionDoesNotBypassJobOrAdvancement(t *testing.T) {
	base := map[string][]pvf.Token{
		"[minimum level]":             {{Type: 0, Value: 50}},
		"[correction equipped level]": {{Type: 0, Value: -5}},
		"[usable job]":                {{Type: 6, Text: "[all]"}},
	}

	jobFields := map[string][]pvf.Token{}
	for k, v := range base {
		jobFields[k] = v
	}
	jobFields["[usable job]"] = []pvf.Token{{Type: 6, Text: "[fighter]"}}
	if err := WearableBy(jobFields, "[weapon]", "swordman", 0, 60); err == nil {
		t.Fatal("expected profession requirement to still reject")
	}

	growFields := map[string][]pvf.Token{}
	for k, v := range base {
		growFields[k] = v
	}
	growFields["[usable grow type]"] = []pvf.Token{{Type: 0, Value: 2}}
	if err := WearableBy(growFields, "[weapon]", "swordman", 3, 60); err == nil {
		t.Fatal("expected advancement requirement to still reject")
	}
}
