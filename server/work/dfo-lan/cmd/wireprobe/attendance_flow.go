package main

import (
	"context"
	"dfolan/internal/attendance"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	// attendanceEventStart / attendanceEventEnd 是 331 的**运营窗口**，逐字取自官服
	// NOTI108 抓包（internal/legion/event_info_official.plain 里 id=331 那条：
	// start=1785801600 / end=1791277198，即 2026-08-04 → 2026-10-06）。
	//
	// attendanceEventStart 同时是**存档的期号**（cycleStart）：换期改这两行 + op108 那
	// 一行，玩家的签到进度会自动从头算（见 attendance.State.Normalize），不需要运维清档。
	//
	// ⚠️ **不要拿窗口当"活动开不开"的判据**。2026-10-10 实测：窗口已经过去（客户端收到的
	// op1960 服务端时间也在窗口之后），而客户端 trace 的 `[event on]` 里 331 **仍然是开的**
	// ⇒ 客户端的开关位图只看 op108 里的 `present`，不做窗口比较。
	// 取值来自 internal/attendance（**单一来源**）：这里只做别名，不再写第二份数值。
	attendanceEventStart = attendance.EventStart
	attendanceEventEnd   = attendance.EventEnd

	// attendanceResetOffsetSeconds 是客户端算"下一次重置"时加的那个偏移：
	// `floor(t/86400)*86400 + 21600`（见文档 §3.3 与 IDA 的 AttendanceDaily_CanStillComplete）。
	// ⇒ **每天 06:00 UTC 换日**。窗口文案写的是 "Reset every day at 09:00 UTC"，
	// 与客户端自己的计算不一致；服务端跟**客户端计算**走，否则"第几天可领"会错位。
	attendanceResetOffsetSeconds = attendance.ResetOffsetSeconds

	// attendanceMailSender 是系统邮件的署名。源里 `[reward mail title]` 只给**键**
	// （`event_331_mail_title`），键→文案的表在客户端字符串表里（服务端要读它得再加一个
	// PVF 域）；署名与正文是**运营文案**，先用可读字面串，源的键记进事件日志备查。
	attendanceMailSender = "7-Day Journey"
)

// attendanceDayNumber 把某一时刻折算成"第几个签到日"（以 06:00 UTC 为界的日号）。
// 只有**差值**有意义：`today > lastClaimDay` 就表示跨过了一次换日。
func attendanceDayNumber(unixSeconds int64) int64 {
	return attendance.DayNumber(unixSeconds)
}

// attendanceAvailableToday 是"今天还能不能领一次"的**唯一判据**。
//
//	claimed      已领天数（0..7），`[allow accumulate attendance] 1` ⇒ 漏签不清零、单调递增
//	lastClaimDay 上一次领奖那天折算出的 day number（-1 = 从没领过）
//
// claimed == 0 表示本期一次都没领过 ⇒ 今天可领（官服新号样本就是 attended=1、第 1 格可领）。
// 领满 7 天后不再可领。
func attendanceAvailableToday(claimed int, lastClaimDay, today int64) bool {
	if claimed >= protocol.AttendanceDailyDays {
		return false
	}
	if claimed <= 0 {
		return true
	}
	return today > lastClaimDay
}

