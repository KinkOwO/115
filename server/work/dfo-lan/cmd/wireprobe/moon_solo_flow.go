package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"
)

// Opt-in contribution profile. No change to the ordinary upstream server.
// Pool/weights are operator policy, not claimed official smart-drop rates.
type moonSoloConfig struct {
	ClientProfile string                 `json:"client_profile"`
	Channel       uint32                 `json:"channel"`
	Remaining     byte                   `json:"test_remaining"`
	Rewards       loot.MoonRewardPolicy  `json:"rewards"`
	// SourceRewards 是**源声明驱动**的翻牌策略（唯一权威来源，见
	// internal/loot/moon_source_rewards.go）：装配时从副本的 [difficulty dropitem group list]
	// 直读，见 cmd/wireprobe/moon_source_config.go。`Rewards` 只为**旧的 JSON 配置档**保留，
	// 运行时不再从它出发。
	SourceRewards loot.MoonSourcePolicy `json:"-"`
	Entry         database.WorldPosition `json:"entry"`
	// RewardPool 是翻牌池的**直读来源描述**（组号与模板@权重），只用于启动日志：
	// 池的成员与权重全部从源推导，不是配置项，也没有对应的 JSON 档。
	RewardPool string `json:"reward_pool,omitempty"`
}
type moonSoloState struct {
	created   bool
	prepared  time.Time
	owner     *dungeon.MoonSoloOwner
	resultAt  time.Time
	plan      *loot.MoonRewardPlan
	claimed   bool
	readySent bool
	recovered bool
	// autoPickAt 是「翻牌布局已发出、玩家一直没选」的兜底计时：到点替他选第一张免费牌。
	// 与通用副本的 worldSession.cardAutoPickAt 同义，但**不能复用那一条** —— 那一条走
	// w.cardPlan + grantFreeCard，沉月湖走的是 w.moon.plan + moonClaim。
	autoPickAt time.Time
}

// defaultMoonSoloConfig 从**直读**推导沉月湖单人配置，取代原先"必须手工提供 JSON 配置档"
// 的做法（业主 2026-10-02：直读模式下不新增 JSON，能直读的一律直读）。
//
// 各项来源：
//
//	Channel   ← 频道目录里 Type = 101 的那一行（ID 即发布用频道号）
//	Entry     ← 该频道的 [seriaRoomTown] 城镇：215/1（副本门口）+ 地图可行走矩形中心
//	Draws     ← 第二层副本 100004137 的 [reward card]（源声明的翻牌张数）
//	Choices   ← 固定产出：门票 ×60、银币 ×15（模板取自第二层声明的门票/硬币组 21291）
//	Equipment ← 1..3 件装备，每件独立走源的掉落/品质判定表（Level 取副本要求的 115）
//	Source    ← 运行时 SaveIdentity()
//
// Remaining 是源里确实没有的一项：源不给"测试次数"这种东西（moonlake.cos 只有
// [weekly max dungeon incount]/[weekly max reward count] 这类周账），所以由代码常量给出。
func defaultMoonSoloConfig(
	dir *catalog.ChannelDirectory,
	towns map[uint32]catalog.TownArea,
	dungeons *catalog.DungeonCatalog,
	lootService *loot.Service,
	booster *catalog.BoosterCatalog,
) (*moonSoloConfig, error) {
	if dir == nil || dungeons == nil || lootService == nil {
		return nil, fmt.Errorf("Moon 需要直读的频道目录、副本目录与掉落服务")
	}
	// 沉月湖频道：源里它的 [channelType] 就是 101。
	const moonChannelType = 101
	attrs, ok := dir.Attributes(moonChannelType)
	if !ok {
		return nil, fmt.Errorf("源 clientchannelinfo.etc 里没有频道类型 %d", moonChannelType)
	}
	town, ok := towns[moonChannelType]
	if !ok {
		return nil, fmt.Errorf("频道 %d 没有可用的专属城镇（需要 [seriaRoomTown] 与可行走地图）", moonChannelType)
	}
	// 第二层是通关结算所在层：翻牌张数从它的 [reward card] 取。
	second, ok := dungeons.Dungeons[100004137]
	if !ok {
		return nil, fmt.Errorf("源里没有月湖第二层副本 100004137")
	}
	// 一层要提到外层作用域：装备的**四个品级组**是它声明的（见 moon_source_config.go 的说明）。
	first, ok := dungeons.Dungeons[100004136]
	if !ok {
		return nil, fmt.Errorf("源里没有月湖第一层副本 100004136")
	}
	if first.BasisLevel == 0 {
		return nil, fmt.Errorf("月湖第一层 100004136 没有 [basis level]")
	}
	draws := second.RewardCard
	if draws == 0 || draws > 16 {
		return nil, fmt.Errorf("月湖第二层 [reward card] = %d，超出可用范围", draws)
	}
	// 产出模型**直读源**：块选择、固定产物、装备池、誓约池全部从 .dgn 的声明装配
	// （D1 只换产出模型：领取事务 / 入库 / 恢复路径一个字节都不动）。
	family := conquestFamily(dungeons, conquestFamilyDungeons)
	if len(family) == 0 {
		return nil, fmt.Errorf("征讨族里一个副本都不在目录里")
	}
	source, pool, err := conquestFlipPolicy(lootService, family, second, booster)
	if err != nil {
		return nil, err
	}
	x, y := town.Spawn()
	cfg := &moonSoloConfig{
		Channel:   moonChannelType,
		Remaining: moonSoloTestRemaining,
		Entry: database.WorldPosition{
			Town: uint32(town.TownID),
			Area: uint32(town.AreaID),
			X:    x,
			Y:    y,
		},
		SourceRewards: source,
		RewardPool:    pool,
	}
	_ = attrs
	return cfg, nil
}

// 月湖翻牌的**固定产出**：模板取自源里第二层声明的门票/硬币组（21291），数量是运营值 ——
// 源里没有"这次给几张"这种表（玩家 2026-10-02 的实测产出：深渊门票 60 张、银币 14~15 个）。
var moonFixedRewards = map[uint32]uint32{
	10362429: 60, // Doom Oracle（深渊入场：终末之启示）
	10362432: 15, // Merchant Guild Silver Coin（迷雾工商协会银币）
}

