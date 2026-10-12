package main

// `DFO_EXTRADROP_FIXED` 的解析守卫 —— 活动 1093「每局固定掉落能量」的开关。
//
// 契约（见 `extradrop_flow.go` 的 `extraDropFixedPerRun`）：
//
//	未设 / 空  ⇒ 10（业主 2026-10-13 定的缺省）
//	off / false / no / 0 ⇒ 0（关掉固定模式，回落官方稀有度表）
//	N (>0)     ⇒ N
//	其它（解析不了）⇒ 10，并且**记一条 warning**（不许静默变成"整段不发"）
//
// 最后一条尤其要守：静默失败在这条线上已经害过一次（见 `next202 §10`）。

import "testing"

func TestExtraDropFixedPerRunReadsTheSwitch(t *testing.T) {
	cases := []struct {
		raw  string
		want uint32
	}{
		{"", 10},     // 未设
		{" ", 10},    // 只有空白 == 未设
		{"off", 0},   // 关掉
		{"OFF", 0},   // 大小写不敏感
		{"Off", 0},   //
		{"false", 0}, //
		{"no", 0},    //
		{"0", 0},     // 0 == off（固定模式本就不接受 0 个）
		{"1", 1},     // 边界：1 是合法的"每局 1 个"
		{"25", 25},   // 普通值
		{" 7 ", 7},   // 两侧空白要吃掉
		{"abc", 10},  // 解析不了 ⇒ 缺省（并要打 warning）
		{"-3", 10},   // 负数不是合法 u32 ⇒ 缺省
		{"10个", 10},  // 带单位 ⇒ 解析不了 ⇒ 缺省
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Setenv("DFO_EXTRADROP_FIXED", tc.raw)
			if got := extraDropFixedPerRun(); got != tc.want {
				t.Fatalf("DFO_EXTRADROP_FIXED=%q -> %d，期望 %d", tc.raw, got, tc.want)
			}
		})
	}
}
