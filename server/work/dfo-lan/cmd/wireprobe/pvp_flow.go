package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/pvp"
	"encoding/hex"
	"strconv"
)

// pvpRooms 是进程内所有决斗场房间。单机/局域网场景下所有连接共享同一份。
//
// 放在包级而不是 gatewayRuntime 上，是因为房间是**跨连接**的实体（A 建房、B 进房），
// 而 gatewayRuntime 是每网关一份的配置投影，语义不同。
var pvpRooms pvp.Manager

// pvpFreePvpChannelType 是决斗场「自由练习场」的频道类型。
//
// 115 客户端自己确证过这个值（client_trace：`set Channel Type :
// [ CHANNEL_INTEGRATED_FREEPVP ]`），对应 clientchannelinfo/channeluiinfo 的
// CHANNEL_INTEGRATED_FREEPVP 符号，城镇是 Town/Fair_PVP.twn（town 10）。
const pvpFreePvpChannelType uint32 = 13

// pvpArenaChannelType 是决斗场「无双/综合」的频道类型（CHANNEL_INTEGRATED_PVP）。
//
// 90 级的分工：type 8 走 practice/arcade（练习/街机，**单人打 APC**），type 13 走自由房间。
// 所以「PK 练习」这类单人内容应挂在 type 8 上。
const pvpArenaChannelType uint32 = 8

// pvpLoggedOpcodes 是本 handler 真正接管的 C→S 命令集合，用于正文取证日志。
// 进 dispatchPvp 只代表"人在 type 13 频道"，频道里还有大量非 PvP 流量
// （2127 进程扫描等 400 字节包），全量记录会把 events.jsonl 刷爆。
var pvpLoggedOpcodes = map[uint16]bool{
	50: true, 51: true, 52: true, 53: true, 54: true,
	55: true, 56: true, 57: true, 58: true, 59: true,
	110: true, 112: true, 195: true, 298: true, 299: true, 1285: true,
}

func (client *gameConnection) pvpChannelType() uint32 {
	if client.channelTypes == nil {
		return 0
	}
	return client.channelTypes[client.channel]
}

func (client *gameConnection) inFreePvpChannel() bool {
	ct := client.pvpChannelType()
	return ct == pvpFreePvpChannelType || ct == pvpArenaChannelType
}

func (client *gameConnection) pvpIdentity() pvp.Identity {
	return pvp.Identity{UserID: uint16(client.selectedCharacterID), Generation: 1}
}

