package main

// 巴卡尔攻坚战（contents/2022/bakalraid，频道 type 82）的网关接线。
//
// 证据分层与 wire 契约见 analysis/tasks/next151：入口 656 建团 → 2089 开战
// （投票爆发串+ACK）→ 12 待机小队（basic/detail/worn/created/ack）→ 2062
// 源地下城进图 → 2073 加载闭环 → CMD39 实体死亡确认 → 2070 房间跳转 →
// 2074 回营 → 1134 终局确认 → 结算串 2285→N13→588→574（Tick 的 settle）→
// 自动冻结/发奖。2069 只核对战报所属图；CMD13 是小队离开。

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
)

// newBakalRewardService wires the raid ledger. A nil rules catalog (content
// not shipped) disables the service: the raid channel then refuses creation
// with the captured waiting-room refusal, exactly as B′ recorded.
func newBakalRewardService(store *database.Store, lootService *loot.Service, rules *catalog.BakalRaidRules) *workflow.BakalRewardService {
	if rules == nil || store == nil || lootService == nil {
		return nil
	}
	return &workflow.BakalRewardService{
		Store:   store,
		Loot:    lootService,
		Rules:   rules,
		Content: "contents/2022/bakalraid",
	}
}

// dispatchBakal routes the raid channel (type 82). It sits between the ispins
// and legion stages: none of the Bakal envelope ids collide with the ispins
// content gate, and the legion stage must not swallow the raid commands.
func (client *gameConnection) dispatchBakal(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type != 1 || !client.bootstrapped || client.worldState == nil || !requestData.verified {
		return dispatchNext
	}
	w := client.worldState
	if w.channelType != legion.BakalChannelType {
		return dispatchNext
	}
	now := time.Now()
	// C35 is the native post-scene readiness signal. Restore before the
	// player opens/registers a raid, and refresh on later ledger/week changes.
	if w.activeDungeon == nil {
		switch requestData.frame.ID {
		case 35, 12, 656, 1353, 650:
			if err := w.syncBakalWeeklyQuota(now, client.output.send, client.event); err != nil {
				client.event(map[string]any{"kind": "bakal_weekly_quota_sync_failed", "error": err.Error()})
				return dispatchClose
			}
		}
	}
	handled, packets, err := client.handleBakalRequest(requestData.frame.ID, requestData.plaintext, now)
	if !handled {
		return dispatchNext
	}
	if err != nil {
		// B′ 实录：拒绝只落事件、无 s2c 应答（raid_entry_refused）。
		event := map[string]any{"kind": "raid_entry_refused", "raid": "bakal", "id": requestData.frame.ID, "reason": err.Error(), "character_id": w.role.ID, "town": w.state.Position.Town, "area": w.state.Position.Area}
		event["raid_run"] = w.bakalRun
		if s := w.activeDungeon; s != nil {
			event["dungeon"], event["map"] = s.Definition.ID, s.Room.Map
			event["grid"], event["loaded"] = [2]byte{s.Room.X, s.Room.Y}, s.Loaded
			event["living"] = s.LivingMonsters()
		}
		if w.bakalRules != nil && w.bakalRules.WaitingRoom != nil {
			event["waiting_town"] = w.bakalRules.WaitingRoom.TownID
			event["waiting_area"] = w.bakalRules.WaitingRoom.AreaID
			event["waiting_map"] = w.bakalRules.WaitingRoom.MapPath
		}
		client.event(event)
		return dispatchHandled
	}
	if client.sendPlan(packets, client.logWorldResponseBody) != nil {
		return dispatchClose
	}
	return dispatchHandled
}

