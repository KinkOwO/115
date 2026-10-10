package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// dispatchEvildom 是次元回廊（内容 0x0d）的统一派发层：
// 待机区组队（CMD12/13）→ 开战（CMD2043 内容 0x0d）→ 进图确认（CMD2045）。
//
// **必须排在 dispatchLegion 之前**：军团家族共用 CMD2043/2045/2046 这套信封，
// 而 legion.Requests() 只认家族命令、不含 CMD12；若不在这里接管，CMD12 会掉到
// 通用队伍链（队伍类型 0x0d 无人认领）—— 客户端表现就是业主报的
// 「创建队伍、输入名称、点确认后没有任何反应」。
//
// ★ 判据必须**自带身份**，不能只看频道类型（2026-10-10 实机证据）：
// 那次实测的事件是 `apocalypse_party_probe channel_type=84`，也就是玩家进的
// 次元回廊频道是 **Type 84（Hall of Dimensions）**；而 `channel.local35.json`
// 里另有一行 Type 50「Evildom」。按 Type 50 单点判据会把 Type 84 的真实请求
// 全部漏掉 —— 那一场里 CMD12 确实到了服务端，却一条 dim_cloister_* 事件都没有。
// （症状对照：`apocalypse_party_probe` 被记下，说明 dispatchLegion 收到了它，
// 而 dispatchEvildom 因为频道类型不等于 50 直接放行了。）
//
// 正确判据分两层，都用**内容身份**而不是频道类型：
//
//	CMD12  —— 请求里的**队伍类型字节 0x0d**（军团家族里只有次元回廊用它：
//	          末世录 0x26 / 伊斯 0x0b / 维纳斯 0x22）；
//	CMD13  —— 只有本连接已经建过次元回廊队伍时接管；
//	CMD2043/2045 —— 信封里的**内容号 0x0d**。
//
// 这条纪律与苏醒之森 2026-10-08 的教训一致：内容号是各内容互不相认的标签，
// 不属于本内容的请求必须放行，绝不代答。
func (client *gameConnection) dispatchEvildom(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	if !requestData.verified {
		return dispatchNext
	}
	w := client.worldState
	s := &client.legionState
	if s == nil {
		return dispatchNext
	}

	// 待机区组队：CMD12 建队、CMD13 离队。判据是请求自带的队伍类型字节。
	if requestData.frame.ID == 12 || requestData.frame.ID == 13 {
		if requestData.frame.ID == 12 && !isEvildomPartyCreate(requestData.plaintext) {
			return dispatchNext
		}
		if requestData.frame.ID == 13 && !s.dimCloisterPartyActive {
			return dispatchNext
		}
		result, err := s.handleEvildomParty(w, requestData.plaintext, requestData.frame.ID)
		if err != nil {
			client.event(map[string]any{
				"kind":         "dim_cloister_party_request_rejected",
				"id":           requestData.frame.ID,
				"channel_type": w.channelType,
				"error":        err.Error(),
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			result = legionResult{Packets: []outboundPacket{{
				"次元回廊待机区队伍请求拒绝应答", 1, requestData.frame.ID, protocol.Refusal(8),
			}}}
		}
		for _, note := range result.Events {
			note["id"] = requestData.frame.ID
			note["channel_type"] = w.channelType
			client.event(note)
		}
		if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	// 开战：CMD2043 必须携带本内容号（0x0d）。内容号不符说明这不是次元回廊的
	// 开战请求 —— 放行给后面的派发层，绝不代答（苏醒之森 2026-10-08 的教训：
	// 代答会让客户端以为开战成功，而服务端从未进入流程）。
	if requestData.frame.ID == legion.CmdStart {
		if len(requestData.plaintext) < legion.EnvelopeSize+8 {
			return dispatchNext
		}
		content := binary.LittleEndian.Uint32(requestData.plaintext[legion.EnvelopeSize-5:])
		if content != legion.DimCloisterContent {
			client.event(map[string]any{
				"kind":         "dim_cloister_start_foreign_content",
				"id":           requestData.frame.ID,
				"content":      content,
				"channel_type": w.channelType,
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			return dispatchNext
		}
		result, windowFrames, err := s.dimCloisterStart(w, requestData.plaintext)
		if err != nil {
			client.event(map[string]any{
				"kind":         "dim_cloister_refused",
				"id":           requestData.frame.ID,
				"reason":       err.Error(),
				"channel_type": w.channelType,
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			return dispatchHandled
		}
		for _, note := range result.Events {
			note["id"] = requestData.frame.ID
			note["channel_type"] = w.channelType
			client.event(note)
		}
		if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		// 窗口状态帧已排进队列：起精确定时器（这条连接上 mineTicker 不触发，
		// 必须走 connectionSession.cloisterWindow）。
		if windowFrames > 0 && client.connection != nil {
			client.connection.scheduleCloisterWindow(dimCloisterWindowDelay)
			client.event(map[string]any{
				"kind":        "dim_cloister_info_scheduled",
				"id":          requestData.frame.ID,
				"delay_ms":    dimCloisterWindowDelay.Milliseconds(),
				"frame_count": windowFrames,
			})
		}
		return dispatchHandled
	}

	// 难度/关卡选择：客户端点右上角 UI 时发 CMD2080
	//（ENUM_CMDPACKET_MYRES_DIMENSION_CLOISTER_OPERATION_SELECT，opcode 表里点名的
	// 次元回廊专有命令）。走同族已验证的路：回操作窗应答（末世录 CMD2354 / 维纳斯
	// CMD2290 同布局），窗口由 ACK 自己打开。
	if requestData.frame.ID == legion.CmdDimCloisterOperationSelect && s.isDimCloister() {
		result, err := s.dimCloisterSelect(w, requestData.plaintext)
		if err != nil {
			client.event(map[string]any{
				"kind":        "dim_cloister_select_rejected",
				"id":          requestData.frame.ID,
				"error":       err.Error(),
				"request_hex": hex.EncodeToString(requestData.plaintext),
			})
			return dispatchHandled
		}
		for _, note := range result.Events {
			note["id"] = requestData.frame.ID
			client.event(note)
		}
		if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		// 到期关窗/自动确认这些待发事件由 dimCloisterSelect 自己排（它才知道截止值），
		// 这里只负责唤醒精确定时器 —— 按**最近一条事件的到期时刻**唤醒，
		// 不要用固定时长（以前写死 30 秒，导致「停留 3 秒自动选难度」要等到 30 秒才生效）。
		if next := s.dimCloisterNextEventDue(); !next.IsZero() && client.connection != nil {
			if d := time.Until(next); d > 0 {
				client.connection.scheduleCloisterWindow(d)
			}
		}
		return dispatchHandled
	}

	// 关窗：CMD2081（ENUM_CMDPACKET_MYRES_DIMENSION_CLOISTER_OPERATION_CLEAR）。
	if requestData.frame.ID == legion.CmdDimCloisterOperationClear && s.isDimCloister() {
		result, err := s.dimCloisterOperationClear(w, requestData.plaintext)
		if err != nil {
			return dispatchHandled
		}
		s.dimCloisterWindowDeadline = time.Time{}
		for _, note := range result.Events {
			note["id"] = requestData.frame.ID
			client.event(note)
		}
		if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	// 进图确认（CMD2045）也在这里接管：它**带着玩家选中的关卡号**，是加载副本的
	// 唯一时机（CMD2043 只开窗口）。
	if requestData.frame.ID == legion.CmdEnterDungeon {
		result, err := s.dimCloisterLoadStage(w, requestData.plaintext)
		if err != nil {
			client.event(map[string]any{
				"kind":         "dim_cloister_stage_refused",
				"id":           requestData.frame.ID,
				"reason":       err.Error(),
				"channel_type": w.channelType,
				"request_hex":  hex.EncodeToString(requestData.plaintext),
			})
			return dispatchHandled
		}
		for _, note := range result.Events {
			note["id"] = requestData.frame.ID
			note["channel_type"] = w.channelType
			client.event(note)
		}
		if client.sendPlan(result.Packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

// isEvildomPartyCreate 判断一个 CMD12 是不是次元回廊的建队请求。
//
// 判据是**请求自带的队伍类型字节 = 0x0d**。不校验频道类型：次元回廊在不同客户端
// 版本上挂过 Type 50（Evildom）与 Type 84（Hall of Dimensions）两行，而待机区
// 建队本来就发生在城镇侧连接上。
//
// 解析失败一律返回 false（放行给别的派发层），不做「猜着代答」。
func isEvildomPartyCreate(p []byte) bool {
	if len(p) < 36 {
		return false
	}
	n := int(binary.LittleEndian.Uint32(p[2:]))
	if n < 1 || n > 32 || n > len(p)-36 {
		return false
	}
	at := 6 + n
	if at+11 > len(p) {
		return false
	}
	return p[at+9] == legion.EvildomPartyType
}

// handleEvildomParty 处理次元回廊待机区的建队/离队。
func (s *legionSession) handleEvildomParty(w *worldSession, p []byte, id uint16) (legionResult, error) {
	switch id {
	case 12:
		return s.dimCloisterCreateParty(w, p)
	case 13:
		return s.dimCloisterLeaveParty(w, p)
	default:
		return legionResult{}, fmt.Errorf("次元回廊待机区不处理 CMD%d", id)
	}
}
