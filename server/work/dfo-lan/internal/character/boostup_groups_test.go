package character

// 真源钉桩：Starter Boost 的穿戴任务按 `[equip grouping]` 分组号计数，分组身份
// 只有 equipmentgrouping.etc 一个来源，即本包的 FameRules.Groups（活动侧不再
// 另写第二个 .etc 解析器）。这里对内层 PVF 复核 donor 实机观测过的那批起步装备
// 仍属源分组 51，防止分组解析漂移把第一关/第八关判据静默改成永不成立。
// 只在显式给出只读内层归档时运行（与活动的 capsule_source_test 同一门禁）。

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"os"
	"slices"
	"testing"
)

func TestBoostStarterEquipmentGroupBinding(t *testing.T) {
	path := os.Getenv("US115_TEST_BOOST_PVF")
	if path == "" {
		t.Skip("explicit read-only source required")
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: path, MaxBytes: 1 << 30})
	if e != nil {
		t.Fatal(e)
	}
	index, e := catalog.LoadItemIndex("../../configs/items.index.json")
	if e != nil {
		t.Fatal(e)
	}
	rules, e := ImportFameRules(a, index)
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{100051284, 100101167, 100151108, 100201080, 100251120,
		100301827, 100313530, 100323420, 100345965, 100354140, 100391018} {
		if !slices.Contains(rules.Groups[id], int32(51)) {
			t.Fatalf("observed starter equipment %d missing source group51: %v", id, rules.Groups[id])
		}
	}
}
