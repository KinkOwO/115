package oracleresearch

// 自然周边界：官方活动 UI 的帮助原文**三次**都写 "reset every Tuesday at **09:00 UTC**"
// （神秘能量 / 生产费用与次数 / 组装槽激活状态）。
//
// 2026-10-12 修正前我们写成"周二 00:00 UTC"，比官方早 9 小时 —— 这个测试把它钉住。

import (
	"testing"
	"time"
)

func TestWeekStartOfIsTuesday0900UTC(t *testing.T) {
	at := func(y int, m time.Month, d, hh int) time.Time {
		return time.Date(y, m, d, hh, 0, 0, 0, time.UTC)
	}
	week := func(y int, m time.Month, d int) int64 {
		return at(y, m, d, 9).Unix()
	}
	cases := []struct {
		what string
		now  time.Time
		want int64
	}{
		// 2026-10-06 是周二、10-13 是下一个周二。
		{"周二 09:00 整点（边界当刻算新的一周）", at(2026, 10, 13, 9), week(2026, 10, 13)},
		{"周二 08:59 之前（还没跨界，仍属上一周）", at(2026, 10, 13, 8), week(2026, 10, 6)},
		{"周二 10:00", at(2026, 10, 13, 10), week(2026, 10, 13)},
		{"周三", at(2026, 10, 14, 0), week(2026, 10, 13)},
		{"周一（10-12）", at(2026, 10, 12, 12), week(2026, 10, 6)},
		{"周二 00:00（旧实现会在这时提前跨周）", at(2026, 10, 13, 0), week(2026, 10, 6)},
	}
	for _, tc := range cases {
		if got := WeekStartOf(tc.now); got != tc.want {
			t.Errorf("%s: WeekStartOf(%s) = %d (%s), want %d (%s)",
				tc.what, tc.now.Format(time.RFC3339), got,
				time.Unix(got, 0).UTC().Format(time.RFC3339), tc.want,
				time.Unix(tc.want, 0).UTC().Format(time.RFC3339))
		}
	}
}

func TestNormalizeResetsRowsUsedOnWeekChange(t *testing.T) {
	old := State{Version: 1, WeekStart: 100, Produced: 7, RowsUsed: [3]bool{true, false, true}}
	same := old.Normalize(100)
	if same != old {
		t.Errorf("同一周不该改动状态: %+v", same)
	}
	next := old.Normalize(200)
	if next.WeekStart != 200 || next.Produced != 0 {
		t.Errorf("跨周应归零次数: %+v", next)
	}
	if next.RowsUsed != [3]bool{} {
		t.Errorf("跨周应把组装槽激活状态也归零（官方：与已产次数同一条周界）: %+v", next.RowsUsed)
	}
}
