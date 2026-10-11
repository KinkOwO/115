package main

// 活动 313 HoneyTime 的**领取**（CMD680）—— 派发段 + 帧序。
//
// ## 帧序（照 331 签到那条已实机验证过的硬约束）
//
//	C→S ch1 op680 (313, 行号)
//	  → ch0 op1294（把"已领"与最新秒数推回去）   ← **必须先于应答**：
//	      客户端那个窗口的成功回调只清在途标志，界面状态由 op1294 刷新（331 同款结论）
//	  → ch1 op680 应答
//	  → ch0 op99（新邮件）
//
// ## 行号口径（L0，见 next194 §12.5）
//
// 客户端发来的行号 = **脚本行首值（0 起）**：实机点第 1 档发 0、点第 2 档发 1
// ⇒ 行号就是 `Tier.Index`，不做 ±1（`honeytime.TierForClaim`）。
//
// ## 为什么只受理"有映射"的行号，别的一律静默放行
//
// 实机两种 680 形态（都用 `ENUM_CMDPACKET_EVENT_REWARD` 这个名字）：
//
//	进城**自动**发：8 字节  `313, 0`    ← 0 不对应任何一档（没有"第 0 档"）
//	玩家**点击**发：16 字节 `313, 1, 0, 0`
//
// 进城那条不是人点的、也没有可领的档（刚上线秒数还是 0），官服只会拒它。但我们**不主动
// 回拒绝帧**：那会在每次进城时给玩家弹一个无意义的失败提示；而且它到底期不期待应答还没证。
// ⇒ 行号映射不到任何档时**记事件 + dispatchNext**（交给兜底采样），什么都不回。
// 只有"行号有映射、但服务端认为不能领"（时间不够 / 今天已领）才回拒绝帧 —— 那必然是真人点击。
//
// ## 幂等
//
// 领取走 `workflow.HoneyTimeService.Claim`：状态更新与系统邮件在**同一事务**里，
// 事件键 `honeytime-313:<期>:<日>:<档>` ⇒ 同一档今天只入账一次（重放读回既有回执）。
import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/honeytime"
	"dfolan/internal/workflow"
	"encoding/hex"
	"fmt"
	"time"
)

