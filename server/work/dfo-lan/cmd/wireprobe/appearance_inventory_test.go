package main

import (
	"bytes"
	"dfolan/internal/inventory"
	"encoding/json"
	"testing"
)

func TestAppearanceRestoresAvatarAndWornInstances(t *testing.T) {
	raw := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":40601,"durability":9}],"special_equipment":{"1":[]}}}`)
	before := bytes.Clone(raw)
	// 第二个参数是装备目录（用于按 PVF 默认孔补时装孔，上游 90896789 加的）。
	// 本用例的存档里没有时装行，补孔分支不会被触发 —— 传 nil 即"没有目录"，
	// 与 appearanceInventory 里的 `if eq != nil` 守卫一致。
	plan, e := appearanceInventory(raw, nil)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 4 || plan[0].ID != 13 || plan[0].Payload[0] != 1 || plan[2].Payload[0] != 3 || plan[3].ID != 14 {
		t.Fatal(plan)
	}
	want, e := inventory.WornPayload(raw)
	if e != nil || !bytes.Equal(want, plan[2].Payload) {
		t.Fatal(e, "worn instance mismatch")
	}
	if !bytes.Equal(before, raw) {
		t.Fatal("state changed")
	}
}
