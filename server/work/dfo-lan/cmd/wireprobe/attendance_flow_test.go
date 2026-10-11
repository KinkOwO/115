package main

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"os"
	"path/filepath"
	"strings"
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

func TestAttendanceGatewayPersistsAndResetsWithGameCalendar(t *testing.T) {
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{SQLitePath: filepath.Join(t.TempDir(), "activity.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	raw, err := os.ReadFile("../../internal/activity/testdata/login.json")
	require.NoError(t, err)
	var tokens []pvf.Token
	require.NoError(t, json.Unmarshal(raw, &tokens))
	rules, err := activity.ParseLogin(tokens)
	require.NoError(t, err)
	version := strings.Repeat("a", 64)
	rules.SourceChecksum, rules.MailTextTitle, rules.MailTextMessage = version, "Attendance", "Source rewards"
	rules.CalendarUI = "live/event/kor/2026/0326_attendancedailyevent/cal.xui"
	index := catalog.ItemIndex{Source: pvf.ArchiveSnapshot{Checksum: version}, Items: map[uint32]catalog.ItemIndexEntry{}}
	for _, d := range rules.Days {
		for _, r := range d.Rewards {
			index.Items[r.Item] = catalog.ItemIndexEntry{ID: r.Item, Kind: "stackable", Path: "fixture.stk"}
		}
	}
	svc, err := workflow.NewAttendanceService(store, rules, index)
	require.NoError(t, err)
	account, err := store.DevelopmentAccount(ctx, "attendance-gateway")
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "Attendance gateway", Request: []byte{}, ConfigVersion: index.Source.SaveIdentity(), State: json.RawMessage(`{"level":115}`)}, 100)
	require.NoError(t, err)
	location := time.FixedZone("HK", 8*3600)
	fatigue := &character.FatigueService{Rules: character.FatigueRules{ResetHour: 6}, Location: location}
	notified := 0
	w := &worldSession{attendance: svc, account: account, role: role, fatigue: fatigue, notifyBoostMail: func(id int64) { require.Equal(t, role.ID, id); notified++ }}
	now := time.Date(2026, 9, 2, 5, 59, 0, 0, location)
	p, err := w.attendanceStatus(ctx, now)
	require.NoError(t, err)
	require.EqualValues(t, 1, binary.LittleEndian.Uint32(p[1:]))
	require.Equal(t, "2026-09-01", w.attendanceDay)
	request := make([]byte, 8)
	binary.LittleEndian.PutUint32(request, 331)
	packets, err := w.claimAttendance(ctx, request, now)
	require.NoError(t, err)
	require.Len(t, packets, 2)
	require.EqualValues(t, 1379, packets[0].ID)
	require.EqualValues(t, 2, packets[0].Payload[5])
	require.EqualValues(t, 680, packets[1].ID)
	require.Equal(t, 1, notified)
	_, err = w.claimAttendance(ctx, request, now)
	require.NoError(t, err)
	require.Equal(t, 1, notified)
	p, err = w.attendanceStatus(ctx, now.Add(time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(p[1:]))
	require.EqualValues(t, 2, p[5])
	require.EqualValues(t, 1, p[6])
	// A new world session restores the same account state, without a new day.
	w.attendanceDay = ""
	p, err = w.attendanceStatus(ctx, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.EqualValues(t, 2, binary.LittleEndian.Uint32(p[1:]))
	binary.LittleEndian.PutUint32(request[4:], 7)
	packets, err = w.claimAttendance(ctx, request, now.Add(2*time.Minute))
	require.Error(t, err)
	require.Equal(t, protocol.AttendanceDailyRefusal115(23), packets[0].Payload)
	require.Equal(t, 1, notified)
}

func TestAttendanceEntrySnapshotPreservesOtherEventsAndDispatch(t *testing.T) {
	s := &workflow.AttendanceService{Rules: activity.Login{Dates: activity.Dates{Start: "2026-10-08 06:00:00", End: "2026-11-12 06:00:00"}, MailTextTitle: "金秋8日丰收之旅", CalendarUI: "live/event/kor/2026/0326_attendancedailyevent/cal.xui"}}
	merged, err := attendanceOpening(s, townEventInfoTable, time.UTC)
	require.NoError(t, err)
	require.EqualValues(t, binary.LittleEndian.Uint16(townEventInfoTable)+1, binary.LittleEndian.Uint16(merged))
	require.Equal(t, townEventInfoTable[2:len(townEventInfoTable)-1], merged[2:len(townEventInfoTable)-1])
	plan := entryPayloads{ChannelEventInfo: merged, Attendance: []byte{0}, Complete: []byte{0}}.packets()
	completed := false
	for _, p := range plan {
		if p.ID == 108 {
			require.Equal(t, merged, p.Payload)
		}
		if p.ID == 124 {
			completed = true
		}
		if p.ID == 1379 {
			require.True(t, completed)
		}
	}
	client := &gameConnection{worldState: &worldSession{attendance: s}, bootstrapped: true}
	for _, id := range []uint32{662, 387, 389} {
		p := make([]byte, 8)
		binary.LittleEndian.PutUint32(p, id)
		require.Equal(t, dispatchNext, client.dispatchAttendance(&clientRequest{frame: wire.Frame{Type: 1, ID: 680}, verified: true, plaintext: p}))
	}
}