// dispatchPvp 处理决斗场的 C→S 命令。
//
// 只认 cmd 表：50 建房 / 51 进房 / 52 座位 / 53 准备 / 54 队伍模式 / 59 选地图 /
// 298 加载完成。应答走**同号**（kind=1），房间广播走 **noti 表**（kind=0）——
// 这是 115 的既有约定（对照官方抓包：`c2s 44 USE_STACKABLE` → `s2c 44` 同号回复）。
//
// ⚠️ 单机前提：这里只把广播发给**发起者自己**。多客户端同频道广播需要接
// hub/gatewayRuntime 的连接表，属于阶段 3（真人对战）的活。
func (client *gameConnection) dispatchPvp(requestData *clientRequest) dispatchAction {
	if !client.inFreePvpChannel() || client.selectedCharacterID == 0 {
		return dispatchNext
	}
	if requestData.frame.Type != 1 || !client.bootstrapped || !requestData.verified {
		return dispatchNext
	}
	id := client.pvpIdentity()
	b := requestData.plaintext

	// 取证：服务端的 client_frame 采样只覆盖**未实现**的包，已实现的 handler 不留正文。
	// 决斗场命令的字段宽度与 90 级参考实现多处不同（cmd52 4B / cmd54 16B u32 mode），
	// 所以这里主动把每条 PvP 命令的正文与决策记下来，便于对着实机样本收敛格式。
	//
	// ⚠️ 只在**本函数真正处理**的 opcode 上记 —— 进这个函数只说明"人在 type 13 频道"，
	// 频道理还有大量非 PvP 流量（2127 进程扫描等），全记会把日志刷爆（实测一次会话 77KB）。
	if pvpLoggedOpcodes[requestData.frame.ID] {
		client.event(map[string]any{"kind": "pvp_command", "opcode": requestData.frame.ID,
			"body_len": len(b), "plain_hex": hex.EncodeToString(b)})
	}

	// 错误应答：115 的 Refusal 形态 = 0 + u16 错误码；19 沿用 90US 的 pvp 拒绝码。
	// 原因用变参，未传的地方保持原样；关键分支必须传，否则下次还得靠猜。
	refuse := func(reasons ...string) dispatchAction {
		reason := ""
		if len(reasons) > 0 {
			reason = reasons[0]
		}
		if err := client.output.send(1, requestData.frame.ID, pvp.RefusalBody(19)); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "pvp_command_refused", "opcode": requestData.frame.ID,
			"body_len": len(b), "plain_hex": hex.EncodeToString(b), "reason": reason})
		return dispatchHandled
	}
	broadcast := func(noti uint16, body []byte) dispatchAction {
		if err := client.output.send(0, noti, body); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	switch requestData.frame.ID {
	case 50:
		q, err := pvp.ParseMake(b)
		if err != nil {
			return refuse()
		}
		var room pvp.Room
		switch q.SpecialMode {
		case 0:
			// 普通 PvP 房间。实机 2026-10-08：客户端在决斗场城镇点「创建房间」发的是
			// SpecialMode=0（8 字节：NameType + Map + Pwd + Special + Flag + 尾巴），
			// 而不是练习房间的 1。90US 的 Manager.Create 正是吃 SpecialMode=0 的这一支。
			room, err = pvpRooms.Create(id, int(client.channel), q)
		case 1:
			// 练习房间：房主占 0 号位，1..7 号位对客户端显示为「关闭」。
			// APC 由客户端本地插入，服务端不占第二个座位。
			room, err = pvpRooms.CreatePractice(id, int(client.channel), q)
		case 3:
			// 街机：同样单人打 APC，Flag 1..3 = 难度档。
			room, err = pvpRooms.CreateArcade(id, int(client.channel), q)
		default:
			return refuse("不支持的 SpecialMode=" + strconv.Itoa(int(q.SpecialMode)))
		}
		if err != nil {
			return refuse()
		}
		client.event(map[string]any{"kind": "pvp_room_created", "room": room.ID, "mode": room.Mode,
			"name": string(q.Name), "map": q.Map, "special_mode": q.SpecialMode})
		if a := broadcast(41, pvp.RoomList([]pvp.Room{room})); a != dispatchHandled {
			return a
		}
		return broadcast(3, pvp.UserState([]pvp.Identity{id}, 2))

	case 51:
		roomID, pw, err := pvp.ParseEnter(b)
		if err != nil {
			return refuse()
		}
		room, err := pvpRooms.Join(id, int(client.channel), roomID, pw)
		if err != nil {
			return refuse()
		}
		client.event(map[string]any{"kind": "pvp_room_joined", "room": room.ID})
		if a := broadcast(41, pvp.RoomList([]pvp.Room{room})); a != dispatchHandled {
			return a
		}
		if err := client.output.send(1, 51, pvp.EnterSuccess(room)); err != nil {
			return dispatchClose
		}
		return broadcast(3, pvp.UserState([]pvp.Identity{id}, 2))

	case 52:
		// 实机 2026-10-08：客户端发 4 字节（`seat state 00 00`），比 90US 的 2 字节多一个尾。
		// 只读前两字节，不卡长度。
		if len(b) < 2 {
			return refuse()
		}
		room, err := pvpRooms.SetSeat(id, b[0], b[1])
		if err != nil {
			return refuse("SetSeat(seat=" + strconv.Itoa(int(b[0])) + ",state=" + strconv.Itoa(int(b[1])) + "): " + err.Error())
		}
		// 房间空了（最后一人离开，leaveLocked 把 State 置 0 并删除）：
		// **只发 RoomState（State=0）告诉客户端"房间没了"，绝不发座位表**。
		// 依据 90 级 pvpPublishDeparture：`if room.State != 0` 才发 Seats。
		// 实机 2026-10-08：对已删除房间发空座位表会让客户端状态机错乱 —— 点 exit 直接闪退
		// （崩溃报告 <LOADINGFAILED>，且崩溃前有 UDPPACKET_* 历史）。
		if room.State == 0 {
			client.event(map[string]any{"kind": "pvp_room_left", "room": room.ID})
			if a := broadcast(42, pvp.RoomState(room)); a != dispatchHandled {
				return a
			}
			// ★ 「离开房间 = 回到 PKC 频道（城镇）」—— 业主 2026-10-08 的思路。
			//
			// 只发 noti42（房间没了）不够：客户端会停在"已删房间 + 无位置"的空状态上，
			// 随后点退出时进入 relay 清理路径而崩（实机 2026-10-08，崩溃报告 <LOADINGFAILED>）。
			// 这里照搬**已有的「离开副本回城」包序列**（cmd/wireprobe/dungeon_flow.go:1084）：
			//   noti23 user_area（落点）+ noti24 area_users（同区玩家）+ noti3 user_state（城镇态）
			// 角色位置全程留在 town 10（进频道时 channel_town_spawn 就是这里），
			// 所以 userAreaPayload() 直接给出正确的回城落点，无需另算坐标。
			if w := client.worldState; w != nil && w.role.ID != 0 {
				if ua, err := w.userAreaPayload(); err != nil {
					client.event(map[string]any{"kind": "pvp_leave_area_error", "error": err.Error()})
				} else if err = client.output.send(0, 23, ua); err != nil {
					return dispatchClose
				}
				if area, err := w.areaPayload(); err != nil {
					client.event(map[string]any{"kind": "pvp_leave_users_error", "error": err.Error()})
				} else if err = client.output.send(0, 24, area); err != nil {
					return dispatchClose
				}
				if state, err := protocol.UserState(w.role.WireID, protocol.UserStateTown); err != nil {
					client.event(map[string]any{"kind": "pvp_leave_state_error", "error": err.Error()})
				} else if err = client.output.send(0, 3, state); err != nil {
					return dispatchClose
				}
				client.event(map[string]any{"kind": "pvp_returned_to_town",
					"town": w.state.Position.Town, "area": w.state.Position.Area})
			}
			return dispatchHandled
		}
		if a := broadcast(42, pvp.RoomState(room)); a != dispatchHandled {
			return a
		}
		return broadcast(43, pvp.Seats(room))

	case 53:
		if len(b) < 1 || b[0] > 1 {
			return refuse("READY 首字节非法: len=" + strconv.Itoa(len(b)))
		}
		room, started, err := pvpRooms.Ready(id, b[0] != 0)
		if err != nil {
			return refuse("Ready: " + err.Error())
		}
		client.event(map[string]any{"kind": "pvp_ready", "value": b[0], "started": started,
			"seat": room.SeatOf(id), "room_state": room.State})
		if err := client.output.send(0, 44, []byte{byte(room.SeatOf(id)), b[0]}); err != nil {
			return dispatchClose
		}
		if started {
			client.event(map[string]any{"kind": "pvp_round_started", "room": room.ID,
				"map": room.Map, "mode": room.Mode})
			if err := client.output.send(0, 45, []byte{byte(room.Map), room.Mode}); err != nil {
				return dispatchClose
			}
		}
		return broadcast(42, pvp.RoomState(room))

	case 54:
		// 实机 2026-10-08：客户端发 16 字节，**mode 是 u32**（实测值 1 和 3），
		// 后面跟 12 字节 0。90US 这里只读 1 字节且只允许 1/2 —— 115 会发 3（擂台类），
		// 所以放到 1..4，别用 90US 的窄校验把客户端自己的合法设置拒掉。
		if len(b) < 4 {
			return refuse()
		}
		mode := b[0]
		room, err := pvpRooms.SetMode(id, mode)
		if err != nil {
			return refuse()
		}
		return broadcast(42, pvp.RoomState(room))

	case 59:
		// 地图索引 u16（小端）。同样不卡长度 —— 115 的 PvP 命令普遍带尾部填充。
		if len(b) < 2 {
			return refuse()
		}
		room, err := pvpRooms.SetMap(id, uint16(b[0])|uint16(b[1])<<8)
		if err != nil {
			return refuse()
		}
		return broadcast(42, pvp.RoomState(room))

	case 298:
		if len(b) != 0 {
			return refuse()
		}
		if _, err := pvpRooms.Loaded(id); err != nil {
			return refuse()
		}
		return dispatchHandled

	case 55, 56, 57, 58, 110, 112, 195, 299, 1285:
		// 战斗期/匹配期的命令：阶段 2 未实现。记录样本，但不伪造应答。
		client.event(map[string]any{"kind": "pvp_command_unimplemented",
			"opcode": requestData.frame.ID, "body_len": len(b)})
		return dispatchHandled
	}
	return dispatchNext
}