// 翻牌里随机装备的件数与掷骰档位。品质不在这里指定：每件都走源的掉落/品质判定表
// （与深渊同一套率），件数是服务端口径（源里同样没有这张表）。
//
// 件数区间 1..4 对应牌面上的 6 个格子（玩家 2026-10-02 核实：装备位最多出到 4 件）；
// 权重 {1,3,3,2} 把数学期望压在 2.67 件 —— 等概率 1..4 会让"只出一件"太常见。
const (
	moonSoloEquipmentMin  = 1
	moonSoloEquipmentMax  = 4
	moonSoloEquipmentRank = 3 // boss 档，装备率最高的那一档
)

var moonSoloEquipmentWeights = []uint32{1, 3, 3, 2}

// moonRewardChoicesFromDungeon 从月湖第二层副本**自己声明**的掉落组里取出翻牌的固定产出。
//
// 为什么还要读副本的组：门票与银币的模板不是本仓编的，它们写在源里第二层声明的
// `[difficulty dropitem group list]`（组 21291）里；本函数只把"哪些模板算固定产出"
// 与"给几张"这两件源里没有的事，限制在**已声明的模板**上 —— 声明的组里没有的模板
// 一律不发（宁可不发，也不从猜测里发）。
//
// 装备那一段不在这里：它由 loot.MoonEquipmentPolicy 描述（件数区间 + 等级 + 怪物档位），
// 每件独立走源的掉落/品质表，所以品质分布与深渊一致。
func moonRewardChoicesFromDungeon(second catalog.DungeonDefinition, svc *loot.Service) ([]loot.MoonRewardChoice, string, error) {
	if svc == nil {
		return nil, "", fmt.Errorf("Moon 翻牌池需要掉落服务")
	}
	groups, ok, e := loot.DungeonGroupIndices(second, 0)
	if e != nil {
		return nil, "", fmt.Errorf("月湖第二层 %d 的 [normal group index]: %w", second.ID, e)
	}
	if !ok || len(groups) == 0 {
		return nil, "", fmt.Errorf("月湖第二层 %d 没有声明 [normal group index]，固定产出无法直读", second.ID)
	}
	seen := map[uint32]bool{}
	var fixed []loot.MoonRewardChoice
	var taken []string
	for _, gid := range groups {
		group, found := svc.Catalog.DropGroupByID(gid)
		if !found {
			continue
		}
		rows := make([]catalog.DropWeight, 0, len(group.Explicit)+len(group.Smart))
		rows = append(rows, group.Explicit...)
		rows = append(rows, group.Smart...)
		before := len(fixed)
		for _, row := range rows {
			count, wanted := moonFixedRewards[row.Template]
			if !wanted || row.Template == 0 || seen[row.Template] {
				continue
			}
			// 能不能结算由与发奖同源的判据说了算：模板声明了但背包类别不明就不发。
			if _, settleable := svc.MoonSettleableStack(row.Template); !settleable {
				continue
			}
			seen[row.Template] = true
			fixed = append(fixed, loot.MoonRewardChoice{Template: row.Template, Count: count, Group: gid})
		}
		if len(fixed) > before {
			taken = append(taken, fmt.Sprintf("%d(+%d)", gid, len(fixed)-before))
		}
	}
	if len(fixed) == 0 {
		return nil, "", fmt.Errorf("月湖第二层声明的掉落组 %v 里没有登记的门票/硬币模板", groups)
	}
	names := make([]string, 0, len(fixed))
	for _, c := range fixed {
		names = append(names, fmt.Sprintf("%d x%d", c.Template, c.Count))
	}
	pool := fmt.Sprintf("groups=%s fixed[%s] equipment=%d..%d level=%d rank=%d",
		strings.Join(taken, ","), strings.Join(names, ","),
		moonSoloEquipmentMin, moonSoloEquipmentMax, second.MinimumLevel, moonSoloEquipmentRank)
	return fixed, pool, nil
}

// moonSoloTestRemaining 是月湖单人的**测试用剩余次数**。源里没有"周账本/剩余次数"这种东西
// （见 交付说明：这是显式测试规则，不是可消耗的官服周账），所以由代码常量给出。
const moonSoloTestRemaining byte = 255


// moonContext 返回本连接的频道上下文（2 字节）。
//
// ⚠️ 不要写死首字节：频道的身份是 `{ServerID, channelID}`（见 channelIdentity），
// 服务端按连接设进 characters.ChannelContext，EntryBasicProbe/EntryAddition 也会用
// 同一个值。moon 名册若用 `{0, channel}` 就会和随后的 USERINFO（`{ServerID, channel}`）
// 冲突 —— 2026-10-02 实测：客户端收到两个不同的队伍上下文，队伍不建出来。
func (w *worldSession) moonContext() [2]byte {
	if w.characters != nil && w.characters.ChannelContext != [2]byte{} {
		return w.characters.ChannelContext
	}
	return [2]byte{0, byte(w.moonConfig.Channel)} // 未启用频道身份时的回退
}
func (w *worldSession) moonInfo(phase uint32, success bool) outboundPacket {
	at := w.moon.prepared
	if w.moon.owner != nil {
		at = w.moon.owner.Session().StartedAt
	}
	p := protocol.MoonLakeBootstrap115(phase, at)
	if phase != 14 && (w.moon.owner != nil || phase == 1 && w.dungeons != nil) {
		var x dungeon.MoonProgress
		if w.moon.owner != nil {
			x = w.moon.owner.Session().MoonProgress()
		} else {
			x, _ = dungeon.MoonInitialProgress(*w.dungeons)
		}
		if phase >= 3 && phase <= 5 {
			x.Floor = 2
		}
		p = protocol.MoonLakeBootstrap115(phase, at, x.Floor)
		if x.GridReady {
			p, _ = protocol.MoonFirstCounts115(p, x.FirstGrid)
			p, _ = protocol.MoonLakeGrid115(p, x.Grid, x.ZermioGrid, x.ZermioDefeated)
		}
		var rows []protocol.MoonNamedRecord115
		for i, n := range x.Named {
			if x.Present[i] {
				rows = append(rows, protocol.MoonNamedRecord115{Slot: byte(i), Grid: n.Spawn.Grid, Dead: n.Dead, ResultIndex: n.ResultIndex})
			}
		}
		p, _ = protocol.MoonLakeNamedInfo115(p, rows)
		p, _ = protocol.MoonLakeGauges115(p, x.Troop, x.Fever)
		p, _ = protocol.MoonLakeCleared115(p, x.Cleared)
		p, _ = protocol.MoonZermioState115(p, x.ZermioHealth, x.ZermioMeter)
	}
	if phase >= 3 && phase <= 5 {
		p, _ = protocol.MoonLakeOutcome115(p, success)
	}
	if phase != 14 {
		bag, e := inventory.ReadBag(w.role.State)
		if e == nil {
			p[16] = byte(min(bag.Coin, 255))
		}
	}
	return outboundPacket{"moon_info", 0, 2622, p}
}
func (w *worldSession) moonRoster() ([]byte, error) {
	return protocol.MoonSoloParty115(w.role.WireID, w.moonContext(), w.moonConfig.Remaining)
}
func (w *worldSession) moonEnterPackets(floor bool) ([]outboundPacket, error) {
	s := w.moon.owner.Session()
	mapBody, e := w.moon.owner.StartMap()
	if e != nil {
		return nil, e
	}
	basic, e := w.characters.EntryBasicProbe(w.role, w.moonContext())
	if e != nil {
		return nil, e
	}
	more, e := w.characters.EntryAddition(w.role)
	if e != nil {
		return nil, e
	}
	roster, e := w.moonRoster()
	if e != nil {
		return nil, e
	}
	state, e := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if e != nil {
		return nil, e
	}
	// 穿戴窗口：普通副本入图会发（dungeon_flow.go 的 dungeon_worn_visuals_sent），
	// 月湖此前漏了它，客户端进本后装备栏是空的，穿脱一件才刷新。
	worn, e := inventory.WornSpaceUpdate(w.role.State)
	if e != nil {
		return nil, e
	}
	handoff := []outboundPacket{}
	if floor {
		// 第一层与第二层是**两个不同的副本**（100004136 / 100004137），换层不是
		// 同一副本里换房间：必须先把旧副本的区域交接走完（等候区 → 区域交接 →
		// 临时出场），再初始化新副本。少了这段，客户端过渡对象会一直保留旧副本
		// 场景的引用，通关回城后 NPC 点不动、对话键与右键移动失效。
		var err error
		if handoff, err = w.moonFloorHandoff(); err != nil {
			return nil, err
		}
	}
	return moonEntryPlan(floor, moonEntryParts{
		Handoff:  handoff,
		Basic:    basic,
		Addition: more,
		Worn:     worn,
		State:    state,
		Roster:   roster,
		Dungeon:  protocol.DungeonInfo(protocol.DungeonInfoState{ID: s.Definition.ID, Maze: s.Maze.Index, Boss: s.Maze.Boss}),
		Map:      mapBody,
		MoonInfo: w.moonInfo(2, false),
	}), nil
}

