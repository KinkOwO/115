package main

import (
	"dfolan/internal/game/wire"
	"dfolan/internal/servermod"
	"encoding/hex"
)

// dispatchAction distinguishes checking the next handler, reading the next
// frame, and closing the connection. It preserves the original loop control.
type dispatchAction uint8

const (
	dispatchNext dispatchAction = iota
	dispatchHandled
	dispatchClose
)

type clientRequest struct {
	frame     wire.Frame
	plaintext []byte
	verified  bool
	err       error
}
type clientDispatchStage func(*gameConnection, *clientRequest) dispatchAction

// Order is part of the routing contract; the type gate stays between the lists.
var beforeClientTypeDispatch = [...]clientDispatchStage{
	// dispatchIspins 必须首位（next79）：伊斯族 CMD2043/2045/2046 与末世录
	// 共用信封、按内容号分流，终局剧情 191 也须先于通用 191 处理器。
	(*gameConnection).dispatchIspins,
	// dispatchVenus 紧随其后：维纳斯待机区 CMD12/13 与家族拦截（内容 106）
	// 必须先于 dispatchLegion，防止落进末世录处理器。
	(*gameConnection).dispatchVenus,
	// dispatchBakal 再其后：巴卡尔频道（type 82）的 raid 信封（656/2089/
	// 2069-2074/2261/1134）必须先于 dispatchLegion 与通用副本处理器。
	(*gameConnection).dispatchBakal,
	// dispatchForest 再随后：苏醒之森（Type 96）待机区 CMD12/13，同理由
	// （各内容自带的队伍类型字节互不相认）。
	(*gameConnection).dispatchForest,
	// dispatchEvildom 再随后：次元回廊（Type 50）待机区 CMD12/13 与开战
	// CMD2043（内容 0x0d）。该内容的队伍类型字节是 0x0d，同样互不相认；
	// 而且 legion.Requests 不含 CMD12，不在这里接管就会被通用队伍链丢掉。
	(*gameConnection).dispatchEvildom,
	(*gameConnection).dispatchSpecialContent,
	(*gameConnection).dispatchCashshopAndBoxes,
	(*gameConnection).dispatchStoryAndAdvancement,
	(*gameConnection).dispatchSpecialEquipment,
	(*gameConnection).dispatchLegion,
	(*gameConnection).dispatchAccountVault,
	(*gameConnection).dispatchEquipmentSkillsAndMoves,
}

var commandDispatch = [...]clientDispatchStage{
	(*gameConnection).dispatchAccountQueries,
	(*gameConnection).dispatchUnifiedOptions,
	(*gameConnection).dispatchSessionTransitions,
	(*gameConnection).dispatchCharacterSkills,
	(*gameConnection).dispatchCosmeticsAndGold,
	(*gameConnection).dispatchEnhancementAndConsumables,
	(*gameConnection).dispatchEquipmentTransactions,
	(*gameConnection).dispatchCharacterNotice,
	// dispatchPvp 排在 dispatchDungeon 之前：决斗场频道的 cmd 50-59 与副本内命令
	// 共用同一套 cmd 表，但频道类型互斥（只有 type 13 才进这里），先判频道更清晰。
	(*gameConnection).dispatchPvp,
	(*gameConnection).dispatchDungeon,
	(*gameConnection).dispatchWorldAndQuests,
	(*gameConnection).dispatchCharacterEntry,
	(*gameConnection).dispatchRoster,
	// dispatchAttendanceClaim 排在 dispatchBoostEvent 之前：两者都处理 CMD680，
	// 签到那一站只在载荷里的活动号 == 331 时接管，其余原样放行（见 attendance_claim.go）。
	(*gameConnection).dispatchAttendanceClaim,
	(*gameConnection).dispatchBoostEvent,
	// dispatchAvatarCloset 处理衣柜（寄存衣柜）的 cmd1102/1103。
	(*gameConnection).dispatchAvatarCloset,
	(*gameConnection).dispatchFixtureResponse,
}

func (client *gameConnection) dispatch(requestData *clientRequest) dispatchAction {
	// server 层 mod 的 protocol.request 接入点：内置分发之前的唯一咽喉。
	// 这里拿到的是**已解密、已判校验和**的报文（client_connection.go 的读循环负责），
	// 所以钩子看到的内容与内置 handler 看到的完全一致。
	// 没有登记任何钩子时零开销跳过（HasRequestHooks 不加锁之外不做任何分配）。
	if servermod.HasRequestHooks() {
		handled := servermod.ObserveRequest(
			client.peer,
			requestData.frame.Type,
			requestData.frame.ID,
			requestData.frame.Raw,
			requestData.plaintext,
			requestData.verified,
			func(kind byte, id uint16, payload []byte) error {
				return client.output.send(kind, id, payload)
			})
		if handled {
			// 钩子整条接手：内置分发表不再看它（应答由 mod 自己经 Reply 发出）。
			return dispatchHandled
		}
	}
	for _, handler := range beforeClientTypeDispatch {
		if result := handler(client, requestData); result != dispatchNext {
			return result
		}
	}
	if result := client.dispatchClientType(requestData); result != dispatchNext {
		return result
	}
	for _, handler := range commandDispatch {
		if result := handler(client, requestData); result != dispatchNext {
			return result
		}
	}
	return dispatchNext
}

func (client *gameConnection) dispatchClientType(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 {
		client.event(map[string]any{"kind": "unsupported_client_type", "type": requestData.frame.Type})
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchFixtureResponse(requestData *clientRequest) dispatchAction {
	if response, ok := client.responses[requestData.frame.ID]; ok {
		if requestData.frame.ID == 1 && client.channelNotice != nil {
			response, requestData.err = channelLoginResponse(client.keys, response, client.channelNotice[12])
			if requestData.err != nil {
				client.event(map[string]any{"kind": "channel_login_error", "error": requestData.err.Error()})
				return dispatchClose
			}
		}
		if requestData.frame.ID == 1 && client.moonConfig != nil && client.channel == client.moonConfig.Channel {
			var e error
			response, e = moonLoginResponse(response, client.keys)
			if e != nil {
				client.event(map[string]any{"kind": "moon_login_error", "error": e.Error()})
				return dispatchClose
			}
		}
		if err := client.output.writeRaw(response); err != nil {
			client.event(map[string]any{"kind": "write_error", "error": err.Error()})
			return dispatchClose
		}
		client.event(map[string]any{"kind": "server_response", "peer": client.peer, "id": requestData.frame.ID, "hex": hex.EncodeToString(response)})
		if requestData.frame.ID == 1 {
			client.bootstrapped = true
			// 固定 CMD1 登录模板不经过 NOTI1 的服务器时钟初始化。
			// 145257F70 是已注册的 CMD1960 原生同步入口；1459A2D70
			// 按 opcode 直接分发，无请求等待态依赖。必须在角色/UI包前
			// 建立时钟，不能等冒险团日期判断已访问空指针后再补发。
			if requestData.err = client.sendServerTime("登录初始化"); requestData.err != nil {
				return dispatchClose
			}
			if client.channelNotice != nil {
				if requestData.err = client.output.send(0, 2435, client.channelNotice); requestData.err != nil {
					return dispatchClose
				}
				client.event(map[string]any{"kind": "channel_identity_sent", "server": client.channelCfg.ServerID, "channel": client.channel})
			}
		}
	}

	return dispatchNext
}
