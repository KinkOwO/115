package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"testing"
)

// 老档兼容：State 里没有 equipment_journal 键时必须读出空账本，不能报错。
func TestEquipmentJournalMissingKeyIsEmpty(t *testing.T) {
	j, e := ReadEquipmentJournal(json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"},"other":1}`))
	if e != nil {
		t.Fatalf("read: %v", e)
	}
	if len(j.Counts) != 0 || len(j.Favorites) != 0 {
		t.Fatalf("expected an empty ledger, got %+v", j)
	}
}

// 写回必须保留其它未知键（与 SaveBag 同一约定）——存档兼容的硬要求。
func TestEquipmentJournalSavePreservesUnknownKeys(t *testing.T) {
	state := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1"},"unknown_block":{"x":[1,2,3]},"level":115}`)
	j, _ := ReadEquipmentJournal(state)
	j, _, e := j.Add(100401592, 3, 99)
	if e != nil {
		t.Fatalf("add: %v", e)
	}
	out, e := SaveEquipmentJournal(state, j)
	if e != nil {
		t.Fatalf("save: %v", e)
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(out, &fields); e != nil {
		t.Fatalf("state is not an object: %v", e)
	}
	for _, key := range []string{"inventory", "unknown_block", "level", EquipmentJournalKey} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("key %q was dropped", key)
		}
	}
	back, e := ReadEquipmentJournal(out)
	if e != nil {
		t.Fatalf("re-read: %v", e)
	}
	if back.Counts[100401592] != 3 {
		t.Fatalf("round trip lost the count: %+v", back.Counts)
	}
}

func TestEquipmentJournalAddCapsAndIsolation(t *testing.T) {
	var j EquipmentJournal
	// 普通：上限 99
	j, n, e := j.Add(100051285, 90, 99)
	if e != nil || n != 90 {
		t.Fatalf("add 90/99 = (%d,%v)", n, e)
	}
	// 超过上限必须报错（调用方整批回滚），而不是截断
	if _, _, e := j.Add(100051285, 10, 99); e == nil {
		t.Fatalf("passing the 99 cap must fail")
	}
	// 誓约：上限 1
	if _, _, e := j.Add(100051304, 2, 1); e == nil {
		t.Fatalf("oath cap 1 must reject 2")
	}
	if _, n, e := j.Add(100051304, 1, 1); e != nil || n != 1 {
		t.Fatalf("oath add 1/1 = (%d,%v)", n, e)
	}
	// 上限 0 = 不可登记
	if _, _, e := j.Add(999, 1, 0); e == nil {
		t.Fatalf("limit 0 must reject")
	}
	// 模板 0 永远不是合法键
	if _, _, e := j.Add(0, 1, 99); e == nil {
		t.Fatalf("template 0 must reject")
	}
	// 值语义：返回的新账本与原账本不共享 map —— 否则一处失败会污染另一处
	base := EquipmentJournal{Version: "equipment-journal-v1", Counts: map[uint32]uint32{1: 1}}
	next, _, e := base.Add(2, 1, 99)
	if e != nil {
		t.Fatalf("add: %v", e)
	}
	next.Counts[1] = 999
	if base.Counts[1] != 1 {
		t.Fatalf("Add leaked the map: base = %+v", base.Counts)
	}
	if _, ok := base.Counts[2]; ok {
		t.Fatalf("Add mutated the receiver: %+v", base.Counts)
	}
}

func TestEquipmentJournalFavoritesNormalize(t *testing.T) {
	var j EquipmentJournal
	// 乱序 + 重复 + 0 ⇒ 归一化为升序去零
	j, got, e := j.ReplaceFavorites(0, []uint32{0x3f5300, 0, 0x3f4900, 0x3f5300})
	if e != nil {
		t.Fatalf("replace: %v", e)
	}
	want := []uint32{0x3f4900, 0x3f5300}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("normalized = %#v, want %#v", got, want)
	}
	// 全量替换：第二次把列表清空（实机"取消收藏"就是这种）
	j, got, e = j.ReplaceFavorites(0, []uint32{0, 0, 0, 0})
	if e != nil || len(got) != 0 {
		t.Fatalf("clear = %#v (%v)", got, e)
	}
	if len(j.Favorites[0]) != 0 {
		t.Fatalf("favorites not replaced: %#v", j.Favorites)
	}
	// 类别越界
	if _, _, e := j.ReplaceFavorites(5, []uint32{1}); e == nil {
		t.Fatalf("category 5 must be rejected")
	}
	// 一类最多 4 个不同分组
	if _, _, e := j.ReplaceFavorites(0, []uint32{1, 2, 3, 4, 5}); e == nil {
		t.Fatalf("5 distinct groups must be rejected")
	}
}