// moonEntryParts 是进场序列里已经构造好的各段包体。把「顺序」与「怎么造包」
// 分开，是为了能在不起整套世界/角色服务的情况下钉住换层的帧序
// （见 moon_solo_flow_test.go）。
type moonEntryParts struct {
	Handoff  []outboundPacket
	Basic    []byte
	Addition []byte
	// Worn 是穿戴窗口（N14）的载荷。普通副本入图是 2 → 2 → 14，月湖此前漏了这一帧，
	// 客户端不刷新装备栏：实机 2026-10-02 表现为「进本后身上装备不显示，穿脱一件装备才出现」。
	Worn     []byte
	State    []byte
	Roster   []byte
	Dungeon  []byte
	Map      []byte
	MoonInfo outboundPacket
}

// moonEntryPlan 按源顺序组装进场通知。
//
// floor=false（首次进场）与普通同层移动保持既有顺序不动。
//
// floor=true（第一层→第二层）在本人资料之前插入跨副本区域交接，并把 N27 移到
// 交接之后：N27 的接收路径会切换客户端的场景上下文，放在交接之前等于还在旧场景
// 里重建上下文。两个分支都只发一次 N27（同版官服换层样本的顺序：
// N2281 → N23(等候区) → N24 → N23(临时255) → 本人资料 → N27 → N28/地图信息）。
func moonEntryPlan(floor bool, p moonEntryParts) []outboundPacket {
	out := []outboundPacket{}
	// 「本人资料 + 穿戴窗口」与普通副本入图同序（N2 外观 → N2 附加 → N14 穿戴）：
	// 少了 N14 客户端不刷新装备栏（实机 2026-10-02：进本后身上装备不显示）。
	profile := []outboundPacket{
		{"moon_actor", 0, 2, p.Basic},
		{"moon_actor_details", 0, 2, p.Addition},
	}
	if len(p.Worn) > 0 {
		profile = append(profile, outboundPacket{"moon_worn", 0, 14, p.Worn})
	}
	if floor {
		out = append(out, outboundPacket{"moon_portal_notify", 0, 2281, protocol.MoonFloorDirectMoveNotice115()})
		out = append(out, p.Handoff...)
		out = append(out, profile...)
		out = append(out, outboundPacket{"moon_select", 0, 27, protocol.EnterDungeonSelection()}, outboundPacket{"moon_actor_state", 0, 3, p.State}, outboundPacket{"moon_roster", 0, 9, p.Roster})
	} else {
		out = append(out, profile...)
		out = append(out, outboundPacket{"moon_actor_state", 0, 3, p.State}, outboundPacket{"moon_roster", 0, 9, p.Roster}, outboundPacket{"moon_select", 0, 27, protocol.EnterDungeonSelection()})
	}
	out = append(out, outboundPacket{"moon_dungeon", 0, 28, p.Dungeon}, outboundPacket{"moon_map", 0, 29, p.Map}, p.MoonInfo)
	return out
}

