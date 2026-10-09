package main

import (
	"context"
	"dfolan/internal/channelrefresh"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"time"
)

// gameConnection owns mutable state for exactly one client. Runtime services
// are shared; characters shadows the shared service when channel context differs.
type gameConnection struct {
	*gatewayRuntime
	characters           *character.Service
	channel              uint32
	channelCfg           channelrefresh.Config
	channelTypes         map[uint32]uint32
	event                func(map[string]any)
	bodySamples          map[uint16]int
	bootstrapped         bool
	buffEnhancementState buffEnhancementSession
	channelNotice        []byte
	comboState           comboSkillSession
	connection           *connectionSession
	cubeContractState    cubeContractSession
	equipmentState       equipmentSession
	frames               <-chan clientRead
	keys                 []byte
	// loginEventFloodSent 选角前的登录事件洪流（next79 §13/§15，
	// ispins_wiring.go sendLoginFloodOnce）每连接只发一次。
	loginEventFloodSent bool
	legionState         legionSession
	mailAlarmRole       int64
	mailChanges         chan struct{}
	mailDeliveryID      int64
	output              *connectionOutput
	peer                string
	purchaseSession     *shopPilotSession
	selectedAddition    []byte
	selectedBasic       []byte
	selectedCharacterID int64
	skillState          skillSession
	sortState           sortSession
	worldState          *worldSession
}

type gameGateway struct {
	runtime      *gatewayRuntime
	channels     channelrefresh.Config
	channelTypes map[uint32]uint32
	event        func(map[string]any)
}

