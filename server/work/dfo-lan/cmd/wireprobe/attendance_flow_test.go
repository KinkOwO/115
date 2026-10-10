package main

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"testing"
	"time"
)

// TestAttendanceDayNumberBoundary 钉住"每天 06:00 UTC 换日"这条口径。
//
// 它来自客户端自己的计算（`floor(t/86400)*86400 + 21600`），服务端必须同口径，
// 否则"第几天可领"会与窗口显示错位。窗口文案写的是 09:00 UTC，两者不一致 ——
// 以**客户端计算**为准（见 attendance_flow.go 的注释）。
func TestAttendanceDayNumberBoundary(t *testing.T) {
	before := time.Date(2026, 10, 10, 5, 59, 59, 0, time.UTC)
	at := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)
	if got := attendanceDayNumber(at.Unix()) - attendanceDayNumber(before.Unix()); got != 1 {
		t.Fatalf("06:00 UTC 边界没有换日（差 %d 天）", got)
	}
	late := time.Date(2026, 10, 10, 23, 59, 59, 0, time.UTC)
	if attendanceDayNumber(late.Unix()) != attendanceDayNumber(at.Unix()) {
		t.Fatal("06:00 到当日 24:00 应属于同一个签到日")
	}
}

// TestAttendanceAvailableToday 钉住"今天还能不能领"的判据。
func TestAttendanceAvailableToday(t *testing.T) {
	const today int64 = 20000
	cases := []struct {
		name         string
		claimed      int
		lastClaimDay int64
		want         bool
	}{
		{"本期一次都没领过 ⇒ 今天可领", 0, today, true},
		{"上次是昨天领的 ⇒ 今天可领", 3, today - 1, true},
		{"今天已经领过 ⇒ 今天不可领", 3, today, false},
		{"领满 7 天 ⇒ 不再可领", protocol.AttendanceDailyDays, today - 5, false},
	}
	for _, tc := range cases {
		if got := attendanceAvailableToday(tc.claimed, tc.lastClaimDay, today); got != tc.want {
			t.Fatalf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestAttendanceForceOverride 钉住调试开关的取值口径：只接受 `<0..7>:<0|1>`。
// 取值不合法必须**不生效**（返回 ok=false）而不是被静默收下 —— 静默改语义会让
// 实机验证得到错误结论。
func TestAttendanceForceOverride(t *testing.T) {
	for _, tc := range []struct {
		raw       string
		claimed   int
		available bool
		ok        bool
	}{
		{"", 0, false, false},
		{"3:1", 3, true, true},
		{"3:0", 3, false, true},
		{"0:1", 0, true, true},
		{"7:0", 7, false, true},
		{" 4 : 1 ", 4, true, true},
		{"8:1", 0, false, false},  // 超过 7 天
		{"-1:1", 0, false, false}, // 负数
		{"3:2", 0, false, false},  // available 只能是 0/1
		{"3", 0, false, false},    // 缺冒号
		{"x:1", 0, false, false},  // 非数字
	} {
		t.Setenv("DFO_ATTENDANCE_FORCE", tc.raw)
		claimed, available, ok := attendanceForceOverride()
		if ok != tc.ok || (ok && (claimed != tc.claimed || available != tc.available)) {
			t.Fatalf("%q -> (%d,%v,%v), want (%d,%v,%v)", tc.raw, claimed, available, ok, tc.claimed, tc.available, tc.ok)
		}
	}
}

// TestAttendanceDailyBodyEncoding 钉住两个端到端形态（**纯函数**，不碰存档）：
//
//	claimed=0 / lastClaimDay=-1          ⇒ 本期全新账号 ⇒ 与官服新号样本同形：
//	                                         attended=1、第 1 格可领
//	claimed=3 / 昨天领过（today-1）      ⇒ 前 3 天已领、第 4 天可领（另一个官服样本）
//	claimed=4 / 今天已领（today）        ⇒ attended=4、前 4 格全部已领、第 5 格未到
func TestAttendanceDailyBodyEncoding(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	today := attendanceDayNumber(now.Unix())

	cases := []struct {
		name         string
		claimed      int
		lastClaimDay int64
		want         string
	}{
		{"本期全新", 0, -1, "00" + "01000000" + "01" + repeatHex("00", 27)},
		{"前 3 天已领、第 4 天可领", 3, today - 1, "00" + "04000000" + "020202" + "01" + repeatHex("00", 24)},
		{"第 4 天领完（同日）", 4, today, "00" + "04000000" + repeatHex("02", 4) + repeatHex("00", 24)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := attendanceDailyBody(now, tc.claimed, tc.lastClaimDay)
			if err != nil {
				t.Fatalf("编码失败: %v", err)
			}
			if got := hex.EncodeToString(body); got != tc.want {
				t.Fatalf("载荷与官服样本不符:\n got=%s\nwant=%s", got, tc.want)
			}
		})
	}
}

// TestAttendanceForceOverrideDrivesBody 钉住调试开关到载荷的那一段换算：
// 开关只给「已领天数 + 今天能不能领」，`3:1` 必须落成 attended=4 / 第 4 格可领。
// 会话侧 method 用的是同一段换算（attendanceDailyState），所以这条用例覆盖了它。
func TestAttendanceForceOverrideDrivesBody(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	t.Setenv("DFO_ATTENDANCE_FORCE", "3:1")
	claimed, available, ok := attendanceForceOverride()
	if !ok || claimed != 3 || !available {
		t.Fatalf("开关解析错: %d %v %v", claimed, available, ok)
	}
	// 开关的 available=true 等价于"上次领取日 = 昨天"。
	lastClaimDay := attendanceDayNumber(now.Unix())
	if available {
		lastClaimDay--
	}
	body, err := attendanceDailyBody(now, claimed, lastClaimDay)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	want := "00" + "04000000" + "020202" + "01" + repeatHex("00", 24)
	if got := hex.EncodeToString(body); got != want {
		t.Fatalf("开关态与官服样本不符:\n got=%s\nwant=%s", got, want)
	}
}

func repeatHex(unit string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += unit
	}
	return out
}
