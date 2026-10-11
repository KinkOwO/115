package protocol

import (
	"encoding/hex"
	"testing"
)

// TestAttendanceDailyStateBytes 钉住 NOTI1379 的固定载荷长度：1 + 4 + 28 = 33。
// 长度在本机 IDB 的 handler（sub_143869190）里是三次读流写死的，改长度就是改协议。
func TestAttendanceDailyStateBytes(t *testing.T) {
	body, err := AttendanceDailyState115(AttendanceDailyStatus115{})
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if len(body) != AttendanceDailyStateBytes {
		t.Fatalf("载荷 %d 字节, want %d", len(body), AttendanceDailyStateBytes)
	}
	if hex.EncodeToString(body) != "0000000000"+"00"+"000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("零值载荷不是全 0: %s", hex.EncodeToString(body))
	}
}

// TestAttendanceDailyProgressMatchesOfficialSamples 用**官服样本**把口径钉死。
//
// 三个样本（《活动开启修复.md》§3.1，同一套 115 客户端的官服抓包）：
//
//	新号进城        `00 01000000 01 00×27`
//	前 3 天已领第 4 天可领 `00 04000000 02 02 02 01 00×24`
//	领完第 4 天后（同日）  `… 02 02 02 02 00×24`（该样本只给了 day_state 尾巴）
//
// 官方样本是 L0′（对方行为日志），所以这三条是**契约级**断言：谁改了
// attended 的算法或 day_state 的取值域，测试立刻红。
func TestAttendanceDailyProgressMatchesOfficialSamples(t *testing.T) {
	cases := []struct {
		name      string
		claimed   int
		available bool
		wantHex   string
	}{
		{
			// 官服：`00 01000000 01 00×27`
			name: "新号：第 1 天可领", claimed: 0, available: true,
			wantHex: "00" + "01000000" + "01" + repeatHex("00", 27),
		},
		{
			// 官服：`00 04000000 02020201 00×24`
			name: "前 3 天已领、第 4 天可领", claimed: 3, available: true,
			wantHex: "00" + "04000000" + "020202" + "01" + repeatHex("00", 24),
		},
		{
			// 官服：领第 4 天之后 day_state 变成 `02 02 02 02`；attended 仍是 4。
			name: "第 4 天领完（同日）", claimed: 4, available: false,
			wantHex: "00" + "04000000" + repeatHex("02", 4) + repeatHex("00", 24),
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body, err := AttendanceDailyState115(AttendanceDailyProgress115(tc.claimed, tc.available))
			if err != nil {
				t.Fatalf("编码失败: %v", err)
			}
			if got := hex.EncodeToString(body); got != tc.wantHex {
				t.Fatalf("载荷与官服样本不符:\n got=%s\nwant=%s", got, tc.wantHex)
			}
		})
	}
}

// TestAttendanceDailyProgressStopsAtSeven 钉住"领满 7 天不再可领"：脚本里只有
// day 0..6 七个 [reward info]，第 8 格必须保持 0，否则会在窗口里点亮一个没有内容的槽。
func TestAttendanceDailyProgressStopsAtSeven(t *testing.T) {
	st := AttendanceDailyProgress115(AttendanceDailyDays, true)
	if st.Attended != AttendanceDailyDays {
		t.Fatalf("领满后 attended=%d, want %d", st.Attended, AttendanceDailyDays)
	}
	for i := 0; i < AttendanceDailyDays; i++ {
		if st.DayState[i] != 2 {
			t.Fatalf("day_state[%d]=%d, want 2", i, st.DayState[i])
		}
	}
	for i := AttendanceDailyDays; i < AttendanceDailySlots; i++ {
		if st.DayState[i] != 0 {
			t.Fatalf("越界槽 day_state[%d]=%d, want 0", i, st.DayState[i])
		}
	}
}

// TestAttendanceDailyStateRejectsOutOfRange 形状校验：day_state 的取值域是 0..2。
func TestAttendanceDailyStateRejectsOutOfRange(t *testing.T) {
	var st AttendanceDailyStatus115
	st.DayState[3] = 3
	if _, err := AttendanceDailyState115(st); err == nil {
		t.Fatal("越界取值没有被拒")
	}
}

func repeatHex(unit string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += unit
	}
	return out
}
