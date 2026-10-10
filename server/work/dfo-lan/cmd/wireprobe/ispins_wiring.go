package main

// ispins_wiring.go — next79 伊斯大陆功能在重构后分发架构上的挂接层。
//
// 原实现的全部挂接点都在 main.go 的会话主循环里（0a122e0 快照，字节契约
// 与实证见 docs/protocol/next79-legion-weekly-open.md）。上游
// domain-consolidation 重构把主循环拆成 client_dispatch_*.go 派发链后，
// 这里集中承载同一批挂接：
//
//   - dispatchIspins：CMD35 重复恢复、待机首帧 N2254/781/782、待机组队、
//     伊斯族请求拦截、终局剧情 191（注册为 beforeClientTypeDispatch 首位，
//     必须排在 dispatchLegion / dispatchStoryAndAdvancement 之前）。
//   - sendLoginFloodOnce：选角前登录洪流（restoreRosterBackgrounds 之后）。
//   - ispinsPostSelection：选角成功后的 N1719 保活与 N2254 登录推送分支。
//
// 调用侧：client_dispatch.go（stage 列表）、client_dispatch_character.go
//（roster 恢复与 637/782 应答）、client_entry.go（选角成功尾部）。

import (
	"encoding/hex"
	"time"

	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// raidInOutSystemReply 是对 c2s 637 探测（ENUM_CMDPACKET_CHARAC_VIEW_HIDDEN_
// CHARAC_INFO）的应答体，与官服抓包帧 86/792 对齐（城镇与伊斯待机入场两处
// 逐字节一致）：flag=01 + u32 0x5e2cd238 + u16 0x0040。官服语义未解析，但
// 2026-10-03 实测：待机连接不应答、城镇连接计数全零时，客户端在伊斯待机区
// 点发起作战直接本地拦截（连 CMD2043 都不发）——同黑鸦计数缺省 0 直接拦截
// 建队请求的先例（protocol/black_purgatory.go）。
var raidInOutSystemReply = []byte{0x01, 0x00, 0x38, 0xd2, 0x2c, 0x5e, 0x40, 0x00}

// sendLoginFloodOnce 发送账号级事件洪流（next79 §13/§15）：官服在 1759
// 之后、SELECT_CHARACTER 之前推送 708 → 1198 → 1336 → 1792（108 事件表
// 已移出洪流，见 entry_flow.go loginFloodPackets 注释）。每连接一次。
func (client *gameConnection) sendLoginFloodOnce() error {
	if client.loginEventFloodSent {
		return nil
	}
	client.loginEventFloodSent = true
	return client.sendPlan(loginFloodPackets(), client.logCharacterResponse)
}

// dispatchIspins 是伊斯大陆族的统一派发层。顺序即契约（原 main.go 帧循环
// 自上而下）：CMD35 恢复 → 待机首帧 N2254 → 待机组队 → 伊斯族拦截 → 终局
// 剧情 191。必须在 dispatchLegion（末世录共用信封）与通用 191 处理器之前。
func (client *gameConnection) dispatchIspins(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !client.bootstrapped || client.worldState == nil {
		return dispatchNext
	}
	w := client.worldState

	// Full Ispins single-player replay: CMD35 is town-position input,
	// the native readiness signal already used for standby quota data.
	// Do not push quota restoration during the final movie/map transition.
	if requestData.frame.ID == 35 && requestData.verified && client.selectedCharacterID != 0 {
		retry, err := w.ispinsRetryRestorePackets(true)
		if err != nil {
			client.event(map[string]any{"kind": "ispins_retry_restore_error", "error": err.Error()})
		} else if len(retry) != 0 {
			if client.sendPlan(retry, client.logWorldResponseBody) != nil {
				return dispatchClose
			}
			w.ispinsRetryPending = false
		}
		plan, err := w.ispinsRepeatRestorePackets()
		if err != nil {
			client.event(map[string]any{"kind": "ispins_repeat_restore_error", "error": err.Error()})
		} else if len(plan) != 0 {
			if client.sendPlan(plan, func(packet outboundPacket) {
				client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": client.selectedCharacterID, "trigger": "post_clear_town_position", "plain_bytes": len(packet.Payload)})
			}) != nil {
				return dispatchClose
			}
			w.ispinsRepeatPending = false
			if w.ispins != nil && w.ispins.finalDone && w.ispins.storyFinished {
				w.ispins = nil
			}
		}
		// This is an extra notification, not ownership of CMD35. The old
		// frame loop continued into town movement and pending standby data.
	}

	// 伊斯频道（Type 81）待机区分支：客户端入场后的第一帧 c2s 是场景
	// 就绪信号（官服为 c2s 35 位置上报，帧 894；N2254 官服帧 998 在其
	// 后送达）。此时补发挂起的 N2254；装载期内发送会硬崩客户端
	// （2026-10-03 实测，client_trace 止于 ImageScheduler LV Changed）。
	if w.pendingLegionEntryInfo && requestData.verified && client.selectedCharacterID != 0 {
		w.pendingLegionEntryInfo = false
		// 待机入场体必须对齐官服 s4 帧 257（旗标全零 + 21/117=7f，
		// 即 login 同形）：帧 331/997 的全开旗标是官服「本周已打满」
		// 状态，replay 会导致面板奖励 0/1 并本地拦截建队
		// （next79 §19，2026-10-03）。
		if payload, err := w.ispinsStandbyQuotaInfo(); err == nil {
			if err := client.output.send(0, legion.NotiIspinsEntryCharacterInfo, payload); err == nil {
				client.event(map[string]any{"kind": "ispins_login_entry_character_info_sent", "character_id": client.selectedCharacterID, "plain_bytes": len(payload), "trigger": "first_post_entry_frame", "frame_id": requestData.frame.ID})
			}
		}
		// 待机区入场后推周本「无限难度」状态（官服帧 1032→1033，
		// 待机版体：状态 u32 f4，与城镇版 367/368 记录值不同，见
		// weekly_difficulty_info_generated.go）。官服在场景就绪
		// （c2s 35 帧 894）之后送达；装载期内推送会硬崩客户端
		// （next79 §17），故与 N2254 同走首帧 c2s 触发窗。这是伊斯
		// 待机区「创建队伍」面板周计数的数据源。
		if err := client.output.send(0, 781, weeklyDifficultyInfoUserStandby); err == nil {
			if err := client.output.send(0, 782, weeklyDifficultyInfoCharacStandby); err == nil {
				client.event(map[string]any{"kind": "weekly_difficulty_info_sent", "character_id": client.selectedCharacterID, "place": "standby", "user_bytes": len(weeklyDifficultyInfoUserStandby), "charac_bytes": len(weeklyDifficultyInfoCharacStandby), "trigger": "first_post_entry_frame", "frame_id": requestData.frame.ID})
			}
		}
	}

	// 伊斯待机区组队（CMD12 建队等，原实现挂在 moonHandle 之后）。
	if requestData.verified {
		handled, packets, e := w.ispinsStandbyPartyHandle(requestData.frame.ID, requestData.plaintext)
		if handled {
			if e != nil {
				client.event(map[string]any{"kind": "ispins_party_request_rejected", "id": requestData.frame.ID, "error": e.Error()})
				packets = []outboundPacket{{"伊斯待机区队伍请求拒绝应答", 1, requestData.frame.ID, protocol.Refusal(8)}}
			}
			if client.sendPlan(packets, client.logCharacterResponse) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
	}

	// 伊斯大陆族（内容号 101）：CMD2047 专属，CMD2043/2045/2046 与末世录
	// 共用信封、按内容号分流。必须在末世录 legion 分发之前拦截。
	if requestData.verified && isIspinsRequest(requestData.frame.ID, requestData.plaintext) {
		ispinsPlan, ispinsNotes, ispinsErr := w.handleIspins(requestData.plaintext, requestData.frame.ID)
		if ispinsErr != nil {
			client.event(map[string]any{"kind": "ispins_refused", "id": requestData.frame.ID, "reason": ispinsErr.Error(), "request_hex": hex.EncodeToString(requestData.plaintext)})
			return dispatchHandled
		}
		for _, note := range ispinsNotes {
			note["id"] = requestData.frame.ID
			client.event(note)
		}
		if client.sendPlan(ispinsPlan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		// The merge preserved the immediate start batch but omitted f384:
		// the native client waits for this N2255 before sending CMD2047.
		// Snapshot before scheduling, as in the confirmed backup main loop.
		if requestData.frame.ID == legion.CmdStart {
			body, err := w.ispinsWaitInfo()
			if err == nil {
				roleID := w.role.ID
				var done <-chan struct{}
				if client.connection != nil {
					done = client.connection.done
				}
				go func() {
					timer := time.NewTimer(2500 * time.Millisecond)
					defer timer.Stop()
					select {
					case <-timer.C:
					case <-done:
						return
					}
					if client.output.send(0, legion.NotiIspinsInfo, body) != nil {
						return
					}
					client.event(map[string]any{"kind": "ispins_info_wait_pushed", "id": legion.NotiIspinsInfo, "character_id": roleID})
				}()
			}
		}
		return dispatchHandled
	}

	// 伊斯大陆终局剧情：此时副本会话已随 CMD2046 结算清空，通用
	// 191 处理器的「必须有活动副本」门槛不适用；N170 载荷也走
	// ispins 族自己的官服向量（f787/f793）。
	if requestData.frame.ID == 191 && requestData.verified && w.ispins != nil && w.ispins.finalDone {
		plan, notes, e := w.ispinsStoryPause(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "ispins_story_pause_refused", "reason": e.Error()})
			return dispatchHandled
		}
		for _, note := range notes {
			note["id"] = requestData.frame.ID
			client.event(note)
		}
		if client.sendPlan(plan, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

// ispinsPostSelection 承载选角成功后的两个 next79 挂接（原 main.go 选角
// 分支尾部）：N1719 保活 goroutine 与登录期 N2254 分支。调用处在
// client_entry.go 的 announceSelf 之后、好感度同步之前。
func (client *gameConnection) ispinsPostSelection(roleID int64) {
	w := client.worldState
	if w == nil {
		return
	}
	// 官服保活：SEC_PING_CHECK(N1719) 每 30s 一次（s4 帧 435-835、
	// switch f645/f697 间隔实测 30s），16B 常量体，客户端回
	// CMD1706（不在 relevantRequest 白名单，走采样记录无副作用）。
	// 待机区长闲置时它几乎唯一的 s2c 流量来源：缺失时客户端
	// ~14 分钟收不到任何下行即超时，弹「请检查网络连接」并退出
	// （next79 §20，052757 会话 21:31:05-21:45:31 实证）。发送
	// 失败（连接已断）即退出 goroutine。
	go func(characterID int64) {
		body := []byte{0, 0, 0, 0, 0xe1, 0x6b, 0xa3, 0x76, 0x3f, 0, 0, 0, 0, 0, 0, 0}
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			if err := client.output.send(0, 1719, body); err != nil {
				return
			}
			client.event(map[string]any{"kind": "sec_ping_check_sent", "character_id": characterID})
		}
	}(roleID)

	// 登录期 N2254（官服世界装载串内下发，双会话实证：普通频道
	// announce s1 f265 / s4 f257 都带这一帧，且与既有 f257 模板
	// 逐字节一致）。它是军团 tab 的状态底座——只发伊斯频道的话，
	// 普通频道里客户端的军团入口门禁会静默拦截点击（连 CMD2043
	// 都不发，next79 §6）。载荷为官服原文回放：条目 flag（7f/00）
	// 语义未解，与频道入口锁定相关（next78 §2.2）。
	// 普通频道沿用 1.1s 延迟（客户端 124 装载完成后子系统才就绪，
	// 参考好感度同步的时序结论）。**伊斯频道（Type 81）例外**：
	// 官服待机区抓包中 N2254 在客户端已在场景内（c2s 35 上报之后，
	// 帧 998 > 894）才送达；固定 1.1s 推送会撞进待机场景装载期，
	// 2026-10-03 实测客户端硬崩（client_trace 仅剩 ImageScheduler
	// LV Changed，USERDMP 为空）。改为挂 pending 标志，等入场后
	// 第一帧 c2s（场景就绪信号）再发（dispatchIspins）。
	if w.channelType == 81 {
		w.pendingLegionEntryInfo = true
		return
	}
	// 次元回廊（实机频道 Type 84，`Channel Type : [ CHANNEL_DIMENSION_CLOISTER_LEGION ]`）
	// 不能拿伊斯那一族的 N2254 当登录底座：同一 opcode 2254 在两个内容里是**不同
	// 结构**（伊斯族的阶段记录 vs 次元回廊 272B 的内容行账本），而客户端的难度窗会
	// 读这几行的旗标。2026-10-10 会话实证：那条把伊斯形态（`@117=7f`）发给了次元
	// 回廊频道，客户端在开窗时 exit=0xC0000005。
	if payload, isCloister, entryErr := dimCloisterLoginLedger(w.channelType, uint64(time.Now().UnixMilli())); isCloister {
		if entryErr != nil {
			client.event(map[string]any{"kind": "dim_cloister_ledger_restore_error", "error": entryErr.Error()})
			return
		}
		go func(characterID int64, body []byte) {
			time.Sleep(1100 * time.Millisecond)
			if err := client.output.send(0, legion.NotiEntryCharacterInfo, body); err != nil {
				return
			}
			client.event(map[string]any{"kind": "dim_cloister_entry_ledger_sent", "character_id": characterID, "plain_bytes": len(body), "trigger": "login", "channel_type": w.channelType})
			// 周本「无限难度」与伊斯同窗（官服在场景就绪后送达）；次元回廊待机区
			// 的周计数面板同样读这两帧。
			if err := client.output.send(0, 781, weeklyDifficultyInfoUserStandby); err != nil {
				return
			}
			if err := client.output.send(0, 782, weeklyDifficultyInfoCharacStandby); err != nil {
				return
			}
			client.event(map[string]any{"kind": "weekly_difficulty_info_sent", "character_id": characterID, "place": "cloister-standby", "user_bytes": len(weeklyDifficultyInfoUserStandby), "charac_bytes": len(weeklyDifficultyInfoCharacStandby)})
		}(roleID, payload)
		return
	}
	quotaBody, quotaErr := w.ispinsLoginQuotaInfo()
	if quotaErr != nil {
		client.event(map[string]any{"kind": "ispins_quota_restore_error", "error": quotaErr.Error()})
		return
	}
	go func(characterID int64, payload []byte) {
		time.Sleep(1100 * time.Millisecond)
		if err := client.output.send(0, legion.NotiIspinsEntryCharacterInfo, payload); err != nil {
			return
		}
		client.event(map[string]any{"kind": "ispins_login_entry_character_info_sent", "character_id": characterID, "plain_bytes": len(payload)})
		// 城镇入场后推周本「无限难度」状态（官服帧 367→368，
		// 272B USER + 400B CHARAC，见 weekly_difficulty_info_
		// generated.go）。官服在场景就绪（c2s 35）之后送达，
		// 与 N2254 同窗；缺失时客户端周本计数缺省 0，伊斯
		// 待机区点创建队伍被「本周奖励已全部领取」拦截。
		if err := client.output.send(0, 781, weeklyDifficultyInfoUserTown); err != nil {
			return
		}
		if err := client.output.send(0, 782, weeklyDifficultyInfoCharacTown); err != nil {
			return
		}
		client.event(map[string]any{"kind": "weekly_difficulty_info_sent", "character_id": characterID, "place": "town", "user_bytes": len(weeklyDifficultyInfoUserTown), "charac_bytes": len(weeklyDifficultyInfoCharacTown)})
	}(roleID, quotaBody)
}