func (gateway *gameGateway) handleClient(c net.Conn, channel uint32) {
	client := &gameConnection{gatewayRuntime: gateway.runtime, characters: gateway.runtime.characters, channel: channel, channelCfg: gateway.channels, channelTypes: gateway.channelTypes, event: gateway.event}
	client.peer = ""
	// Recover after the reader, timers, world membership and socket have closed.
	// recover() must run in the directly deferred function. Fill the peer at
	// report time so even an initialization panic keeps its available identity.
	defer recoverConnection("", client.channel, c, func(v map[string]any) {
		v["peer"] = client.peer
		client.event(v)
	})
	defer c.Close()
	client.peer = c.RemoteAddr().String()
	if client.config.ChannelIdentity || client.channelCfg.SynchronizeIdentity {
		ctx, notice, identityErr := channelIdentity(client.channelCfg, client.channel)
		if identityErr != nil || client.characters == nil {
			client.event(map[string]any{"kind": "channel_identity_error", "error": fmt.Sprint(identityErr), "characters_present": client.characters != nil})
			return
		}
		localCharacters := *client.characters
		localCharacters.ChannelContext = ctx
		client.characters = &localCharacters
		client.channelNotice = notice
	}
	client.keys = make([]byte, wire.SessionKeyBytes)
	for i := range client.keys {
		client.keys[i] = byte(i%127 + 1)
	}
	client.bootstrapped = false
	client.bodySamples = map[uint16]int{}
	var sessionErr error
	client.purchaseSession, sessionErr = newShopPilotSession()
	if sessionErr != nil {
		client.event(map[string]any{"kind": "shop_session_error", "error": sessionErr.Error()})
		return
	}
	client.purchaseSession.keys = client.keys
	if (client.config.VaultPurchaseCandidate || client.config.VaultPurchaseRelease || client.config.ShopRelease) && client.vaultService != nil {
		client.purchaseSession.vaultRules = &client.vaultService.Rules
	}
	client.legionState.catalog = client.apocalypseCatalog
	client.legionState.clock = client.apocalypseClock
	client.legionState.channelType = client.channelTypes[client.channel]
	if client.worldService != nil {
		client.worldState = &worldSession{characters: client.characters, service: client.worldService, store: client.gameStore, account: client.developmentAccount, flags: client.townPolicy.Flags, dungeons: client.dungeonCatalog, townArrivalScenes: client.townArrivalScenes, tutorials: client.tutorialRoutes, tutorialDungeons: client.tutorialDungeons, professions: client.characters.Catalog, fatigue: client.fatigueService, quests: client.questService, progression: client.progressionService, rewards: client.rewards, loot: client.lootService, items: client.itemService, shop: client.shopService, selectionBoxes: client.selectionBoxes, vault: client.vaultService, skinCatalog: client.skinCatalog, soloPartyBootstrap: client.config.SoloPartyBootstrap, hub: client.hub, scaleDeathFromHP: client.config.ScaleDeathFromHP, oathGrades: client.oathGradePair, oathTable: client.oathGradeTable, oathFromGear: client.config.OathGradesFromGear, oathProgressClears: client.config.OathProgressClears, oathProgressDungeons: client.oathProgressSet, deferredClearSet: client.deferredClearSet, oathInject: client.oathInjectSpecs, omenHold: client.config.OmenHold, omenState: client.omenState, omenInfo: client.omenInfoBytes, attunementReward: client.attunementRewardSpec}
		// 启动器自动拾取只对本机回环连接生效，避免把本地便利开关扩散给局域网玩家。
		if addr, ok := c.RemoteAddr().(*net.TCPAddr); ok {
			client.worldState.autoPickup = addr.IP.IsLoopback() && os.Getenv("DFO_AUTO_PICKUP") == "1"
		}
		client.worldState.serverID = client.channelCfg.ServerID
		client.worldState.channelType = client.channelTypes[client.channel]
		client.worldState.eliteChannelDirectory = client.gatewayRuntime.channelDirectory
		client.worldState.eliteChannelInfo = client.gatewayRuntime.channelInfo
		client.worldState.eliteChannelID = client.channel
		client.worldState.bakalRules = client.bakalRaidRules
		client.worldState.bakalRewards = client.bakalRewardService
		// 特殊征讨频道（SemiRaid/Legion，towns 表里有专属城镇的）的位置隔离：
		// 会话内位置不覆盖普通频道的共享行（黑鸦 73 / 矿区 106 既有模式的全频道推广）。
		if _, isolated := client.gatewayRuntime.channelTowns[client.channelTypes[client.channel]]; isolated {
			client.worldState.channelWorldIsolated = true
		}
		if client.worldState.specialTowns == nil {
			client.worldState.specialTowns = map[uint32]bool{}
			for _, a := range client.gatewayRuntime.channelTowns {
				client.worldState.specialTowns[a.TownID] = true
			}
		}
		client.worldState.channelGuideDungeon = client.gatewayRuntime.channelGuides[client.channelTypes[client.channel]]
		// 蔚蓝号（征服频道 102）的源驱动翻牌策略（nil = 未装配 ⇒ 退回通用翻牌）。
		// 真正生效还要 activeDungeon 的副本号等于策略记的结算层，见 azureFlipActive()。
		// ⚠️ 漏了这一行不会报错、只会静默走老路（2026-10-09 实测代价：一轮实机白跑）。
		client.worldState.azureFlipCfg = client.azureFlip
		if client.moonConfig != nil && client.channel == client.moonConfig.Channel {
			client.worldState.moonConfig = client.moonConfig
		}
		// Starter Boost 662：会话拿装配层冻结的活动目录；nil = 活动关闭，一切照旧。
		client.worldState.boostup = client.boostCatalog
		// 末世录（频道 Type 119）：CMD2062 直进要在 dungeon 派发层先于通用
		// 「没有 activeDungeon 就拒绝」的守卫被接管，所以把 legionSession 挂到
		// worldSession 上；apocalypse 指针在 apocalypseRun() 里随 run 建立同步
		// 写回 worldState，两者始终指向同一份状态。
		client.worldState.legion = &client.legionState
		client.legionState.world = client.worldState
		client.worldState.apocalypse = client.legionState.apocalypse
	}
	if client.worldState != nil {
		defer client.worldState.departArea()
	}
	client.output = newConnectionOutput(c, client.keys, client.peer, client.event)
	client.event(map[string]any{"kind": "accept", "peer": client.peer})
	if err := client.output.writeRaw(client.raw); err != nil {
		client.event(map[string]any{"kind": "write_error", "error": err.Error()})
		return
	}
	client.event(map[string]any{"kind": "server_frame", "peer": client.peer, "hex": hex.EncodeToString(client.raw)})
	client.connection = newConnectionSession(client.worldState != nil && client.worldState.moonConfig != nil)
	defer client.connection.close()
	client.frames = clientFrames(c, client.connection.done)
	client.mailChanges = client.connection.mailChanges
	client.serve()
}

