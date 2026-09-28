package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Opt-in contribution profile. No change to the ordinary upstream server.
// Pool/weights are operator policy, not claimed official smart-drop rates.
type moonSoloConfig struct {
	ClientProfile string                `json:"client_profile"`
	Channel       uint32                `json:"channel"`
	Remaining     byte                  `json:"test_remaining"`
	Rewards       loot.MoonRewardPolicy `json:"rewards"`
	Entry         storage.WorldPosition `json:"entry"`
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
}

func loadMoonSoloConfig(path string, s *loot.Service) (*moonSoloConfig, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c moonSoloConfig
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	if c.ClientProfile != "2.38.3.25" || c.Channel < 1 || c.Channel > 255 || c.Remaining == 0 || c.Entry.Town != 215 || c.Entry.Area != 1 || c.Entry.Return != nil {
		return nil, fmt.Errorf("Moon contribution requires explicit client profile/channel/test counter/source entry")
	}
	if e = s.ValidateMoonRewards(c.Rewards); e != nil {
		return nil, e
	}
	return &c, nil
}
func (w *worldSession) moonContext() [2]byte { return [2]byte{0, byte(w.moonConfig.Channel)} }
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
	if floor {
		out = append(out, outboundPacket{"moon_portal_notify", 0, 2281, protocol.MoonFloorDirectMoveNotice115()})
		out = append(out, p.Handoff...)
		out = append(out, outboundPacket{"moon_actor", 0, 2, p.Basic}, outboundPacket{"moon_actor_details", 0, 2, p.Addition})
		out = append(out, outboundPacket{"moon_select", 0, 27, protocol.EnterDungeonSelection()}, outboundPacket{"moon_actor_state", 0, 3, p.State}, outboundPacket{"moon_roster", 0, 9, p.Roster})
	} else {
		out = append(out, outboundPacket{"moon_actor", 0, 2, p.Basic}, outboundPacket{"moon_actor_details", 0, 2, p.Addition}, outboundPacket{"moon_actor_state", 0, 3, p.State}, outboundPacket{"moon_roster", 0, 9, p.Roster}, outboundPacket{"moon_select", 0, 27, protocol.EnterDungeonSelection()})
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
		saved, e := w.loot.RecoverMoonRewards(ctx, w.role)
		cancel()
		if saved.ID != 0 {
			w.role = saved
		}
		if e != nil && !errors.Is(e, loot.ErrMoonBagFull) {
			return nil, e
		}
		body, err := w.loot.Bootstrap(w.role)
		if err != nil {
			return nil, err
		}
		return []outboundPacket{{"moon_pending_rewards_restored", 0, 13, body}}, nil
	}
	if !w.moon.readySent {
		p, e := protocol.DungeonSpecialRewardInfo115([]protocol.SpecialRewardRemaining115{{Dungeon: 100004137, Remaining: w.moonConfig.Remaining}, {Dungeon: 100004134, Remaining: w.moonConfig.Remaining}}, nil)
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
		return true, []outboundPacket{{"moon_party_actor", 0, 2, basic}, {"moon_party_details", 0, 2, detail}, {"moon_party_created", 0, 9, roster}}, nil
	case 2284:
		mode, e := protocol.DecodeSemiRaidStart115(p)
		if e != nil {
			return fail(e)
		}
		if mode != 24 || !w.moon.created || w.moon.owner != nil || w.state.Position.Town != 215 || w.state.Position.Area != 2 {
			return fail(fmt.Errorf("Moon start requires own solo party at source waiting area"))
		}
		if w.dungeons == nil || w.loot == nil {
			return fail(fmt.Errorf("Moon resources unavailable"))
		}
		if w.inTutorial || w.specialWarpPending {
			return fail(fmt.Errorf("Moon character is busy"))
		}
		if e = w.service.ValidatePosition(w.level, w.odyssey, w.state.Position); e != nil {
			return fail(e)
		}
		if _, e = dungeon.SelectMoon(*w.dungeons, protocol.DungeonSelection{ID: 100004136, Party: 65535}, w.level, nil, 0); e != nil {
			return fail(e)
		}
		if _, e = dungeon.MoonInitialProgress(*w.dungeons); e != nil {
			return fail(e)
		}
		if e = w.loot.ValidateMoonRewards(w.moonConfig.Rewards); e != nil {
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
			plan, e := w.loot.FreezeMoonReward(ctx, w.role, s, w.moonConfig.Rewards)
			cancel()
			if e != nil {
				return fail(e)
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
			if req.Option < 2 {
				return fail(fmt.Errorf("Moon return only; start again through NPC"))
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
	saved, _, e := w.loot.ClaimMoonReward(ctx, w.role, w.moon.plan.Run)
	if e != nil {
		return nil, e
	}
	w.role = saved
	w.moon.claimed = true
	body, e := w.loot.Bootstrap(saved)
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
