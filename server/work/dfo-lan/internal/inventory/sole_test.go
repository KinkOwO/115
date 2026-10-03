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

// withSoleNative 在原版（国服）口径下跑一个测试，结束后恢复默认（单机）口径。
func withSoleNative(t *testing.T) {
	t.Helper()
	SetSoleQualityNative(true)
	t.Cleanup(func() { SetSoleQualityNative(false) })
}

// 单次增量遵守国服的分段规则：保底 +1、不跨段、节点后必暴击。（开关打开时）
//
// 业主口径（2026-10-02）：全程只涨不掉；每段上限来自源 `[quality group]`
// （0/25/50/75/100），单次提升不得越过当前段上限。
func TestSoleQualityGainRespectsBandsAndFloor(t *testing.T) {
	withSoleNative(t)
	info, ok := soleTestRules().Info(100346156)
	if !ok {
		t.Fatal("test rules lost 100346156")
	}
	for q := 0; q < info.MaxQuality; q++ {
		for i := 0; i < 40; i++ {
			gain := soleQualityGain(info, q)
			if gain < 1 {
				t.Fatalf("gain %d at quality %d: must always advance (100%% success)", gain, q)
			}
			if cap := info.BandCap(q); q+gain > cap {
				t.Fatalf("gain %d at quality %d crosses the band cap %d", gain, q, cap)
			}
		}
	}
	// 满精度没有增量可言。
	if gain := soleQualityGain(info, info.MaxQuality); gain != 0 {
		t.Fatalf("gain at the cap = %d, want 0", gain)
	}
}

// 分段节点（25/50/75）之后必定大成功：绝不可能只 +1。（开关打开时）
func TestSoleQualityGainAfterBandNodeIsAlwaysGreat(t *testing.T) {
	withSoleNative(t)
	info, _ := soleTestRules().Info(100346156)
	for _, q := range []int{25, 50, 75} {
		if !info.BandNode(q) {
			t.Fatalf("quality %d should be a band node", q)
		}
		for i := 0; i < 200; i++ {
			if gain := soleQualityGain(info, q); gain < 2 {
				t.Fatalf("node %d rolled gain %d, want >= 2", q, gain)
			}
		}
	}
	// 末段上限不是节点（到顶就该停手）。
	if info.BandNode(info.MaxQuality) {
		t.Fatal("the max quality is not a band node")
	}
}

// 临界值（24/49/74/99）永远只 +1：保底 1 加封顶，结果只能是节点本身。（开关打开时）
func TestSoleQualityBandEdgeAlwaysArrivesAtNode(t *testing.T) {
	withSoleNative(t)
	info, _ := soleTestRules().Info(100346156)
	for _, q := range []int{24, 49, 74, 99} {
		for i := 0; i < 100; i++ {
			if gain := soleQualityGain(info, q); gain != 1 {
				t.Fatalf("edge %d rolled gain %d, want exactly 1", q, gain)
			}
		}
	}
}

// 原版口径下从 0 走到满精度：只涨不掉、从不跨段，并记录实测期望次数。
func TestSoleQualityWalkReachesMaxWithoutCrossingBands(t *testing.T) {
	withSoleNative(t)
	info, _ := soleTestRules().Info(100346156)
	const trials = 300
	total := 0
	for i := 0; i < trials; i++ {
		steps, q := 0, 0
		for q < info.MaxQuality {
			gain := soleQualityGain(info, q)
			if gain < 1 {
				t.Fatalf("walk stalled at quality %d (gain %d)", q, gain)
			}
			if cap := info.BandCap(q); q+gain > cap {
				t.Fatalf("walk from %d with gain %d crosses band cap %d", q, gain, cap)
			}
			q += gain
			steps++
			if steps > 500 {
				t.Fatalf("walk from 0 did not terminate (at %d)", q)
			}
		}
		total += steps
	}
	// 宽带上界，防止将来把概率/幅度改到离谱的区间；业主目标 ~25 次。
	avg := total / trials
	if avg < 10 || avg > 90 {
		t.Fatalf("average %d refinements to reach max quality is out of a sane range", avg)
	}
	t.Logf("原版口径实测平均 %d 次满精度（业主口径约 25 次）", avg)
}

// 默认（单机）口径：单次增量落在 5..20 且两端都能取到 —— 与开关打开时完全不同。
func TestSoleQualityGainDefaultIsSoloRange(t *testing.T) {
	SetSoleQualityNative(false)
	if SoleQualityNative() {
		t.Fatal("默认必须是单机口径（开关关）")
	}
	info, _ := soleTestRules().Info(100346156)
	minGain, maxGain := catalog.SoleQualityGainMax+1, 0
	for i := 0; i < 3000; i++ {
		gain := soleQualityGain(info, 0)
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
	// 默认口径**不做分段封顶**：20 + 20 可以直接到 40（原版会停在 25 段附近的可达范围内）。
	plan, err := PlanSoleQuality(soleTestRules(), 100346156, 20, 20, 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QualityAfter != 40 || plan.Gain != 20 {
		t.Fatalf("plan = after %d gain %d, want 40/20（默认口径按 [max quality] 封顶）", plan.QualityAfter, plan.Gain)
	}
}

// 判定侧必须尊重分段上限：超出当前段的部分一律截断（不跨段）。（开关打开时）
func TestPlanSoleQualityCapsWithinBand(t *testing.T) {
	withSoleNative(t)
	rules := soleTestRules()
	// 30 在 [25,50) 段 ⇒ 一次 +20 只能到 50，绝不能跨到 75。
	plan, err := PlanSoleQuality(rules, 100346156, 30, 20, 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QualityAfter != 50 || plan.Gain != 20 {
		t.Fatalf("plan = after %d gain %d, want 50/20", plan.QualityAfter, plan.Gain)
	}
	// 30 + 40 也只能到该段上限 50。
	plan, err = PlanSoleQuality(rules, 100346156, 30, 40, 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QualityAfter != 50 || plan.Gain != 20 {
		t.Fatalf("plan = after %d gain %d, want 50/20", plan.QualityAfter, plan.Gain)
	}
	// 末段 [75,100) 的上限就是 [max quality]。
	plan, err = PlanSoleQuality(rules, 100346156, 99, 50, 0)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.QualityAfter != 100 || plan.Gain != 1 {
		t.Fatalf("plan = after %d gain %d, want 100/1", plan.QualityAfter, plan.Gain)
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
	got, err := ReadSoleQualityReceipt(state, "key-1")
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
	if _, err := ReadSoleQualityReceipt(state, "key-2"); err == nil {
		t.Fatal("a mismatched key must not replay")
	}
}