// moonFloorHandoff 构造第一层→第二层之间那段「跨副本区域交接」。
//
// 三层职责：让客户端把旧副本的过渡对象放掉（N23 等候区）、把本人放进同一个
// 等候区的区域名单（N24）、再用一条「临时离场」表示把本人从该区域挪出去
// （N23 区域 255）。之后才轮到本人资料与 N27。
//
// 不变量：
//   - 全部用本连接、本角色、本局数据构造，不复制官服的实体 id / 坐标 / 中继地址；
//   - 等候区与临时出场都用**本人当前的合法世界坐标**（215/2）。区域 255 只出现在
//     这一帧线路上，不写 state.Position、不落库、不作为重登或城镇出口位置；
//   - 不改 run / 已完成阶段 / 分值 / 怪物编号空间 / 奖励防重身份；
//   - 不发奖、不扣材料、不刷新疲劳或复活额度；
//   - 不调用完整回城业务（moonReturn），只做纯构包。
func (w *worldSession) moonFloorHandoff() ([]outboundPacket, error) {
	pos := w.state.Position
	if pos.Town != 215 || pos.Area != 2 {
		return nil, fmt.Errorf("Moon floor handoff needs the character standing at the waiting area 215/2")
	}
	self := protocol.AreaUser{ActorServerID: w.role.WireID, X: pos.X, Y: pos.Y, Flags: w.flags}
	waiting, e := protocol.UserArea(pos.Town, pos.Area, self)
	if e != nil {
		return nil, e
	}
	users, e := protocol.AreaUsers(pos.Town, pos.Area, []protocol.AreaUser{self})
	if e != nil {
		return nil, e
	}
	offscreen, e := protocol.UserArea(pos.Town, 255, self)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{
		{"moon_handoff_waiting_area", 0, 23, waiting},
		{"moon_handoff_area_users", 0, 24, users},
		{"moon_handoff_offscreen", 0, 23, offscreen},
	}, nil
}
func (w *worldSession) moonTick(now time.Time) ([]outboundPacket, error) {
	if w.moonConfig == nil || w.role.ID == 0 {
		return nil, nil
	}
	if !w.moon.recovered {
		w.moon.recovered = true
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		saved, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).RecoverMoonRewards(ctx, w.role)
		cancel()
		if saved.ID != 0 {
			w.role = saved
		}
		if e != nil && !errors.Is(e, loot.ErrMoonBagFull) {
			return nil, e
		}
		body, err := w.loot.Bootstrap(workflow.LootRole(w.role))
		if err != nil {
			return nil, err
		}
		return []outboundPacket{{"moon_pending_rewards_restored", 0, 13, body}}, nil
	}
	if !w.moon.readySent {
		// N1726 是两张表的完整快照。第一张表客户端已经用来显示「可再领 N 次奖励」
		// （实机 2026-10-02：发 255 时右下角 System 就写 "You can receive Advanced Dungeon
		// rewards 255 more times."）；第二张表的业务含义在协议层尚未确立，而月湖源里有
		// [use exclusive semi raid eplp] 1 —— 清关面板 Continue? 上的 Retry 是否可点很可能
		// 读的就是这张表：那次实机里客户端**从未发出** C72 option 0/5，说明它自己判定
		// 「不能再开」，而不是被服务端拒绝。所以这里把第二张表也按同一额度下发，作为
		// 一次对照实验 —— 待实机验证，若 Retry 仍灰则要往客户端侧（IDA）继续查。
		rewarded := []protocol.SpecialRewardRemaining115{{Dungeon: 100004137, Remaining: w.moonConfig.Remaining}, {Dungeon: 100004134, Remaining: w.moonConfig.Remaining}}
		episode := []protocol.SpecialRewardRemaining115{{Dungeon: 100004137, Remaining: w.moonConfig.Remaining}}
		p, e := protocol.DungeonSpecialRewardInfo115(rewarded, episode)
		if e != nil {
			return nil, e
		}
		count, e := protocol.DungeonTestRemaining(100004137, uint32(w.moonConfig.Remaining))
		if e != nil {
			return nil, e
		}
		w.moon.readySent = true
		return []outboundPacket{{"moon_local_test_rewards", 0, 1726, p}, {"moon_local_test_entries", 0, 537, count}}, nil
	}
	if w.moon.owner == nil && !w.moon.prepared.IsZero() && !now.Before(w.moon.prepared.Add(3*time.Second)) {
		if w.state.Position.Town != 215 || w.state.Position.Area != 2 {
			w.moon.prepared = time.Time{}
			return []outboundPacket{w.moonInfo(14, false)}, nil
		}
		seed, e := randomSeed()
		if e != nil {
			return nil, e
		}
		r, e := dungeon.NewMoonSolo(*w.dungeons, w.level, w.role.WireID, seed, now)
		if e != nil {
			return nil, e
		}
		w.moon.owner = r
		w.activeDungeon = r.Session()
		w.leaveScene()
		w.resetCards()
		w.completionSent, w.resultSent = false, false
		p, e := w.moonEnterPackets(false)
		if e != nil {
			return nil, e
		}
		return p, nil
	}
	if w.moon.owner != nil && (w.moon.owner.LoadExpired(now) || !w.moon.owner.Session().Completed() && !now.Before(w.moon.prepared.Add(3*time.Second+time.Hour))) {
		return w.moonReturn(false)
	}
	if w.moon.plan != nil && !w.resultSent && !now.Before(w.moon.resultAt) {
		state := protocol.ConquestClearReward115{}
		state.Present[0] = true
		state.Rewards[0] = w.moon.plan.WireRows()
		body, e := protocol.ConquestClearRewardBody115(state)
		if e != nil {
			return nil, e
		}
		w.resultSent = true
		return []outboundPacket{{"moon_clear_reward", 0, 35, body}, w.moonInfo(4, true)}, nil
	}
	return nil, nil
}
func (w *worldSession) moonHandle(id uint16, p []byte, now time.Time, event func(map[string]any)) (bool, []outboundPacket, error) {
	if w.moonConfig == nil || w.role.ID == 0 {
		return false, nil, nil
	}
	fail := func(e error) (bool, []outboundPacket, error) { return true, nil, e }
	switch id {
	case 12:
		if w.activeDungeon != nil || !w.moon.prepared.IsZero() {
			return fail(fmt.Errorf("Moon party busy"))
		}
		if w.state.Position.Town != 215 {
			return false, nil, nil
		}
		if len(p) < 36 {
			return fail(fmt.Errorf("short Moon party options"))
		}
		nameBytes := int(binary.LittleEndian.Uint32(p[2:]))
		at := 6 + nameBytes
		if nameBytes < 0 || nameBytes > 255 || at+30 > len(p) || p[at+9] != 27 || p[at+29] != 0 {
			return fail(fmt.Errorf("not a supported Moon party create"))
		}
		w.moon.created = true
		roster, e := w.moonRoster()
		if e != nil {
			return fail(e)
		}
		basic, e := w.characters.EntryBasicProbe(w.role, w.moonContext())
		if e != nil {
			return fail(e)
		}
		detail, e := w.characters.EntryAddition(w.role)
		if e != nil {
			return fail(e)
		}
		// **帧序**：实测只有这个顺序客户端不崩（2026-10-02）——
		//   2(USERINFO) → 2(USERINFO) → 9(PARTY_INFO)
		// 把 9 提前到最前会让客户端直接闪退；原顺序虽然不闪退，但队伍没建出来。
		// 说明问题不在帧序，而在这些帧的**内容**（USERINFO 的上下文/形状，或 9 名册
		// 与客户端读序的匹配），下一步按 opcode 2 的 reader 逐字段核。
		return true, []outboundPacket{{"moon_party_actor", 0, 2, basic}, {"moon_party_details", 0, 2, detail}, {"moon_party_created", 0, 9, roster}}, nil
	case 15:
		// 等候区红门（源 moonlake.cos 的 `[party waiting area] 215 2 217 212`）：
		// 它才是月湖的官方入口，客户端对这里**不会**渲染选图卡片（月湖不在
		// legionsystem.cos 的内容表里），所以绝不能落进普通 [dungeon gate] 的成功
		// 分支 —— 那条会回「gate_ack + 空选图 N27」，客户端去开一个没有内容的
		// 选图面板，场景上下文切换失败，实机 2026-10-02 18:23:50 整屏黑屏后连接被强关。
		return w.moonPortalStart(now)
	case 2284:
		mode, e := protocol.DecodeSemiRaidStart115(p)
		if e != nil {
			return fail(e)
		}
		if mode != 24 {
			return fail(fmt.Errorf("Moon start mode %d is not the source content start", mode))
		}
		if e = w.moonStartPreflight(); e != nil {
			return fail(e)
		}
		if w.moon.prepared.IsZero() {
			w.moon.prepared = now
		}
		return true, []outboundPacket{w.moonInfo(1, false), {"moon_start_ack", 1, 2284, []byte{1}}}, nil
	case 13:
		if !w.moon.created {
			return false, nil, nil
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("unsupported Moon leave"))
			}
		}
		if w.moon.owner != nil {
			out, e := w.moonReturn(false)
			if e != nil {
				return fail(e)
			}
			w.moon.created = false
			return true, append(out, outboundPacket{"moon_party_gone", 0, 9, protocol.MoonSoloPartyGone115(w.moonContext())}), nil
		}
		w.moon = moonSoloState{readySent: true, recovered: true}
		return true, []outboundPacket{w.moonInfo(14, false), {"moon_party_gone", 0, 9, protocol.MoonSoloPartyGone115(w.moonContext())}}, nil
	}
	if id == 42 && w.moon.owner == nil && !w.moon.prepared.IsZero() {
		if len(p) != 0 {
			return fail(fmt.Errorf("Moon cancel body"))
		}
		w.moon.prepared = time.Time{}
		return true, []outboundPacket{{"moon_cancel_ack", 1, 42, []byte{1}}, w.moonInfo(14, false)}, nil
	}
	if w.moon.owner == nil {
		return false, nil, nil
	}
	r := w.moon.owner
	s := r.Session()
	switch id {
	case 16, 117:
		return fail(fmt.Errorf("Moon uses content start and real boss death, not ordinary selection/check"))
	case 37:
		if s.Loaded {
			return true, []outboundPacket{{"moon_loaded_repeat", 1, 37, []byte{1}}}, nil
		}
		out, e := w.finishDungeonLoading(p)
		if e != nil {
			return fail(e)
		}
		if e = r.Loaded(now); e != nil {
			return fail(e)
		}
		rows, e := r.MoonEnterRoom(r.Stamp(), w.role.WireID, now)
		if e != nil {
			return fail(e)
		}
		if len(rows) > 0 {
			body, e := protocol.UnassignedMonsterAdd115(rows)
			if e != nil {
				return fail(e)
			}
			out = append(out, outboundPacket{"moon_room_dynamic", 0, 2194, body})
		}
		return true, append(out, w.moonInfo(2, false)), nil
	case 39:
		death, e := protocol.DecodeMonsterDeath(p)
		if e != nil {
			return fail(e)
		}
		if s.MoonRetired[uint16(death.Entity)] {
			return true, []outboundPacket{{"moon_retired_ack", 1, 39, []byte{1}}}, nil
		}
		changed, e := s.ConfirmDeath(death.Entity, death.Killer, w.role.WireID)
		if e != nil {
			return fail(e)
		}
		if !changed && w.deathSent[uint16(death.Entity)] {
			return true, []outboundPacket{{"moon_death_repeat", 1, 39, []byte{1}}}, nil
		}
		if e = r.MoonScoreDeath(r.Stamp(), w.role.WireID, uint16(death.Entity)); e != nil {
			return fail(e)
		}
		named, e := r.MoonAfterDeath(r.Stamp(), w.role.WireID)
		if e != nil {
			return fail(e)
		}
		// Existing ordinary consequences are retained on floor1. Floor2's source
		// excludes normal drops and XP; never call an incompatible high-level roll.
		out := []outboundPacket{w.moonInfo(2, false)}
		if s.Definition.ID == 100004136 {
			// event 一路传下去：常态怪死亡里还会记 drop_roll_skipped / omen_clear。
			base, e := w.monsterDeath(p, event)
			if e != nil {
				return fail(e)
			}
			out = append(out, base...)
		} else {
			out = append(out, outboundPacket{"moon_death_ack", 1, 39, []byte{1}}, outboundPacket{"monster_death_confirmed", 0, 38, protocol.MonsterDeathConfirmed(uint16(death.Entity))})
		}
		if named != nil {
			body, e := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{named.Spawn})
			if e != nil {
				return fail(e)
			}
			out = append(out, outboundPacket{"moon_named", 0, 2194, body}, w.moonInfo(2, false))
		}
		if w.deathSent == nil {
			w.deathSent = map[uint16]bool{}
		}
		w.deathSent[uint16(death.Entity)] = true
		if s.Definition.ID == 100004137 && s.Completed() && !w.completionSent {
			w.completionSent = true
			out = append(out, w.moonInfo(3, true), outboundPacket{"dungeon_clear_enabled", 0, 31, protocol.DungeonClearEnabled()})
		}
		return true, out, nil
	case 45:
		request, e := protocol.DecodeDungeonRoomTransition(p)
		if e != nil {
			return fail(e)
		}
		seed, e := randomSeed()
		if e != nil {
			return fail(e)
		}
		if e = r.Move(*w.dungeons, request, seed, now); e != nil {
			return fail(e)
		}
		w.activeDungeon = r.Session()
		body, e := r.StartMap()
		if e != nil {
			return fail(e)
		}
		return true, []outboundPacket{{"moon_move_ack", 1, 45, []byte{1}}, {"moon_map", 0, 29, body}, w.moonInfo(2, false)}, nil
	case 2062:
		if e := protocol.DecodeMoonFloorRequest115(p); e != nil {
			return fail(e)
		}
		if !s.MoonPortalReady() {
			return fail(fmt.Errorf("Moon portal not open"))
		}
		// 旧副本必须在 AdvanceMoonFloor 之前取快照：它之后 activeDungeon 已经是
		// 第二层，拿更新后的值当「旧副本」会永远不成立。
		previous := uint32(0)
		if w.activeDungeon != nil {
			previous = w.activeDungeon.Definition.ID
		}
		if previous != 100004136 {
			return fail(fmt.Errorf("Moon floor handoff requires the first floor, have %d", previous))
		}
		seed, e := randomSeed()
		if e != nil {
			return fail(e)
		}
		if _, e = r.AdvanceMoonFloor(r.Stamp(), w.role.WireID, *w.dungeons, seed, now); e != nil {
			return fail(e)
		}
		w.activeDungeon = r.Session()
		if w.activeDungeon.Definition.ID != 100004137 {
			return fail(fmt.Errorf("Moon floor handoff did not reach the second floor: %d", w.activeDungeon.Definition.ID))
		}
		w.completionSent = false
		out, e := w.moonEnterPackets(true)
		return true, out, e
	case 2276:
		if e := protocol.DecodeMoonFever115(p); e != nil {
			return fail(e)
		}
		score, _, e := r.UseMoonFever(r.Stamp(), w.role.WireID, now)
		if e != nil {
			return fail(e)
		}
		body, e := protocol.MoonFeverReply115(score)
		if e != nil {
			return fail(e)
		}
		return true, []outboundPacket{{"moon_fever", 1, 2276, body}, w.moonInfo(2, false)}, nil
	case 2277:
		req, e := protocol.DecodeMoonZermioReport115(p)
		if e != nil {
			return fail(e)
		}
		_, _, e = r.ReportMoonZermio(r.Stamp(), w.role.WireID, req, now)
		if e != nil {
			return fail(e)
		}
		return true, []outboundPacket{{"moon_escape_ack", 1, 2277, []byte{1}}, w.moonInfo(2, false)}, nil
	case 46:
		req, e := protocol.DecodePlayResult(p)
		if e != nil {
			return fail(e)
		}
		if req.Actor != w.role.WireID || !s.Completed() || s.Definition.ID != 100004137 || !w.completionSent {
			return fail(fmt.Errorf("Moon result before owned final"))
		}
		if w.moon.plan == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			var plan loot.MoonRewardPlan
			var audit loot.MoonSourcePlan
			var e error
			if w.moonConfig != nil && w.moonConfig.SourceRewards.Source != "" {
				plan, audit, e = (&workflow.LootService{Store: w.store, Loot: w.loot}).FreezeMoonSourceReward(ctx, w.role, s, w.moonConfig.SourceRewards)
			} else {
				// 只有旧 JSON 配置档（已被 defaultMoonSoloConfig 取代）才走这条。
				plan, e = (&workflow.LootService{Store: w.store, Loot: w.loot}).FreezeMoonReward(ctx, w.role, s, w.moonConfig.Rewards)
			}
			cancel()
			if e != nil {
				return fail(e)
			}
			// 源驱动那一份把"源声明了什么 / 实发什么 / 跳过了什么"记进日志，便于实机对账。
			// D4：宁缺勿编 —— 发不出去的要写明原因，而不是静默丢掉。
			if audit.Summary != "" {
				log.Printf("月湖翻牌（源驱动）：%s 角色=%d 挑战=%s", audit.Summary, w.role.ID, s.RunID)
				for _, sk := range audit.Skipped {
					log.Printf("月湖翻牌跳过：%s / %s / %s", sk.What, sk.Reason, sk.Detail)
				}
			}
			w.moon.plan = &plan
			w.moon.resultAt = now.Add(3 * time.Second)
		}
		return true, nil, nil
	case 69, 70:
		if !w.resultSent || w.moon.plan == nil {
			return fail(fmt.Errorf("Moon cards before own result"))
		}
		if len(p) != 0 {
			return fail(fmt.Errorf("unexpected Moon card stage body"))
		}
		if id == 69 {
			w.cardScrolled = true
			return true, []outboundPacket{{"card_scroll_ack", 1, 69, []byte{1}}}, nil
		}
		if !w.cardScrolled {
			return fail(fmt.Errorf("Moon card layout before scroll"))
		}
		// 布局发出后给玩家 3 秒选牌；到点还没选就替他选第一张（见 autoPickMoonCard）。
		//
		// 沉月湖的翻牌走 dispatchSpecialContent -> moonHandle，**不经过**
		// dispatchDungeon 里那段「card_layout_ack 时置 cardAutoPickAt」的后处理，
		// 所以这里要自己置一次计时器；否则客户端停在「一张都没翻」。
		w.moon.autoPickAt = now.Add(3 * time.Second)
		w.cardLayoutSent = true
		return true, []outboundPacket{{"card_layout_ack", 1, 70, protocol.CardLayout()}}, nil
	case 71, 1426:
		index := byte(0)
		if id == 71 {
			req, e := protocol.DecodeCardSelection(p)
			if e != nil {
				return fail(e)
			}
			if req.Side != 0 {
				return fail(fmt.Errorf("Moon paid card not enabled"))
			}
			index = req.Index
		} else if len(p) != 0 {
			return fail(fmt.Errorf("Moon skip body"))
		}
		if !w.resultSent || !w.cardLayoutSent {
			return fail(fmt.Errorf("Moon card pick before layout"))
		}
		out, e := w.moonClaim(index)
		return true, out, e
	case 42, 72:
		if id == 42 && len(p) != 0 {
			return fail(fmt.Errorf("Moon giveup body"))
		}
		var req protocol.SettlementExit
		if id == 72 {
			var e error
			req, e = protocol.DecodeSettlementExit(p)
			if e != nil {
				return fail(e)
			}
			if req.State == 2 {
				return true, []outboundPacket{{"moon_exit_focus", 1, 72, protocol.SettlementExitSuccess(req)}}, nil
			}
			switch req.Option {
			case 0, protocol.SettlementExitSeamless:
				// 清关面板上的「重挑战」：option 0 是 dstr 479 "Restart the dungeon."，
				// option 5 是 EPLP 无缝续刷（客户端副本模块右缘那一下，见 protocol/cards.go
				// 的 SettlementExitSeamless 注释：gateway 只要别拒它）。两者都重开一局月湖单人，
				// 不必再回城退队重建。
				// 上一局的奖励先落地：重开不能把已经冻结的收据丢掉（满包按既有语义留待重登补发）。
				var claimed []outboundPacket
				if s.Completed() && s.Definition.ID == 100004137 && w.moon.plan != nil && !w.moon.claimed {
					var e error
					if claimed, e = w.moonClaim(0); e != nil && !errors.Is(e, loot.ErrMoonBagFull) {
						return fail(e)
					}
				}
				out, e := w.moonRechallenge(time.Now())
				if e != nil {
					return fail(e)
				}
				plan := append([]outboundPacket{{"settlement_exit_ack", 1, 72, protocol.SettlementExitSuccess(req)}}, claimed...)
				return true, append(plan, out...), nil
			case 1:
				// option 1 是"留在选图流程"。月湖没有选图卡片（它不在 legionsystem.cos
				// 的内容表里），所以这条只能拒绝；玩家要再开就点 NPC/走红门。
				return fail(fmt.Errorf("Moon has no dungeon-selection flow"))
			}
		}
		var out []outboundPacket
		success := s.Completed() && s.Definition.ID == 100004137
		if success && w.moon.plan == nil {
			return fail(fmt.Errorf("Moon clear receipt still preparing"))
		}
		if success && !w.moon.claimed {
			var e error
			out, e = w.moonClaim(0)
			if e != nil {
				// A durable full-bag reward stays recoverable; other errors are not ignored.
				if !errors.Is(e, loot.ErrMoonBagFull) {
					return fail(e)
				}
			}
		}
		route, e := w.moonReturn(success)
		if e != nil {
			return fail(e)
		}
		if id == 72 {
			route[0] = outboundPacket{"settlement_exit_ack", 1, 72, protocol.SettlementExitSuccess(req)}
		}
		return true, append(out, route...), nil
	}
	return false, nil, nil
}