// handleBakalRequest switches the raid envelope. handled=false lets the frame
// fall through to the generic dungeon dispatch (position reports, heartbeats).
func (client *gameConnection) handleBakalRequest(id uint16, p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	switch id {
	case legion.CmdBakalCreateRaid:
		return client.bakalCreateRaid(p, now)
	case legion.CmdBakalRaidUpdate:
		// c2s 657 (8B) — 成功实录里等待期出现过一次；语义不透明，仅记录。
		client.event(map[string]any{"kind": "bakal_raid_update", "raid": w.bakalRun})
		return true, nil, nil
	case 1353:
		return client.bakalOwnedInfo(p)
	case 661:
		actor, position, err := protocol.DecodeRaidAssignment(p)
		if err != nil {
			return true, nil, err
		}
		if w.bakal == nil || w.bakalRecruitment == nil || w.bakalRules == nil || actor != w.role.WireID || actor != w.bakalRecruitment.Leader || position > uint32(w.bakalRules.RaidMemberMax) || position > 255 {
			return true, nil, fmt.Errorf("raid assignment outside owned source formation")
		}
		next := *w.bakalRecruitment
		next.MemberPosition = byte(position)
		next.MemberArea = uint16(w.state.Position.Area)
		body, err := protocol.RaidAssignmentUpdate(next)
		if err != nil {
			return true, nil, err
		}
		*w.bakalRecruitment = next
		return true, []outboundPacket{{"raid_formation_member_updated", 0, 578, body}, {"raid_formation_ack", 1, 661, []byte{1, 0, 0, 0, 0}}}, nil
	case 2121:
		// Native control handshake; handover client_raid_entrance.go:178..185
		// validates both control values and sends Kind1/CMD2121/{01}.
		if _, err := protocol.DecodeRaidUpdateControl(p); err != nil {
			return true, nil, err
		}
		return true, []outboundPacket{{"raid_update_control_ack", 1, 2121, []byte{1}}}, nil
	case 650:
		if len(p) < 1 || len(p) > 16 || p[0] > 1 {
			return true, nil, fmt.Errorf("invalid native raid entry-cost query")
		}
		for _, v := range p[1:] {
			if v != 0 {
				return true, nil, fmt.Errorf("nonzero native raid entry-cost padding")
			}
		}
		return true, []outboundPacket{{"raid_entry_cost_info_sent", 0, 585, make([]byte, 4)}, {"raid_entry_cost_ack", 1, 650, []byte{1}}}, nil
	case legion.CmdBakalStartRaid:
		return client.bakalStartRaid(p, now)
	case legion.CmdBakalBattleReport:
		return client.bakalBattleReport(p, now)
	case legion.CmdBakalScriptWarp:
		return client.bakalScriptWarp(p)
	case legion.CmdBakalLoadingDone:
		return client.bakalLoadingDone(p)
	case 37:
		if w.bakal == nil || w.activeDungeon == nil {
			return false, nil, nil
		}
		return client.bakalFinishLoading(p, false)
	case legion.CmdBakalCampReturn:
		return client.bakalCampReturn(p, now)
	case 2072:
		if w.bakal == nil || w.bakalRecruitment == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.bakalRecruitment.MemberPosition < 1 || int(w.bakalRecruitment.MemberPosition) > w.bakalRules.RaidMemberMax {
			return true, nil, fmt.Errorf("commander use outside owned loaded raid party")
		}
		slot, err := protocol.DecodeBakalCommanderBuff(p)
		if err != nil {
			return true, nil, err
		}
		frames, err := w.bakal.UseRaidBuffAt(slot, now)
		if err != nil {
			return true, nil, err
		}
		packets := bakalOutgoing(frames)
		for i := range packets {
			if packets[i].ID == 2288 {
				binary.LittleEndian.PutUint32(packets[i].Payload[9:], uint32(w.bakalRecruitment.MemberPosition-1))
			}
		}
		client.event(map[string]any{"kind": "bakal_commander_used", "slot": slot, "member_position": w.bakalRecruitment.MemberPosition, "raid": w.bakalRun})
		return true, packets, nil
	case 72:
		// The witnessed in-raid Retreat UI sends EPLP state1/option2,
		// independently of the2074 script return sender.
		if w.bakal == nil {
			return false, nil, nil
		}
		request, err := protocol.DecodeSettlementExit(p)
		if err != nil {
			return true, nil, err
		}
		if request.State != 1 || request.Option != 2 {
			return false, nil, nil
		}
		plan, err := w.bakalRetreatPlanAt(now, legion.BakalReturnVoluntary)
		if err != nil {
			return true, nil, err
		}
		return true, append(plan, outboundPacket{"bakal_retreat_ui_ack", 1, 72, protocol.SettlementExitSuccess(request)}), nil
	case legion.CmdBakalFinalConfirm:
		return client.bakalFinalConfirm(p)
	case legion.CmdBakalGiveup:
		// c2s 2261 (0B) 无应答；B′ 里上游拒绝过它，这里同样不受理。
		client.event(map[string]any{"kind": "bakal_giveup_observed", "raid": w.bakalRun})
		return true, nil, nil
	case 12:
		return client.bakalStandbyPartyCreate(p)
	case 13:
		return client.bakalRewardClaimOrLeave(p, now)
	case 22, 707:
		// Town map changes are shared with the ordinary world handler.
		// The successful raid enters via the native 2062 envelope.
		return false, nil, nil
	case 2062:
		if w.bakal != nil {
			return client.enterBakalPortal(id, p, now)
		}
		return false, nil, nil
	}
	return false, nil, nil
}

