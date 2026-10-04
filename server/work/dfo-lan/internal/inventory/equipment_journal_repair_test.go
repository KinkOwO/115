package inventory

import (
	"dfolan/internal/catalog"
	"testing"
)

// 装备库账本对账（`equipment_journal_repair.go`）的用例。
//
// 真源链：角色实际带着的装备（穿戴 + 背包装备区）→ carried → 账本份数。
// 上限判据一律走 `JournalLimit`（与分解/变换同一条），所以规则表用与真实源同形的合成表。

func repairRules() catalog.EquipmentJournalRules {
	// `[oath]` 这类按类型收紧到 1（真实源里只有它被显式声明）。
	return catalog.EquipmentJournalRules{
		Maximum:       99,
		MaximumByType: []catalog.JournalTypeLimit{{Kind: "[oath]", Rarity: 2, Maximum: 1}},
	}
}

func repairService(t *testing.T) *ItemService {
	t.Helper()
	s, _ := primerTransformService(t)
	return s
}

// 只报告：不改账本，把"漏记"与"虚计"都列出来。
func TestJournalReconcileReportOnly(t *testing.T) {
	s := repairService(t)
	rules := repairRules()
	s.Journal = &rules
	bag := Bag{Version: "ordinary-bag-v1",
		Worn:      []BagEquipment{{Slot: 36, Template: 100401592}}, // 戴着但账上没有
		Equipment: []BagEquipment{{Slot: 50, Template: 100401597}},
	}
	ledger := EquipmentJournal{Counts: map[uint32]uint32{100401600: 5, 100401597: 1}}

	out, report := ReconcileEquipmentJournal(bag, ledger, s.Equipment, s.Journal, JournalReconcileReportOnly)
	if got := len(report.Changes); got != 0 {
		t.Fatalf("report 模式不许产生改动：%+v", report.Changes)
	}
	if len(report.Missing) != 1 || report.Missing[0] != 100401592 {
		t.Fatalf("missing = %v, want [100401592]", report.Missing)
	}
	if len(report.Orphans) != 1 || report.Orphans[0] != 100401600 {
		t.Fatalf("orphans = %v, want [100401600]", report.Orphans)
	}
	// 账本必须一字未动。
	if out.Counts[100401600] != 5 || out.Counts[100401597] != 1 || out.Counts[100401592] != 0 {
		t.Fatalf("report 模式不许改账本：%v", out.Counts)
	}
}

// 补记：把"身上带着却没登记"的条目补回来（**不减少任何已有份数**）。
func TestJournalReconcileRestoresCarriedTemplates(t *testing.T) {
	s := repairService(t)
	rules := repairRules()
	s.Journal = &rules
	const crystal = uint32(100401592)
	if limit, ok := JournalLimit(s.Equipment, s.Journal, crystal); !ok || limit == 0 {
		t.Fatalf("前提失效：%d 不可登记（limit=%d/%v）", crystal, limit, ok)
	}
	bag := Bag{Version: "ordinary-bag-v1",
		Worn: []BagEquipment{{Slot: 36, Template: crystal}, {Slot: 37, Template: crystal}},
	}
	ledger := EquipmentJournal{Counts: map[uint32]uint32{100401600: 5, 100401597: 0}}
	out, report := ReconcileEquipmentJournal(bag, ledger, s.Equipment, s.Journal, JournalReconcileRestoreCarried)
	if len(report.Changes) != 1 {
		t.Fatalf("changes = %+v, want exactly the missing carried template", report.Changes)
	}
	change := report.Changes[0]
	if change.Template != crystal || change.Before != 0 || change.After != 2 || change.Carried != 2 {
		t.Fatalf("change = %+v, want {tpl:%d before:0 after:2 carried:2}", change, crystal)
	}
	if out.Counts[crystal] != 2 {
		t.Fatalf("counts = %v, want the two carried crystals registered", out.Counts)
	}
	// 既有的份数一个都不许少（包括那条 0 份的"已登记"记录）。
	if out.Counts[100401600] != 5 {
		t.Fatalf("existing counts must be preserved: %v", out.Counts)
	}
	if _, kept := out.Counts[100401597]; !kept {
		t.Fatalf("「已登记但 0 份」的条目必须保留（与「从未登记」是两种状态）：%v", out.Counts)
	}
}