// moonStartPreflight 是月湖入场的**共用前置校验**：NPC 点 Start（C2284）与等候区
// 红门（C15）必须走同一条路进来，否则两个入口迟早各自漂移 —— 一边放行、另一边
// 拒绝，玩家看到的就是「NPC 能进、红门黑屏」这种半好状态。
//
// 它只做与副作用无关的判断；写状态（moon.prepared）留给调用方，因为两个入口的
// 回执形状不同：C2284 回 start_ack，红门回 gate_ack。
func (w *worldSession) moonStartPreflight() error {
	if !w.moon.created || w.moon.owner != nil || w.state.Position.Town != 215 || w.state.Position.Area != 2 {
		return fmt.Errorf("Moon start requires own solo party at source waiting area")
	}
	if w.dungeons == nil || w.loot == nil {
		return fmt.Errorf("Moon resources unavailable")
	}
	if w.inTutorial || w.specialWarpPending {
		return fmt.Errorf("Moon character is busy")
	}
	if e := w.service.ValidatePosition(w.level, w.odyssey, w.state.Position); e != nil {
		return e
	}
	if _, e := dungeon.SelectMoon(*w.dungeons, protocol.DungeonSelection{ID: 100004136, Party: 65535}, w.level, nil, 0); e != nil {
		return e
	}
	if _, e := dungeon.MoonInitialProgress(*w.dungeons); e != nil {
		return e
	}
	// 源驱动策略是现在唯一在用的那份；只有走旧的 JSON 配置档时才退回老校验。
	// ⚠️ 2026-10-09 回归教训：策略从 Rewards 搬到 SourceRewards 时漏改了这一行，
	// 结果 `ValidateMoonRewards(空策略)` 必失败 ⇒ NPC/红门**一律进不去**。
	if w.moonConfig.SourceRewards.Source != "" {
		return w.loot.ValidateMoonSourcePolicy(w.moonConfig.SourceRewards)
	}
	return w.loot.ValidateMoonRewards(w.moonConfig.Rewards)
}