// bakalCreateRaid is CMD656: create the raid in the waiting room. B 层建团
// 无 s2c 应答（拒绝除外）；raid id 由服务端铸造（真实派生不透明，见 next151 §6）。
func (client *gameConnection) bakalCreateRaid(p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	request, err := protocol.DecodeRaidCreateRequest(p)
	if err != nil {
		return true, nil, err
	}
	if w.bakalRules == nil || w.bakalRewards == nil {
		// 拒绝文本与 B′ 实录逐字节一致（交接包 raid_entry_refused）。
		return true, nil, legion.ErrBakalWaitingRoomNotBound
	}
	// 建团前先过周清门（A 层 main.bakalRaidAdmission）。
	if w.role.ID == 0 || w.role.Name == "" || !w.bakalRules.OwnsWaitingRoom(w.state.Position.Town, w.state.Position.Area) {
		return true, nil, legion.ErrBakalWaitingRoomNotBound
	}
	if w.bakal != nil && w.bakal.Stage() != legion.BakalOpeningEnded && w.bakal.Stage() != legion.BakalOpeningFailed {
		return true, nil, fmt.Errorf("character already owns a raid")
	}
	if request.Kind != byte(w.bakalRules.RaidID) {
		return true, nil, fmt.Errorf("Bakal create request does not match source raid kind")
	}
	if w.bakalRewards != nil {
		if err := workflow.BakalRaidAdmission(w.bakalRules, w.role, now); err != nil {
			return true, nil, err
		}
	}
	opening, err := legion.PrepareBakalOpening(w.bakalRules, false)
	if err != nil {
		return true, nil, err
	}
	if client.gatewayRuntime == nil {
		return true, nil, fmt.Errorf("Bakal team registry missing")
	}
	teamID, err := protocol.RaidTeamID(w.serverID, w.channelType, client.bakalTeamSequence.Add(1))
	if err != nil {
		return true, nil, err
	}
	var state character.State
	if err = json.Unmarshal(w.role.State, &state); err != nil {
		return true, nil, err
	}
	var fame uint32
	if w.characters != nil && w.characters.Equipment != nil {
		fame, err = w.characters.EquipmentFame(w.role.State)
		if err != nil {
			return true, nil, err
		}
	}
	recruitment := protocol.RaidRecruitment{ID: teamID, Create: request, Leader: w.role.WireID, LeaderName: w.role.Name, LeaderProfession: w.role.Profession, LeaderAdvancement: state.Advancement, LeaderLevel: w.level, MemberCount: 1, MemberPosition: 0, MemberArea: uint16(w.state.Position.Area), MemberMax: uint32(w.bakalRules.RaidMemberMax), LeaderFame: fame}
	details, err := protocol.RaidOwnedDetails(recruitment)
	if err != nil {
		return true, nil, err
	}
	w.bakal = opening
	w.bakalRecruitment = &recruitment
	w.bakalRun = fmt.Sprintf("bakal-%d", now.UnixNano())
	w.bakalName = w.role.Name
	client.event(map[string]any{"kind": "raid_created_waiting", "raid": "bakal", "raid_id": w.bakalRun,
		"name": request.Title, "wire_team_id": teamID, "channel_type": legion.BakalChannelType, "town": w.state.Position.Town, "area": w.state.Position.Area})
	ack := binary.LittleEndian.AppendUint32([]byte{1}, teamID)
	return true, []outboundPacket{{"raid_created_owned_details", 0, 578, details}, {"raid_create_ack", 1, 656, ack}}, nil
}

func (client *gameConnection) bakalOwnedInfo(p []byte) (bool, []outboundPacket, error) {
	w := client.worldState
	mode, id, err := protocol.DecodeRaidInfoRequest(p)
	if err != nil {
		return true, nil, err
	}
	if w.bakal == nil || w.bakalRecruitment == nil || id != w.bakalRecruitment.ID || (mode != 1 && mode != 2) {
		return true, nil, fmt.Errorf("no owned native raid details")
	}
	var body []byte
	w.bakalRecruitment.MemberArea = uint16(w.state.Position.Area)
	if mode == 1 {
		body, err = protocol.RaidMemberInfoSuccess(*w.bakalRecruitment)
	} else {
		body, err = protocol.RaidFullInfoSuccess(*w.bakalRecruitment)
	}
	if err != nil {
		return true, nil, err
	}
	return true, []outboundPacket{{"raid_owned_details_sent", 1, 1353, body}, {"raid_member_info_ready", 1, 1220, []byte{1}}}, nil
}

// Town movement changes the area used by the native pre-start predicate.
// Waiting for a subsequent661/1353 leaves the client's member cache stale.
func (w *worldSession) syncBakalMemberArea(send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w == nil || w.channelType != legion.BakalChannelType || w.bakal == nil || w.bakalRecruitment == nil {
		return nil
	}
	area := w.state.Position.Area
	if area > 65535 {
		return fmt.Errorf("Bakal member area outside native wire range")
	}
	if w.bakalRecruitment.MemberArea == uint16(area) {
		return nil
	}
	next := *w.bakalRecruitment
	next.MemberArea = uint16(area)
	body, err := protocol.RaidAssignmentUpdate(next)
	if err != nil {
		return err
	}
	if err := send(0, 578, body); err != nil {
		return err
	}
	*w.bakalRecruitment = next
	event(map[string]any{"kind": "raid_member_area_synchronized", "character_id": w.role.ID, "town": w.state.Position.Town, "area": area, "wire_team_id": next.ID})
	return nil
}

