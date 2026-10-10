package attendance

import (
	"encoding/json"
	"testing"
)

// scriptFixture 是本机 PVF 里 `attendancedailyevent.evt` 的**逐字导出**
// （cmd/pvfinspect -files 的 .txt 产物）。用内联字面量而不是读 PVF：解析器要能在
// 没有归档的环境里被钉住，而"真脚本能被解析"这条另由启动期的域装载保证。
const scriptFixture = `
[event period]
` + "`2026-07-06 09:00:00` `2026-10-06 09:00:00`" + `
[/event period]

[db table name]
` + "`event_2603_daily_attendance`" + `
[allow accumulate attendance]
1 
[level limit]
0 
[reward mail title]
` + "`event_331_mail_title`" + `
[reward mail message]
` + "`event_331_mail_msg`" + `
[ui path]
` + "`Live/Event/Kor/2026/0326_AttendanceDailyEvent/main.xui`" + `
[first login open popup]
1 
[reward infos]

[reward info]

[day]
0 
[reward items]
590015045 1000 590015133 1 
[/reward items]

[/reward info]

[reward info]

[day]
1 
[reward items]
590015045 1000 590015046 1 
[/reward items]

[special reward]

[/reward info]

[reward info]

[day]
2 
[reward items]
590015045 1000 590015133 1 
[/reward items]

[/reward info]

[reward info]

[day]
3 
[reward items]
590015045 1000 590015209 1 
[/reward items]

[/reward info]

[reward info]

[day]
4 
[reward items]
590015045 1000 590015134 1 
[/reward items]

[/reward info]

[reward info]

[day]
5 
[reward items]
590015045 1000 590015133 1 
[/reward items]

[/reward info]

[reward info]

[day]
6 
[reward items]
590015045 1000 590015968 1 
[/reward items]

[/reward info]

[/reward infos]
`

// TestParseRealScript 钉住真脚本的解析结果：7 天、每天都两件（1000 + 1）、
// 邮件键与两个开关。
func TestParseRealScript(t *testing.T) {
	c, err := Parse(scriptFixture)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}
	if len(c.Days) != MaxDays {
		t.Fatalf("天数 %d, want %d", len(c.Days), MaxDays)
	}
	for _, d := range c.Days {
		if len(d.Rewards) != 2 {
			t.Fatalf("day %d 的奖励件数 %d, want 2", d.Index, len(d.Rewards))
		}
		if d.Rewards[0].Template != 590015045 || d.Rewards[0].Count != 1000 {
			t.Fatalf("day %d 的第一件不是 590015045×1000：%+v", d.Index, d.Rewards[0])
		}
		if d.Rewards[1].Count != 1 {
			t.Fatalf("day %d 的第二件数量 %d, want 1", d.Index, d.Rewards[1].Count)
		}
	}
	wantSecond := []uint32{590015133, 590015046, 590015133, 590015209, 590015134, 590015133, 590015968}
	for i, want := range wantSecond {
		got, ok := c.DayRewards(i)
		if !ok || got[1].Template != want {
			t.Fatalf("day %d 的第二件 %v, want %d", i, got, want)
		}
	}
	if c.MailTitleKey != "event_331_mail_title" || c.MailMessageKey != "event_331_mail_msg" {
		t.Fatalf("邮件键解析错：%q / %q", c.MailTitleKey, c.MailMessageKey)
	}
	if !c.Accumulate || !c.FirstLoginPopup {
		t.Fatalf("开关解析错：accumulate=%v first_login_popup=%v", c.Accumulate, c.FirstLoginPopup)
	}
	if c.DBTableName != "event_2603_daily_attendance" {
		t.Fatalf("表名 %q", c.DBTableName)
	}
	if _, ok := c.DayRewards(MaxDays); ok {
		t.Fatal("第 8 天不该有奖励")
	}
}