// moonPortalStart 处理等候区红门的 C15（业主 2026-10-02 的口径：未满足条件先给提示，
// 满足条件时红门本身就是入口）。
//
// 分支语义：
//   - 不在等候区 215/2 → handled=false，原样交给普通 [dungeon gate] 路径。月湖频道里
//     别的门、以及所有普通城镇的门都不能被这条分支吃掉；
//   - 在等候区但前置校验没过（未建队 / 已经在副本里 / 教程中 / 资源或翻牌策略不可用）
//     → 返回 error。上层 main.go 对 moonHandle 的错误统一用 moonRefusal(id, p) 回否定
//     回执，红门那条是 N15 Refusal(4)，与普通门的失败路径同形状（main.go:4149-4158），
//     客户端弹提示并留在等候区，不再黑屏；
//   - 校验通过 → 与 NPC 点 Start 完全同态：记下 3 秒准备的起点（幂等：倒计时期间再碰
//     一次红门只重发阶段1，不重置那 3 秒），回 gate_ack(15,{1}) + N2622 阶段1；之后由
//     moonTick 建 run 并进场。
func (w *worldSession) moonPortalStart(now time.Time) (bool, []outboundPacket, error) {
	if w.moonConfig == nil || w.role.ID == 0 {
		return false, nil, nil
	}
	if w.state.Position.Town != 215 || w.state.Position.Area != 2 {
		return false, nil, nil
	}
	if e := w.moonStartPreflight(); e != nil {
		return true, nil, e
	}
	if w.moon.prepared.IsZero() {
		w.moon.prepared = now
	}
	return true, []outboundPacket{{"moon_portal_start_ack", 1, 15, []byte{1}}, w.moonInfo(1, false)}, nil
}