// bakalStartRaid is CMD2089: start the war. The opening burst (vote start,
// vote state, member assigned, preparing, countdown) goes out first, the CMD
// ack last (timeline 000→005).
func (client *gameConnection) bakalStartRaid(p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, legion.ErrBakalStartRequiresMember
	}
	if err := legion.DecodeBakalStartRaid(p); err != nil {
		return true, nil, err
	}
	// Replace the historical actor2 fixture with this raid's real member.
	if w.bakalRecruitment == nil {
		return true, nil, fmt.Errorf("Bakal start requires owned member details")
	}
	// Failure ends the run, not membership. Retrying resets source state
	// and ledger identity while retaining the native leader and formation.
	if w.bakal.Stage() == legion.BakalOpeningFailed {
		opening, err := legion.PrepareBakalOpening(w.bakalRules, false)
		if err != nil {
			return true, nil, err
		}
		w.bakal = opening
		w.bakalRun = fmt.Sprintf("bakal-%d", now.UnixNano())
	}
	if w.bakal.Stage() == legion.BakalOpeningIdle {
		w.bakalRecruitment.MemberRecoveryUntil = 0
		w.bakalDeathReturnAt = time.Time{}
	}
	w.bakalRecruitment.MemberArea = uint16(w.state.Position.Area)
	member, err := protocol.RaidAssignmentUpdate(*w.bakalRecruitment)
	if err != nil {
		return true, nil, err
	}
	frames, err := w.bakal.Start(w.bakalName, now)
	if err != nil {
		return true, nil, err
	}
	packets := bakalOutgoing(frames)
	lifecycle, err := w.bakalLifecyclePlan()
	if err != nil {
		return true, nil, err
	}
	packets = append(packets, lifecycle...)
	w.bakalFailureNotified = false
	for i := range packets {
		if packets[i].ID == legion.NotiBakalMemberAssigned {
			packets[i].Payload = member
		}
	}
	packets = append(packets, outboundPacket{"巴卡尔开战确认", 1, legion.CmdBakalStartRaid, legion.BakalAck()})
	return true, packets, nil
}

// bakalStandbyPartyCreate is the raid-channel CMD12: B 层实录 007→011 应答串
// ——basic(2)、detail(2)、worn restored(14)、created(9)、ack(12)，与伊斯/
// 维纳斯待机区同族（先例链：黑鸦→伊斯→维纳斯→巴卡尔）。
func (client *gameConnection) bakalStandbyPartyCreate(p []byte) (bool, []outboundPacket, error) {
	w := client.worldState
	request, err := legion.DecodeBakalSubparty(p)
	if err != nil {
		return true, nil, err
	}
	basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
	if err != nil {
		return true, nil, err
	}
	detail, err := w.characters.EntryAddition(w.role)
	if err != nil {
		return true, nil, err
	}
	worn, err := inventory.WornSpaceUpdate(w.role.State)
	if err != nil {
		return true, nil, err
	}
	created := legion.BakalSubpartyCreatedFrame(request.Name, request.Capacity, request.PartyType)
	packets := []outboundPacket{
		{"bakal_subparty_actor", 0, 2, basic},
		{"bakal_subparty_details", 0, 2, detail},
		{"bakal_subparty_worn_restored", 0, 14, worn},
		{"bakal_subparty_created", 0, 9, created},
		{"bakal_subparty_ack", 1, 12, legion.BakalAck()},
	}
	w.soloPartyReady = true
	if w.bakalRecruitment != nil {
		w.bakalRecruitment.MemberPosition = 0
	}
	return true, packets, nil
}

// CMD13 leaves the standby party. Handover settlement pays automatically;
// its later six CMD13 samples were unimplemented, not reward confirmations.
func (client *gameConnection) bakalRewardClaimOrLeave(p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil || w.bakalRewards == nil {
		return false, nil, nil
	}
	switch w.bakal.Stage() {
	case legion.BakalOpeningIdle, legion.BakalOpeningEnded, legion.BakalOpeningFailed:
		if len(p) != 8 {
			return true, nil, fmt.Errorf("bakal leave payload %d bytes, want 8", len(p))
		}
		w.soloPartyReady = false
		w.bakal = nil
		w.bakalRecruitment = nil
		w.bakalRun = ""
		packets := []outboundPacket{
			{"bakal_party_gone", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}
		return true, packets, nil
	}
	return true, nil, fmt.Errorf("bakal reward claim outside settled raid")
}

// bakalBattleReport is CMD2069: the client's kill report for a boss room.
func (client *gameConnection) bakalBattleReport(p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, legion.ErrBakalWarpOutsideCombat
	}
	report, err := legion.DecodeBakalBattleReport(p)
	if err != nil {
		return true, nil, err
	}
	if w.activeDungeon == nil || !w.activeDungeon.Loaded || report.Dungeon != w.activeDungeon.Definition.ID {
		return true, nil, legion.ErrBakalWarpOutsideCombat
	}
	if report.Map != w.activeDungeon.Room.Map {
		return true, nil, fmt.Errorf("Bakal battle report outside owned map")
	}
	// Damage reports do not establish a death. CMD39 confirms native entities.
	plan, err := w.bakalConfirmDefeats(now)
	return true, plan, err
}

