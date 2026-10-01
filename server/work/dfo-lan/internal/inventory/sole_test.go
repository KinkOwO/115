package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"testing"
)

// 手工搭一张与源同形的秘宝规则表（不依赖 PVF），用来钉住判定分支。
func soleTestRules() *catalog.SoleEquipmentRules {
	return &catalog.SoleEquipmentRules{
		Path: catalog.SoleEquipmentSystemPath,
		Items: map[uint32]catalog.SoleEquipmentInfo{
			100346156: {
				Template:   100346156,
				MaxQuality: 100,
				Groups: map[int][]catalog.SoleEquipmentMaterial{
					0: {{Template: 10413524, Amount: 30}, {Template: 10361515, Amount: 40}, {Template: 10401346, Amount: 800}},
					1: {{Template: 10413524, Amount: 30}, {Template: 10361515, Amount: 40}, {Template: 0, Amount: 4000000}},
				},
				Boundaries: []int{0, 25, 50, 75, 100},
			},
		},
	}
}

// 材料组由**请求的 selector** 决定，不是由精度推。
//
// 2026-10-02 实机纠正：这块面板第三项材料可以在「实物」与「金币」之间用切换按钮选择，
// 原实现按"精度 ≥ 50 才切金币"推组，前 5 次恰好与玩家选择一致才没暴露。
func TestPlanSoleQualityUsesRequestedGroup(t *testing.T) {
	rules := soleTestRules()
	// 同一个精度（83）下，组 0 与组 1 都必须可算 —— 这正是"按精度推"做不到的。
	for _, c := range []struct {
		group int
		gold  bool
	}{{0, false}, {1, true}} {
		plan, err := PlanSoleQuality(rules, 100346156, 83, 10, c.group)
		if err != nil {
			t.Fatalf("group %d at quality 83: %v", c.group, err)
		}
		if plan.GroupIndex != c.group {
			t.Fatalf("group = %d, want %d", plan.GroupIndex, c.group)
		}
		last := plan.Cost[len(plan.Cost)-1]
		if last.Gold() != c.gold {
			t.Fatalf("group %d tail = %+v, want gold=%v", c.group, last, c.gold)
		}
	}
	// 不存在的组号必须拒绝（绝不猜）。
	if _, err := PlanSoleQuality(rules, 100346156, 10, 10, 7); err == nil {
		t.Fatal("an unknown material group must be refused")
	} else if RefusalOf(err) != RefusalMaterials {
		t.Fatalf("refusal kind = %v, want materials", RefusalOf(err))
	}
}

func TestPlanSoleQualityCapsAtMaxQuality(t *testing.T) {
	rules := soleTestRules()
	// 95 + 20 会被 [max quality] 截断到 100，实际增量回填 5。
	plan, err := PlanSoleQuality(rules, 100346156, 95, 20, 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QualityAfter != 100 || plan.Gain != 5 {
		t.Fatalf("plan = after %d gain %d, want 100/5", plan.QualityAfter, plan.Gain)
	}
	// 满精度再提升必须拒绝（源里没有任何依据）。
	if _, err := PlanSoleQuality(rules, 100346156, 100, 10, 0); err == nil {
		t.Fatal("quality at the cap must be refused")
	} else if RefusalOf(err) != RefusalLimit {
		t.Fatalf("refusal kind = %v, want limit", RefusalOf(err))
	}
	// 超出源声明的上限也要拒绝（防御脏存档）。
	if _, err := PlanSoleQuality(rules, 100346156, 200, 10, 0); err == nil {
		t.Fatal("quality beyond the declared cap must be refused")
	}
}

func TestPlanSoleQualityRejectsNonSoleTemplates(t *testing.T) {
	rules := soleTestRules()
	// 100051304 是普通稀有防具（只在调适表里），不是秘宝 ⇒ 不能拿秘宝规则去提升。
	if _, err := PlanSoleQuality(rules, 100051304, 0, 10, 0); err == nil {
		t.Fatal("a non-sole template must be refused")
	} else if RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("refusal kind = %v, want unsupported", RefusalOf(err))
	}
	// 规则未装载同样拒绝（绝不猜数值）。
	if _, err := PlanSoleQuality(nil, 100346156, 0, 10, 0); err == nil {
		t.Fatal("an absent rules table must be refused")
	}
}

// 单次增量必须落在 [5,20] 且**两端都要能取到**（业主口径：每次在 5..20 之间随机）。
//
// 断言"两端都出现"是为了防住两类改坏边界又不易察觉的写法：`rand.Int(hi-lo)+lo`（取不到 20）
// 或把常量改窄。2000 次均匀采样下漏掉任一端的概率约 (15/16)^2000 ≈ 0。
func TestSoleQualityGainStaysInSourceRange(t *testing.T) {
	const rounds = 2000
	minGain, maxGain := catalog.SoleQualityGainMax+1, 0
	for i := 0; i < rounds; i++ {
		gain := soleQualityGain()
		if gain < catalog.SoleQualityGainMin || gain > catalog.SoleQualityGainMax {
			t.Fatalf("gain %d outside [%d,%d]", gain, catalog.SoleQualityGainMin, catalog.SoleQualityGainMax)
		}
		if gain < minGain {
			minGain = gain
		}
		if gain > maxGain {
			maxGain = gain
		}
	}
	if minGain != catalog.SoleQualityGainMin || maxGain != catalog.SoleQualityGainMax {
		t.Fatalf("sampled range %d..%d does not cover [%d,%d]",
			minGain, maxGain, catalog.SoleQualityGainMin, catalog.SoleQualityGainMax)
	}
}

func TestSoleQualityReceiptRoundTrip(t *testing.T) {
	receipt := SoleQualityReceipt{Template: 100346156, Container: 3, Slot: 22,
		QualityBefore: 40, QualityAfter: 55, QualityGain: 15, MaxQuality: 100, GroupIndex: 1,
		Spent: []SoleQualitySpend{{Template: 10361515, Amount: 40, FromStorage: true, StorageAmount: 40}}}
	state, err := writeSoleQualityReceipt(json.RawMessage(`{"level":115}`), "key-1", receipt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := readSoleQualityReceipt(state, "key-1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.QualityAfter != 55 || got.Slot != 22 || len(got.Spent) != 1 ||
		!got.Spent[0].FromStorage || got.Spent[0].StorageAmount != 40 {
		t.Fatalf("round trip = %+v", got)
	}
	// 未知字段必须原样保留（存档向前兼容）。
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(state, &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(fields["level"]) != "115" {
		t.Fatalf("unrelated field lost: %s", fields["level"])
	}
	if _, err := readSoleQualityReceipt(state, "key-2"); err == nil {
		t.Fatal("a mismatched key must not replay")
	}
}
