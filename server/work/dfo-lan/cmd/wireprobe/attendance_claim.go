package main

// 活动 331 每日签到的**领取**（CMD680）—— 派发段 + 帧序。
//
// ## 官服帧序（文档 §3.2，逐条照做）
//
//	C→S op680 (331, 行号)
//	  → ch0 op1379（该天改 2）      ← **必须先于应答**：客户端 331 对象的成功回调只清在途
//	                                  标志（+0x58），界面状态只由 op1379 刷新
//	  → ch1 op680 应答
//	  → ch0 op99（新邮件）
//
// ## 为什么派发段只认 680 且只看活动号
//
// `680` 是**通用活动领奖**帧，662/665 也走它（boostup_training / boostup_challenge）。
// 这里只在「载荷里的活动号 == 331」时才接管，其余原样交给后面的阶段 —— 否则会把
// 别的活动的领奖吃掉（本仓踩过"领取路径不同却复用同一兜底函数"的坑，见 next184）。
//
// ## 幂等
//
// 领取走 `workflow.AttendanceService.Claim`：状态更新与系统邮件在**同一事务**里，
// 事件键 `attendance-331:<期>:<天>` ⇒ 重复点同一天只会入账一次（重放读回既有回执）。
import (
	"context"
	"dfolan/internal/attendance"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"encoding/hex"
	"fmt"
	"time"
)

// dispatchAttendanceClaim 是入站派发链里的一站：只管 CMD680 且活动号为 331 的那些帧。
func (client *gameConnection) dispatchAttendanceClaim(requestData *clientRequest) dispatchAction {
	w := client.worldState
	if requestData.frame.Type != 1 || !requestData.verified || !client.bootstrapped || w == nil || w.attendance == nil {
		return dispatchNext
	}
	if requestData.frame.ID != 680 {
		return dispatchNext
	}
	event, day, err := protocol.DecodeAttendanceClaim115(requestData.plaintext)
	if err != nil {
		// 载荷太短：不是我们能判断的帧，交给后面的阶段（它们会记 unimplemented_sample）。
		return dispatchNext
	}
	if event != attendance.EventID {
		// 别的活动（662/665）的 680 —— 不接管。
		return dispatchNext
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, claimErr := w.claimAttendanceDay(ctx, day)
	if claimErr != nil {
		client.event(map[string]any{
			"kind":         "attendance_claim_refused",
			"character_id": w.role.ID,
			"day":          day,
			"reason":       claimErr.Error(),
			"request_hex":  hex.EncodeToString(requestData.plaintext),
		})
	}
	// 拒绝帧本身就在 plan 里（kind=1 回执），有错也要先把包发出去再收日志。
	if client.sendPlan(plan, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

// claimAttendanceDay 领取第 day 天（0 起，客户端点的就是它认为可领的那一格）。
func (w *worldSession) claimAttendanceDay(ctx context.Context, day uint32) ([]outboundPacket, error) {
	// 失败码取官服口径（文档 §2）：19 = 不在活动期 / 102 = 包内容不对。
	// 具体原因写进服务端事件，"客户端看到什么"与"服务端为什么拒绝"分开记。
	refuse := func(code uint16, err error) ([]outboundPacket, error) {
		return []outboundPacket{{"attendance_claim_refused", 1, 680, protocol.EventRefusal115(attendance.EventID, code, false)}}, err
	}
	if w == nil || w.attendance == nil {
		return refuse(19, fmt.Errorf("attendance 331 catalog is not prepared"))
	}
	if w.role.ID == 0 || w.store == nil {
		return refuse(19, fmt.Errorf("attendance claim requires a persisted character"))
	}
	now := time.Now()
	today := attendanceDayNumber(now.Unix())
	svc := &workflow.AttendanceService{Store: w.store, Catalog: w.attendance}
	role, receipt, applied, err := svc.Claim(ctx, w.role, int(day), today,
		int64(attendanceEventStart), attendanceMailSender, attendanceMailText(int(day)))
	if err != nil {
		return refuse(102, err)
	}
	// 会话里的角色必须换成刚写库的那一份：进城/刷新推的 op1379 按它现算，
	// 不换就会一直推旧的那一天（症状：领完窗口还是"第 1 天可领"）。
	w.role = role

	_, body, err := w.attendanceDailyState(now)
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{
		// 先推状态（该天变 2），再回应答 —— 顺序是硬约束（见文件头）。
		{"attendance_daily_state_sent", 0, 1379, body},
		{"attendance_claim_reply", 1, 680, protocol.AttendanceClaimReply115(attendance.EventID, day)},
	}
	// 新邮件入箱 → 推 op99（文档 §3.2 的最后一步）。取不到就只丢这一帧。
	if alarm, _, alarmErr := w.mailboxAlarm(ctx); alarmErr != nil {
		fmt.Printf("attendance claim: mailbox alarm failed: %v\n", alarmErr)
	} else if len(alarm) > 0 {
		plan = append(plan, outboundPacket{"attendance_mail_alarm", 0, 99, alarm})
	}
	fmt.Printf("attendance claim day=%d applied=%v mail=%d state=%+v rewards=%d\n",
		day, applied, receipt.MailID, receipt.State, len(receipt.Rewards))
	return plan, nil
}

// attendanceMailText 是系统邮件的正文。
//
// 源里 `[reward mail message]` 只给键（`event_331_mail_msg`），键→文案的表在客户端
// 字符串表里；服务端要读它得再加一个 PVF 域。正文属于**运营文案**（不是玩法规则），
// 先用可读字面串，并把源里的键记进事件日志备查（见 next193 的待办）。
func attendanceMailText(day int) string {
	return fmt.Sprintf("Day %d attendance reward.", day+1)
}
