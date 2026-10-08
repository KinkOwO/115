package main

import (
	"dfolan/internal/pvp"
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

func (client *gameConnection) pvpChannelType() uint32 {
	if client.channelTypes == nil {
		return 0
	}
	return client.channelTypes[client.channel]
}

func (client *gameConnection) inFreePvpChannel() bool {
	return client.pvpChannelType() == pvpFreePvpChannelType
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

	// 错误应答：115 的 Refusal 形态 = 0 + u16 错误码；19 沿用 90US 的 pvp 拒绝码。
	refuse := func() dispatchAction {
		if err := client.output.send(1, requestData.frame.ID, pvp.RefusalBody(19)); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "pvp_command_refused", "opcode": requestData.frame.ID, "body_len": len(b)})
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
			room, err = pvpRooms.CreatePractice(id, int(client.channel), q)
		case 3:
			// 街机（单人打 APC）：客户端本地跑 AI 与伤害，服务端只记流程。
			// 尚未实现 —— 显式拒绝，不伪造一个空房间。
			client.event(map[string]any{"kind": "pvp_arcade_unimplemented", "map": q.Map, "flag": q.Flag})
			return refuse()
		default:
			return refuse()
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
			return refuse()
		}
		if a := broadcast(42, pvp.RoomState(room)); a != dispatchHandled {
			return a
		}
		return broadcast(43, pvp.Seats(room))

	case 53:
		if len(b) < 1 || b[0] > 1 {
			return refuse()
		}
		room, started, err := pvpRooms.Ready(id, b[0] != 0)
		if err != nil {
			return refuse()
		}
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
