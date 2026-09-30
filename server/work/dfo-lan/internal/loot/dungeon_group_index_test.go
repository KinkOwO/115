package loot

import (
	"testing"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-DUNGEON-GROUP-INDEX] `[normal group index]` 的读法测试。
//
// 期望值全部来自只读观察（副本脚本原文），不是发明的：
//
//	100005014: [1 21251 1 21476]              -> [21251 21476]
//	100005068: [1 21251 1 21600]              -> [21251 21600]
//	100003295: [2 10900 21030 5 10014 ...]    -> [10900 21030 10014 ...]
func TestExtractGroupIndices(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   []int32
		want []uint32
	}{
		{"单段一组", []int32{1, 21251}, []uint32{21251}},
		{"两段各一组", []int32{1, 21251, 1, 21476}, []uint32{21251, 21476}},
		{"两段 2+5", []int32{2, 10900, 21030, 5, 10014, 10015, 10024, 10021, 11085},
			[]uint32{10900, 21030, 10014, 10015, 10024, 10021, 11085}},
		{"段内 3 组", []int32{3, 10012, 1011, 1221}, []uint32{10012, 1011, 1221}},
		{"空", []int32{}, nil},
		{"零个数的空段", []int32{0, 1, 21476}, []uint32{21476}},
	} {
		got, e := ExtractGroupIndices(tc.in)
		if e != nil {
			t.Fatalf("%s: %v", tc.name, e)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Fatalf("%s: got %v want %v", tc.name, got, tc.want)
			}
		}
	}
	// 形状不认识时必须报错，不能悄悄吞掉。
	for _, bad := range [][]int32{
		{3, 1, 2}, // 个数 3 但只有 2 个组号
		{1, 0},    // 组号 0
		{1, -5},   // 负组号
		{-1, 5},   // 负个数
	} {
		if _, e := ExtractGroupIndices(bad); e == nil {
			t.Fatalf("形状 %v 应被拒绝", bad)
		}
	}
}

// 实机副本 100005014「深渊：最终调律者」必须能从自己的脚本里读到组索引。
// 这是「装备不爆」的直接修复点：它不在 dungeondropinfo 里，只能走这条路。
func TestAbyss100005014DeclaresGroups(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Skip("dungeons.full.json 不可用:", e)
	}
	d, ok := c.Dungeons[100005014]
	if !ok {
		t.Fatal("100005014 不在 dungeons.full.json")
	}
	ids, ok, e := DungeonGroupIndices(d, 0)
	if e != nil {
		t.Fatal(e)
	}
	if !ok || len(ids) == 0 {
		t.Fatal("100005014 应声明组索引")
	}
	t.Logf("100005014 难度0 声明的组: %v", ids)
	want := []uint32{21251, 21476}
	if len(ids) != len(want) {
		t.Fatalf("got %v want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v want %v", ids, want)
		}
	}
}

// 多难度副本：100005068 一个难度一个块，组号递增。
func TestAbyss100005068DeclaresPerDifficultyGroups(t *testing.T) {
	c, e := catalog.LoadDungeons("../../configs/dungeons.full.json")
	if e != nil {
		t.Skip("dungeons.full.json 不可用:", e)
	}
	d, ok := c.Dungeons[100005068]
	if !ok {
		t.Skip("100005068 不在 catalog")
	}
	for diff, want := range map[int]uint32{0: 21600, 1: 21601, 2: 21602} {
		ids, ok, e := DungeonGroupIndices(d, diff)
		if e != nil {
			t.Fatal(e)
		}
		if !ok {
			t.Fatalf("难度 %d 应声明组", diff)
		}
		if len(ids) < 1 {
			t.Fatalf("难度 %d 组为空", diff)
		}
		found := false
		for _, id := range ids {
			if id == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("难度 %d: got %v, 应含 %d", diff, ids, want)
		}
	}
	// 难度越界时返回 ok=false（不做取模回绕），调用方保留全局表结果。
	if _, ok, e := DungeonGroupIndices(d, 99); e != nil || ok {
		t.Fatalf("难度越界应 ok=false, 得到 ok=%v err=%v", ok, e)
	}
}
