package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"fmt"
	"testing"
)

// fakeEquipmentCatalog 以**接口**形式注入装备定义。
//
// 本测试在 loot 包内，够不到 inventory.EquipmentCatalog 的未导出索引（index 是小写），
// 而 JournalLimit 只要求 inventory.EquipmentDefinitioner —— 正好可以用最小实现顶上。
// 生产路径传的是服务端真实目录（Service.Equipment），同一套判据。
type fakeEquipmentCatalog map[uint32]inventory.EquipmentDefinition

func (f fakeEquipmentCatalog) Definition(id uint32) (inventory.EquipmentDefinition, error) {
	d, ok := f[id]
	if !ok {
		return d, fmt.Errorf("equipment definition missing: %d", id)
	}
	return d, nil
}

// journalDef 造一条装备定义。收录判据只读三个字段：
// [minimum level]（必须 == 115）、[rarity]（必须在 {2,3,4,6,8}）、[equipment type]（收紧上限用）。
func journalDef(id uint32, minimumLevel, rarity int32, kind string) inventory.EquipmentDefinition {
	fields := map[string][]pvf.Token{
		"[minimum level]": {{Type: 0, Value: minimumLevel}},
		"[rarity]":        {{Type: 0, Value: rarity}},
	}
	if kind != "" {
		fields["[equipment type]"] = []pvf.Token{{Type: 3, Text: kind}}
	}
	return inventory.EquipmentDefinition{ID: id, Fields: fields}
}

// 规格 CMD/0026-DISJOINTITEM：CMD26「分解」同时就是客户端的「装备库添加」。
// 玩家 2026-09-30 报告里那只耳环 100391006（minimum level 115 / rarity 6）正是该被收录的那类。
func TestJournalRegistrationsAddsDeletedEquipment(t *testing.T) {
	rules := catalog.EquipmentJournalRules{Maximum: 99}
	cat := fakeEquipmentCatalog{
		100391006: journalDef(100391006, 115, 6, "[earring]"),
		100051285: journalDef(100051285, 115, 2, ""),
	}
	// 登记用的模板来自**服务端背包**那一行（bySlot 由 Disjoint 之前的背包快照构建），
	// 不是请求里客户端上报的 Template。
	bySlot := map[uint16]uint32{12: 100391006, 13: 100051285}

	ledger, added, skipped, e := journalRegistrations(
		inventory.EquipmentJournal{}, bySlot, []uint16{12, 13}, cat, &rules)
	if e != nil {
		t.Fatalf("registrations: %v", e)
	}
	if len(skipped) != 0 {
		t.Fatalf("unexpected skips: %+v", skipped)
	}
	if len(added) != 2 {
		t.Fatalf("added = %+v, want 2", added)
	}
	if ledger.Counts[100391006] != 1 || ledger.Counts[100051285] != 1 {
		t.Fatalf("counts = %+v, want both 1", ledger.Counts)
	}
	// 上限回落：[max equipment count by equipment type] 在源里只显式声明了 `[oath]`，
	// 所以 `[earring]` 这类走普通上限 99（一度被"过度收紧"成拒绝，见 catalog 的注释）。
	for _, r := range added {
		if r.Limit != 99 || r.After != 1 {
			t.Fatalf("registration = %+v, want limit 99 after 1", r)
		}
	}
}

// 达上限**只跳过收录、不拒绝分解**；背包里查不到那一行也必须有可见原因（不静默放行）。
func TestJournalRegistrationsSkipsWithoutFailing(t *testing.T) {
	rules := catalog.EquipmentJournalRules{
		Maximum:       99,
		MaximumByType: []catalog.JournalTypeLimit{{Kind: "[oath]", Rarity: 2, Maximum: 1}},
	}
	cat := fakeEquipmentCatalog{
		100051285: journalDef(100051285, 115, 2, ""),       // 普通 ⇒ 上限 99
		900000001: journalDef(900000001, 115, 2, "[oath]"), // 誓约 ⇒ 按类型收紧到 1
		100401610: journalDef(100401610, 110, 2, ""),       // 等级不是 115 ⇒ 不可登记
	}
	ledger := inventory.EquipmentJournal{Counts: map[uint32]uint32{
		100051285: 99, // 已满
		900000001: 1,  // 誓约上限 1，已满
	}}
	bySlot := map[uint16]uint32{10: 100051285, 20: 900000001, 30: 100401610}
	// 31 号槽故意不在 bySlot 里：模拟"背包快照与删除结果对不上"。
	slots := []uint16{10, 20, 30, 31}

	next, added, skipped, e := journalRegistrations(ledger, bySlot, slots, cat, &rules)
	if e != nil {
		t.Fatalf("cap must not fail the disassembly: %v", e)
	}
	if len(added) != 0 {
		t.Fatalf("added = %+v, want none", added)
	}
	want := []struct {
		slot   uint16
		reason string
	}{
		{10, "cap reached"},
		{20, "cap reached"},
		{30, "not registrable"},
		{31, "no bag row"},
	}
	if len(skipped) != len(want) {
		t.Fatalf("skipped = %+v, want %d entries", skipped, len(want))
	}
	for i, w := range want {
		if skipped[i].Slot != w.slot || skipped[i].Reason != w.reason {
			t.Fatalf("skip[%d] = %+v, want slot %d reason %q", i, skipped[i], w.slot, w.reason)
		}
	}
	if next.Counts[100051285] != 99 || next.Counts[900000001] != 1 || len(next.Counts) != 2 {
		t.Fatalf("ledger must be untouched: %+v", next.Counts)
	}
}

// 规则表 / 装备目录没装 ⇒ 不收录、不报错、也**不记 skip**：整条特性是关的，
// 不是"这一件被跳过"（否则回执里会刷满假的跳过原因）。
func TestJournalRegistrationsDisabledWithoutRulesOrCatalog(t *testing.T) {
	bySlot := map[uint16]uint32{12: 100391006}
	cat := fakeEquipmentCatalog{100391006: journalDef(100391006, 115, 6, "[earring]")}
	slots := []uint16{12}

	next, added, skipped, e := journalRegistrations(
		inventory.EquipmentJournal{}, bySlot, slots, cat, nil)
	if e != nil || len(added) != 0 || len(skipped) != 0 || len(next.Counts) != 0 {
		t.Fatalf("nil rules: next=%+v added=%+v skipped=%+v err=%v", next, added, skipped, e)
	}

	next, added, skipped, e = journalRegistrations(
		inventory.EquipmentJournal{}, bySlot, slots, nil, &catalog.EquipmentJournalRules{Maximum: 99})
	if e != nil || len(added) != 0 || len(skipped) != 0 || len(next.Counts) != 0 {
		t.Fatalf("nil catalog: next=%+v added=%+v skipped=%+v err=%v", next, added, skipped, e)
	}
}