// moonRechallenge 重开一局月湖单人：清关面板上的「重挑战」走这里（CMD72 option 0 与 option 5）。
//
// 与 NPC 的 C2284 不同，它**不经过准备期** —— 玩家刚通关、人就在副本里，客户端要的是直接
// 再来一局。准入只有一条：这次必须**已经通关**（没通关时这两个选项本来也不该出现，出现就
// 拒绝，免得把一次没打完的挑战当成新一局重开）。
//
// 帧序参考普通副本的重开（card_flow.go 的 restartDungeon）：客户端此刻还停在结算面板上，
// 原生 handler 已经把副本 module 拆掉了，直接推入场序列会落到已拆卸的场景（实机教训
// 0xC0000005）。所以先补一帧门头（gate_ack(15,{1})），再走月湖自己的入场序列
// （N2/N2/N14/N3/N9/N27/N28/N29/N2622）。
func (w *worldSession) moonRechallenge(now time.Time) ([]outboundPacket, error) {
	if w.moonConfig == nil || w.moon.owner == nil || w.dungeons == nil {
		return nil, fmt.Errorf("Moon rechallenge needs an owned run")
	}
	if !w.moon.owner.Session().Completed() || w.moon.owner.Session().Definition.ID != 100004137 {
		return nil, fmt.Errorf("Moon rechallenge requires a committed clear")
	}
	seed, e := randomSeed()
	if e != nil {
		return nil, e
	}
	r, e := dungeon.NewMoonSolo(*w.dungeons, w.level, w.role.WireID, seed, now)
	if e != nil {
		return nil, e
	}
	w.moon.owner, w.activeDungeon = r, r.Session()
	w.moon.plan, w.moon.claimed, w.moon.resultAt = nil, false, time.Time{}
	w.moon.prepared = time.Time{}
	w.completionSent, w.resultSent = false, false
	w.resetCards()
	w.leaveScene()
	enter, e := w.moonEnterPackets(false)
	if e != nil {
		return nil, e
	}
	return append([]outboundPacket{{"moon_rechallenge_gate", 1, 15, []byte{1}}}, enter...), nil
}