// 2610 每类只有 3 槽：第 4 槽存得下但编不出去，必须能被调用方看见（不静默丢）。
func TestEquipmentJournalFavoritesFourthSlotIsReported(t *testing.T) {
	var j EquipmentJournal
	j, _, e := j.ReplaceFavorites(0, []uint32{10, 20, 30, 40})
	if e != nil {
		t.Fatalf("replace: %v", e)
	}
	if got := j.FavoritesFor(0); len(got) != JournalFavoriteSlots {
		t.Fatalf("encoded slots = %v, want %d", got, JournalFavoriteSlots)
	}
	extra := j.ExtraFavoriteSlots()
	if len(extra[0]) != 1 || extra[0][0] != 40 {
		t.Fatalf("extra = %#v, want category 0 -> [40]", extra)
	}
}

// 收录资格：minLevel==115 ∧ rarity∈{2,3,4,6,8}；kind 取 [equipment type]（如 `[oath]`）。
func TestJournalLimitEligibility(t *testing.T) {
	rules, e := catalog.ParseEquipmentJournalRules(journalRulesSample)
	if e != nil {
		t.Fatalf("rules: %v", e)
	}
	def := func(id uint32, minLevel, rarity int32, kind string) EquipmentDefinition {
		fields := map[string][]pvf.Token{
			"[minimum level]": {{Type: 0, Value: minLevel}},
			"[rarity]":        {{Type: 0, Value: rarity}},
		}
		if kind != "" {
			fields["[equipment type]"] = []pvf.Token{{Type: 3, Text: kind}}
		}
		return EquipmentDefinition{ID: id, Fields: fields}
	}
	cat := &EquipmentCatalog{index: map[uint32]EquipmentDefinition{
		1: def(1, 115, 2, ""),         // 普通 115 紫 ⇒ 99
		2: def(2, 115, 8, ""),         // 普通 115 最高档 ⇒ 99
		3: def(3, 115, 2, "[oath]"),   // 誓约 115 稀有 ⇒ 1
		4: def(4, 115, 5, ""),         // rarity 5 不在集合 ⇒ 不可登记
		5: def(5, 110, 2, ""),         // 等级不是 115 ⇒ 不可登记
		6: def(6, 115, 2, "[weapon]"), // 类型未在源里声明 ⇒ 回落普通上限 99
		7: {ID: 7, Fields: map[string][]pvf.Token{"[impossible disjoint]": {{Type: 0, Value: 1}},
			"[minimum level]": {{Type: 0, Value: 115}}, "[rarity]": {{Type: 0, Value: 2}}}}, // 禁拆 ⇒ 不可登记
	}}
	cases := []struct {
		template uint32
		want     uint32
		ok       bool
	}{
		{1, 99, true},
		{2, 99, true},
		{3, 1, true},
		{4, 0, false},
		{5, 0, false},
		{6, 99, true},
		{7, 0, false},
		{999, 0, false}, // 目录里没有
		{0, 0, false},
	}
	for _, c := range cases {
		got, ok := JournalLimit(cat, &rules, c.template)
		if ok != c.ok || got != c.want {
			t.Fatalf("JournalLimit(%d) = (%d,%v), want (%d,%v)", c.template, got, ok, c.want, c.ok)
		}
	}
	if _, ok := JournalLimit(nil, &rules, 1); ok {
		t.Fatalf("nil catalog must not be registrable")
	}
	if _, ok := JournalLimit(cat, nil, 1); ok {
		t.Fatalf("nil rules must not be registrable")
	}
}

// 与 cmd/equipmentjournalimport 的产物同形的最小规则表（只覆盖判据用到的两段）。
const journalRulesSample = "[max equipment count] 99\n" +
	"[max equipment count by equipment type]\n" +
	"`[oath]`, 2, 1\n" +
	"`[oath]`, 8, 1\n" +
	"[/max equipment count by equipment type]\n"
