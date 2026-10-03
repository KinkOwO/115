package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/npcpresence"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"dfolan/internal/world"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type worldSession struct {
	npcPresenceIndex       *npcpresence.Index
	npcPresenceIndexErr    error
	lastFame               uint32
	fameInitialized        bool
	moonConfig             *moonSoloConfig
	moon                   moonSoloState
	characters             *character.Service
	pilotDeath             *odysseyDeath
	service                *world.Service
	store                  *storage.Store
	account                int64
	serverID               uint32
	role                   storage.Character
	level                  byte
	adventureSnapshot      [32]byte
	channelType            uint32
	// channelWorldIsolated 标记当前连接在特殊征讨频道（towns 表有专属城镇）。
	// true 时会话内位置不落普通频道共享行；specialTowns 是全部特殊城镇集合，
	// 用于把共享行里的历史污染位置修回默认落点。
	channelWorldIsolated bool
	specialTowns         map[uint32]bool
	// channelGuideDungeon 是本频道 [guide dungeon index] 直读值（SemiRaid/Legion
	// 频道的红门直接进这个副本）；0 = 无（普通频道）。
	channelGuideDungeon uint32
	bleedingMineCreated    bool
	bleedingMineReady      bool
	bleedingMineRoster     []int64
	bleedingMineStart      *bleedingMineStart
	blackPurgatory         blackPurgatoryState
	adventureEliteSnapshot [32]byte
	// odyssey mirrors character.OdysseyRole for this session. It selects which
	// source level gate the world service applies: an Arad Odyssey character
	// follows the client's [odyssey enter level] instead of [need level].
	odyssey          bool
	state            storage.WorldState
	flags            [3]byte
	dungeons         *catalog.DungeonCatalog
	tutorials        *catalog.TutorialCatalog
	tutorialDungeons *catalog.DungeonCatalog
	professions      catalog.Characters
	inTutorial       bool
	fatigue          *character.FatigueService
	lastFatigueDay   string
	quests           *quest.Service
	progression      *character.ProgressionService
	loot             *loot.Service
	items            *inventory.ItemService
	shop             *workflow.ShopService
	selectionBoxes   *catalog.SelectionBoxes
	vault            *workflow.VaultService

	townArrivalScenes   map[uint32]catalog.TownArrivalScene
	approvedDungeonGate uint32
	pendingTownArrival  *dungeon.Session
	// craftPending / craftPendingAt 记录上一次装备库制作（CMD2259）请求的指纹与
	// 时间戳（UnixNano）。**同一个正文客户端会发两次**（"变换" → "确定"），
	// 而且两次的 plain_hex 逐字节相同 ⇒ 只能由服务端记状态来区分第一步与第二步。
	// 见 analysis/tasks/next126 §8。
	craftPending   string
	craftPendingAt int64

	// skinCatalog maps an `[add skin storage]` template to its PVF skin key; nil
	// disables the CMD507 action 169 flow.
	skinCatalog map[uint32]catalog.SkinStorageEntry
	drops       *loot.Session
	deathSent   map[uint16]bool
	// scaleDeathFromHP 打开「定盘机关血量触底时由服务端宣布它死亡」这条兜底路径
	// （见 scale_death.go）。默认关闭，开启方式是 -scale-death-from-hp
	// 或 DFO_SCALE_DEATH_FROM_HP=1。
	scaleDeathFromHP bool
	scaleHP          map[uint32]float64
	scaleForced      map[uint16]bool
	// oathGrades 是**显式覆盖**的引子/誓约档位（noti 2838 的载荷）；零值 = 不覆盖，
	// 由角色穿戴的装备决定，见 oath_info.go。
	oathGrades [2]uint16
	// oathTable 把誓约/引子装备的稀有度换算成机制档位
	// （internal/inventory/oath_grade.go）。恒发 45 曾让隐藏 BOSS 场场登场。
	// 只在 -oath-grades-from-gear 打开时参与档位换算。
	oathTable *inventory.OathGradeTable
	// oathFromGear 打开旧规则（按穿戴装备算档位）—— **诊断用，默认关**。
	// 它有个已知硬伤：客户端脱不下誓约槽，所以穿上 primeval 就永久 oath=45。
	oathFromGear bool
	// oathProgressClears 是保底阈值：oathProgressDungeons 里的副本通关这么多场后，
	// 下一场下发 oath=45（必出一次隐藏 BOSS），并在通关时归零；<=0 = 关闭保底。
	oathProgressClears int
	// oathProgressDungeons 是计入保底的副本集合（默认只有小深渊 100005014）。
	oathProgressDungeons map[uint32]bool
	// omenInfo 是**显式注入**的 noti 2836 载荷（征兆队伍状态，69 字节）；空 = 按
	// 角色存档里的真实档数生成（-omen-state）。它是征兆 UI 的唯一数据源，
	// 几何与语义见 omen_info.go。
	omenInfo []byte
	// oathInject 是本轮要注入给客户端的候选通知（诊断用，默认空），见 oath_probe.go。
	oathInject []oathInjectSpec
	// oathNext 是注入队列的游标：每进一次副本推进一格，见 oathInjectNext。
	oathNext int
	// omenHold 是诊断入口：把玩家直接放到指定征兆阶段，省掉刷场次（-1 = 不动）。
	// 它只在会话里生效一次，之后仍按通关正常累积/结算；-omen-state 打开时会写回存档。
	omenHold        int
	omenHoldApplied bool
	// omenState 打开「征兆 = 角色存档级状态」这条线（-omen-state，见 omen_state.go）：
	// 进本读存档、结算写回、noti 2836 按真实档数下发，并让隐藏 BOSS 由「满档结算」
	// 驱动，而不是按通关场次。关闭时征兆只活在内存账本里。
	omenState bool
	// omenHeldRun 是本场开始时的征兆持有档数（进本时从存档读出，见 loadOmenRunState）。
	omenHeldRun uint32
	// omenHeldReady 表示本场已经读过征兆存档（读失败会直接上抛，所以为真即可信）。
	omenHeldReady bool
	// omenOrthaierDue 表示这一场会下发 oath=45（召唤隐藏 BOSS）。它由存档里的
	// orthaire_pending 得出，noti 2838 与 noti 2836 共用这一份判断。
	omenOrthaierDue bool
	// oathTierRun 是本场**实际下发**的天平 oath 档位（40..45），由 oathInfoPackets 算完后写入。
	// omenInfoPackets 用它把「天平颜色」映射成星蕴石档位（见 omen_info.go 的
	// omenGradeForOathTier）—— 这是**我们一起补的映射**：源里星蕴石品质只由
	// noti 2836 的 grade 决定，而 grade 原本只是征兆持有档数，与天平档位无关
	// （2026-10-01 实机验证：固定 oath=45 仍掉 Unique 档箱子）。
	// 0 = 本场还没算过，omenInfoPackets 会回落到 grade 1。
	oathTierRun uint16
	// omenReported 是本会话已经记过事件的征兆结算序号（见 noteOmenClear）。
	omenReported uint64
	// scaleRun 是上面两张表所归属的副本运行号。同一会话里重进副本会把 entity 从
	// 0x1000 重新发一遍，只按 entity 去重会让第二场之后再也不可能判死（实战踩过），
	// 所以换 RunID 时必须整表清空。
	scaleRun         string
	activeDungeon    *dungeon.Session
	selectingDungeon bool
	completionSent   bool
	completionErr    error
	// [MERGE-20260928-DIAG] 最近一次场景换图的决策路径，由 dispatch 落进 events。
	sceneDiag          string
	sceneDiagFrom      uint32
	sceneDiagTo        uint32
	resultSent         bool
	cardPlan           *loot.CardPlan
	cardScrolled       bool
	cardLayoutSent     bool
	cardAutoPickAt     time.Time
	cardReceipt        *loot.CardReceipt
	answeredQuests     map[uint16]bool
	communicationQuest uint16
	communicationNPC   uint32
	communicationTown  uint32
	communicationArea  uint32
	communicationUntil time.Time
	soloPartyBootstrap bool
	soloPartyReady     bool
	specialWarpPending bool
	// hub and peer publish this actor to the other connected players. Both stay
	// nil when the gateway runs without a multiplayer hub.
	hub  *lanHub
	peer *lanPeer
	// lastMotion and lastSpeed are the most recent values this client reported with
	// CMD 35. NOTI 22 carries them so the other clients animate the movement.
	lastMotion byte
	lastSpeed  uint16
	// joinedPeers caches the peers the last enterArea introduced, so their actor
	// info can be sent after this connection own entry packets. peerPoses holds
	// their NOTI 22 bodies in the same order, so index i always matches
	// joinedPeers[i].
	joinedPeers []*lanPeer

	// poseRefreshAfter counts down the position reports still to come before the
	// peers of a freshly entered scene are re-announced with the pose they are
	// actually in. A CMD 35 only ever arrives once the client has finished
	// loading the scene - it comes after the whole entry command burst, or after
	// a CMD 36 area change - so the first one is already a safe moment. It is not
	// sent during the entry sequence itself: doing that crashes the client.
	poseRefreshAfter    int
	adventureReady      bool
	seasonLevelSnapshot [32]byte
	seasonOathSnapshot  [32]byte
}