// autoPickMoonCard 是沉月湖的「翻牌倒计时到点自动选第一张」。
//
// 与通用副本的 autoPickSettlementCard 同形，但**不能复用那一条**：那一条走
// w.cardPlan + grantFreeCard，沉月湖走的是 w.moon.plan + moonClaim。
//
// 实机 BUG（2026-10-08）：倒计时结束后服务端什么都不发，玩家一张牌都翻不到 ——
// 根因是沉月湖的翻牌走 dispatchSpecialContent -> moonHandle，绕过了 dispatchDungeon
// 里那段「card_layout_ack 时置 cardAutoPickAt」的后处理。由 client_connection 的
// tick 调用（见那里对 w.moon.plan 的说明）。
//
// ⚠️ 2026-10-09 事故后**按 moon_autopick_test.go（完好）+ 通用版 autoPickSettlementCard
// 重写**：这份文件在拆分脚本里被截成 0 字节，原文丢失，函数体是按它的门禁测试复原的。
func (w *worldSession) autoPickMoonCard(now time.Time) ([]outboundPacket, error) {
	if w == nil || w.moon.autoPickAt.IsZero() || now.Before(w.moon.autoPickAt) {
		return nil, nil
	}
	// 没有本局（人已经回城/换场）：什么都不做，也不 panic。
	if w.moon.owner == nil {
		return nil, nil
	}
	// 奖单还没冻结（case 46 还没跑）：清掉计时器，免得每个 tick 空转。
	if !w.resultSent || w.moon.plan == nil {
		w.moon.autoPickAt = time.Time{}
		return nil, nil
	}
	// 已经领过（玩家自己选了）：不能再替他选一张。
	if w.moon.claimed {
		return nil, nil
	}
	// 布局还没发出去：不发，否则会在玩家还没看到牌的时候就定死第一张。
	if !w.cardLayoutSent {
		return nil, nil
	}
	packets, err := w.moonClaim(0)
	if err != nil {
		// 未提交则保留奖单，稍后重试；不吞奖。
		w.moon.autoPickAt = now.Add(5 * time.Second)
		return nil, err
	}
	w.moon.autoPickAt = time.Time{}
	return packets, nil
}

func (w *worldSession) moonClaim(index byte) ([]outboundPacket, error) {
	if w.moon.plan == nil {
		return nil, fmt.Errorf("Moon frozen reward missing")
	}
	card, e := protocol.CardSelected(int(index))
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).ClaimMoonReward(ctx, w.role, w.moon.plan.Run)
	if e != nil {
		return nil, e
	}
	w.role = saved
	w.moon.claimed = true
	body, e := w.loot.Bootstrap(workflow.LootRole(saved))
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"moon_reward_inventory", 0, 13, body}, {"card_selection_ack", 1, 71, card}, w.moonInfo(5, true)}, nil
}
func (w *worldSession) moonReturn(success bool) ([]outboundPacket, error) {
	ua, e := w.userAreaPayload()
	if e != nil {
		return nil, e
	}
	area, e := w.areaPayload()
	if e != nil {
		return nil, e
	}
	state, e := protocol.UserState(w.role.WireID, protocol.UserStateTown)
	if e != nil {
		return nil, e
	}
	basic, e := w.characters.EntryBasicProbe(w.role, w.moonContext())
	if e != nil {
		return nil, e
	}
	more, e := w.characters.EntryAddition(w.role)
	if e != nil {
		return nil, e
	}
	phase := uint32(14)
	if success {
		phase = 5
	}
	out := []outboundPacket{{"dungeon_leave_ack", 1, 42, []byte{1}}, w.moonInfo(phase, success), {"dungeon_return_area", 0, 23, ua}, {"dungeon_return_users", 0, 24, area}, {"moon_town_state", 0, 3, state}, {"moon_town_actor", 0, 2, basic}, {"moon_town_attributes", 0, 2, more}}
	worn, e := inventory.WornSpaceUpdate(w.role.State)
	if e != nil {
		return nil, e
	}
	out = append(out, outboundPacket{"moon_worn", 0, 14, worn})
	w.activeDungeon = nil
	w.selectingDungeon = false
	if w.pilotDeath != nil && w.pilotDeath.Dead {
		w.pilotDeath.Dead = false
		body, err := protocol.PlayerDeathState(w.role.WireID)
		if err == nil {
			body[2] = 1
			out = append(out, outboundPacket{"moon_return_revived", 0, 32, body})
		}
	}
	w.drops = nil
	w.deathSent = nil
	w.completionSent, w.resultSent = false, false
	w.resetCards()
	w.enterArea()
	created := w.moon.created
	w.moon = moonSoloState{created: created, readySent: true, recovered: true}
	return out, nil
}

// Login fixtures use this gateway's existing synthetic key schedule. Only
// re-encode its successful CMD1 response for the opted-in Moon channel.
func moonLoginResponse(raw, keys []byte) ([]byte, error) {
	if e := wire.ValidateServer(raw); e != nil {
		return nil, e
	}
	if raw[0] != 1 || binary.LittleEndian.Uint16(raw[1:]) != 1 {
		return nil, fmt.Errorf("Moon login fixture is not CMD1")
	}
	p, e := wire.DecryptPayload(keys, 1, raw[wire.ServerHeaderSize:])
	if e != nil {
		return nil, e
	}
	if len(p) < 5 || p[0] != 1 {
		return nil, fmt.Errorf("Moon requires a successful login fixture")
	}
	p[3] = 101
	cipher, e := wire.EncryptPayload(keys, 1, p)
	if e != nil {
		return nil, e
	}
	return wire.ServerFrame(1, 1, cipher)
}