// bakalScriptWarp is CMD2070: scripted warp inside the loaded owned combat.
// The captured body carries two coordinate groups whose room mapping is not
// in the evidence, so the warp re-publishes the current room's location and
// the dispatch layer adds the CMD ack (B 层 2070×9，每次带 ACK）。
func (client *gameConnection) bakalScriptWarp(p []byte) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, legion.ErrBakalWarpOutsideCombat
	}
	r, err := legion.DecodeBakalRoomWarp115(p)
	if err != nil {
		return true, nil, err
	}
	if w.activeDungeon == nil || w.bakalRules.DungeonCatalog == nil {
		return true, nil, legion.ErrBakalWarpOutsideCombat
	}
	ack := outboundPacket{"bakal_script_warp_ack", 1, legion.CmdBakalScriptWarp, legion.BakalAck()}
	m := w.bakalRules.Monsters["bakal"]
	if w.bakalCurrent == uint32(w.bakalRules.NormalPhase.EnterBakalDungeon) && w.bakal.SecondPhase() && r.Grid == m.SecondGrid && [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} == m.SecondGrid {
		return true, []outboundPacket{ack}, nil
	}
	if !w.activeDungeon.Loaded {
		return true, nil, legion.ErrBakalWarpOutsideCombat
	}
	if w.bakalCurrent == uint32(w.bakalRules.NormalPhase.EnterBakalDungeon) && m.SecondTemplate != 0 && r.Grid == m.SecondGrid {
		owned := false
		for _, actor := range w.activeDungeon.Monsters {
			owned = owned || actor.Template == m.Template && actor.Rank == 3
		}
		arena := false
		for _, loc := range w.bakalRules.Locations {
			if loc.Dungeon == w.bakalCurrent && loc.SpecificX != nil && loc.SpecificY != nil && [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y} == [2]byte{byte(*loc.SpecificX), byte(*loc.SpecificY)} {
				arena = true
			}
		}
		if !owned || !arena {
			return true, nil, fmt.Errorf("phase cinematic has no source first-stage arena actor")
		}
		hp, err := w.bakal.CheckPhaseCinematic()
		if err != nil {
			return true, nil, err
		}
		next, packets, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Position: r.Grid, Record: r.Record, Dungeon: w.bakalCurrent, RaidCinematic: true})
		if err != nil {
			return true, nil, err
		}
		frames, err := w.bakal.AcceptPhaseCinematic(time.Now())
		if err != nil {
			return true, nil, err
		}
		// Retire the source first-stage instance without inventing a death
		// reward; re-entering its old room must not recreate that actor.
		for _, actor := range w.activeDungeon.Monsters {
			if actor.Template == m.Template && actor.Rank == 3 {
				next.Dead[actor.Entity] = true
			}
		}
		w.activeDungeon = next
		frames = append(frames, w.bakal.ReportHealth(legion.BakalMonsterType("bakal"), w.bakalLocation, hp)...)
		for i := range packets {
			if packets[i].Kind == 1 && packets[i].ID == 45 {
				packets = append(packets[:i], packets[i+1:]...)
				break
			}
		}
		return true, append(append([]outboundPacket{ack}, bakalOutgoing(frames)...), packets...), nil
	}
	// Native returns may target another grid of the owned source maze.
	// MoveRaidReturn retains the loaded/cleared/source-target gates.
	next, packets, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Position: r.Grid, Record: r.Record, Dungeon: w.activeDungeon.Definition.ID, RaidReturn: true})
	if err != nil {
		return true, nil, err
	}
	location, err := w.bakalLocationForRoom(next)
	if err != nil {
		return true, nil, err
	}
	w.activeDungeon = next
	w.bakalLocation = location
	for i := range packets {
		if packets[i].Kind == 1 && packets[i].ID == 45 {
			packets[i] = ack
		}
	}
	return true, packets, nil
}

// bakalLoadingDone is CMD2073: the client finished loading the entered room.
func (client *gameConnection) bakalLoadingDone(p []byte) (bool, []outboundPacket, error) {
	return client.bakalFinishLoading(p, true)
}

