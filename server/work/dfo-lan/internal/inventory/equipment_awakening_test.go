package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

// 手工搭一张与源同形的规则表（不依赖 PVF），用来钉住判定分支。
func testAwakeningRules() *catalog.EquipmentAwakeningRules {
	rows := func(stages ...int) []catalog.EquipmentAwakeningRow {
		var out []catalog.EquipmentAwakeningRow
		for _, s := range stages {
			out = append(out, catalog.EquipmentAwakeningRow{Stage: s, Items: []catalog.EquipmentAwakeningItem{
				{Template: catalog.EquipmentAwakeningGoldTemplate, Amount: 150000},
				{Template: 10361512, Amount: 75},
			}})
		}
		return out
	}
	rates := map[int]int{0: 100, 1: 100, 2: 100, 3: 100}
	base := func(stage int, upgrades map[uint32]catalog.EquipmentAwakeningUpgrade) catalog.EquipmentAwakeningInfo {
		return catalog.EquipmentAwakeningInfo{
			Level: 115, Rarity: "rare", Stage: stage,
			Groups: []catalog.EquipmentAwakeningCostGroup{
				{Index: 1, Rows: rows(0, 1, 2, 3)},
				{Index: 2, Rows: rows(0, 1, 2, 3)},
			},
			Refunds:  []catalog.EquipmentAwakeningRow{{Stage: 0}},
			Rates:    rates,
			Upgrades: upgrades,
		}
	}
	upgrade := map[uint32]catalog.EquipmentAwakeningUpgrade{
		101001149: {Count: 1, Targets: []uint32{101001150}},
		117010253: {Count: 0},
	}
	return &catalog.EquipmentAwakeningRules{
		MaxLevel: 3,
		Infos: []catalog.EquipmentAwakeningInfo{
			base(0, upgrade), base(1, upgrade), base(2, upgrade), base(3, upgrade),
		},
	}
}

func TestPlanAwakeningAdvancesStage(t *testing.T) {
	plan, err := PlanAwakening(testAwakeningRules(), 101001149, 115, 2, 0, 1, protocol.EquipmentAwakeningNoTarget)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if plan.Upgraded || plan.StageAfter != 1 || plan.TemplateAfter != 101001149 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Group.Index != 1 || len(plan.Cost.Items) != 2 || plan.Rate != 100 {
		t.Fatalf("plan cost/group = %+v", plan)
	}
}

func TestPlanAwakeningRejectsForeignTargetBeforeCap(t *testing.T) {
	_, err := PlanAwakening(testAwakeningRules(), 101001149, 115, 2, 0, 1, 999999)
	if RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("err = %v (kind %v), want unsupported", err, RefusalOf(err))
	}
}

func TestPlanAwakeningUpgradesAtCap(t *testing.T) {
	plan, err := PlanAwakening(testAwakeningRules(), 101001149, 115, 2, 3, 2, 101001150)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if !plan.Upgraded || plan.StageAfter != 0 || plan.TemplateAfter != 101001150 {
		t.Fatalf("plan = %+v", plan)
	}
	if plan.Group.Index != 2 {
		t.Fatalf("group = %d, want 2", plan.Group.Index)
	}
}

func TestPlanAwakeningRejectsBadUpgradeTargets(t *testing.T) {
	rules := testAwakeningRules()
	if _, err := PlanAwakening(rules, 101001149, 115, 2, 3, 1, 999999); RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("unknown candidate: err = %v (kind %v)", err, RefusalOf(err))
	}
	if _, err := PlanAwakening(rules, 101001149, 115, 2, 3, 1, protocol.EquipmentAwakeningNoTarget); RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("missing candidate: err = %v (kind %v)", err, RefusalOf(err))
	}
	// 源里候选数为 0 的模板（太初装备全是这种）必须在阶段 3 被拒。
	if _, err := PlanAwakening(rules, 117010253, 115, 2, 3, 1, 101001150); RefusalOf(err) != RefusalLimit {
		t.Fatalf("zero-candidate template: err = %v (kind %v)", err, RefusalOf(err))
	}
}

