package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/hex"
	"fmt"
	"time"
)

func (client *gameConnection) dispatchSpecialContent(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.characters != nil && client.selectedCharacterID != 0 && requestData.frame.ID == 1462 {
		// 1402359F0发送无正文请求，实机20260929_005313已确认。
		if len(requestData.plaintext) != 0 {
			client.event(map[string]any{"kind": "账号角色资料请求被拒绝", "error": "请求正文应为空"})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		payload, roster, err := client.characters.AllServerRoster(ctx, client.developmentAccount, client.fatigueService, time.Now())
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "账号角色资料读取失败", "error": err.Error()})
			return dispatchHandled
		}
		// 1444FCE40读取服务器数量u8及各服务器编号u8、角色数u16。
		count := len(roster)
		counts := []byte{1, client.characters.ChannelContext[0], byte(count), byte(count >> 8)}
		if err = client.output.send(0, 1396, counts); err != nil {
			return dispatchClose
		}
		if err = client.output.send(0, 2, payload); err != nil {
			return dispatchClose
		}
		if client.worldState != nil && client.worldState.channelType == 106 {
			client.worldState.bleedingMineRoster = roster
			// 名单索引与本次下发的账号列表一致，重开编队时也恢复已保存配置。
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			profile, err := client.worldState.bleedingMineProfile(ctx, roster)
			cancel()
			if err != nil {
				client.event(map[string]any{"kind": "赤红铁矿编队恢复失败", "error": err.Error()})
			} else if err = client.output.send(profile.Kind, profile.ID, profile.Payload); err != nil {
				return dispatchClose
			}
		}
		client.event(map[string]any{"kind": "账号编队角色资料已同步", "server": client.characters.ChannelContext[0], "characters": count, "plain_bytes": len(payload), "attempt": "1/3，CMD1462实机请求及原生读取链已核对"})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.worldState != nil && requestData.frame.ID == 2316 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		packets, err := client.worldState.createBleedingMine(ctx, requestData.plaintext)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "赤红铁矿创建失败", "id": requestData.frame.ID, "error": err.Error()})
			return dispatchHandled
		}
		if client.sendPlan(packets, client.logWorldResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.worldState != nil && requestData.frame.ID == 2317 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		packets, err := client.worldState.saveBleedingMineTeam(ctx, requestData.plaintext)
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "赤红铁矿编队保存失败", "id": requestData.frame.ID, "error": err.Error()})
			// 14073D1E0将错误3映射到通用保存失败提示，不读额外正文。
			if err := client.output.send(1, 2317, []byte{0, 3, 0}); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(packets, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": client.worldState.role.ID, "plain_hex": hex.EncodeToString(packet.Payload), "attempt": "1/3，保存请求与原生通知读取链已核对"})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.worldState != nil && requestData.frame.ID == 2318 {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		start, plan, err := client.worldState.prepareBleedingMineStart(ctx, requestData.plaintext)
		cancel()
		if err != nil {
			// 14073D500失败分支解除确认框设置的输入锁。
			client.event(map[string]any{"kind": "赤红铁矿开战拒绝", "id": requestData.frame.ID, "error": err.Error()})
			if client.output.send(1, 2318, []byte{0, 3, 0}) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, err := preparePackets(client.keys, plan)
		if err != nil {
			client.event(map[string]any{"kind": "赤红铁矿开战编码失败", "error": err.Error()})
			if client.output.send(1, 2318, []byte{0, 3, 0}) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if err := client.output.writePrepared(prepared, func(packet preparedPacket) {
			client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": client.worldState.role.ID,
				"plain_hex": hex.EncodeToString(packet.Payload)})
		}); err != nil {
			return dispatchClose
		}
		client.worldState.bleedingMineStart = start
		client.event(map[string]any{"kind": "赤红铁矿等待原生选图", "group": start.Group,
			"dungeon": start.Dungeon, "members": start.Members, "attempt": "1/3，原生开战状态与C15发送链已核对"})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && client.bootstrapped && requestData.verified && client.worldState != nil && client.selectedCharacterID != 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		handled, packets, e := client.worldState.blackPurgatoryHandle(ctx, requestData.frame.ID, requestData.plaintext)
		cancel()
		if handled {
			if e != nil {
				client.event(map[string]any{"kind": "黑鸦请求被拒绝", "id": requestData.frame.ID, "error": e.Error()})
				packets = []outboundPacket{{"黑鸦请求拒绝应答", 1, requestData.frame.ID, protocol.Refusal(8)}}
			}
			prepared, err := preparePackets(client.keys, packets)
			if err != nil {
				client.event(map[string]any{"kind": "黑鸦响应编码失败", "error": err.Error()})
				return dispatchClose
			}
			if err := client.output.writePrepared(prepared, func(packet preparedPacket) {
				client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": client.selectedCharacterID,
					"plain_hex": hex.EncodeToString(packet.Payload)})
			}); err != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		// 蔚蓝号（Azure Main, channelType 102）：与月湖同层挂载，但它自己按频道门控，
		// 对其它频道一律 return false（不影响普通频道与月湖）。
		handled, packets, e = client.worldState.azureHandle(requestData.frame.ID, requestData.plaintext, time.Now(), client.event)
		if handled {
			if e != nil {
				client.event(map[string]any{"kind": "azure_main_request_rejected", "id": requestData.frame.ID, "error": e.Error()})
				packets = nil
			}
			if client.sendPlan(packets, func(packet outboundPacket) {
				client.event(map[string]any{"kind": packet.Name, "id": packet.ID})
			}) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		handled, packets, e = client.worldState.moonHandle(requestData.frame.ID, requestData.plaintext, time.Now(), client.event)
		if handled {
			if e != nil {
				client.event(map[string]any{"kind": "moon_request_rejected", "id": requestData.frame.ID, "error": e.Error()})
				packets = moonRefusal(requestData.frame.ID, requestData.plaintext)
			}
			if client.sendPlan(packets, func(packet outboundPacket) {
				client.event(map[string]any{"kind": packet.Name, "id": packet.ID})
			}) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
	}

	return dispatchNext
}