func (client *gameConnection) bakalFinishLoading(p []byte, native bool) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, fmt.Errorf("missing active Bakal opening")
	}
	if native {
		if err := legion.DecodeBakalLoadingDone(p); err != nil {
			return true, nil, err
		}
		p = make([]byte, 16)
	}
	if w.activeDungeon == nil || w.activeDungeon.Definition.ID != w.bakalCurrent {
		return true, nil, fmt.Errorf("Bakal loading without an owned dungeon")
	}
	// Both37 and2073 arrive for the same map. A second completion must not
	// rebuild actor/bag state or retrigger source map initialization.
	if w.activeDungeon.Loaded {
		id := uint16(37)
		if native {
			id = legion.CmdBakalLoadingDone
		} else if len(p) != 16 {
			return true, nil, fmt.Errorf("invalid repeated Bakal loading body")
		}
		return true, []outboundPacket{{"bakal_loading_repeat_ack", 1, id, legion.BakalAck()}}, nil
	}
	// Reuse the normal load plan; its command ACK is replaced by native 2073.
	packets, err := w.finishDungeonLoading(p)
	if err != nil {
		return true, nil, err
	}
	if native {
		for i := range packets {
			if packets[i].Kind == 1 && packets[i].ID == 37 {
				packets[i].ID = legion.CmdBakalLoadingDone
				packets[i].Name = "bakal_loading_ack"
			}
		}
	}
	w.activeDungeon.Loaded = true
	if err := w.bakal.LoadingDone(w.bakalCurrent); err != nil {
		return true, nil, err
	}
	boss, err := w.bakalLoadedBoss()
	if err != nil {
		return true, nil, err
	}
	packets = append(packets, boss...)
	// N27/N28 start a new client scene. Restore the current source-owned
	// inventory after the first load handshake, never on the duplicate37/2073.
	packets = append(packets, bakalOutgoing([]legion.BakalFrame{w.bakal.BuffInventorySnapshot()})...)
	if w.bakalCurrent == uint32(w.bakalRules.NormalPhase.FinalClearDungeon) {
		client.event(map[string]any{"kind": "bakal_final_scene_started", "raid": w.bakalRun,
			"dungeon": w.bakalCurrent, "map": w.activeDungeon.Room.Map,
			"settlement_deadline":    w.bakal.FinalDeadline().Unix(),
			"source_timeout_seconds": w.bakalRules.NormalPhase.SettlementTimer.Secs,
			"source_movie_times":     w.bakalRules.MovieEndTimes})
	}
	client.event(map[string]any{"kind": "bakal_loaded", "raid": w.bakalRun, "dungeon": w.bakalCurrent, "location": w.bakalLocation, "map": w.activeDungeon.Room.Map, "grid": [2]byte{w.activeDungeon.Room.X, w.activeDungeon.Room.Y}, "placements": w.bakal.MonsterPlacements(), "living": w.activeDungeon.LivingMonsters(), "location_defeated": w.bakal.IsLocationDefeated(w.bakalLocation)})
	return true, packets, nil
}

// bakalCampReturn is CMD2074: retreat to the camp of the loaded room.
func (client *gameConnection) bakalCampReturn(p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, legion.ErrBakalInvalidCampReturn
	}
	if _, err := legion.DecodeBakalCampReturn(p); err != nil {
		return true, nil, err
	}
	plan, err := w.bakalRetreatPlanAt(now, legion.BakalReturnVoluntary)
	return true, plan, err
}

func (w *worldSession) bakalRetreatPlan() ([]outboundPacket, error) {
	return w.bakalRetreatPlanAt(time.Now(), legion.BakalReturnVoluntary)
}

func (w *worldSession) bakalRetreatPlanAt(now time.Time, cause legion.BakalReturnCause) ([]outboundPacket, error) {
	if w == nil || w.bakal == nil || w.activeDungeon == nil || !w.activeDungeon.RaidManaged || !w.activeDungeon.Loaded || w.activeDungeon.Definition.ID != w.bakalCurrent {
		return nil, legion.ErrBakalInvalidCampReturn
	}
	if !w.bakal.CanRetreat() {
		return nil, legion.ErrBakalInvalidCampReturn
	}
	if cause == legion.BakalReturnVoluntary && w.pilotDeath != nil && w.pilotDeath.Dead {
		cause = legion.BakalReturnDeath
	}
	camp, err := w.bakalCampPlan()
	if err != nil {
		return nil, err
	}
	frames, err := w.bakal.CampReturnAt(now, cause)
	if err != nil {
		return nil, err
	}
	w.bakalCurrent, w.bakalLocation = 0, legion.BakalCampLocation
	w.activeDungeon = nil
	w.pilotDeath = nil
	w.bakalDeathReturnAt = time.Time{}
	w.deathSent = nil
	w.drops = nil
	w.resetCards()
	plan := append(camp, bakalOutgoing(frames)...)
	if w.bakalRecruitment != nil {
		next := *w.bakalRecruitment
		next.MemberArea = uint16(w.state.Position.Area)
		until := w.bakal.RecoveryUntil()
		next.MemberRecoveryUntil = 0
		if !until.IsZero() {
			next.MemberRecoveryUntil = uint32(until.Unix())
		}
		body, err := protocol.RaidAssignmentUpdate(next)
		if err != nil {
			return nil, err
		}
		*w.bakalRecruitment = next
		plan = append(plan, outboundPacket{"bakal_recovery_started", 0, 578, body})
	}
	return plan, nil
}

// bakalFinalConfirm is CMD1134: the post-clear final-map confirm. The captured
// body carries the final map id at offset 4; the opening validates it against
// the recorded BakalFinalMap.
func (client *gameConnection) bakalFinalConfirm(p []byte) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil {
		return true, nil, fmt.Errorf("missing active Bakal opening")
	}
	request, err := legion.DecodeBakalFinalConfirm(p)
	if err != nil {
		return true, nil, err
	}
	if err := w.bakal.FinalConfirm(request.FinalMap); err != nil {
		return true, nil, err
	}
	return true, nil, nil
}

