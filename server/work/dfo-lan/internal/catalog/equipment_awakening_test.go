package catalog

import (
	"strings"
	"testing"
)

// 一段与源同形的最小规则表：`[condition]` 两档品质、两个材料组、返还与升品映射齐全。
const awakeningSystemSample = `
[max awakening] 3
[infos]
 [info]
  [condition] 115 ` + "`rare`" + ` 0
  [need materials]
   [group] 1
    0 2 0 150000 10361512 75
    1 2 0 150000 10361512 75
    2 2 0 150000 10361512 75
    3 3 0 150000 10361513 60 10400396 2
   [/group]
   [group] 2
    0 2 10401346 30 10361512 75
    1 2 10401346 30 10361512 75
    2 2 10401346 30 10361512 75
    3 3 10401346 30 10361513 60 10400396 2
   [/group]
  [/need materials]
  [refund materials]
   0 0
   1 1 10361512 75
   2 1 10361512 150
   3 1 10361512 225
  [/refund materials]
  [rates]
   0 100
   1 100
   2 100
   3 100
  [/rates]
  [upgrade result]
   101001149 1 101001150
   117010253 0
  [/upgrade result]
 [/info]
 [info]
  [condition] 115 ` + "`primeval`" + ` 5
  [need materials]
   [group] 1
    0 2 0 80000 10415191 15
   [/group]
  [/need materials]
  [refund materials]
   0 0
  [/refund materials]
  [rates]
   0 100
  [/rates]
  [upgrade result]
   100401592 2 100401596 100401600
  [/upgrade result]
 [/info]
[/infos]
`

func TestParseEquipmentAwakeningRules(t *testing.T) {
	rules, err := ParseEquipmentAwakeningRules(awakeningSystemSample)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rules.MaxLevel != 3 {
		t.Fatalf("max awakening = %d, want 3", rules.MaxLevel)
	}
	if len(rules.Infos) != 2 {
		t.Fatalf("infos = %d, want 2", len(rules.Infos))
	}

	info, ok := rules.Info(115, "rare", 0)
	if !ok {
		t.Fatal("missing (115, rare, 0)")
	}
	if got := info.GroupIndexes(); len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Fatalf("group indexes = %v, want [1 2]", got)
	}
	group, ok := info.Group(1)
	if !ok {
		t.Fatal("missing group 1")
	}
	row, ok := group.Row(3)
	if !ok {
		t.Fatal("missing stage 3 row")
	}
	want := []EquipmentAwakeningItem{
		{Template: EquipmentAwakeningGoldTemplate, Amount: 150000},
		{Template: 10361513, Amount: 60},
		{Template: 10400396, Amount: 2},
	}
	if len(row.Items) != len(want) {
		t.Fatalf("stage 3 items = %v, want %v", row.Items, want)
	}
	for i := range want {
		if row.Items[i] != want[i] {
			t.Fatalf("stage 3 item %d = %v, want %v", i, row.Items[i], want[i])
		}
	}
	if !row.Items[0].Gold() {
		t.Fatal("first stage-3 item must be gold")
	}
	if rate, ok := info.Rate(2); !ok || rate != 100 {
		t.Fatalf("rate(2) = %d/%v, want 100", rate, ok)
	}
	up, ok := info.Upgrade(101001149)
	if !ok || up.Count != 1 || len(up.Targets) != 1 || up.Targets[0] != 101001150 {
		t.Fatalf("upgrade(101001149) = %+v/%v", up, ok)
	}
	if up, ok := info.Upgrade(117010253); !ok || up.Count != 0 || len(up.Targets) != 0 {
		t.Fatalf("117010253 must be present with zero candidates, got %+v/%v", up, ok)
	}
	if refund, ok := info.Refund(2); !ok || len(refund.Items) != 1 || refund.Items[0].Amount != 150 {
		t.Fatalf("refund(2) = %+v/%v", refund, ok)
	}
	// 跨块升品：第二条 `[condition]`（primeval 5）里的条目也必须能被全表查到 ——
	// 升品动作发生在阶 3，而源把不同来源的候选分散写在任意块里（见 Rules.Upgrades）。
	if up, ok := rules.UpgradeSource(100401592); !ok || len(up.Targets) != 2 {
		t.Fatalf("cross-block upgrade(100401592) = %+v/%v, want 2 candidates", up, ok)
	}
	if up, ok := rules.UpgradeSource(101001149); !ok || len(up.Targets) != 1 || up.Targets[0] != 101001150 {
		t.Fatalf("cross-block upgrade(101001149) = %+v/%v", up, ok)
	}
	if _, ok := rules.UpgradeSource(99999999); ok {
		t.Fatal("an unknown template must not resolve to an upgrade")
	}

	if _, ok := rules.Info(115, "rare", 5); ok {
		t.Fatal("(115, rare, 5) must not exist")
	}
	if stages := rules.StagesFor(115, "primeval"); len(stages) != 1 || stages[0] != 5 {
		t.Fatalf("primeval stages = %v, want [5]", stages)
	}
	// 源里"项目数"是上限（数量为 0 的项被省略），所以声明 3 只带 2 对必须接受。
	if _, err := ParseEquipmentAwakeningRules(strings.Replace(awakeningSystemSample, "3 3 0 150000 10361513 60 10400396 2", "3 3 0 150000 10361513 60", 1)); err != nil {
		t.Fatalf("an omitted zero-amount item must be accepted: %v", err)
	}
	// 反过来，实际项目数超过声明值必须报错。
	if _, err := ParseEquipmentAwakeningRules(strings.Replace(awakeningSystemSample, "3 3 0 150000 10361513 60 10400396 2", "3 1 0 150000 10361513 60", 1)); err == nil {
		t.Fatal("more items than declared must be rejected")
	}
	if _, err := ParseEquipmentAwakeningRules(strings.Replace(awakeningSystemSample, "100401592 2 100401596 100401600", "100401592 1 100401596 100401600", 1)); err == nil {
		t.Fatal("more upgrade candidates than declared must be rejected")
	}
	// 缺 [max awakening] ⇒ 必须报错（不能默认某个上限）。
	if _, err := ParseEquipmentAwakeningRules(strings.Replace(awakeningSystemSample, "[max awakening] 3", "[max awakening] 0", 1)); err == nil {
		t.Fatal("a zero [max awakening] must be rejected")
	}
}

func TestEquipmentAwakeningRarityNames(t *testing.T) {
	cases := map[int]string{2: "rare", 3: "unique", 4: "epic", 6: "legendary", 8: "primeval"}
	for value, want := range cases {
		got, ok := EquipmentAwakeningRarityName(value)
		if !ok || got != want {
			t.Fatalf("rarity %d = %q/%v, want %q", value, got, ok, want)
		}
	}
	if _, ok := EquipmentAwakeningRarityName(9); ok {
		t.Fatal("rarity 9 must be out of range")
	}
}
