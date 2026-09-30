package inventory

import (
	"encoding/json"
	"testing"
)

// 耐久上限 clamp：超出源 `.equ` `[durability]` 的实例耐久必须被压回上限。
//
// 实机 2026-09-30：存档里出现 `100/48` 的武器 ⇒ 客户端判定该装备非法 ⇒
// 表现是"装备库登记不上 / 分解点不动 / 装备变换界面卡死"。
func TestClampDurabilityCapsToSourceLimit(t *testing.T) {
	SetDurabilityLimit(func(template uint32) (uint16, bool) {
		switch template {
		case 117010253, 117010280:
			return 48, true
		case 100301825:
			return 0, true // 上限 0 = 该部位本来就没有耐久上限（首饰类）
		}
		return 0, false
	})
	defer SetDurabilityLimit(nil)

	items := []BagEquipment{
		{Slot: 12, Template: 117010253, Durability: 100}, // 超上限 → 压到 48
		{Slot: 19, Template: 100301825, Durability: 100}, // 上限 0 → 不动
		{Slot: 13, Template: 999999, Durability: 100},    // 查不到上限 → 不动
		{Slot: 14, Template: 117010280, Durability: 30},  // 未超上限 → 不动
	}
	clampDurability(items)

	if items[0].Durability != 48 {
		t.Fatalf("超上限的耐久应被压到 48，实际 %d", items[0].Durability)
	}
	if items[1].Durability != 100 {
		t.Fatalf("上限为 0 的部位不该被改，实际 %d", items[1].Durability)
	}
	if items[2].Durability != 100 {
		t.Fatalf("查不到上限时不该被改，实际 %d", items[2].Durability)
	}
	if items[3].Durability != 30 {
		t.Fatalf("未超上限时不该被改，实际 %d", items[3].Durability)
	}
}

// 未安装上限源时行为与本修复前完全一致（不做任何修正）。
func TestClampDurabilityNoopWithoutSource(t *testing.T) {
	SetDurabilityLimit(nil)
	items := []BagEquipment{{Slot: 12, Template: 117010253, Durability: 100}}
	clampDurability(items)
	if items[0].Durability != 100 {
		t.Fatalf("未安装上限源时不该改：%d", items[0].Durability)
	}
}

// SaveBag 落库时必须 clamp，且**不能就地改到调用方那份 Bag**。
func TestSaveBagClampsDurabilityWithoutMutatingInput(t *testing.T) {
	SetDurabilityLimit(func(template uint32) (uint16, bool) {
		if template == 117010253 {
			return 48, true
		}
		return 0, false
	})
	defer SetDurabilityLimit(nil)

	bag := Bag{
		Version: "ordinary-bag-v1",
		Worn:    []BagEquipment{{Slot: 12, Template: 117010253, Durability: 100}},
	}
	state := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"},"other":1}`)
	out, e := SaveBag(state, bag)
	if e != nil {
		t.Fatalf("SaveBag: %v", e)
	}

	var fields map[string]json.RawMessage
	if e := json.Unmarshal(out, &fields); e != nil {
		t.Fatalf("unmarshal out: %v", e)
	}
	if string(fields["other"]) != "1" {
		t.Fatalf("SaveBag 应保留其它顶层键，out=%s", string(out))
	}
	var saved Bag
	if e := json.Unmarshal(fields["inventory"], &saved); e != nil {
		t.Fatalf("unmarshal inventory: %v", e)
	}
	if len(saved.Worn) != 1 || saved.Worn[0].Durability != 48 {
		t.Fatalf("落库的耐久应为 48，实际 %+v", saved.Worn)
	}
	if bag.Worn[0].Durability != 100 {
		t.Fatalf("入参 Bag 不该被就地修改，实际 %d", bag.Worn[0].Durability)
	}
}