func (client *gameConnection) dispatchLegion(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && legion.Requests(requestData.frame.ID) && client.bootstrapped && requestData.verified && client.worldState != nil {
		// Legion / apocalypse family. CMD2043/2354/2045 are handled in
		// town and CMD2355 inside the dungeon; the rest of the family is
		// routed here so an unimplemented packet is logged as an
		// explicit refusal instead of vanishing.
		legionPlan, legionErr := client.legionState.handle(client.worldState, requestData.plaintext, requestData.frame.ID)
		// The request body is logged whether or not the opcode is
		// answered. Settling X1 (next64 §6.2) — whether the caller's
		// appended length already contains the 13-byte envelope — is
		// the point of P1's observability, so both the byte count and
		// the raw bytes are kept.
		legionBytes, legionHex := legionRequestBody(requestData.plaintext)
		if legionErr != nil {
			client.event(map[string]any{"kind": "legion_refused", "id": requestData.frame.ID, "reason": legionErr.Error(), "request_bytes": legionBytes, "request_hex": legionHex})
			return dispatchHandled
		}
		for _, note := range legionPlan.Events {
			note["id"] = requestData.frame.ID
			note["request_bytes"] = legionBytes
			note["request_hex"] = legionHex
			client.event(note)
		}
		if client.sendPlan(legionPlan.Packets, func(packet outboundPacket) {
			client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "character_id": client.worldState.role.ID, "request_bytes": legionBytes, "request_hex": legionHex, "plain_hex": hex.EncodeToString(packet.Payload)})
		}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchDungeon(requestData *clientRequest) dispatchAction {
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 15 && client.worldState.channelGuideDungeon != 0 {
		// SemiRaid 频道红门（Azure Main 100004131 等）：客户端在 SemiRaid 门口
		// 等的是直接进本，不是「选图 → C16」流程 —— 走旧路径回 N27 选图状态会让
		// 客户端黑屏等选图数据（2026-10-03 15:17 实测）。照月湖红门模式：
		// ACK15 + 进图帧序列（N2,N2,N14,N3,N27,N28,N29 —— 与军团进图同款，
		// N3/N27 是服务端驱动进场的必备帧），dungeon 会话照 CMD16 落地。
		if !requestData.verified {
			client.event(map[string]any{"kind": "semiraid_gate_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		sel := protocol.DungeonSelection{ID: client.worldState.channelGuideDungeon, Difficulty: 0, Party: 65535}
		entryCtx, entryCancel := context.WithTimeout(context.Background(), 5*time.Second)
		sess, _, e := client.worldState.prepareDungeonEntry(sel)
		if e != nil {
			entryCancel()
			client.event(map[string]any{"kind": "semiraid_gate_refused", "character_id": client.selectedCharacterID, "dungeon": sel.ID, "reason": e.Error()})
			if client.output.send(1, 15, protocol.Refusal(4)) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		frames, fe := client.worldState.dungeonEntryPlanImpl(entryCtx, "semiraid_gate_ack", 15, sel, sess, &client.worldState.characters.ChannelContext)
		entryCancel()
		if fe != nil {
			client.event(map[string]any{"kind": "semiraid_gate_plan_error", "character_id": client.selectedCharacterID, "dungeon": sel.ID, "reason": fe.Error()})
			if client.output.send(1, 15, protocol.Refusal(4)) != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		// NOTI28 之前补 N3（actor 状态 → 副本态）与 NOTI27（选图上下文），
		// 顺序对齐沉月湖成功序列（…N14, N3, N9, N27, N28…）。
		actorState, se := protocol.UserState(client.worldState.role.WireID, protocol.UserStateDungeon)
		if se != nil {
			client.event(map[string]any{"kind": "semiraid_actor_state_error", "character_id": client.selectedCharacterID, "reason": se.Error()})
		}
		azureRun := client.worldState.channelType == azureMainChannelType
		infoInserted := false
		if azureRun {
			// N2621 的 1 小时倒计时以本趟开始为基准。
			client.worldState.azure.runStarted = time.Now()
			client.worldState.azure.infoRoom = 0
			client.worldState.azure.cleared = nil
			client.worldState.azure.kraken = 0
			client.worldState.azure.revivesLeft = azureMainReviveLimit
		}
		inserted := false
		plan := make([]outboundPacket, 0, len(frames)+4)
		for _, pkt := range frames {
			if pkt.ID == 28 && !inserted {
				if actorState != nil {
					plan = append(plan, outboundPacket{"semiraid_actor_state_dungeon", 0, 3, actorState})
				}
				plan = append(plan, outboundPacket{"semiraid_dungeon_selection", 0, 27, protocol.EnterDungeonSelection()})
				// [AZURE-20261004] C15（红门）路径的客户端在收到 NOTI27 后停在「选图」状态：
				// 紧接着补一个 select_ack(16) 才会离开该状态、重新接受普通房间门。
				// 少了它，进图后整场都不发 CMD45 MOVE_MAP（与 2062 直达那次实测同症状，
				// 见 docs/protocol/next49-odyssey-direct-move.md 的「闭环」解法）。
				plan = append(plan, outboundPacket{"semiraid_dungeon_select_ack", 1, 16, []byte{1}})
				inserted = true
			}
			if azureRun && pkt.ID == 29 && !infoInserted {
				// 官服帧序 N28(#414) → N2621(#416) → N29(#417)：副本信息之后、
				// 首张 START_MAP 之前。缺了这一帧，客户端清场后不走门（实机 2026-10-04）。
				plan = append(plan, client.worldState.azureMainInfo(time.Now()))
				infoInserted = true
			}
			plan = append(plan, pkt)
		}
		prepared, pe := preparePackets(client.keys, plan)
		if pe != nil {
			client.event(map[string]any{"kind": "semiraid_encode_error", "error": pe.Error()})
			return dispatchHandled
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); e != nil {
			return dispatchClose
		}
		// dungeon 会话落地（照 CMD16 pending 应用 + 军团落地段）：
		w := client.worldState
		w.deathSent = map[uint16]bool{}
		w.drops = nil
		w.resetCards()
		w.completionSent = false
		w.completionErr = nil
		w.resultSent = false
		w.selectingDungeon = false
		w.approvedDungeonGate = 0
		w.pendingTownArrival = nil
		loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
		loyaltyPackets, loyaltyErr := w.refreshCreatureLoyalty(loyaltyCtx, time.Now(), true)
		loyaltyCancel()
		if loyaltyErr != nil {
			client.event(map[string]any{"kind": "creature_loyalty_error", "character_id": client.selectedCharacterID, "error": loyaltyErr.Error()})
		} else if client.sendPlan(loyaltyPackets, nil) != nil {
			return dispatchClose
		}
		w.leaveScene()
		w.activeDungeon = sess
		client.event(map[string]any{"kind": "dungeon_session_started", "dungeon": sess.Definition.ID, "maze": sess.Maze.Index, "map": sess.Room.Map, "monsters": len(sess.Monsters), "source": "semiraid_gate"})
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 15 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "dungeon_gate_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		plan, e := client.worldState.dungeonGate(requestData.plaintext)
		if e != nil {
			client.worldState.approvedDungeonGate = 0
			client.event(map[string]any{"kind": "dungeon_gate_rejected", "reason": e.Error()})
			if e = client.output.send(1, 15, protocol.Refusal(4)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, e := preparePackets(client.keys, plan)
		if e != nil {
			client.event(map[string]any{"kind": "dungeon_encode_error", "error": e.Error()})
			return dispatchHandled
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); e != nil {
			client.event(map[string]any{"kind": "dungeon_write_error", "error": e.Error()})
			return dispatchClose
		}
		client.worldState.selectingDungeon = true
		client.worldState.approvedDungeonGate, _ = protocol.DecodeDungeonGate(requestData.plaintext)
		client.worldState.pendingTownArrival = nil
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && dungeonRequest(requestData.frame.ID) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "dungeon_request_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		var plan []outboundPacket
		var pending *dungeon.Session
		var townArrivalLoading bool
		var e error
		switch requestData.frame.ID {
		case 16:
			pending, plan, e = client.worldState.selectDungeon(requestData.plaintext)
		case 1852:
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			pending, plan, e = client.worldState.startBlackPurgatory(ctx, requestData.plaintext)
			cancel()
		case 37:
			if client.worldState.activeDungeon == nil && client.worldState.pendingTownArrival != nil {
				scene := client.worldState.townArrivalScenes[client.worldState.pendingTownArrival.Definition.ID]
				if client.worldState.state.Position.Town != scene.Town || client.worldState.state.Position.Area != scene.Area {
					client.worldState.pendingTownArrival = nil
					e = fmt.Errorf("town arrival scene origin changed before loading")
					break
				}
				client.worldState.activeDungeon = client.worldState.pendingTownArrival
				townArrivalLoading = true
			}
			plan, e = client.worldState.finishDungeonLoading(requestData.plaintext)
			if e != nil && townArrivalLoading {
				client.worldState.activeDungeon = nil
				townArrivalLoading = false
			}
		case 38:
			pending, plan, e = client.worldState.interactDoor(requestData.plaintext)
		case 39:
			client.worldState.completionErr = nil
			plan, e = client.worldState.monsterDeath(requestData.plaintext, client.event)
			// 蔚蓝号：本房间打空时补一帧 N2621（官服 #488 的位置 —— 最后一怪确认
			// 死亡之后、下一张 N29 之前）。这是蔚蓝号唯一「进图后」的进度帧；缺了它
			// 客户端清场后不发 CMD45（实机 2026-10-04 10:25）。
			if e == nil {
				if info, ok := client.worldState.azureRoomClearedInfo(time.Now()); ok {
					plan = append(plan, info)
				}
			}
		case 2329:
			plan, e = client.worldState.scaleStatus(requestData.plaintext, client.event)
		case 40:
			plan, e = client.worldState.playerDeath(requestData.plaintext, requestData.frame.Raw)
			if e == nil && client.worldState.bleedingMineStart == nil {
				// [MERGE-20260928-DEATH-FAIL-TIMEOUT] 原生「倒计时结束 → 挑战失败」
				// 由服务端推进：客户端进复活 UI 后只会等，不会发请求。死亡后等待
				// deathFailTimeout，期间没复活就下发 NOTI33 (FAIL_CLEAR_DUNGEON)，
				// 驱动失败结算与回城。此前只有 Elvenmere(100003126) 会发，其它副本
				// 死亡后永远停在 Dead 界面（实机 2026-09-28：倒计时结束不回城，
				// 剧情叠在死亡界面上卡死）。
				w := client.worldState
				select {
				case <-client.connection.done:
				default:
					time.AfterFunc(deathFailTimeout, func() {
						d := w.pilotDeath
						if d == nil || !d.Dead || w.activeDungeon == nil {
							return
						}
						// [AZURE-DEATH-AFTER-CLEAR] 结算已经走完的**只回城、不补 FAIL_CLEAR**：
						// 补了会把一场已经通关并发了奖的挑战标成失败。没结算的还是照旧走失败链。
						skipFailClear := w.resultSent
						// reason 100 = timeout（0 是「默认死亡」）。
						if !skipFailClear {
							if err := client.output.send(0, 33, protocol.DungeonFailClear(100)); err != nil {
								return
							}
						}
						// 只发 FAIL_CLEAR 不够：客户端收到后只播死亡镜头，不会自己
						// 离开副本 —— 实机 2026-09-28 客户端 trace 里
						// `RECV ENUM_NOTIPACKET_FAIL_CLEAR_DUNGEON` 之后 25 秒毫无
						// 动作，直到玩家手动发 GIVEUP_GAME(42) 才回城。
						// 这里照「放弃」那条路径把玩家送回城。
						leave, e := w.leaveDungeon()
						if e != nil {
							client.event(map[string]any{"kind": "death_fail_leave_error", "error": e.Error()})
							return
						}
						if client.sendPlan(leave, func(p outboundPacket) {
							if p.ID == 1361 {
								client.event(map[string]any{"kind": p.Name, "character_id": w.role.ID, "id": p.ID, "type": p.Kind, "plain_hex": hex.EncodeToString(p.Payload), "path": "death_timeout"})
							}
						}) != nil {
							return
						}
						// 主循环在发出 dungeon_leave_ack 时会清掉副本会话
						// （main.go 的 `p.Name == "dungeon_leave_ack"` 分支），
						// 这里绕过了那段，必须自己清 —— 否则客户端回城后发来的
						// 门请求仍会命中一个已离开的会话。
						w.activeDungeon = nil
						w.bleedingMineStart = nil
						client.event(map[string]any{"kind": "death_fail_timeout", "run": d.Run, "steps": len(leave) + 1})
					})
				}
			}
		case 43:
			plan, e = client.worldState.pickup(requestData.plaintext)
		case 117:
			plan, e = client.worldState.bossCheck(requestData.plaintext)
		case 45:
			if client.worldState.pilotDeath != nil && client.worldState.activeDungeon != nil && client.worldState.pilotDeath.Run == client.worldState.activeDungeon.RunID && client.worldState.pilotDeath.Dead {
				e = fmt.Errorf("room movement requires living player")
			} else {
				pending, plan, e = client.worldState.moveDungeonRoom(requestData.plaintext)
			}
		case 1654:
			// 蔚蓝号的「清关信息应答」（配 s2c 1658）。官服尾段：N1658(空) → 客户端
			// CMD1654 → N2621 阶段 4/5。
			//
			// ⚠️ 非蔚蓝号会话**必须保持原来的"什么都不做"** —— 伊斯大陆也发 CMD1654
			// （见 internal/game/protocol/ispins_settlement.go），它此前落进采样分支照样能用；
			// 这里若回 Refusal 会把伊斯大陆的结算打坏。所以 else 分支只留注释、不发包。
			if client.worldState.channelType == azureMainChannelType {
				plan = client.worldState.azureClearInfo()
			}
		case 46:
			plan, e = client.worldState.dungeonResult(requestData.plaintext)
		case 69, 70:
			plan, e = client.worldState.cardStage(requestData.frame.ID, requestData.plaintext)
		case 71:
			plan, e = client.worldState.cardPick(requestData.plaintext)
		case 72:
			pending, plan, e = client.worldState.settlementExit(requestData.plaintext)
		case 449:
			plan, e = client.worldState.tournamentSelectState(requestData.plaintext)
		case 450:
			plan, e = client.worldState.tournamentSelect(requestData.plaintext)
		case 132:
			plan, e = client.worldState.returnFromDungeonSelection(requestData.plaintext)
		case 2319:
			plan, e = client.worldState.giveUpBleedingMine(requestData.plaintext)
		case 1461:
			plan, e = client.worldState.bleedingMineDeath(requestData.plaintext)
		case 2320:
			plan, e = client.worldState.settleBleedingMineStage(requestData.plaintext)
		case 2321:
			plan, e = client.worldState.finishBleedingMine(requestData.plaintext)
		case 2325:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			plan, e = client.worldState.composeBleedingMineRewards(ctx, requestData.plaintext, requestData.frame.Raw)
			cancel()
		case 2322, 2323:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if requestData.frame.ID == 2322 {
				plan, e = client.worldState.claimBleedingMineRewards(ctx, requestData.plaintext)
			} else {
				plan, e = client.worldState.openBleedingMineRewards(ctx, requestData.plaintext)
			}
			cancel()
		case 2327:
			plan, e = client.worldState.reviveBleedingMine(requestData.plaintext, requestData.frame.Raw, time.Now())
		case 42:
			if len(requestData.plaintext) != 0 {
				e = fmt.Errorf("unexpected give-up body")
			} else if client.worldState.bleedingMineStart != nil {
				plan, e = client.worldState.endBleedingMine()
				if e == nil {
					plan = append([]outboundPacket{{"dungeon_leave_ack", 1, 42, []byte{1}}}, plan...)
				}
			} else {
				plan, e = client.worldState.leaveDungeon()
			}
		case 2015:
			plan, e = client.worldState.elvenmereTeleport(requestData.plaintext)
		case 2062:
			pending, plan, e = client.worldState.directMoveDungeon(requestData.plaintext)
		}
		if e != nil {
			client.event(map[string]any{"kind": "dungeon_request_refused", "id": requestData.frame.ID, "reason": e.Error()})
			if requestData.frame.ID == 69 || requestData.frame.ID == 70 {
				return dispatchHandled
			}
			if requestData.frame.ID == 71 {
				if client.output.send(1, 71, client.worldState.cardSnapshot()) != nil {
					return dispatchClose
				}
				return dispatchHandled
			}
			if requestData.frame.ID == 72 {
				if request, err := protocol.DecodeSettlementExit(requestData.plaintext); err == nil {
					if client.output.send(1, 72, protocol.SettlementExitRefused(request.Option)) != nil {
						return dispatchClose
					}
				}
				return dispatchHandled
			}
			if requestData.frame.ID == 43 {
				if request, decodeErr := protocol.DecodePickup(requestData.plaintext); decodeErr == nil {
					body, encodeErr := protocol.PickupRefused(request.Object)
					if encodeErr != nil || client.output.send(1, 43, body) != nil {
						return dispatchClose
					}
				}
				return dispatchHandled
			}
			// CMD39 failure reads a monster u16; NOTI132 has no generic
			// command refusal. Never send the generic error shape there.
			if requestData.frame.ID == 39 || requestData.frame.ID == 46 || requestData.frame.ID == 117 || requestData.frame.ID == 132 || requestData.frame.ID == 2015 || requestData.frame.ID == 2062 || requestData.frame.ID == 2319 {
				return dispatchHandled
			}
			refusalCode := uint16(4)
			if requestData.frame.ID == 1852 {
				// 黑鸦原生应答14525C4C0使用错误8表示无法开始，不套用普通选图错误4。
				refusalCode = 8
			}
			if requestData.frame.ID >= 2320 && requestData.frame.ID <= 2325 {
				refusalCode = 3
				// 14073D080的错误8专指未参与探索，不能拿来表示邮箱已满。
				if requestData.frame.ID == 2322 && e == errBleedingMineClaimActor {
					refusalCode = 8
				}
			}
			if e = client.output.send(1, requestData.frame.ID, protocol.Refusal(refusalCode)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, e := preparePackets(client.keys, plan)
		if e != nil {
			client.event(map[string]any{"kind": "dungeon_encode_error", "error": e.Error()})
			// Domain state may already be committed. Close this failed
			// transport and restore from storage on reconnect.
			return dispatchClose
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); e != nil {
			return dispatchClose
		}
		if requestData.frame.ID == 39 && client.worldState.completionErr != nil {
			client.event(map[string]any{"kind": "dungeon_completion_error", "map": client.worldState.activeDungeon.Room.Map, "error": client.worldState.completionErr.Error()})
			client.worldState.completionErr = nil
		}
		if pending != nil && requestData.frame.ID == 16 {
			if scene, ok := client.worldState.townArrivalScenes[pending.Definition.ID]; ok {
				client.worldState.pendingTownArrival = pending
				client.worldState.selectingDungeon = false
				client.worldState.approvedDungeonGate = 0
				client.event(map[string]any{"kind": "town_arrival_scene_waiting_for_load", "quest": scene.QuestID, "town": scene.Town, "area": scene.Area, "dungeon": scene.DungeonID})
				pending = nil
			}
		}
		if townArrivalLoading {
			client.worldState.deathSent = map[uint16]bool{}
			client.worldState.drops = nil
			client.worldState.resetCards()
			client.worldState.completionSent = false
			client.worldState.completionErr = nil
			client.worldState.resultSent = false
			client.worldState.pendingTownArrival = nil
			client.worldState.leaveScene()
			client.worldState.selectingDungeon = false
			client.worldState.approvedDungeonGate = 0
			client.event(map[string]any{"kind": "dungeon_session_started", "dungeon": client.worldState.activeDungeon.Definition.ID, "maze": client.worldState.activeDungeon.Maze.Index, "map": client.worldState.activeDungeon.Room.Map, "monsters": len(client.worldState.activeDungeon.Monsters), "quests_changed": false, "town_arrival": true})
		}
		if pending != nil {
			if requestData.frame.ID == 16 || requestData.frame.ID == 72 || requestData.frame.ID == 1852 || requestData.frame.ID == 2062 {
				client.worldState.deathSent = map[uint16]bool{}
				client.worldState.drops = nil
				client.worldState.resetCards()
			}
			client.worldState.activeDungeon = pending
			loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
			loyaltyPackets, loyaltyErr := client.worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), true)
			loyaltyCancel()
			if loyaltyErr != nil {
				client.event(map[string]any{"kind": "creature_loyalty_error", "character_id": client.selectedCharacterID, "error": loyaltyErr.Error()})
			} else {
				if client.sendPlan(loyaltyPackets, nil) != nil {
					return dispatchClose
				}
			}
			// A dungeon is a private instance: this actor leaves the shared town.
			client.worldState.leaveScene()
			client.worldState.completionSent = false
			client.worldState.completionErr = nil
			client.worldState.resultSent = false
			client.worldState.selectingDungeon = false
			client.worldState.approvedDungeonGate = 0
			client.event(map[string]any{"kind": "dungeon_session_started", "dungeon": pending.Definition.ID, "maze": pending.Maze.Index, "map": pending.Room.Map, "monsters": len(pending.Monsters), "quests_changed": false})
		}
		if requestData.frame.ID == 37 && client.worldState.activeDungeon != nil {
			client.worldState.activeDungeon.Loaded = true
			client.worldState.activeDungeon.TryComplete()
			if completed, err := client.worldState.completeDungeon(); err != nil {
				client.event(map[string]any{"kind": "dungeon_completion_error", "map": client.worldState.activeDungeon.Room.Map, "error": err.Error()})
			} else if len(completed) > 0 {
				if client.sendPlan(completed, func(packet outboundPacket) {
					client.event(map[string]any{"kind": packet.Name, "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload), "character_id": client.selectedCharacterID})
					if packet.Name == "dungeon_clear_enabled" || packet.Name == "赤红铁矿领主通关确认" {
						client.worldState.completionSent = true
					}
				}) != nil {
					return dispatchClose
				}
			}
		}
		for _, p := range plan {
			if p.Name == "solo_party_initialized" {
				client.worldState.soloPartyReady = true
			}
			if p.Name == "card_scroll_ack" {
				client.worldState.cardScrolled = true
			}
			if p.Name == "card_layout_ack" {
				// 布局发出后给玩家 3 秒选牌；到点还没选就替他选第一张（见
				// autoPickSettlementCard）。**不再只给黑鸦开** —— 见该函数注释。
				if !client.worldState.cardLayoutSent && client.worldState.cardReceipt == nil && client.worldState.activeDungeon != nil {
					client.worldState.cardAutoPickAt = time.Now().Add(3 * time.Second)
				}
				client.worldState.cardLayoutSent = true
			}
			if p.Name == "dungeon_return_users" {
				// Back in town: exchange actor info with everyone standing there.
				if e = client.worldState.announceSelf(client.event); e != nil {
					client.event(map[string]any{"kind": "area_presence_error", "error": e.Error()})
				}
			}
			mineEnded := p.Name == "赤红铁矿开战会话结束" && client.worldState.bleedingMineStart != nil
			if (p.Name == "settlement_exit_ack" || p.Name == "dungeon_leave_ack" || mineEnded) && pending == nil {
				client.worldState.bleedingMineStart = nil
				client.worldState.activeDungeon = nil
				loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				loyaltyPackets, loyaltyErr := client.worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), false)
				loyaltyCancel()
				if loyaltyErr != nil {
					client.event(map[string]any{"kind": "creature_loyalty_error", "character_id": client.selectedCharacterID, "error": loyaltyErr.Error()})
				} else {
					if client.sendPlan(loyaltyPackets, nil) != nil {
						return dispatchClose
					}
				}
				client.worldState.drops = nil
				client.worldState.deathSent = nil
				client.worldState.completionSent = false
				client.worldState.resultSent = false
				client.worldState.resetCards()
				if p.Name == "settlement_exit_ack" {
					// selectingDungeon was already set by settlementExit from
					// the decoded request, independently of the ACK envelope.
					client.event(map[string]any{"kind": "settlement_exit_flag", "character_id": client.worldState.role.ID, "selecting_dungeon": client.worldState.selectingDungeon, "payload_len": len(p.Payload)})
				} else {
					client.worldState.selectingDungeon = false
					client.worldState.approvedDungeonGate = 0
				}
				// A legion run lives inside a dungeon, so leaving it ends
				// the run. The client normally says so itself; this is
				// the backstop for a player who just walks out (P6).
				if note, closed := client.legionState.abandonOnLeave(p.Name, client.worldState.role.ID); closed {
					note["id"] = requestData.frame.ID
					client.event(note)
				}
			}
			if p.Name == "monster_death_confirmed" {
				if client.worldState.deathSent == nil {
					client.worldState.deathSent = map[uint16]bool{}
				}
				// Same failure mode as the acknowledgement index above: the
				// read used to be bare indexing on a payload whose width is
				// only guaranteed elsewhere.
				entity, ok := monsterDeathEntity(p.Payload)
				if !ok {
					client.event(map[string]any{"kind": "monster_death_ack_short_payload", "character_id": client.worldState.role.ID, "length": len(p.Payload)})
				} else {
					client.worldState.deathSent[entity] = true
					if client.worldState.drops != nil && len(client.worldState.drops.Skipped[entity]) > 0 {
						client.event(map[string]any{"kind": "drop_rules_pending", "entity": entity, "rules": client.worldState.drops.Skipped[entity]})
					}
				}
			}
			if p.Name == "dungeon_clear_enabled" || p.Name == "赤红铁矿领主通关确认" {
				client.worldState.completionSent = true
			}
			if p.Name == "dungeon_clear_reward" {
				client.worldState.resultSent = true
			}
			// [MERGE-20260928-DIAG] 把场景换图的决策路径落进 events，便于实机取证。
			if p.Name == "dungeon_next_map_sent" && client.worldState.sceneDiag != "" {
				client.event(map[string]any{
					"kind": "scene_transition_diag", "character_id": client.worldState.role.ID,
					"from_map": client.worldState.sceneDiagFrom, "to_map": client.worldState.sceneDiagTo,
					"detail": client.worldState.sceneDiag,
				})
				client.worldState.sceneDiag = ""
			}
		}
		if requestData.frame.ID == 42 {
			client.worldState.activeDungeon = nil
			loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
			loyaltyPackets, loyaltyErr := client.worldState.refreshCreatureLoyalty(loyaltyCtx, time.Now(), false)
			loyaltyCancel()
			if loyaltyErr != nil {
				client.event(map[string]any{"kind": "creature_loyalty_error", "character_id": client.selectedCharacterID, "error": loyaltyErr.Error()})
			} else {
				if client.sendPlan(loyaltyPackets, nil) != nil {
					return dispatchClose
				}
			}
			if note, closed := client.legionState.abandonOnLeave("CMD42 dungeon leave", client.worldState.role.ID); closed {
				note["id"] = requestData.frame.ID
				client.event(note)
			}
		}
		if requestData.frame.ID == 42 || requestData.frame.ID == 132 {
			client.worldState.selectingDungeon = false
			client.worldState.approvedDungeonGate = 0
		}
		if returnedToTown(plan) {
			refresh, err := client.worldState.graduateOdysseyAtTown()
			if err != nil {
				client.event(map[string]any{"kind": "odyssey_graduation_error", "character_id": client.selectedCharacterID, "reason": err.Error()})
				return dispatchClose
			}
			if client.sendPlan(refresh, client.logCharacterResponse) != nil {
				return dispatchClose
			}
			if len(refresh) > 0 {
				if err = client.worldState.announceSelf(client.event); err != nil {
					return dispatchClose
				}
			}
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchWorldAndQuests(requestData *clientRequest) dispatchAction {
	if client.worldState != nil && client.bootstrapped && requestData.frame.ID == 191 && requestData.verified {
		r, e := protocol.DecodeStoryPause(requestData.plaintext)
		if e != nil || client.worldState.activeDungeon == nil || client.worldState.role.ID == 0 {
			client.event(map[string]any{"kind": "story_pause_refused", "reason": "invalid state or absent owned dungeon"})
			return dispatchHandled
		}
		p, e := protocol.StoryPauseNotice(client.worldState.role.WireID, r)
		if e != nil {
			return dispatchHandled
		}
		if e = client.output.send(0, 170, p); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "story_pause_restored", "character_id": client.worldState.role.ID, "state": r.State, "story_kind": r.Kind})
		return dispatchHandled
	}
	if client.worldState != nil && client.bootstrapped && (requestData.frame.ID == 35 || requestData.frame.ID == 36 || requestData.frame.ID == 1418) {
		if client.worldState.activeDungeon != nil {
			client.event(map[string]any{"kind": "dungeon_world_request_pending", "id": requestData.frame.ID, "origin_preserved": true})
			return dispatchHandled
		}
		if !requestData.verified {
			client.event(map[string]any{"kind": "world_rejected", "id": requestData.frame.ID, "error": "checksum rejected"})
			return dispatchHandled
		}
		if client.worldState.pendingTownArrival != nil {
			if client.worldState.isTownArrivalOriginSync(requestData.frame.ID, requestData.plaintext) {
				client.event(map[string]any{"kind": "town_arrival_scene_origin_sync", "dungeon": client.worldState.pendingTownArrival.Definition.ID, "id": requestData.frame.ID})
			} else {
				client.event(map[string]any{"kind": "town_arrival_scene_remains_town", "dungeon": client.worldState.pendingTownArrival.Definition.ID, "id": requestData.frame.ID})
				client.worldState.pendingTownArrival = nil
			}
		}
		if e := client.worldState.handle(requestData.frame.ID, requestData.plaintext, client.output.send, client.event); e != nil {
			client.event(map[string]any{"kind": "world_error", "id": requestData.frame.ID, "error": e.Error()})
		}
		return dispatchHandled
	}
	if client.questService != nil && client.bootstrapped && client.worldState != nil && (requestData.frame.ID == 1422 || requestData.frame.ID == 2278) {
		if !requestData.verified || client.worldState.role.ID == 0 || client.worldState.activeDungeon != nil {
			client.event(map[string]any{"kind": "act_quest_clear_refused", "id": requestData.frame.ID, "reason": "invalid session or checksum"})
			return dispatchHandled
		}
		if requestData.frame.ID == 1422 {
			if e := protocol.DecodeClearQuestTicket(requestData.plaintext); e != nil {
				client.event(map[string]any{"kind": "act_quest_clear_refused", "id": requestData.frame.ID, "reason": e.Error()})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			count, e := (&workflow.QuestService{Store: client.gameStore, Quest: client.questService}).ClearActQuests(ctx, client.worldState.role)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "act_quest_clear_refused", "id": requestData.frame.ID, "reason": e.Error()})
				return dispatchHandled
			}
			if e = client.output.send(1, 1422, protocol.ClearQuestTicketAccepted()); e != nil {
				return dispatchClose
			}
			client.event(map[string]any{"kind": "act_quests_cleared", "character_id": client.worldState.role.ID, "count": count})
		} else {
			// The native CMD1422 success branch immediately requests CMD2278.
			// Its sender writes no body; publish the quest snapshots here.
			if len(requestData.plaintext) != 0 {
				client.event(map[string]any{"kind": "act_quest_refresh_refused", "reason": "unexpected request body"})
				return dispatchHandled
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			plan, e := client.worldState.actQuestRefresh(ctx)
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "act_quest_refresh_refused", "reason": e.Error()})
				return dispatchHandled
			}
			if client.sendPlan(plan, client.logWorldAction) != nil {
				return dispatchClose
			}
		}
		return dispatchHandled
	}
	if client.questService != nil && client.bootstrapped && requestData.frame.Type == 1 && requestData.frame.ID == 467 {
		if !requestData.verified || client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID || len(requestData.plaintext) != 0 {
			client.event(map[string]any{"kind": "image_communication_rejected", "reason": "invalid request or session"})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		qid, npc, lookupErr := client.questService.ImageCommunicationTarget(ctx, client.worldState.role)
		cancel()
		if lookupErr != nil {
			client.event(map[string]any{"kind": "image_communication_rejected", "reason": lookupErr.Error(), "character_id": client.selectedCharacterID})
			return dispatchHandled
		}
		ack := protocol.ImageCommunicationAck(npc)
		if e := client.output.send(1, 467, ack); e != nil {
			return dispatchClose
		}
		client.worldState.communicationQuest = qid
		client.worldState.communicationNPC = npc
		client.worldState.communicationTown = client.worldState.state.Position.Town
		client.worldState.communicationArea = client.worldState.state.Position.Area
		client.worldState.communicationUntil = time.Now().Add(20 * time.Second) // PVF [summon time] = 20000 ms
		client.event(map[string]any{"kind": "image_communication_ack", "character_id": client.selectedCharacterID, "quest": qid, "npc": npc, "attempt": "2/3", "plain_hex": hex.EncodeToString(ack)})
		return dispatchHandled
	}
	if client.questService != nil && client.bootstrapped && requestData.frame.ID == 33 && client.worldState != nil && requestData.verified {
		plan, e := client.worldState.questInteraction(requestData.plaintext)
		if diagnostic := client.worldState.npcPresenceShadow(requestData.plaintext); diagnostic != nil {
			if e != nil {
				diagnostic["legacy_outcome"] = "refused"
			} else if len(plan) == 0 {
				diagnostic["legacy_outcome"] = "no_progress"
			} else {
				diagnostic["legacy_outcome"] = "handled"
			}
			client.event(diagnostic)
		}
		if e != nil {
			client.event(map[string]any{"kind": "quest_interaction_refused", "reason": e.Error()})
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logWorldAction) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.questService != nil && client.bootstrapped && requestData.frame.ID == 34 {
		if !requestData.verified || client.worldState.role.ID == 0 {
			client.event(map[string]any{"kind": "quest_rejected", "error": "invalid checksum or no character"})
			return dispatchHandled
		}
		r, e := protocol.DecodeQuestSubmit(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "quest_rejected", "error": e.Error()})
			return dispatchHandled
		}
		plan, e := client.worldState.finishQuest(r)
		if e != nil {
			client.event(map[string]any{"kind": "quest_submit_refused", "character_id": client.worldState.role.ID, "quest": r.ID, "reason": e.Error()})
			if e = client.output.send(1, 34, protocol.QuestSubmitRefused()); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		prepared, e := preparePackets(client.keys, plan)
		if e != nil {
			client.event(map[string]any{"kind": "quest_submit_encode_error", "error": e.Error()})
			return dispatchClose
		}
		if e = client.output.writePrepared(prepared, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "character_id": client.worldState.role.ID, "quest": r.ID})
		}); e != nil {
			client.event(map[string]any{"kind": "quest_submit_write_error", "quest": r.ID, "error": e.Error()})
			return dispatchClose
		}
		if client.worldState.answeredQuests == nil {
			client.worldState.answeredQuests = map[uint16]bool{}
		}
		client.worldState.answeredQuests[r.ID] = true
		return dispatchHandled
	}
	if client.questService != nil && client.bootstrapped && (requestData.frame.ID == 31 || requestData.frame.ID == 32) {
		if !requestData.verified || client.worldState.role.ID == 0 {
			client.event(map[string]any{"kind": "quest_rejected", "error": "invalid checksum or no selected character"})
			return dispatchHandled
		}
		qid, e := protocol.DecodeQuestRequest(requestData.frame.ID, requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "quest_rejected", "error": e.Error()})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var response []byte
		if requestData.frame.ID == 31 {
			var state storage.QuestState
			state, e = client.questService.Accept(ctx, client.worldState.role, qid)
			response = protocol.QuestAccepted(qid, state.Progress)
		} else {
			e = client.gameStore.AbandonQuest(ctx, client.developmentAccount, client.worldState.role.ID, qid)
			response = protocol.QuestAbandoned(qid)
		}
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "quest_rejected", "quest": qid, "error": e.Error()})
			return dispatchHandled
		}
		if e = client.output.send(1, requestData.frame.ID, response); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "quest_saved_and_sent", "character_id": client.worldState.role.ID, "quest": qid, "operation": requestData.frame.ID, "plain_hex": hex.EncodeToString(response), "client_acceptance": "pending"})
		if requestData.frame.ID == 31 {
			// Accepting one [collision quest] branch removes its
			// siblings from the offer list; push the refreshed list so
			// the unchosen faction quests disappear from the client
			// immediately instead of at the next level-up/finish/relog.
			// Must use kind=0 (quest-book update push, same as the
			// finish/CMD2278 refresh paths). kind=1 would route the
			// payload to the client's shop-buy response parser (CMD21
			// is also the buy request opcode) and crash DFO.exe.
			refreshCtx, refreshCancel := context.WithTimeout(context.Background(), 5*time.Second)
			refreshBody, refreshErr := client.worldState.availableQuestPayload(refreshCtx)
			refreshCancel()
			if refreshErr != nil {
				client.event(map[string]any{"kind": "quest_available_refresh_error", "quest": qid, "error": refreshErr.Error()})
			} else if e = client.output.send(0, 21, refreshBody); e != nil {
				return dispatchClose
			} else {
				client.event(map[string]any{"kind": "available_quests_refreshed_after_accept", "character_id": client.worldState.role.ID, "quest": qid})
			}
			// A quest is normally accepted while standing at the
			// very NPC it names, so its objective can already be
			// satisfied the moment it is accepted.
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			e = client.worldState.settleProximityObjectives(ctx, client.output.send, client.event)
			cancel()
			if e != nil {
				return dispatchClose
			}
		}
		return dispatchHandled
	}

	return dispatchNext
}