func (w *worldSession) enter(role storage.Character, spawn storage.WorldPosition) error {
	var state character.State
	if e := json.Unmarshal(role.State, &state); e != nil {
		return e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// The gate mode is the character's own creation marker minus the
	// graduation mark, not the launcher's DFO_ODYSSEY_MODE override: the
	// client applies its per-character flag too, and a graduated character
	// must pass the regular level gates.
	odyssey := character.OdysseyMember(role)
	saved, e := w.service.Enter(ctx, w.account, role.ID, state.Level, odyssey, spawn)
	if e != nil {
		return e
	}
	// 旧入口候选曾把矿区位置写入普通城镇存档；该位置在普通频道
	// 不具备创建资格。仅修复这类旧位置，使用现有配置的默认落点。
	if saved.Position.Town == 218 {
		if spawn.Town == 218 {
			return fmt.Errorf("普通频道默认落点不能使用赤红铁矿区域")
		}
		saved, e = w.store.SaveWorld(ctx, w.account, role.ID, saved, spawn)
		if e != nil {
			return e
		}
	}
	// 特殊征讨频道（SemiRaid/Legion）的专属城镇曾经由会话位置保存写进普通频道
	// 共享行（月湖 215 / Azure 213 / 军团 239）：**普通频道**恢复到该位置会被客户端
	// 以「对立阵营起始点」拒绝。修回默认落点，而不是拒绝进入。
	// ⚠️ 只修普通频道（!channelWorldIsolated）：特殊频道自己恢复专属城镇位置是
	// 合法的（[102] 行的 213/2 就是 Azure 门口），不能误修 —— 2026-10-03 实测
	// 无条件修复会把 Azure 频道的落点改回普通世界 Elvenguard。
	if !w.channelWorldIsolated && w.specialTowns[saved.Position.Town] {
		if w.specialTowns[spawn.Town] {
			return fmt.Errorf("普通频道默认落点不能使用特殊征讨频道城镇 %d", spawn.Town)
		}
		saved, e = w.store.SaveWorld(ctx, w.account, role.ID, saved, spawn)
		if e != nil {
			return e
		}
	}
	w.role, w.level, w.state, w.odyssey = role, state.Level, saved, odyssey
	w.blackPurgatory = blackPurgatoryState{}
	if w.channelType == 73 {
		// blackpurgatory.etc的85/1招募大厅连回原版85/0房间。
		// 坐标来自black_purgatory_gate.map的[gate]，不写普通城镇存档。
		entry := blackPurgatoryEntry()
		if e := w.service.ValidatePosition(w.level, w.odyssey, entry); e != nil {
			return fmt.Errorf("黑鸦频道落点无效：%w", e)
		}
		w.state.Position = entry
		if _, _, e := w.blackPurgatoryQuota(ctx, "", "recover", time.Now()); e != nil {
			return fmt.Errorf("恢复黑鸦入场次数：%w", e)
		}
	}
	w.bleedingMineCreated, w.bleedingMineReady = false, false
	w.bleedingMineRoster = nil
	w.bleedingMineStart = nil
	if w.channelType == 106 {
		// 当前 clientchannelinfo.etc 指定赤红铁矿赛丽亚房间为218/0；
		// 坐标取 town/bleedingmine.twn 的[gate]，不复用剧情城镇。
		// 每次进入先回独立房间，不能恢复没有当前编队的副本准备区。
		entry := storage.WorldPosition{Town: 218, Area: 0, X: 562, Y: 234}
		if e := w.service.ValidatePosition(w.level, w.odyssey, entry); e != nil {
			return fmt.Errorf("赤红铁矿频道落点无效：%w", e)
		}
		w.state.Position = entry
	}
	w.moon = moonSoloState{}
	if w.moonConfig != nil {
		if e := w.service.ValidatePosition(w.level, w.odyssey, w.moonConfig.Entry); e != nil {
			return e
		}
		// Explicit contribution test-channel spawn only; never write a dungeon
		// coordinate into the ordinary world-position store.
		if w.state.Position.Town != 215 {
			w.state.Position = w.moonConfig.Entry
		}
	}
	w.lastFatigueDay = ""
	w.adventureSnapshot = [32]byte{}
	w.adventureEliteSnapshot = [32]byte{}
	w.adventureReady = false
	w.seasonLevelSnapshot = [32]byte{}
	w.seasonOathSnapshot = [32]byte{}
	w.activeDungeon = nil
	w.pilotDeath = nil
	w.soloPartyReady = false
	w.specialWarpPending = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.pendingTownArrival = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.resetCards()
	w.answeredQuests = nil
	w.drops = nil
	w.deathSent = nil
	return nil
}

// privateArea reports whether an area is a personal instance. The client shows
// a single actor inside Seria's room no matter how crowded the town is, so the
// same area never publishes other players. The flag comes from the imported
// source map's own [is seria room warp] marker.
func (w *worldSession) privateArea(pos storage.WorldPosition) bool {
	if w.service == nil || w.service.Catalog.Areas == nil {
		return false
	}
	area, ok := w.service.Catalog.Areas[catalog.AreaKey(pos.Town, pos.Area)]
	return ok && area.SeriaReturnWarp
}

// enterArea publishes this actor into its current area and returns the peers
// that share the scene. It must run before the area list is serialized.
//
// Moving also refreshes the area this actor came from: the client keeps drawing
// an actor it was never told to drop, so leaving a scene without pushing an
// updated list there leaves a ghost behind.
func (w *worldSession) enterArea() {
	if w.hub == nil || w.peer == nil || w.role.ID == 0 {
		w.joinedPeers = nil
		return
	}
	p := w.state.Position
	w.joinedPeers = w.hub.publish(w.peer, w.peer.channel, p.Town, p.Area, p.X, p.Y, w.privateArea(p))
	w.poseRefreshAfter = 1
}

// leaveScene unpublishes this actor while it is somewhere nobody else can see,
// such as inside a dungeon, and removes it from the town it left.
func (w *worldSession) leaveScene() {
	if w.hub == nil || w.peer == nil {
		return
	}
	w.hub.retire(w.peer)
}

// introducePeers introduces the players already standing in the scene to this
// client. It has to run before the area list that places them.
func (w *worldSession) introducePeers(send func(byte, uint16, []byte) error) error {
	if w.hub == nil || w.peer == nil || w.role.ID == 0 {
		return nil
	}
	for _, o := range w.joinedPeers {
		if e := send(0, 2, w.hub.basicInfo(o)); e != nil {
			return e
		}
		if len(o.addition) > 0 {
			if e := send(0, 2, o.addition); e != nil {
				return e
			}
		}
	}
	return nil
}

// announceSelf introduces this client to the players already in the scene and
// places it there, so they see it without waiting for it to move.
func (w *worldSession) announceSelf(event func(map[string]any)) error {
	if w.hub == nil || w.peer == nil || w.role.ID == 0 {
		return nil
	}
	p := w.state.Position
	others := w.joinedPeers
	// Introduce this client to them. The area list alone is not enough: the
	// client queues an actor it has no user info for and never draws it, so
	// skipping this step looks exactly like "only the newcomer is visible".
	//
	// The placement needs NOTI 23 as well. NOTI 24 only places actors while the
	// client is loading the scene, and a player already standing there has it
	// loaded, so a refreshed list does nothing for them: without the explicit
	// placement the newcomer stays invisible until it moves once and its own
	// position update arrives.
	placement, placementErr := w.userAreaPayload()
	for _, o := range others {
		if o.send == nil {
			continue
		}
		if e := o.send(0, 2, w.hub.basicInfo(w.peer)); e != nil {
			continue
		}
		if len(w.peer.addition) > 0 {
			o.send(0, 2, w.peer.addition)
		}
		if placementErr == nil {
			o.send(0, 23, placement)
		}
	}
	event(map[string]any{"kind": "area_presence_published", "character_id": w.role.ID, "actor": w.role.WireID, "town": p.Town, "area": p.Area, "private": w.peer.private, "other_players": len(others)})
	return nil
}

// broadcastMove tells the other actors in this scene where this one stands. It
// reuses NOTI 23, the placement notification the client already consumes for
// area changes, so no unrecovered position layout is needed.
func (w *worldSession) broadcastMove() {
	if w.hub == nil || w.peer == nil || w.role.ID == 0 {
		return
	}
	p := w.state.Position
	others := w.hub.move(w.peer, p.X, p.Y, w.lastMotion, w.lastSpeed)
	if len(others) == 0 {
		return
	}
	payload, e := protocol.UserPosition(w.role.WireID, p.X, p.Y, w.lastMotion, w.lastSpeed)
	if e != nil {
		return
	}
	for _, o := range others {
		if o.send == nil {
			continue
		}
		o.send(0, 22, payload)
	}
}

// departArea removes this actor for good - a disconnect, a return to the
// character list or a channel switch - so it does not keep standing there.
// notePositionReport counts a CMD 35 and, once enough have arrived after an
// area change, re-announces where everyone in the scene stands and which pose
// they are in. The area list only carries coordinates and the client rebuilds
// every actor from it, so without this a player returning from an instance
// sees the others standing in the default facing regardless of where they
// actually face.
func (w *worldSession) notePositionReport(event func(map[string]any)) {
	w.adventureReady = true
	if w.poseRefreshAfter <= 0 {
		return
	}
	w.poseRefreshAfter--
	if w.poseRefreshAfter > 0 {
		return
	}
	count := w.refreshPeerPoses()
	event(map[string]any{"kind": "peer_poses_refreshed", "peers": count})
}

func (w *worldSession) refreshPeerPoses() int {
	if w.hub == nil || w.peer == nil || w.role.ID == 0 || w.peer.send == nil {
		return 0
	}
	poses := w.hub.posesOf(w.hub.shared(w.peer))
	sent := 0
	for _, p := range poses {
		// The client applies a facing as part of moving an actor, and a NOTI 22 that
		// repeats the coordinate it already has reads as no movement at all - which is
		// exactly why the actor kept the default facing after a scene rebuild. Nudging
		// the actor a couple of pixels and putting it back makes the client walk it
		// into place, so it ends up facing the recorded direction.
		nudge := p
		if nudge.x > 2 {
			nudge.x -= 2
		}
		if step, e := protocol.UserPosition(nudge.actor, nudge.x, nudge.y, nudge.motion, nudge.speed); e == nil {
			w.peer.send(0, 22, step)
		}
		if back, e := protocol.UserPosition(p.actor, p.x, p.y, p.motion, p.speed); e == nil {
			if e := w.peer.send(0, 22, back); e == nil {
				sent++
			}
		}
	}
	return sent
}

func (w *worldSession) departArea() {
	if w.hub == nil || w.peer == nil {
		return
	}
	w.hub.depart(w.peer)
}

// prevVillage decodes CMD 1418 ENUM_CMDPACKET_PREV_VILLAGE into the area-change
// request it stands for. The body is empty by construction (live 2026-09-23:
// 13-byte bare headers, 39 in a row), so nothing is parsed out of it and no
// field layout is guessed; the destination is the Return stamp this character
// picked up on the way into the room. A character with no stamp is refused
// instead of being sent somewhere plausible.
func (w *worldSession) prevVillage(p []byte) (protocol.AreaChangeRequest, error) {
	return previousVillageRequest(w.state.Position, p, w.activeDungeon != nil, w.selectingDungeon)
}

func (w *worldSession) areaPayload() ([]byte, error) {
	p := w.state.Position
	if w.hub == nil || w.peer == nil {
		return protocol.AreaUsers(p.Town, p.Area, []protocol.AreaUser{{ActorServerID: w.role.WireID, X: p.X, Y: p.Y, Flags: w.flags}})
	}
	return areaUsersPayload(p.Town, p.Area, w.hub.roster(w.peer))
}
func (w *worldSession) userAreaPayload() ([]byte, error) {
	p := w.state.Position
	return protocol.UserArea(p.Town, p.Area, protocol.AreaUser{ActorServerID: w.role.WireID, X: p.X, Y: p.Y, Flags: w.flags})
}

func previousVillageRequest(old storage.WorldPosition, body []byte, inDungeon, selectingDungeon bool) (protocol.AreaChangeRequest, error) {
	if len(body) != 0 {
		return protocol.AreaChangeRequest{}, errors.New("prev village needs empty body")
	}
	if inDungeon || selectingDungeon {
		return protocol.AreaChangeRequest{}, errors.New("prev village requires town character")
	}
	if old.Return == nil {
		return protocol.AreaChangeRequest{}, errors.New("no previous village")
	}
	ret := old.Return
	return protocol.AreaChangeRequest{Town: ret.Town, Area: ret.Area, X: ret.X, Y: ret.Y,
		PreviousTown: old.Town, PreviousArea: uint16(old.Area)}, nil
}

func (w *worldSession) handle(id uint16, p []byte, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w.role.ID == 0 {
		return errors.New("world request before character selection")
	}
	old := w.state
	next := old.Position
	switch id {
	case 35:
		r, e := protocol.DecodePositionRequest(p)
		if e != nil {
			return e
		}
		w.lastMotion, w.lastSpeed = r.Motion, r.Speed
		w.notePositionReport(event)
		next.X, next.Y = r.X, r.Y
		// A movement report only stays inside the area the character is already
		// in, so it uses the restoration gate rather than the entry gate.
		if e = w.service.ValidateRestoredPosition(w.level, w.odyssey, next); e != nil {
			return e
		}
	case 36:
		r, e := protocol.DecodeAreaChangeRequest(p)
		if e != nil {
			w.specialWarpPending = false
			return e
		}
		if e = w.validateBleedingMineArea(r); e == nil {
			e = w.validateBlackPurgatoryArea(r)
		}
		if e == nil {
			next, e = w.areaTransition(r)
		}
		if e != nil {
			event(map[string]any{"kind": "area_refused", "town": r.Town, "area": r.Area, "reason": e.Error()})
			// Code 8 is the native level refusal; code 4 reaches the generic refusal
			// and clears the client's in-flight transition at 145297a9b.
			code := uint16(4)
			if errors.Is(e, world.ErrLevel) {
				code = 8
			}
			refusal, err := protocol.AreaChangeFailure(code, r.Town, r.Area)
			if err != nil {
				return err
			}
			return send(1, 36, refusal)
		}
	case 1418:
		r, e := previousVillageRequest(old.Position, p, w.activeDungeon != nil, w.selectingDungeon)
		if e != nil {
			event(map[string]any{"kind": "prev_village_refused", "reason": e.Error()})
			// No destination to name, so refuse at the area the character is
			// already standing in; that clears the client's in-flight transition.
			refusal, err := protocol.AreaChangeFailure(4, old.Position.Town, old.Position.Area)
			if err != nil {
				return err
			}
			return send(1, 1418, refusal)
		}
		if e = w.validateBlackPurgatoryArea(r); e == nil {
			next, e = w.areaTransition(r)
		}
		if e != nil {
			event(map[string]any{"kind": "prev_village_refused", "town": r.Town, "area": r.Area, "reason": e.Error()})
			refusal, err := protocol.AreaChangeFailure(4, r.Town, r.Area)
			if err != nil {
				return err
			}
			return send(1, 1418, refusal)
		}
	default:
		return errors.New("unknown world request")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if w.channelType == 106 && next.Town == 218 {
		// 矿区位置属于当前频道会话，不能覆盖普通频道的城镇落点。
		w.state.Position = next
		event(map[string]any{"kind": "赤红铁矿会话位置更新", "character_id": w.role.ID, "position": next, "request": id})
	} else if w.channelType == 73 && next.Town == 85 {
		w.state.Position = next
		event(map[string]any{"kind": "黑鸦会话位置更新", "character_id": w.role.ID, "position": next, "request": id})
	} else if w.channelWorldIsolated {
		// 特殊征讨频道（SemiRaid/Legion）会话内位置只留在会话里，不覆盖普通频道
		// 共享行 —— 黑鸦 73 / 矿区 106 既有模式推广到月湖 101 / Azure 102 / 军团 119。
		w.state.Position = next
		event(map[string]any{"kind": "isolated_channel_position_update", "character_id": w.role.ID, "position": next, "channel_type": w.channelType, "request": id})
	} else {
		saved, e := w.store.SaveWorld(ctx, w.account, w.role.ID, old, next)
		if e != nil {
			return e
		}
		w.state = saved
		event(map[string]any{"kind": "world_position_saved", "character_id": w.role.ID, "position": next, "revision": saved.Revision, "request": id})
	}
	if id == 36 || id == 1418 {
		// The acknowledgement echoes the request's own opcode: the client is
		// waiting on the command it sent, and CMD 1418 replays the CMD 36
		// area-change frame sequence otherwise unchanged.
		if e := send(1, id, protocol.AreaChangeSuccess()); e != nil {
			return e
		}
		// NOTI23 is a distinct transition stage: its self branch invokes
		// 146d12cd0 to refresh area-specific warp state and 144f400a0 for
		// area objectives. NOTI24 alone only populated the destination scene.
		userArea, e := w.userAreaPayload()
		if e != nil {
			return e
		}
		if e = send(0, 23, userArea); e != nil {
			return e
		}
		// Republish before serializing the area list so it already carries this
		// actor at its destination, together with everyone else standing there.
		// The same call tells the area just left to drop this actor.
		w.enterArea()
		// Introduce the players already here before the list that places them.
		if e = w.introducePeers(send); e != nil {
			return e
		}
		payload, e := w.areaPayload()
		if e != nil {
			return e
		}
		if e = send(0, 24, payload); e != nil {
			return e
		}
		if e = w.announceSelf(event); e != nil {
			return e
		}
		a := w.service.Catalog.Areas[catalog.AreaKey(next.Town, next.Area)]
		event(map[string]any{"kind": "area_change_sent", "town": next.Town, "area": next.Area, "map": a.Map.Path, "sha256": a.Map.SHA256, "client_acceptance": "pending"})
	} else {
		w.broadcastMove()
	}
	if id == 35 {
		if e := w.syncBleedingMinePreparation(send, event); e != nil {
			return e
		}
	}
	return w.settleProximityObjectives(ctx, send, event)
}

// Objectives decided by where the character stands settle after the move or
// transition is acknowledged: reaching a source rectangle, and standing at the
// NPC a quest names. Without this a [meet npc] or [reach the range] step stays
// pending forever and its whole chain halts, which reads in game as "the quest
// will not complete" and then "there are no further quests".
func (w *worldSession) settleProximityObjectives(ctx context.Context, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w.quests == nil || w.role.ID == 0 {
		return nil
	}
	at := w.state.Position
	advanced, e := w.quests.ProximityProgress(ctx, w.role, quest.Position{Town: at.Town, Area: at.Area, X: at.X, Y: at.Y}, func(npc uint32) ([2]uint16, bool) {
		return w.service.NPCPosition(w.state.Position, npc)
	}, func(npc uint32) ([2]uint16, bool) {
		return w.service.PhaseNPCPosition(w.state.Position, npc)
	})
	if e != nil {
		event(map[string]any{"kind": "quest_proximity_error", "character_id": w.role.ID, "error": e.Error()})
		return nil
	}
	if len(advanced) == 0 {
		return nil
	}
	active, e := w.quests.Active(ctx, w.role)
	if e != nil {
		return e
	}
	body, e := protocol.QuestTriggers(active)
	if e != nil {
		return e
	}
	if e = send(0, 291, body); e != nil {
		return e
	}
	event(map[string]any{"kind": "quest_proximity_advanced", "character_id": w.role.ID, "quests": advanced, "position": w.state.Position})
	return nil
}