// enterBakalPortal is native CMD2062. Its requested dungeon is selected from
// the source catalog; it never substitutes a different uncleared boss.
func (client *gameConnection) enterBakalPortal(id uint16, p []byte, now time.Time) (bool, []outboundPacket, error) {
	w := client.worldState
	if w.bakal == nil || w.bakalRules == nil {
		return false, nil, nil
	}
	switch w.bakal.Stage() {
	case legion.BakalOpeningActive, legion.BakalOpeningFinal:
	default:
		return false, nil, nil
	}
	if w.bakal.Recovering(now) {
		client.event(map[string]any{"kind": "bakal_recovery_entry_refused", "raid": w.bakalRun, "until": w.bakal.RecoveryUntil().Unix(), "id": id})
		return true, nil, fmt.Errorf("Bakal recovery has not expired")
	}
	if w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Dead {
		return true, nil, fmt.Errorf("Bakal portal requires living player")
	}
	r, err := protocol.DecodeBakalPortal115(p)
	if err != nil {
		return true, nil, err
	}
	grid := [2]byte{byte(r.TargetGrid[0]), byte(r.TargetGrid[1])}
	requested := grid
	// Native portals request the normal post-combat room. A living source
	// placement instead enters SPECIFIC LOCATION XY (the actual arena).
	// Restore the matching half of raid_bakal_portal.go's source binding;
	// the boss spawn gate alone leaves dragons in an empty service room.
	resolved := false
	for _, loc := range w.bakalRules.Locations {
		// A cleared arena's exit is a death ACT, not a fresh-room fixture.
		// Native UI can keep targeting that arena after its actor is gone;
		// land on the source normal route until an explicit CREATE revives it.
		if loc.Dungeon == r.Dungeon && loc.SpecificX != nil && loc.SpecificY != nil && requested == [2]byte{byte(*loc.SpecificX), byte(*loc.SpecificY)} && w.bakal.IsLocationDefeated(uint32(loc.Index)) && loc.Dungeon != uint32(w.bakalRules.NormalPhase.EnterBakalDungeon) {
			grid = [2]byte{byte(loc.X), byte(loc.Y)}
			continue
		}
		if loc.Dungeon != r.Dungeon || loc.SpecificX == nil || loc.SpecificY == nil || requested != [2]byte{byte(loc.X), byte(loc.Y)} || w.bakal.IsLocationDefeated(uint32(loc.Index)) {
			continue
		}
		for _, placement := range w.bakal.MonsterPlacements() {
			if placement.Location != loc.Index {
				continue
			}
			if resolved {
				return true, nil, fmt.Errorf("ambiguous occupied source Bakal portal")
			}
			grid = [2]byte{byte(*loc.SpecificX), byte(*loc.SpecificY)}
			monster := w.bakalRules.Monsters[placement.Name]
			if w.bakal.SecondPhase() && monster.SecondTemplate != 0 {
				grid = monster.SecondGrid
			}
			resolved = true
		}
	}
	packets, err := w.bakalDungeonEntry(r.Dungeon, byte(r.Difficulty), &grid, now, false)
	if err != nil {
		return true, nil, err
	}
	client.event(map[string]any{"kind": "bakal_portal_selection", "raid": w.bakalRun,
		"request_id": id, "dungeon": w.bakalCurrent, "location": w.bakalLocation, "requested_grid": requested, "resolved_grid": grid, "occupied_arena": resolved})
	return true, packets, nil
}