// serve retains the single connection loop for incoming frames and timers.
func (client *gameConnection) serve() {
	for {
		var incoming clientRead
		select {
		case incoming = <-client.frames:
		case now := <-client.connection.mineTicker.C:
			if client.bootstrapped && client.selectedCharacterID != 0 && client.worldState != nil {
				cardPackets, cardErr := client.worldState.autoPickSettlementCard(now)
				if cardErr != nil {
					client.event(map[string]any{"kind": "黑鸦自动翻牌待重试", "character_id": client.selectedCharacterID, "error": cardErr.Error()})
				}
				// 沉月湖的翻牌走的是另一条领取路径（w.moon.plan + moonClaim），
				// 上面那条通用自动选牌对它不适用 —— 见 autoPickMoonCard 的注释。
				moonCardPackets, moonCardErr := client.worldState.autoPickMoonCard(now)
				if moonCardErr != nil {
					client.event(map[string]any{"kind": "月湖自动翻牌待重试", "character_id": client.selectedCharacterID, "error": moonCardErr.Error()})
				}
				if client.sendPlan(append(cardPackets, moonCardPackets...), client.logCharacterResponseBody) != nil {
					return
				}
				quotaPackets, quotaErr := client.worldState.refreshBlackPurgatoryQuota(now)
				if quotaErr != nil {
					client.event(map[string]any{"kind": "黑鸦次数同步失败", "error": quotaErr.Error()})
				}
				if client.sendPlan(quotaPackets, client.logCharacterResponseBody) != nil {
					return
				}
				packets, err := client.worldState.bleedingMineTimeout(now)
				if err != nil {
					client.event(map[string]any{"kind": "赤红铁矿超时退出失败", "error": err.Error()})
				}
				if client.sendPlan(packets, client.logCharacterResponseBody) != nil {
					return
				}
				// 末世录终局结算：清关那一刻挂起，约 400ms 后发翻牌链。
				if grantPackets, grantEvents := client.worldState.apocalypseSettlementDue(now); len(grantPackets) > 0 || len(grantEvents) > 0 {
					for _, note := range grantEvents {
						client.event(note)
					}
					if client.sendPlan(grantPackets, client.logWorldResponseBody) != nil {
						return
					}
				}
				// 末世录难度框强制关闭：开窗 15 秒未确认难度就关窗并把状态复位成
				// 未选（业主 2026-10-08，防「开了窗不选就卡死」）。
				if closePackets, closeEvents := client.worldState.apocalypseSelectWindowDue(now); len(closePackets) > 0 || len(closeEvents) > 0 {
					for _, note := range closeEvents {
						client.event(note)
					}
					if client.sendPlan(closePackets, client.logWorldResponseBody) != nil {
						return
					}
				}
				// 末世录阶段推进兜底：清关后客户端没有发 CMD2062 时，由服务端自己
				// 把下一关推进去（发全套进图帧）。见 apocalypseStageAdvanceDelay。
				//
				// 先 sweep：只要房间已清空而没有任何挂起推进就补排一次 —— 客户端
				// 有时不发 CMD117（投影不跑），只靠投影会把推进链打断。
				for _, note := range client.worldState.apocalypseAdvanceSweep(now) {
					client.event(note)
				}
				if advPackets, advEvents := client.worldState.apocalypseAdvanceDue(now); len(advPackets) > 0 || len(advEvents) > 0 {
					for _, note := range advEvents {
						client.event(note)
					}
					if client.sendPlan(advPackets, client.logWorldResponseBody) != nil {
						return
					}
				}
				// 超时只打开矿区失败选项，保留会话供结束探索或放弃处理。
				packets, err = client.worldState.blackPurgatoryTimeout(now)
				if err != nil {
					client.event(map[string]any{"kind": "黑鸦超时退出失败", "error": err.Error()})
				}
				if client.sendPlan(packets, client.logCharacterResponse) != nil {
					return
				}
				packets, err = client.worldState.ispinsTimeout(now)
				if err != nil {
					client.event(map[string]any{"kind": "ispins_timeout_error", "error": err.Error()})
				}
				if client.sendPlan(packets, client.logWorldResponseBody) != nil {
					return
				}
				// 维纳斯阶段倒计时到期：判定挑战失败、回待机区并复位 run（venus_stage_timeout）。
				packets = client.worldState.venusStageTimeout(now, client.event)
				packets = append(packets, client.worldState.apocalypseStageTimeout(now, client.event)...)
				if client.sendPlan(packets, client.logWorldResponseBody) != nil {
					return
				}
				// 苏醒之森关卡倒计时到期（60 分钟/关，同款失败链路）。
				packets = client.worldState.forestStageTimeout(now, client.event)
				if client.sendPlan(packets, client.logWorldResponseBody) != nil {
					return
				}
				// 苏醒之森 Extreme 的「净化开始」横幅演完 → 下发挂起的进图帧列。
				// 精确路径见 connection_session.scheduleForestBanner（N2568 自带
				// delay[4]，进图帧列必须落在那个窗口内）；这里的 1 秒 tick 只是
				// 定时器失效时的兜底（force=false 时按 fallback 期限判定）。
				if entryPackets, entryEvents := client.worldState.forestEntryDue(now, false); len(entryPackets) > 0 || len(entryEvents) > 0 {
					for _, note := range entryEvents {
						client.event(note)
					}
					if client.sendPlan(entryPackets, client.logWorldResponseBody) != nil {
						return
					}
				}
				// 维纳斯难度选择窗倒计时归 0：推原生 close ACK 自动关窗。
				packets = client.worldState.venusOperationClose(now, client.event)
				if client.sendPlan(packets, client.logWorldResponseBody) != nil {
					return
				}
				// 巴卡尔开战时钟：开战 burst、血量发布与结算串（2285→N13→588→574）
				// 都从这一秒 tick 驱动（settle 前一瞬注入奖励冻结与全量背包）。
				if client.tickBakalOpening(now) != nil {
					return
				}
			}
			continue
		case now := <-client.connection.moonTicks():
			if client.bootstrapped && client.selectedCharacterID != 0 {
				packets, e := client.worldState.moonTick(now)
				if e != nil {
					client.event(map[string]any{"kind": "moon_tick_error", "error": e.Error()})
				}
				if client.sendPlan(packets, func(packet outboundPacket) {
					client.event(map[string]any{"kind": packet.Name, "id": packet.ID})
				}) != nil {
					return
				}
			}
			continue
		case now := <-client.connection.forestBanner:
			// 苏醒之森 Extreme「净化开始」横幅到期（N2568 自带 delay[4]）：
			// 精确下发挂起的进图帧列。worldSession 只有主循环碰，无并发。
			if client.bootstrapped && client.selectedCharacterID != 0 && client.worldState != nil {
				packets, events := client.worldState.forestEntryDue(now, true)
				for _, note := range events {
					client.event(note)
				}
				if client.sendPlan(packets, client.logWorldResponseBody) != nil {
					return
				}
			}
			continue
		case <-client.connection.mailTicker.C:
			select {
			case client.mailChanges <- struct{}{}:
			default:
			}
			continue
		case <-client.mailChanges:
			if client.bootstrapped && client.selectedCharacterID != 0 && client.worldState != nil && client.worldState.characters != nil {
				adventureCtx, adventureCancel := context.WithTimeout(context.Background(), 5*time.Second)
				adventurePackets, adventureErr := client.worldState.refreshAdventure(adventureCtx)
				adventureCancel()
				if adventureErr != nil {
					client.event(map[string]any{"kind": "adventure_refresh_error", "reason": adventureErr.Error()})
				} else {
					if client.sendPlan(adventurePackets, func(packet outboundPacket) {
						if packet.ID == 2799 {
							client.event(map[string]any{"kind": "season_level_synced", "character_id": client.selectedCharacterID, "attempt": "2/3（源阶段及领奖reader已核实）", "id": packet.ID, "plain_hex": hex.EncodeToString(packet.Payload)})
						}
					}) != nil {
						return
					}
				}
				mailCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				alarm, latest, err := client.worldState.mailboxAlarm(mailCtx)
				cancel()
				if err != nil {
					client.event(map[string]any{"kind": "mailbox_alarm_error", "character_id": client.selectedCharacterID, "reason": err.Error()})
				} else if client.mailAlarmRole != client.selectedCharacterID || latest > client.mailDeliveryID {
					// 仅登录和真正的新投递发送 NOTI99，避免读信/领取后销毁详情对象。
					if err = client.output.send(0, 99, alarm); err != nil {
						return
					}
					client.mailAlarmRole, client.mailDeliveryID = client.selectedCharacterID, latest
					client.event(map[string]any{"kind": "mailbox_delivery_notified", "character_id": client.selectedCharacterID, "latest_mail_id": latest})
				}
			}
			continue
		case now := <-client.connection.dailyTicker.C:
			if client.bootstrapped && client.selectedCharacterID != 0 && client.worldState != nil {
				p, e := client.worldState.refreshDailyFatigue(now)
				if e != nil {
					client.event(map[string]any{"kind": "fatigue_daily_error", "error": e.Error()})
				}
				if e == nil && p != nil {
					if e = client.output.send(0, 36, p); e != nil {
						return
					}
					client.event(map[string]any{"kind": "fatigue_daily_refresh", "character_id": client.selectedCharacterID})
				}
				loyaltyCtx, loyaltyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				loyaltyPackets, loyaltyErr := client.worldState.refreshCreatureLoyalty(loyaltyCtx, now, client.worldState.activeDungeon != nil)
				loyaltyCancel()
				if loyaltyErr != nil {
					client.event(map[string]any{"kind": "creature_loyalty_error", "character_id": client.selectedCharacterID, "error": loyaltyErr.Error()})
				} else {
					if client.sendPlan(loyaltyPackets, nil) != nil {
						return
					}
				}
			}
			continue
		}
		frame, err := incoming.frame, incoming.err
		if err != nil {
			client.event(map[string]any{"kind": "close", "peer": client.peer, "error": err.Error()})
			return
		}
		entry := map[string]any{"kind": "client_frame", "peer": client.peer, "type": frame.Type, "id": frame.ID, "bytes": len(frame.Raw)}
		var plaintext []byte
		verified := false
		// An implemented command always retains its body. Everything
		// else is sampled up to a per-command cap, so the wire format
		// of a feature this build does not implement is captured by
		// ordinary play rather than guessed at from another build.
		//
		// A parameterless command is a bare 13-byte header. The client
		// writes no fields, so there is no ciphertext at all; bodies are
		// padded to 16, so any command that carries data arrives as 29
		// bytes or more. command_layouts36.json lists 7 (leave game),
		// 42 and 69 (dungeon exit), 63, 67, 120 and 1301 (village
		// return) as body 0. Demanding 14 bytes before verifying made
		// every one of them fail the checksum gate unread, which is why
		// the escape menu did nothing: the frame is complete, its
		// payload is simply empty, and the checksum then covers only
		// the two header bytes.
		// **判定与采样必须解耦**：`verified` 是业务前提（下面 frame.Type==1 的分发都靠它），
		// 而 `retainRequestBody` 只决定"要不要把正文写进诊断日志"、且有 BodySampleLimit(8)
		// 上限。2026-10-02 实机踩过：CMD2258 当时没列入 observedGameRequest ⇒ 走采样分支
		// ⇒ 同一会话第 8 次之后每帧 `verified=false` ⇒ 客户端表现"调适 8 次后无法继续、
		// 重进客户端又能再来 8 次"。这里始终解密并校验，只把正文记录压在采样上限内。
		if len(frame.Raw) >= wire.ClientHeaderSize {
			if p, e := wire.DecryptPayload(client.keys, frame.ID, frame.Raw[13:]); e == nil {
				plaintext = p
				verified = wire.Checksum(append(append([]byte{}, frame.Raw[11:13]...), p...)) == frame.Raw[7]
				if retainRequestBody(frame.ID, client.bodySamples) {
					entry["hex"] = hex.EncodeToString(frame.Raw)
					entry["plain_hex"] = hex.EncodeToString(p)
					entry["checksum_ok"] = verified
					if !observedGameRequest(frame.ID) {
						entry["unimplemented_sample"] = true
					}
				}
			} else {
				if retainRequestBody(frame.ID, client.bodySamples) {
					entry["hex"] = hex.EncodeToString(frame.Raw)
				}
				entry["decode_error"] = e.Error()
			}
		}
		client.event(entry)
		if client.dispatch(&clientRequest{frame: frame, err: err, plaintext: plaintext, verified: verified}) == dispatchClose {
			return
		}
	}
}