// TestParseRejectsOddRewardPairs 钉住"宁可拒绝不要猜"：成对值出现奇数个直接报错。
func TestParseRejectsOddRewardPairs(t *testing.T) {
	bad := "[reward infos]\n[reward info]\n[day]\n0\n[reward items]\n590015045 1000 590015133\n[/reward items]\n[/reward info]\n[/reward infos]\n"
	if _, err := Parse(bad); err == nil {
		t.Fatal("奇数个 [reward items] 值没有被拒")
	}
}

// TestStateRoundTrip 钉住状态在角色 state JSON 里的读写：不认识的键必须原样保留。
func TestStateRoundTrip(t *testing.T) {
	raw := json.RawMessage(`{"other_key":{"a":1},"boost_up115":{"version":1}}`)
	st := State{Version: 1, EventID: EventID, CycleStart: 1785801600, ClaimedDays: 3, LastClaimDay: 20600}
	encoded, err := WriteState(raw, st)
	if err != nil {
		t.Fatalf("写失败: %v", err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatalf("产物不是对象: %v", err)
	}
	if _, ok := fields["other_key"]; !ok {
		t.Fatal("写回时丢了别的键")
	}
	if _, ok := fields["boost_up115"]; !ok {
		t.Fatal("写回时丢了 boost_up115")
	}
	back, err := ReadState(encoded)
	if err != nil {
		t.Fatalf("读失败: %v", err)
	}
	if back != st {
		t.Fatalf("往返不一致:\n got=%+v\nwant=%+v", back, st)
	}
	// 没有这一段时是「本期全新」。
	fresh, err := ReadState(json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("空 state 读失败: %v", err)
	}
	if fresh.ClaimedDays != 0 || fresh.LastClaimDay != -1 {
		t.Fatalf("全新状态不对: %+v", fresh)
	}
}

// TestNormalizeResetsOnNewCycle 钉住"换期自动重置"：cycleStart 变了就从头算，
// 不需要运维手工清档。
func TestNormalizeResetsOnNewCycle(t *testing.T) {
	st := State{Version: 1, EventID: EventID, CycleStart: 1000, ClaimedDays: 5, LastClaimDay: 20000}
	if got := st.Normalize(1000); got.ClaimedDays != 5 {
		t.Fatalf("同期被误重置: %+v", got)
	}
	if got := st.Normalize(2000); got.ClaimedDays != 0 || got.LastClaimDay != -1 || got.CycleStart != 2000 {
		t.Fatalf("换期没有重置: %+v", got)
	}
}

// TestAvailableTodayAndClaim 钉住领取判据与领取后的状态推进（官服两个样本的语义）：
//
//	从未领过        ⇒ 今天可领，领完 claimed=1
//	昨天领过        ⇒ 今天可领
//	今天已领过      ⇒ 今天不可领（重复点不给第二次）
//	领满 7 天       ⇒ 永不可领
func TestAvailableTodayAndClaim(t *testing.T) {
	const today int64 = 20691
	fresh := State{Version: 1, EventID: EventID, CycleStart: 1, LastClaimDay: -1}
	if !fresh.AvailableToday(today) {
		t.Fatal("全新状态今天应当可领")
	}
	claimed, err := fresh.Claim(today)
	if err != nil {
		t.Fatalf("领取失败: %v", err)
	}
	if claimed.ClaimedDays != 1 || claimed.LastClaimDay != today {
		t.Fatalf("领取后状态不对: %+v", claimed)
	}
	if claimed.AvailableToday(today) {
		t.Fatal("同一天不应再可领")
	}
	if !claimed.AvailableToday(today + 1) {
		t.Fatal("换日后应当可领")
	}
	if _, err := claimed.Claim(today); err == nil {
		t.Fatal("同一天重复领取没有被拒")
	}
	full := State{Version: 1, EventID: EventID, CycleStart: 1, ClaimedDays: MaxDays, LastClaimDay: today - 10}
	if full.AvailableToday(today) {
		t.Fatal("领满 7 天后不应再可领")
	}
}