// attendanceForceOverride 读调试开关 `DFO_ATTENDANCE_FORCE=<已领天数>:<今天可领 0|1>`，
// 例如 `3:1` = 前 3 天已领、第 4 天可领（官服样本形态）。
//
// 开关设在**启动那个 cmd 会话的环境变量**里，不要写进 configs/pvf-default.json：
// launcher 的 `applyEnvironment` 对 profile 的环境键是严格白名单，未知键会让服务端起不来。
//
//	set DFO_ATTENDANCE_FORCE=3:1 && 调试启动-本地构建.cmd
//
// 取值不合法时返回 ok=false（调用方按存档走并记一条事件），不静默改语义。
func attendanceForceOverride() (claimed int, available bool, ok bool) {
	raw := strings.TrimSpace(os.Getenv("DFO_ATTENDANCE_FORCE"))
	if raw == "" {
		return 0, false, false
	}
	parts := strings.SplitN(raw, ":", 2)
	if len(parts) != 2 {
		return 0, false, false
	}
	gotClaimed, errClaimed := strconv.Atoi(strings.TrimSpace(parts[0]))
	gotAvailable, errAvailable := strconv.Atoi(strings.TrimSpace(parts[1]))
	if errClaimed != nil || errAvailable != nil {
		return 0, false, false
	}
	if gotClaimed < 0 || gotClaimed > protocol.AttendanceDailyDays || (gotAvailable != 0 && gotAvailable != 1) {
		return 0, false, false
	}
	return gotClaimed, gotAvailable == 1, true
}

// attendanceDailyBody 由「已领天数 + 上次领取日」算出 NOTI1379 载荷（纯函数，便于钉住）。
func attendanceDailyBody(now time.Time, claimed int, lastClaimDay int64) ([]byte, error) {
	available := attendanceAvailableToday(claimed, lastClaimDay, attendanceDayNumber(now.Unix()))
	// Flag 恒 0：官服样本全 0，非 0 会在客户端把 +0x48 置 1（强制红点并影响活动 332）。
	return protocol.AttendanceDailyState115(protocol.AttendanceDailyProgress115(claimed, available))
}

// attendanceDailyState 是**会话侧**的状态来源：读角色存档 →（可选）调试开关覆盖 → 编码。
//
// 返回的 State 是"归一化到本期之后"的进度（换期会自动从头算），领取路径与推送路径
// 共用它，避免两处各推一次口径。
func (w *worldSession) attendanceDailyState(now time.Time) (attendance.State, []byte, error) {
	st := attendance.State{Version: 1, EventID: attendance.EventID,
		CycleStart: int64(attendanceEventStart), LastClaimDay: -1}
	if w != nil && w.role.ID != 0 {
		stored, err := attendance.ReadState(w.role.State)
		if err != nil {
			return attendance.State{}, nil, err
		}
		st = stored.Normalize(int64(attendanceEventStart))
	}
	if forced, forcedAvailable, ok := attendanceForceOverride(); ok {
		// 开关只给"已领天数"和"今天能不能领"，把它翻回 lastClaimDay 才能走同一条
		// 真判据（attendanceAvailableToday），而不是在开关分支里另立一套。
		st.ClaimedDays = forced
		st.LastClaimDay = attendanceDayNumber(now.Unix())
		if forcedAvailable {
			st.LastClaimDay--
		}
	}
	body, err := attendanceDailyBody(now, st.ClaimedDays, st.LastClaimDay)
	return st, body, err
}

// logAttendanceDailyState 把**解码后**的状态记进会话日志。
//
// 实机核对窗口渲染时，这就是唯一的服务端真值：日志里的 flag/attended/day_state
// 必须与窗口里看到的"前 N 天打勾、第 N+1 天可点"对上（文档 §3.1 的官服样本给出了
// 三种期望形态）。
func (client *gameConnection) logAttendanceDailyState(roleID int64, body []byte) {
	if len(body) != protocol.AttendanceDailyStateBytes {
		return
	}
	attended := uint32(body[1]) | uint32(body[2])<<8 | uint32(body[3])<<16 | uint32(body[4])<<24
	client.event(map[string]any{
		"kind":         "attendance_daily_state_sent",
		"id":           1379,
		"character_id": roleID,
		"flag":         body[0],
		"attended":     attended,
		"day_state":    fmt.Sprintf("%v", body[5:]),
		"forced":       strings.TrimSpace(os.Getenv("DFO_ATTENDANCE_FORCE")),
		"plain_hex":    hex.EncodeToString(body),
	})
}