// dispatchHoneyTimeClaim 是入站派发链里的一站：只管 CMD680 且活动号为 313 的那些帧。
func (client *gameConnection) dispatchHoneyTimeClaim(requestData *clientRequest) dispatchAction {
	w := client.worldState
	if requestData.frame.Type != 1 || !requestData.verified || !client.bootstrapped || w == nil || w.honeyTime == nil {
		return dispatchNext
	}
	if requestData.frame.ID != 680 {
		return dispatchNext
	}
	event, _, err := protocol.DecodeAttendanceClaim115(requestData.plaintext)
	if err != nil {
		// 载荷太短：不是我们能判断的帧，交给后面的阶段（它们会记 unimplemented_sample）。
		return dispatchNext
	}
	if event != honeytime.EventID {
		// 别的活动（331/662/665）的 680 —— 不接管。
		return dispatchNext
	}
	// ★ 行号在**第 3 个 u32**（不是第 2 个）：实机第 1 档点的是 `(1,0)`、第 2 档点的是 `(1,1)`，
	// 变的是第 3 个。按 331 的口径读第 2 个会把"点第 2 档"读成"又点第 1 档"（见协议文件注释）。
	action, row, err := protocol.DecodeHoneyTimeRequest115(requestData.plaintext)
	if err != nil {
		// 进城那条 8 字节自动帧（只有活动号 + 动作类型 0，没有档位下标）：不是一次领取。
		client.event(map[string]any{
			"kind":         "honey_time_claim_ignored",
			"character_id": w.role.ID,
			"reason":       "no offset field (sync frame on entry)",
			"request_hex":  hex.EncodeToString(requestData.plaintext),
		})
		return dispatchNext
	}
	ignore := func(reason string) dispatchAction {
		client.event(map[string]any{
			"kind":         "honey_time_claim_ignored",
			"character_id": w.role.ID,
			"action":       action,
			"offset":       row,
			"reason":       reason,
			"request_hex":  hex.EncodeToString(requestData.plaintext),
		})
		return dispatchNext
	}
	if action == protocol.HoneyTimeActionSync {
		// 进城自动帧：不是领取。
		return ignore("sync frame (action 0)")
	}
	// ★ 动作类型决定**哪条轨**（不是"点击/非点击"）：上游从 UI 虚拟回调 14412c770 读出
	// 四个数组对应 action 1/2/3/5（**跳过 4**），后面那个 u32 是数组内的 0 起下标。
	// 本季脚本只有 daily(第 0)/period(第 1) 两块 ⇒ action 1 = 日档、action 2 = 期间轨。
	// 见 protocol.HoneyTimeTrackForAction。
	target, known := protocol.HoneyTimeTrackForAction(action)
	if !known {
		return ignore("unknown claim action")
	}
	switch target {
	case protocol.HoneyTimeTrackSpecified, protocol.HoneyTimeTrackExtendDaily:
		// 本季脚本**没有** specified / extend_daily 的块 ⇒ 没有正面样本，不猜语义。
		return ignore("action maps to a group this season's script does not declare")
	case protocol.HoneyTimeTrackPeriod:
		if _, ok := w.honeyTime.PeriodTierForClaim(int(row)); !ok {
			return ignore("no period tier maps to this offset")
		}
	default:
		if _, ok := w.honeyTime.TierForClaim(int(row)); !ok {
			return ignore("no daily tier maps to this offset")
		}
	}
	track := int(target)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan, claimErr := w.claimHoneyTimeTier(ctx, track, int(row))
	if claimErr != nil {
		client.event(map[string]any{
			"kind":         "honey_time_claim_refused",
			"character_id": w.role.ID,
			"row":          row,
			"track":        track,
			"action":       action,
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

// claimHoneyTimeTier 领取某条轨上、行号 row 对应的那一档（行号 0 起 = 脚本行首值）。
//
// track：`workflow.TrackDaily` = 面板上面那行"每天的"；`workflow.TrackPeriod` = 下面那行
// "累计在线时长"。
func (w *worldSession) claimHoneyTimeTier(ctx context.Context, track, row int) ([]outboundPacket, error) {
	// 失败码取官服口径（与 331 同一套）：19 = 不在活动期 / 102 = 包内容不对。
	// 具体原因写进服务端事件，"客户端看到什么"与"服务端为什么拒绝"分开记。
	refuse := func(code uint16, err error) ([]outboundPacket, error) {
		return []outboundPacket{{"honey_time_claim_refused", 1, 680, protocol.EventRefusal115(honeytime.EventID, code, false)}}, err
	}
	if w == nil || w.honeyTime == nil {
		return refuse(19, fmt.Errorf("honeytime 313 catalog is not prepared"))
	}
	if w.role.ID == 0 || w.store == nil {
		return refuse(19, fmt.Errorf("honeytime claim requires a persisted character"))
	}
	now := time.Now()
	today := honeytime.Today(now.Unix())
	svc := &workflow.HoneyTimeService{Store: w.store, Catalog: w.honeyTime}
	role, receipt, applied, err := svc.Claim(ctx, w.role, track, row, today,
		int64(honeyTimeEventStart), honeyTimeMailSender, honeyTimeMailText(w.honeyTime, track, row))
	if err != nil {
		return refuse(102, err)
	}
	// 会话里的角色必须换成刚写库的那一份：推的 1294 按它现算，
	// 不换就会一直推"没领过"的状态（症状：领完按钮还是亮的）。
	w.role = role

	// 帧序是硬约束（见文件头）：**状态帧排在应答之前**。
	// 但状态帧编码失败（多半是诊断开关被设成了非法值）**不能连应答也不回** ——
	// 奖励已经发出去了，应答必须回，否则客户端那个按钮会一直停在"在途"状态。
	plan := make([]outboundPacket, 0, 3)
	if body, _, bodyErr := honeyTimeStateBody(receipt.State); bodyErr != nil {
		fmt.Printf("honeytime claim: state frame encode failed: %v\n", bodyErr)
	} else {
		plan = append(plan, outboundPacket{"honey_time_state_sent", 0, honeyTimeNotiID, body})
	}
	plan = append(plan, outboundPacket{"honey_time_claim_reply", 1, 680, protocol.AttendanceClaimReply115(honeytime.EventID, uint32(row))})
	// 新邮件入箱 → 推 op99（文档 §3.2 的最后一步）。取不到就只丢这一帧。
	if alarm, _, alarmErr := w.mailboxAlarm(ctx); alarmErr != nil {
		fmt.Printf("honeytime claim: mailbox alarm failed: %v\n", alarmErr)
	} else if len(alarm) > 0 {
		plan = append(plan, outboundPacket{"honey_time_mail_alarm", 0, 99, alarm})
	}
	fmt.Printf("honeytime claim track=%d row=%d tier=%d applied=%v mail=%d daily_s=%d daily_claimed=0x%x period_s=%d period_claimed=0x%x rewards=%d\n",
		track, row, receipt.TierIndex, applied, receipt.MailID,
		receipt.State.Seconds, receipt.State.Claimed,
		receipt.State.PeriodSeconds, receipt.State.PeriodClaimed, len(receipt.Rewards))
	return plan, nil
}

// honeyTimeMailText 是系统邮件的正文。
//
// 源里**没有**文案键（逐字核对过 honeytime.evt：只有 `[reward postal type] 1` 指明走邮件，
// 连 `[reward mail title]` / `[reward mail message]` 都没有），所以正文只能由服务端给。
// 正文属于**运营文案**（不是玩法规则）。写**累计分钟**而不是行号：行号是 0 起的内部口径
// （第 1 档的行号是 0），直接写进文案会让玩家看到 "tier 0" 这种怪东西。
func honeyTimeMailText(cat *honeytime.Catalog, track, row int) string {
	if cat != nil {
		if track == workflow.TrackPeriod {
			// 期间轨的门槛是 20 小时一档，用**小时**写才对得上玩家的认知。
			if minutes, ok := cat.PeriodMinutesForTier(row); ok {
				return fmt.Sprintf("Honey Time: %d hours of accumulated online time reached.", minutes/60)
			}
		} else if minutes, ok := cat.MinutesForTier(row); ok {
			return fmt.Sprintf("Honey Time: %d minutes of online time reached.", minutes)
		}
	}
	return "Honey Time: online time reward."
}