func (client *gameConnection) sendPlan(plan []outboundPacket, sent func(outboundPacket)) error {
	var moonEntry map[string]any
	for _, p := range plan {
		if p.Name == "moon_dungeon" && client.worldState != nil {
			moonEntry = client.worldState.eliteMoonEntryObservation()
			break
		}
	}
	return sendPacketPlan(plan, client.output.send, func(p outboundPacket) {
		if p.Name == "moon_dungeon" && moonEntry != nil {
			client.event(moonEntry)
		}
		if p.ID == 1754 || p.ID == 1382 || p.ID == 1879 {
			event := adventureEliteDiagnostic(client.worldState, p.ID, []outboundPacket{p}, nil)
			event["kind"] = "adventure_elite_packet_sent"
			client.event(event)
		}
		if sent != nil {
			sent(p)
		}
	})
}

func (client *gameConnection) logCharacterResponseBody(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload), "character_id": client.selectedCharacterID})
}

func (client *gameConnection) logCharacterResponse(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.selectedCharacterID})
}

func (client *gameConnection) logWorldResponseBody(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": client.worldState.role.ID, "plain_hex": hex.EncodeToString(p.Payload)})
}

func (client *gameConnection) logResponseBody(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
}

func (client *gameConnection) logWorldResponse(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "character_id": client.worldState.role.ID, "id": p.ID})
}

func (client *gameConnection) logWorldAction(p outboundPacket) {
	client.event(map[string]any{"kind": p.Name, "character_id": client.worldState.role.ID})
}

func (client *gameConnection) sendServerTime(reason string) error {
	now := time.Now()
	payload, err := protocol.ServerTimeSuccess(now)
	if err != nil {
		client.event(map[string]any{"kind": "server_time_error", "error": err.Error()})
		return err
	}
	if err = client.output.send(1, 1960, payload); err != nil {
		return err
	}
	client.event(map[string]any{"kind": "server_time_sent", "id": 1960, "reason": reason, "unix_seconds": now.Unix(), "plain_hex": hex.EncodeToString(payload)})
	return nil
}