// tickBakalOpening drives the raid clock from the connection's one-second
// mine ticker: the freeze and the full-bag splice land in the same tick the
// settlement burst goes out (timeline 237→240), then the burst is delivered.
func (client *gameConnection) tickBakalOpening(now time.Time) error {
	w := client.worldState
	if w == nil || w.bakal == nil {
		return nil
	}
	if w.bakal.Stage() == legion.BakalOpeningFailed && w.bakalFailureNotified {
		return nil
	}
	var camp []outboundPacket
	if w.bakal.SettleDue(now) {
		if err := w.bakalSettleSplice(now); err != nil {
			client.event(map[string]any{"kind": "bakal_settlement_pending", "raid": w.bakalRun, "error": err.Error()})
			return nil
		}
		var err error
		camp, err = w.bakalCampPlan()
		if err != nil {
			client.event(map[string]any{"kind": "bakal_settlement_pending", "raid": w.bakalRun, "error": err.Error()})
			return nil
		}
	}
	frames := w.bakal.Tick(now)
	if err := w.bakal.EventError(); err != nil {
		client.event(map[string]any{"kind": "bakal_source_event_pending", "raid": w.bakalRun, "error": err.Error()})
		return nil
	}
	deathPlan, err := client.tickBakalDeathReturn(now)
	if err != nil {
		client.event(map[string]any{"kind": "bakal_death_return_pending", "raid": w.bakalRun, "error": err.Error()})
		return nil
	}
	if w.bakalRecruitment != nil && w.bakalRecruitment.MemberRecoveryUntil != 0 && (w.bakal.Stage() != legion.BakalOpeningActive || !w.bakal.Recovering(now)) {
		next := *w.bakalRecruitment
		next.MemberRecoveryUntil = 0
		body, err := protocol.RaidAssignmentUpdate(next)
		if err != nil {
			return err
		}
		frames = append(frames, legion.BakalFrame{Name: "bakal_recovery_finished", ID: 578, Body: body})
		*w.bakalRecruitment = next
		client.event(map[string]any{"kind": "bakal_recovery_finished", "raid": w.bakalRun})
	}
	if w.bakal.Stage() == legion.BakalOpeningActive && w.activeDungeon != nil && w.activeDungeon.Loaded {
		boss, err := w.bakalLoadedBoss()
		if err != nil {
			return err
		}
		for _, packet := range boss {
			frames = append(frames, legion.BakalFrame{Name: packet.Name, ID: packet.ID, Body: packet.Payload})
		}
	}
	if w.bakal.NeedsCampReturn() {
		var err error
		camp, err = w.bakalCampPlan()
		if err != nil {
			client.event(map[string]any{"kind": "bakal_camp_return_pending", "raid": w.bakalRun, "error": err.Error()})
			return nil
		}
	}
	if w.bakal.Stage() == legion.BakalOpeningFailed {
		var err error
		camp, err = w.bakalCampPlan()
		if err != nil {
			client.event(map[string]any{"kind": "bakal_failure_return_pending", "raid": w.bakalRun, "error": err.Error()})
			return nil
		}
		if len(frames) == 0 {
			frames = []legion.BakalFrame{{Name: "bakal_script_phase_ended", ID: legion.NotiBakalRaidState, Body: []byte(legion.BakalRaidStateEnded)}}
		}
	}
	lifecycle, err := w.bakalLifecyclePlan()
	if err != nil {
		return err
	}
	for _, p := range lifecycle {
		frames = append(frames, legion.BakalFrame{Name: p.Name, ID: p.ID, Body: p.Payload})
	}
	if len(frames) == 0 && len(deathPlan) == 0 {
		return nil
	}
	if err := client.sendPlan(append(append(camp, bakalOutgoing(frames)...), deathPlan...), client.logWorldResponseBody); err != nil {
		client.event(map[string]any{"kind": "bakal_tick_write_failed", "error": err.Error()})
		return err
	}
	if w.bakal.NeedsCampReturn() {
		w.bakal.AcknowledgeCampReturn()
		w.activeDungeon = nil
		w.bakalCurrent = 0
		w.bakalLocation = legion.BakalCampLocation
		w.deathSent = nil
		w.drops = nil
		w.resetCards()
	}
	if w.bakal.Stage() == legion.BakalOpeningEnded || w.bakal.Stage() == legion.BakalOpeningFailed {
		w.activeDungeon = nil
		w.bakalCurrent = 0
		w.bakalLocation = legion.BakalCampLocation
		w.deathSent = nil
		w.drops = nil
		w.resetCards()
		if err := w.syncBakalWeeklyQuota(now, client.output.send, client.event); err != nil {
			return err
		}
	}
	if w.bakal.Stage() == legion.BakalOpeningFailed {
		w.bakalFailureNotified = true
	}

	return nil
}

// bakalSettleSplice freezes the week ledger and splices the full-bag N13
// (loot.Service.Bootstrap) into the settlement burst before it goes out.
func (w *worldSession) bakalSettleSplice(now time.Time) error {
	id, ok := w.bakal.SettlementIdentity(now)
	if !ok {
		return fmt.Errorf("Bakal settlement identity unavailable")
	}
	if w.bakalRewards == nil || w.loot == nil {
		return fmt.Errorf("Bakal clear requires durable source rewards")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wireID := w.role.WireID
	plan, saved, err := w.bakalRewards.Freeze(ctx, w.role, w.bakalRun, id.ClearedCount, now)
	if err != nil {
		return err
	}
	saved.WireID = wireID
	w.role = saved
	// Handover raid_bakal_rewards.go:31/39/47: Freeze -> Claim -> Bootstrap.
	if plan.Eligible {
		_, saved, err = w.bakalRewards.Claim(ctx, w.role, w.bakalRun, now)
		if err != nil {
			return err
		}
		saved.WireID = wireID
		w.role = saved
	}
	body, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return err
	}
	w.bakal.SetRewardInventory(body)
	return nil
}

// bakalOutgoing converts the state machine's frames to the gateway plan. The
// wire layer owns the kind: command envelopes ack with kind 1, notices 0.
func bakalOutgoing(frames []legion.BakalFrame) []outboundPacket {
	packets := make([]outboundPacket, 0, len(frames)+1)
	for _, f := range frames {
		packets = append(packets, outboundPacket{f.Name, 0, f.ID, f.Body})
	}
	return packets
}