// 不在收录范围的模板**不补记**，只如实报告原因（与分解/变换同一条判据）。
func TestJournalReconcileSkipsUnregistrable(t *testing.T) {
	s := repairService(t)
	rules := repairRules()
	s.Journal = &rules
	const primeval = uint32(100401635)
	if _, ok := JournalLimit(s.Equipment, s.Journal, primeval); ok {
		t.Skipf("%d 在本夹具里可登记，本用例不再适用", primeval)
	}
	bag := Bag{Version: "ordinary-bag-v1", Worn: []BagEquipment{{Slot: 44, Template: primeval}}}
	out, report := ReconcileEquipmentJournal(bag, EquipmentJournal{Counts: map[uint32]uint32{}},
		s.Equipment, s.Journal, JournalReconcileRestoreCarried)
	if len(report.Changes) != 1 || report.Changes[0].After != 0 {
		t.Fatalf("changes = %+v, want one refused change", report.Changes)
	}
	if len(out.Counts) != 0 {
		t.Fatalf("不可登记的模板不许写进账本：%v", out.Counts)
	}
}

// 补记受上限约束：誓约核心每模板只允许 1 份。
func TestJournalReconcileRespectsTypeLimit(t *testing.T) {
	s := repairService(t)
	const oath = uint32(100610079)
	_, rarity, ok := s.equipmentGradeRarity(oath)
	if !ok {
		t.Fatalf("读不到 %d 的稀有度", oath)
	}
	// 用真实稀有度声明"这一类上限 1"。
	s.Journal = &catalog.EquipmentJournalRules{
		Maximum:       99,
		MaximumByType: []catalog.JournalTypeLimit{{Kind: "[oath]", Rarity: uint32(rarity), Maximum: 1}},
	}
	bag := Bag{Version: "ordinary-bag-v1",
		Worn: []BagEquipment{{Slot: 47, Template: oath}, {Slot: 48, Template: oath}},
	}
	out, report := ReconcileEquipmentJournal(bag, EquipmentJournal{Counts: map[uint32]uint32{}},
		s.Equipment, s.Journal, JournalReconcileRestoreCarried)
	if len(report.Changes) != 1 || !report.Changes[0].Capped {
		t.Fatalf("changes = %+v, want one capped change", report.Changes)
	}
	if out.Counts[oath] != 1 {
		t.Fatalf("counts[oath] = %d, want the 1-per-template cap", out.Counts[oath])
	}
}

// 清零模式：只有明确要求时才把"账上有、手上没有"的份数清掉，且 **0 份条目仍然保留**。
func TestJournalReconcileZeroUncarriedKeepsZeroRows(t *testing.T) {
	s := repairService(t)
	rules := repairRules()
	s.Journal = &rules
	bag := Bag{Version: "ordinary-bag-v1"}
	ledger := EquipmentJournal{Counts: map[uint32]uint32{100401600: 5, 100323436: 2}}
	out, report := ReconcileEquipmentJournal(bag, ledger, s.Equipment, s.Journal, JournalReconcileZeroUncarried)
	if len(report.Zeroed) != 2 {
		t.Fatalf("zeroed = %+v, want both orphan entries", report.Zeroed)
	}
	if out.Counts[100401600] != 0 || out.Counts[100323436] != 0 {
		t.Fatalf("counts = %v, want zeros", out.Counts)
	}
	if len(out.Counts) != 2 {
		t.Fatalf("0 份条目必须保留（删掉就等于「从未登记」）：%v", out.Counts)
	}
}

// 拿不到规则表时只做"补 1 份"的最小修复，绝不凭空加量。
func TestJournalReconcileWithoutRulesOnlyRestoresOne(t *testing.T) {
	s := repairService(t)
	bag := Bag{Version: "ordinary-bag-v1",
		Worn: []BagEquipment{{Slot: 36, Template: 100401635}, {Slot: 44, Template: 100401635}},
	}
	out, report := ReconcileEquipmentJournal(bag, EquipmentJournal{Counts: map[uint32]uint32{}},
		s.Equipment, nil, JournalReconcileRestoreCarried)
	if len(report.Changes) != 1 || report.Changes[0].After != 1 {
		t.Fatalf("changes = %+v, want a minimal 1-copy restore", report.Changes)
	}
	if out.Counts[100401635] != 1 {
		t.Fatalf("counts = %v, want 1 (no invented copies)", out.Counts)
	}
}

// 模式名解析。
func TestJournalReconcileModeParsing(t *testing.T) {
	for name, want := range map[string]JournalReconcileMode{
		"":                JournalReconcileReportOnly,
		"report":          JournalReconcileReportOnly,
		"restore-carried": JournalReconcileRestoreCarried,
		"zero-uncarried":  JournalReconcileZeroUncarried,
	} {
		got, e := JournalReconcileModeFromString(name)
		if e != nil || got != want {
			t.Fatalf("JournalReconcileModeFromString(%q) = %v/%v, want %v", name, got, e, want)
		}
	}
	if _, e := JournalReconcileModeFromString("nope"); e == nil {
		t.Fatal("unknown mode must be refused")
	}
}