func (w *worldSession) attendanceStatus(ctx context.Context, now time.Time) ([]byte, error) {
	if w == nil || w.attendance == nil {
		return nil, nil
	}
	if w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.AccountID != w.account {
		return nil, fmt.Errorf("attendance requires owned role and game calendar")
	}
	day := w.attendanceClock().Day(now)
	r, err := w.attendance.Observe(ctx, w.role, now.In(w.attendanceClock().Location), day)
	if errors.Is(err, workflow.ErrAttendanceInactive) {
		w.attendanceDay = day
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	states, err := w.attendance.Rules.DayStates(r.Progress)
	if err != nil {
		return nil, err
	}
	p, err := protocol.AttendanceDailyStatus115(0, r.Progress.Visits, states)
	if err == nil {
		w.attendanceDay = day
	}
	return p, err
}

func (w *worldSession) claimAttendance(ctx context.Context, body []byte, now time.Time) ([]outboundPacket, error) {
	day, err := protocol.DecodeAttendanceDailyClaim115(body)
	refuse := func(err error) ([]outboundPacket, error) {
		// Existing local execution uses 23 for a refused operation. This is
		// a compatibility error, not an invented official attendance mapping.
		return []outboundPacket{{"attendance_claim_refused", 1, 680, protocol.AttendanceDailyRefusal115(23)}}, err
	}
	if err != nil {
		return refuse(err)
	}
	if w == nil || w.attendance == nil || w.attendanceClock() == nil || w.attendanceClock().Location == nil || w.role.AccountID != w.account {
		return refuse(fmt.Errorf("attendance is unavailable"))
	}
	r, err := w.attendance.Claim(ctx, w.role, now.In(w.attendanceClock().Location), w.attendanceClock().Day(now), day)
	if err != nil {
		return refuse(err)
	}
	states, err := w.attendance.Rules.DayStates(r.Progress)
	if err != nil {
		return nil, err
	}
	status, err := protocol.AttendanceDailyStatus115(0, r.Progress.Visits, states)
	if err != nil {
		return nil, err
	}
	// Match the captured 331 order: absolute progress, successful CMD680,
	// then the existing mailbox notifier. A retry never generates new mail.
	plan := []outboundPacket{{"attendance_claim_status", 0, 1379, status}, {"attendance_claim_ack", 1, 680, protocol.EventReply115(protocol.AttendanceDailyEvent115, [10]uint32{})}}
	if r.CreatedMail && w.notifyBoostMail != nil {
		w.notifyBoostMail(w.role.ID)
	}
	return plan, nil
}

func (client *gameConnection) dispatchAttendance(request *clientRequest) dispatchAction {
	if request.frame.Type != 1 || request.frame.ID != 680 || !request.verified || !client.bootstrapped || len(request.plaintext) < 4 || binary.LittleEndian.Uint32(request.plaintext) != protocol.AttendanceDailyEvent115 {
		return dispatchNext
	}
	if client.worldState == nil || client.worldState.attendance == nil {
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, err := client.worldState.claimAttendance(ctx, request.plaintext, time.Now())
	if err != nil {
		client.event(map[string]any{"kind": "attendance_claim_error", "character_id": client.selectedCharacterID, "reason": err.Error()})
	}
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

func attendanceOpening(s *workflow.AttendanceService, base []byte, location *time.Location) ([]byte, error) {
	if s == nil || location == nil || s.Rules.MinimumLevel != 0 {
		return nil, fmt.Errorf("attendance native level-gate branch is not supported")
	}
	start, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.Start, location)
	if err != nil {
		return nil, err
	}
	end, err := time.ParseInLocation("2006-01-02 15:04:05", s.Rules.Dates.End, location)
	if err != nil || start.Unix() < 0 || end.Unix() > 4294967295 {
		return nil, fmt.Errorf("attendance calendar exceeds native timestamp")
	}
	return protocol.AppendAttendanceOpening115(base, s.Rules.MailTextTitle, s.Rules.CalendarUI, uint32(start.Unix()), uint32(end.Unix()))
}
