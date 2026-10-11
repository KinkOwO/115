package honeytime

import (
	"encoding/json"
	"testing"
	"time"
)

// TestTodayUsesSixAMUTC 钉住换日口径：与签到共用同一条客户端规则（06:00 UTC）。
// 两个活动必须同口径 —— 各写一份迟早漂移。
func TestTodayUsesSixAMUTC(t *testing.T) {
	before := time.Date(2026, 10, 10, 5, 59, 59, 0, time.UTC)
	at := time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)
	if Today(at.Unix())-Today(before.Unix()) != 1 {
		t.Fatal("06:00 UTC 边界没有换日")
	}
	if Today(at.Unix()) != Today(time.Date(2026, 10, 10, 23, 0, 0, 0, time.UTC).Unix()) {
		t.Fatal("06:00 到当日 24:00 应属于同一天")
	}
}

// TestNormalizeResetsOnNewDayAndCycle 钉住两条重置：**跨天**与**换期**。
// 业主实机确认过跨天/换角色都要回到阶段 1，所以这两条是交付判据本身。
func TestNormalizeResetsOnNewDayAndCycle(t *testing.T) {
	const day int64 = 20736
	st := State{Version: 1, EventID: EventID, CycleStart: 1000, Day: day, Seconds: 1800}

	if got := st.Normalize(1000, day); got.Seconds != 1800 {
		t.Fatalf("同一天被误重置: %+v", got)
	}
	// 跨天（Day 不是今天）⇒ 从头开始，且新 Day 记成今天。
	if got := st.Normalize(1000, day+1); got.Seconds != 0 || got.Day != day+1 {
		t.Fatalf("跨天没有重置: %+v", got)
	}
	// 换期（CycleStart 变了）⇒ 同样从头开始。
	if got := st.Normalize(2000, day); got.Seconds != 0 || got.CycleStart != 2000 {
		t.Fatalf("换期没有重置: %+v", got)
	}
}

// TestAddSecondsCapsAtLastStep 钉住封顶值本身，以及"已封顶就不再算变化"。
//
// ★ 为什么连数值一起钉：3600 是**按绝对阈值读脚本**算错的（见 honeytime.go 的订正）；
// 那个错会让第 4 档（累计 120 分钟）永远点不亮。谁把它"简化"回去，这里立刻红。
func TestAddSecondsCapsAtLastStep(t *testing.T) {
	if MaxDailySeconds != 14400 {
		t.Fatalf("封顶值被改了：%d（应为 14400 = 日档 6 行阈值全累加 = 4 小时）", MaxDailySeconds)
	}
	st := State{Version: 1, EventID: EventID, CycleStart: 1, Day: 1}
	if !st.AddSeconds(600) || st.Seconds != 600 {
		t.Fatalf("首次累计: %+v", st)
	}
	if st.AddSeconds(0) {
		t.Fatal("加 0 不应算变化")
	}
	if !st.AddSeconds(MaxDailySeconds) || st.Seconds != MaxDailySeconds {
		t.Fatalf("封顶失败: %+v", st)
	}
	if st.AddSeconds(60) {
		t.Fatal("已封顶后不应再算变化")
	}
}

// TestStateRoundTrip 钉住状态在角色 state JSON 里的读写：其它键必须原样保留。
func TestStateRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"other":{"a":1},"event_attendance115":{"version":1,"event_id":331}}`)
	st := State{Version: 1, EventID: EventID, CycleStart: 1785801600, Day: 20736, Seconds: 1234}
	encoded, err := WriteState(raw, st)
	if err != nil {
		t.Fatalf("写失败: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("产物不是对象: %v", err)
	}
	for _, key := range []string{"other", "event_attendance115"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("写回时丢了 %s", key)
		}
	}
	back, err := ReadState(encoded)
	if err != nil {
		t.Fatalf("读失败: %v", err)
	}
	if back != st {
		t.Fatalf("往返不一致:\n got=%+v\nwant=%+v", back, st)
	}
	fresh, err := ReadState(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("空 state 读失败: %v", err)
	}
	if fresh.Seconds != 0 || fresh.Day != -1 {
		t.Fatalf("全新状态不对: %+v", fresh)
	}
	if _, err := ReadState(json.RawMessage(`{"event_honeytime115":{"version":1,"event_id":313,"seconds":99999}}`)); err == nil {
		t.Fatal("越界的秒数没有被拒")
	}
}

// TestPeriodTrackSurvivesDayRollover 钉住"跨天只清日档、期间轨留着"。
//
// ★ 为什么重要：期间轨统计的是**整期累计在线**（门槛 20/40/60/80/100 小时），
// 与日档"每天重来"不是一回事。如果 `Normalize` 跨天把期间轨也清了，玩家的 80 小时进度
// 每天 06:00 UTC 都会被抹掉 —— 而且是**静默**的（面板只会显示进度少了）。
func TestPeriodTrackSurvivesDayRollover(t *testing.T) {
	st := State{Version: 1, EventID: EventID, CycleStart: 100, Day: 42,
		Seconds: 500, Claimed: 0b1, PeriodSeconds: 100000, PeriodClaimed: 0b11}

	// 跨天：日档清零，期间轨原样。
	got := st.Normalize(100, 43)
	if got.Day != 43 || got.Seconds != 0 || got.Claimed != 0 {
		t.Fatalf("跨天应清日档: %+v", got)
	}
	if got.PeriodSeconds != 100000 || got.PeriodClaimed != 0b11 {
		t.Fatalf("跨天不该清期间轨: %+v", got)
	}

	// 换期：整份重来（含期间轨）。
	if got := st.Normalize(200, 42); got.PeriodSeconds != 0 || got.PeriodClaimed != 0 {
		t.Fatalf("换期应连期间轨一起清: %+v", got)
	}

	// 同日同期：不动。
	if got := st.Normalize(100, 42); got != st {
		t.Fatalf("同日同期不该变: %+v -> %+v", st, got)
	}
}

// TestAddPeriodSecondsAndClaim 钉住期间轨的累计/封顶/幂等。
func TestAddPeriodSecondsAndClaim(t *testing.T) {
	st := State{Version: 1, EventID: EventID, CycleStart: 1, Day: 1}
	if !st.AddPeriodSeconds(30, 360000) || st.PeriodSeconds != 30 {
		t.Fatalf("首次累计: %+v", st)
	}
	if st.AddPeriodSeconds(0, 360000) {
		t.Fatal("加 0 不该算变化")
	}
	if !st.AddPeriodSeconds(360000, 360000) || st.PeriodSeconds != 360000 {
		t.Fatalf("封顶失败: %+v", st)
	}
	if st.AddPeriodSeconds(60, 360000) {
		t.Fatal("已封顶不该再算变化")
	}
	// 容错：目录没装（capacity=0）时不该累计，也不该报变化。
	fresh := State{Version: 1, EventID: EventID, CycleStart: 1, Day: 1}
	if fresh.AddPeriodSeconds(30, 0) {
		t.Fatal("capacity=0 时不该累计")
	}

	// 领取：同档重复必须被拒。
	next, err := fresh.ClaimPeriod(2)
	if err != nil || !next.PeriodClaimedTier(2) || next.PeriodClaimedCount() != 1 {
		t.Fatalf("ClaimPeriod(2) = %+v err=%v", next, err)
	}
	if _, err := next.ClaimPeriod(2); err == nil {
		t.Fatal("同一档重复领没有被拒")
	}
	if _, err := fresh.ClaimPeriod(-1); err == nil {
		t.Fatal("负档号没有被拒")
	}
}