func TestPlanAwakeningRejectsUnknownShape(t *testing.T) {
	rules := testAwakeningRules()
	if _, err := PlanAwakening(rules, 101001149, 115, 2, 0, 7, protocol.EquipmentAwakeningNoTarget); RefusalOf(err) != RefusalMaterials {
		t.Fatalf("unknown group: err = %v (kind %v)", err, RefusalOf(err))
	}
	if _, err := PlanAwakening(rules, 101001149, 115, 2, 9, 1, protocol.EquipmentAwakeningNoTarget); RefusalOf(err) != RefusalLimit {
		t.Fatalf("unknown stage: err = %v (kind %v)", err, RefusalOf(err))
	}
	if _, err := PlanAwakening(rules, 101001149, 115, 8, 0, 1, protocol.EquipmentAwakeningNoTarget); RefusalOf(err) != RefusalLimit {
		t.Fatalf("unsupported rarity: err = %v (kind %v)", err, RefusalOf(err))
	}
	if _, err := PlanAwakening(nil, 101001149, 115, 2, 0, 1, 0); RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("missing rules: err = %v (kind %v)", err, RefusalOf(err))
	}
}

func TestPlanAwakeningRejectsNonFullRate(t *testing.T) {
	rules := testAwakeningRules()
	rules.Infos[0].Rates = map[int]int{0: 50, 1: 100, 2: 100, 3: 100}
	if _, err := PlanAwakening(rules, 101001149, 115, 2, 0, 1, protocol.EquipmentAwakeningNoTarget); RefusalOf(err) != RefusalUnsupported {
		t.Fatalf("non-full rate: err = %v (kind %v)", err, RefusalOf(err))
	}
}

func TestConsumeBagTemplate(t *testing.T) {
	bag := Bag{Items: []BagItem{
		{Slot: 3, Template: 10361512, Amount: 100},
		{Slot: 4, Template: 10361512, Amount: 50},
		{Slot: 5, Template: 10400396, Amount: 2},
	}}
	next, taken, err := consumeBagTemplate(bag, 10361512, 120)
	if err != nil {
		t.Fatalf("consume: %v", err)
	}
	if taken != 120 {
		t.Fatalf("taken = %d, want 120", taken)
	}
	if len(next.Items) != 2 {
		t.Fatalf("items = %+v, want the first stack removed", next.Items)
	}
	if next.Items[0].Slot != 4 || next.Items[0].Amount != 30 {
		t.Fatalf("first stack = %+v, want slot 4 with 30", next.Items[0])
	}
	// 原背包不被就地改写（事务里要能整体回滚）。
	if bag.Items[0].Amount != 100 || len(bag.Items) != 3 {
		t.Fatalf("source bag mutated: %+v", bag.Items)
	}
	if _, _, err := consumeBagTemplate(bag, 10361512, 151); RefusalOf(err) != RefusalMaterials {
		t.Fatalf("short fall: err = %v (kind %v)", err, RefusalOf(err))
	}
	if _, _, err := consumeBagTemplate(bag, 999, 1); RefusalOf(err) != RefusalMaterials {
		t.Fatalf("absent template: err = %v (kind %v)", err, RefusalOf(err))
	}
}

func TestAwakeningReceiptRoundTrip(t *testing.T) {
	receipt := AwakeningReceipt{StageBefore: 0, StageAfter: 1, TemplateBefore: 101001149, TemplateAfter: 101001149,
		Spent: []AwakeningSpend{{Template: 10361512, Amount: 75, FromStorage: true}}}
	state, err := writeAwakeningReceipt(json.RawMessage(`{"level":115}`), "key-1", receipt)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ReadAwakeningReceipt(state, "key-1")
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got.StageAfter != 1 || got.TemplateBefore != 101001149 || len(got.Spent) != 1 || !got.Spent[0].FromStorage {
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
	if _, err := ReadAwakeningReceipt(state, "key-2"); err == nil {
		t.Fatal("a mismatched key must not replay")
	}
}
