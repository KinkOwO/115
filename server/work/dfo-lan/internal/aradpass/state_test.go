package aradpass

import "testing"

// 任务经验口径（源里 `<序号> <键> <次数> <经验>`）：
// 次数是"要重复多少次"，经验是**做满那一次**给一次；计数**按天**清零；当天上限封顶。
func TestRecordMissionExpRules(t *testing.T) {
	c := loadFixture(t)
	day := "2026-04-05"

	// `[season mission] 2 disjoint 100 800`：100 次才给 800。
	st := State{Version: stateVersion}
	st, got := st.RecordMission(c, day, MissionTrackSeason, 2, 99)
	if got != 0 {
		t.Fatalf("未做满不应给经验，得到 %d", got)
	}
	if st.MissionCount(MissionTrackSeason, 2) != 99 {
		t.Fatalf("计数 = %d", st.MissionCount(MissionTrackSeason, 2))
	}
	st, got = st.RecordMission(c, day, MissionTrackSeason, 2, 1)
	if got != 800 || st.Exp != 800 || st.DailyExp != 800 {
		t.Fatalf("做满应给 800：got=%d exp=%d daily=%d", got, st.Exp, st.DailyExp)
	}
	// 再做同一条：当天已发过，不再给经验（计数封顶在目标次数）。
	st, got = st.RecordMission(c, day, MissionTrackSeason, 2, 50)
	if got != 0 || st.Exp != 800 {
		t.Fatalf("重复做满不应再发：got=%d exp=%d", got, st.Exp)
	}
	if n := st.MissionCount(MissionTrackSeason, 2); n != 100 {
		t.Fatalf("计数应封顶在 100，得到 %d", n)
	}
	// 等级按累计经验换算：800 → 未到 1200 ⇒ 2 级。
	if lv := st.Level(c); lv != 2 {
		t.Fatalf("800 经验应为 2 级，得到 %d", lv)
	}
}

// 当天上限 7150（源 `[weekly gainable exp]`；本机按业主口径当作**当天**上限）：
// 超出的部分丢弃，不累积到明天。
func TestRecordMissionDailyCap(t *testing.T) {
	c := loadFixture(t)
	if c.WeeklyGainableExp == 0 {
		t.Fatal("夹具应带 weekly gainable exp")
	}
	day := "2026-04-05"
	// DailyKey 必须与本次一致，否则会先被当成换日而清零（正确行为）。
	st := State{Version: stateVersion, DailyKey: day, DailyExp: c.WeeklyGainableExp - 100}
	st, got := st.RecordMission(c, day, MissionTrackSeason, 1 /* proper_dungeon_clear 100 次 800 经验 */, 100)
	if got != 100 {
		t.Fatalf("应只发到当天上限：got=%d", got)
	}
	if st.DailyExp != c.WeeklyGainableExp {
		t.Fatalf("当天经验 = %d，应封顶在 %d", st.DailyExp, c.WeeklyGainableExp)
	}
	st, got = st.RecordMission(c, day, MissionTrackSeason, 12, 5)
	if got != 0 {
		t.Fatalf("当天上限已满不应再发，得到 %d", got)
	}
}

// 换日：计数与经验池整组清零，新的一天重新开始（业主口径：**每天都能再刷一轮**）。
func TestRecordMissionDailyRollover(t *testing.T) {
	c := loadFixture(t)
	st := State{Version: stateVersion}
	st, _ = st.RecordMission(c, "2026-04-05", MissionTrackSeason, 2, 100)
	if st.DailyExp == 0 {
		t.Fatal("第一天应拿到经验")
	}
	// 换日后再做满同一条：应当**再给一次**（这正是每日口径要的效果）。
	st, got := st.RecordMission(c, "2026-04-06", MissionTrackSeason, 2, 100)
	if got != 800 {
		t.Fatalf("换日后做满应再给 800，得到 %d", got)
	}
	if st.DailyKey != "2026-04-06" || st.DailyExp != 800 || st.MissionCount(MissionTrackSeason, 2) != 100 {
		t.Fatalf("换日状态不对：day=%s daily=%d count=%d",
			st.DailyKey, st.DailyExp, st.MissionCount(MissionTrackSeason, 2))
	}
}

// RollDaily 的首次迁移：老存档没有 DailyKey（空串）时**只补键、不清进度**，
// 否则升级那一刻会把玩家当天的进度误清。换日才清。
func TestRollDailyFirstMigrationKeepsProgress(t *testing.T) {
	st := State{Version: stateVersion, Missions: map[string]uint32{"2:1": 1}, DailyExp: 300}
	next := st.RollDaily("2026-04-05")
	if next.DailyKey != "2026-04-05" {
		t.Fatalf("应补上 DailyKey，得到 %q", next.DailyKey)
	}
	if next.Missions["2:1"] != 1 || next.DailyExp != 300 {
		t.Fatalf("首次迁移不该清进度：missions=%v daily=%d", next.Missions, next.DailyExp)
	}
	// 同一天重复调用：保持不变。
	if again := next.RollDaily("2026-04-05"); again.DailyExp != 300 || again.Missions["2:1"] != 1 {
		t.Fatalf("同一天不该清：%+v", again)
	}
	// 真正换日：任务计数、已发标记与经验池都清，已领奖励（Claimed）**不清**。
	withClaim := next
	withClaim.Claimed = map[string]bool{"0:1": true}
	rolled := withClaim.RollDaily("2026-04-06")
	if rolled.Missions != nil || rolled.Rewarded != nil || rolled.DailyExp != 0 {
		t.Fatalf("换日应清任务与经验池：%+v", rolled)
	}
	if !rolled.Claimed["0:1"] {
		t.Fatal("已领奖励不该被换日清掉（那是赛季一次性的）")
	}
}

// 状态读写要保留角色 State 里的其它键。
func TestStateRoundTripPreservesUnknownKeys(t *testing.T) {
	raw := []byte(`{"level":115,"bag":{"gold":7}}`)
	next, err := WriteState(raw, State{Version: stateVersion, Exp: 900})
	if err != nil {
		t.Fatalf("WriteState: %v", err)
	}
	back, err := ReadState(next)
	if err != nil || back.Exp != 900 {
		t.Fatalf("ReadState: exp=%d err=%v", back.Exp, err)
	}
	if !contains(string(next), `"level":115`) || !contains(string(next), `"gold":7`) {
		t.Fatalf("其它键被丢了：%s", next)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
