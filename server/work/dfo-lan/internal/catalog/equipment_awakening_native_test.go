package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

// 真实内层归档对照：把直读解析器压到当次源上。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115us\server\work\client-build\Script.inner.pvf
//	go test ./internal/catalog/ -run AwakeningNative -count=1
//
// 未设置环境变量时跳过（与其它 *_native_test.go 同一约定）。
func openAwakeningArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	p := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if p == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native awakening parity")
	}
	a, err := OpenTestArchiveCached(p, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatalf("open archive: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })
	return a
}

func TestEquipmentAwakeningRulesNative(t *testing.T) {
	a := openAwakeningArchive(t)
	rules, err := ImportEquipmentAwakeningRules(a)
	if err != nil {
		t.Fatalf("import rules: %v", err)
	}
	if rules.MaxLevel != 3 {
		t.Fatalf("max awakening = %d, want 3", rules.MaxLevel)
	}
	// 源当前是 5 种品质 ×（0,1,2,3,5）去掉 primeval 的 4 档 —— 共 21 条。
	if len(rules.Infos) != 21 {
		t.Fatalf("infos = %d, want 21", len(rules.Infos))
	}
	for _, rarity := range []string{"rare", "unique", "legendary", "epic"} {
		if stages := rules.StagesFor(115, rarity); len(stages) != 5 {
			t.Fatalf("%s stages = %v, want 5 entries", rarity, stages)
		}
	}
	if stages := rules.StagesFor(115, "primeval"); len(stages) != 1 || stages[0] != 0 {
		t.Fatalf("primeval stages = %v, want [0]", stages)
	}

	info, ok := rules.Info(115, "rare", 0)
	if !ok {
		t.Fatal("missing (115, rare, 0)")
	}
	group, ok := info.Group(1)
	if !ok {
		t.Fatal("missing group 1")
	}
	row, ok := group.Row(0)
	if !ok {
		t.Fatal("missing stage 0 row of group 1")
	}
	want := []EquipmentAwakeningItem{
		{Template: EquipmentAwakeningGoldTemplate, Amount: 150000},
		{Template: 10361512, Amount: 75},
	}
	if len(row.Items) != len(want) {
		t.Fatalf("stage 0 items = %v, want %v", row.Items, want)
	}
	for i := range want {
		if row.Items[i] != want[i] {
			t.Fatalf("stage 0 item %d = %v, want %v", i, row.Items[i], want[i])
		}
	}

	// 太初（primeval）装备在源里没有升品候选（全 0 候选），必须如实反映。
	primeval, ok := rules.Info(115, "primeval", 0)
	if !ok {
		t.Fatal("missing (115, primeval, 0)")
	}

	// 跨块升品（实机 2026-10-02 的根因）：`100051304`（稀有防具）的升品条目写在
	// `[condition] 115 rare 1` 块里，而升品动作发生在**阶 3** ⇒ 只按当前档的块查会
	// 永远查不到、每次升品都被拒（现象："1–3 次成功，第 4 次无法升品"）。
	crossBlock, ok := rules.UpgradeSource(100051304)
	if !ok {
		t.Fatal("100051304 must be resolvable through the cross-block upgrade table")
	}
	if len(crossBlock.Targets) != 12 {
		t.Fatalf("100051304 candidates = %d (%v), want 12", len(crossBlock.Targets), crossBlock.Targets)
	}
	// 反证：它**不在** rare 3 块里 —— 若哪天源改了、它出现在同一块，本注释与实现都要重看。
	if rare3, ok := rules.Info(115, "rare", 3); ok {
		if _, exists := rare3.Upgrade(100051304); exists {
			t.Fatal("100051304 unexpectedly lives in the rare 3 block; the cross-block merge comment is stale")
		}
	}
	// 阶 3 的成本行必须是实机截图那一行：100000 金币 + 10361513×40 + 10400396×1。
	rare3, ok := rules.Info(115, "rare", 3)
	if !ok {
		t.Fatal("missing (115, rare, 3)")
	}
	rareGroup, ok := rare3.Group(1)
	if !ok {
		t.Fatal("missing rare 3 group 1")
	}
	rareRow, ok := rareGroup.Row(3)
	if !ok {
		t.Fatal("missing rare stage 3 row")
	}
	wantRare3 := []EquipmentAwakeningItem{
		{Template: EquipmentAwakeningGoldTemplate, Amount: 100000},
		{Template: 10361513, Amount: 40},
		{Template: 10400396, Amount: 1},
	}
	if len(rareRow.Items) != len(wantRare3) {
		t.Fatalf("rare stage 3 items = %v, want %v", rareRow.Items, wantRare3)
	}
	for i := range wantRare3 {
		if rareRow.Items[i] != wantRare3[i] {
			t.Fatalf("rare stage 3 item %d = %v, want %v", i, rareRow.Items[i], wantRare3[i])
		}
	}
	// epic 阶段 0 的成本行（实机 100261128 走的就是这块）：金币 1,500,000 + 10361515×15。
	epic, ok := rules.Info(115, "epic", 0)
	if !ok {
		t.Fatal("missing (115, epic, 0)")
	}
	epicGroup, ok := epic.Group(1)
	if !ok {
		t.Fatal("missing epic group 1")
	}
	epicRow, ok := epicGroup.Row(0)
	if !ok {
		t.Fatal("missing epic stage 0 row")
	}
	wantEpic := []EquipmentAwakeningItem{
		{Template: EquipmentAwakeningGoldTemplate, Amount: 1500000},
		{Template: 10361515, Amount: 15},
	}
	if len(epicRow.Items) != len(wantEpic) {
		t.Fatalf("epic stage 0 items = %v, want %v", epicRow.Items, wantEpic)
	}
	for i := range wantEpic {
		if epicRow.Items[i] != wantEpic[i] {
			t.Fatalf("epic stage 0 item %d = %v, want %v", i, epicRow.Items[i], wantEpic[i])
		}
	}
	if up, ok := primeval.Upgrade(117010253); !ok || up.Count != 0 || len(up.Targets) != 0 {
		t.Fatalf("117010253 = %+v/%v, want present with zero candidates", up, ok)
	}
	// rare 阶段的升品映射：101001149（稀有）→ 101001150。
	if up, ok := info.Upgrade(101001149); !ok || len(up.Targets) != 1 || up.Targets[0] != 101001150 {
		t.Fatalf("upgrade(101001149) = %+v/%v", up, ok)
	}
	// 每个材料行都必须有对应的成功率（本版本全 100）—— 这是"能安全结算"的充要条件。
	for _, i := range rules.Infos {
		for _, g := range i.Groups {
			for _, row := range g.Rows {
				if rate, ok := i.Rate(row.Stage); !ok || rate != 100 {
					t.Fatalf("(115, %s, %d) group %d stage %d rate = %d/%v, want 100",
						i.Rarity, i.Stage, g.Index, row.Stage, rate, ok)
				}
			}
		}
	}
}

func TestEquipmentAwakeningOptionsNative(t *testing.T) {
	a := openAwakeningArchive(t)
	options, err := ImportEquipmentAwakeningOptions(a)
	if err != nil {
		t.Fatalf("import options: %v", err)
	}
	if len(options.Entries) != 263 {
		t.Fatalf("option entries = %d, want 263", len(options.Entries))
	}
	// 实机样本 117010280（lbow 升品后模板）的 `[equipment awakening option]` = 175。
	path, ok := options.Path(175)
	if !ok {
		t.Fatal("missing option 175")
	}
	const want = "equipmentawakeningoption/legacyweapon/lbow/equipmentawakening_primval_up.etc"
	if path != want {
		t.Fatalf("option 175 = %q, want %q", path, want)
	}
	full := "etc/115lvability/" + path
	if _, ok := a.FindFile(full); !ok {
		t.Fatalf("%s is missing from the archive", full)
	}
}
