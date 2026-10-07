package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/raid"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// Diagnostic candidate only until native packet-consumption and scene loading
// are verified. DFO_RAID_OPEN_EVENTS_PROBE also enables the F7 availability
// candidate. Never answer START_RAID with success before phase support exists.
func (c *gameConnection) dispatchRaidEntrance(q *clientRequest) dispatchAction {
	if os.Getenv("DFO_RAID_OPEN_EVENTS_PROBE") != "1" || q.frame.Type != 1 {
		return dispatchNext
	}
	switch q.frame.ID {
	case 12, 650, 656, 657, 658, 661, 1353, 2121, 2070, 2071, 2072, 2073, 2074, 2089, 2062:
	default:
		return dispatchNext
	}
	if !q.verified || !c.bootstrapped || c.worldState == nil || c.gatewayRuntime == nil {
		// 诊断（2026-10-05 巴卡尔实测）：CMD12 到达后被此处静默吞掉，无法区分是哪个
		// 前置条件不满足。对该命令表内的 id 打一条跳过原因，其余行为不变。
		c.event(map[string]any{"kind": "raid_handler_skipped", "id": q.frame.ID,
			"verified": q.verified, "bootstrapped": c.bootstrapped,
			"world": c.worldState != nil, "runtime": c.gatewayRuntime != nil})
		return dispatchHandled
	}
	w := c.worldState
	if _, ok := protocol.RaidKindForChannel(w.channelType); !ok {
		return dispatchNext
	}
	refuse := func(reason string) dispatchAction {
		c.event(map[string]any{"kind": "raid_entry_refused", "id": q.frame.ID, "character_id": w.role.ID, "reason": reason})
		if c.output.send(1, q.frame.ID, protocol.Refusal(19)) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	switch q.frame.ID {
	case 2070:
		next, plan, err := w.bakalNativeRoomWarp(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		plan = append([]outboundPacket{{"bakal_script_warp_ack", 1, 2070, []byte{1}}}, plan...)
		if c.sendPlan(plan, c.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		w.activeDungeon = next
		c.event(map[string]any{"kind": "bakal_script_warp", "dungeon": next.Definition.ID, "map": next.Room.Map})
		return dispatchHandled
	case 2074:
		area, err := protocol.DecodeBakalCampReturn115(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		plan, err := w.bakalNativeCampReturn(area)
		if err != nil {
			return refuse(err.Error())
		}
		if c.sendPlan(plan, c.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 2071, 2072:
		if w.channelType != 82 || !w.raidWaiting || w.bakalOpening == nil || !w.bakalOpening.Active() || w.activeDungeon == nil {
			return refuse("Bakal report outside owned combat")
		}
		var err error
		if q.frame.ID == 2071 {
			slot, value, e := protocol.DecodeBakalHealthReport115(q.plaintext)
			if e != nil {
				return refuse(e.Error())
			}
			if _, e := w.bakalOpening.AuthorizeSlot(slot, w.activeDungeon.Definition.ID, w.activeDungeon.Room.Map, time.Now()); e != nil {
				return refuse(e.Error())
			}
			err = w.bakalOpening.ReportHealth(slot, value, time.Now())
		} else {
			value, e := protocol.DecodeBakalBuffUse115(q.plaintext)
			if e != nil {
				return refuse(e.Error())
			}
			err = w.bakalOpening.UseRaidBuff(uint32(value), time.Now())
		}
		if err != nil {
			return refuse(err.Error())
		}
		plan, err := w.bakalScriptPlan(w.bakalOpening.DrainEffects())
		if err != nil {
			return refuse(err.Error())
		}
		plan = append([]outboundPacket{{"bakal_report_ack", 1, q.frame.ID, []byte{1}}}, plan...)
		if c.sendPlan(plan, c.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 2062:
		// 巴卡尔入口自动补建分队（2026-10-05 实测）：客户端未必主动发 CMD12（分队注册），
		// 缺它入口永远被拒。在"建队+开场均已就绪、仅缺分队"时，按 23:17 会话观测到的
		// CMD12 报文形态（默认名 "Party : 1"）就地合成并注册，然后放行 2062 继续走
		// 常规入口处理（bakalPortalSelection 此时 soloPartyReady 已置位）。
		if w.channelType == 82 && w.raidWaiting && w.bakalOpening != nil && !w.soloPartyReady && w.activeDungeon == nil {
			team, owned := c.raidTeams.get(w.role.ID, c.channel)
			if owned && team.Recruitment.ID == c.ownedRaidID && team.Recruitment.Leader == w.role.WireID && team.Recruitment.MemberCount == 1 && w.characters != nil {
				// 按 RaidSoloSubParty115 的解码契约合成完整 61B 报文：
				// 00 00 + u32 标题长 + "Party : <分配位>" + 30B 选项块(want) + 16B 零填充。
				title := fmt.Sprintf("Party : %d", team.Recruitment.MemberPosition)
				syn := []byte{0x00, 0x00}
				syn = binary.LittleEndian.AppendUint32(syn, uint32(len(title)))
				syn = append(syn, title...)
				syn = append(syn, []byte{4, 255, 255, 255, 255, 5, 0, 0, 0, 0, 0, 0, 7, 7, 7, 7, 7, 7, 7, 7, 255, 255, 255, 255, 0, 0, 0, 0, 0, 0}...)
				syn = append(syn, make([]byte, 3)...) // real frame: 48B plain = 6+9+30+3 tail (padding requires <16)
				party, err := protocol.RaidSoloSubParty115(w.role.WireID, w.characters.ChannelContext, team.Recruitment.MemberPosition, syn)
				if err != nil {
					return refuse(err.Error())
				}
				basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
				if err != nil {
					return refuse(err.Error())
				}
				detail, err := w.characters.EntryAddition(w.role)
				if err != nil {
					return refuse(err.Error())
				}
				restore, err := inventory.NonAvatarWornSpaceUpdate(w.role.State)
				if err != nil {
					return refuse(err.Error())
				}
				plan := []outboundPacket{{"bakal_subparty_actor", 0, 2, basic}, {"bakal_subparty_details", 0, 2, detail}}
				if len(restore) != 0 {
					plan = append(plan, outboundPacket{"bakal_subparty_worn_restored", 0, 14, restore})
				}
				plan = append(plan, outboundPacket{"bakal_subparty_created", 0, 9, party}, outboundPacket{"bakal_subparty_ack", 1, 12, []byte{1}})
				if err := c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
					return dispatchClose
				}
				w.soloPartyReady = true
				c.event(map[string]any{"kind": "bakal_subparty_autoprovisioned", "character_id": w.role.ID})
			}
		}
		return dispatchNext
	case 12:
		team, owned := c.raidTeams.get(w.role.ID, c.channel)
		if w.channelType != 82 || !w.raidWaiting || !owned || team.Recruitment.ID != c.ownedRaidID || team.Recruitment.Leader != w.role.WireID || team.Recruitment.MemberCount != 1 || w.bakalOpening == nil || w.activeDungeon != nil || w.characters == nil {
			// 诊断：逐条件展开，定位分队注册被拒的确切前置（2026-10-05 巴卡尔实测）。
			c.event(map[string]any{"kind": "bakal_subparty_guard", "channel_type": w.channelType,
				"raid_waiting": w.raidWaiting, "owned": owned,
				"raid_id_match": owned && team.Recruitment.ID == c.ownedRaidID,
				"leader_match":  owned && team.Recruitment.Leader == w.role.WireID,
				"member_count": func() int {
					if owned {
						return int(team.Recruitment.MemberCount)
					}
					return -1
				}(),
				"opening": w.bakalOpening != nil, "active_dungeon": w.activeDungeon != nil,
				"characters": w.characters != nil})
			return refuse("Bakal subparty requires the real owned preparing member")
		}
		party, err := protocol.RaidSoloSubParty115(w.role.WireID, w.characters.ChannelContext, team.Recruitment.MemberPosition, q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if err != nil {
			return refuse(err.Error())
		}
		detail, err := w.characters.EntryAddition(w.role)
		if err != nil {
			return refuse(err.Error())
		}
		// Native mode1 USERINFO clears worn slots absent from its visual
		// projection. Restore actual non-avatar rows afterwards, as in the
		// established Clone reattach flow; avatar/creature rows stay owned by it.
		restore, err := inventory.NonAvatarWornSpaceUpdate(w.role.State)
		if err != nil {
			return refuse(err.Error())
		}
		plan := []outboundPacket{{"bakal_subparty_actor", 0, 2, basic}, {"bakal_subparty_details", 0, 2, detail}}
		if len(restore) != 0 {
			plan = append(plan, outboundPacket{"bakal_subparty_worn_restored", 0, 14, restore})
		}
		plan = append(plan, outboundPacket{"bakal_subparty_created", 0, 9, party}, outboundPacket{"bakal_subparty_ack", 1, 12, []byte{1}})
		if err := c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
			return dispatchClose
		}
		w.soloPartyReady = true
		return dispatchHandled
	case 2073:
		if err := protocol.DecodeRaidStartRequest(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		if w.channelType != 82 || !w.raidWaiting || w.bakalOpening == nil || !w.bakalOpening.Active() || w.activeDungeon == nil {
			return refuse("Bakal loading completion outside active owned dungeon")
		}
		symbols, err := w.bakalEnterSymbolPlan(w.activeDungeon, true)
		if err != nil {
			return refuse(err.Error())
		}
		spawn, err := w.bakalLoadedBoss(w.activeDungeon, time.Now())
		if err != nil {
			return refuse(err.Error())
		}
		plan := append([]outboundPacket{{"bakal_loading_ack", 1, 2073, []byte{1}}}, symbols...)
		plan = append(plan, spawn...)
		if err = c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
			return dispatchClose
		}
		w.activeDungeon.Loaded = true
		c.event(map[string]any{"kind": "bakal_loaded", "dungeon": w.activeDungeon.Definition.ID, "map": w.activeDungeon.Room.Map, "spawn_packets": len(spawn)})
		return dispatchHandled
	case 650:
		if _, err := protocol.DecodeRaidUpdateControl(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		// CMD650 is entry-cost information, not recruitment subscription.
		// Native NOTI585 (144ce24f0) clears both query-pending flags,
		// including +230. An ACK alone leaves entry-cost queries pending. No cost records are advertised until the PVF
		// cost/inventory projection is available; an empty vector is valid.
		if c.output.send(0, 585, protocol.RaidEmptyEntryCostInfo()) != nil || c.output.send(1, 650, []byte{1}) != nil {
			return dispatchClose
		}
		c.event(map[string]any{"kind": "raid_entry_cost_info_sent", "records": 0, "id": 650})
		return dispatchHandled
	case 2121:
		if _, err := protocol.DecodeRaidUpdateControl(q.plaintext); err != nil {
			return refuse(err.Error())
		}
		if c.output.send(1, 2121, []byte{1}) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 661:
		if w.bakalOpening != nil {
			return refuse("raid assignments are frozen after start")
		}
		actor, position, err := protocol.DecodeRaidAssignment(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		rules, ok := c.raidEntrances[w.channelType]
		if !ok || !w.raidWaiting {
			return refuse("not in owned raid waiting room")
		}
		team, err := c.raidTeams.assign(w.role.ID, c.channel, actor, position, rules.MemberMax)
		if err != nil {
			return refuse(err.Error())
		}
		update, err := protocol.RaidAssignmentUpdate(team.Recruitment)
		if err != nil {
			return refuse(err.Error())
		}
		if c.output.send(0, 578, update) != nil || c.output.send(1, 661, protocol.RaidAssignmentSuccess()) != nil {
			return dispatchClose
		}
		c.event(map[string]any{"kind": "raid_member_assigned", "raid_id": team.Recruitment.ID, "actor": actor, "position": position})
		return dispatchHandled
	case 656:
		req, err := protocol.DecodeRaidCreateRequest(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		kind, ok := protocol.RaidKindForChannel(w.channelType)
		if !ok || kind != req.Kind {
			return refuse("raid kind does not match channel")
		}
		rules, ok := c.raidEntrances[w.channelType]
		if !ok {
			return refuse("PVF raid waiting room is not bound")
		}
		if w.activeDungeon != nil || w.selectingDungeon || w.role.ID <= 0 || w.role.WireID == 0 {
			return refuse("character is not available in town")
		}
		if uint32(w.level) < rules.Waiting.MinimumLevel {
			return refuse("waiting-room level requirement")
		}
		var state character.State
		if err := json.Unmarshal(w.role.State, &state); err != nil {
			return refuse("invalid saved character state")
		}
		advance, err := state.WireAdvancement()
		if err != nil {
			return refuse(err.Error())
		}
		if rules.Waiting.AreaID == 0 || rules.Waiting.AreaID > 65535 {
			return refuse("native waiting area exceeds member wire field")
		}
		recruitment := protocol.RaidRecruitment{Create: req, Leader: w.role.WireID, LeaderProfession: w.role.Profession, LeaderAdvancement: advance, LeaderLevel: w.level, LeaderName: w.role.Name, MemberCount: 1, MemberArea: uint16(rules.Waiting.AreaID)}
		team, err := c.raidTeams.create(w.role.ID, c.channel, c.channelCfg.ServerID, recruitment, w.state.Position)
		if err != nil {
			return refuse(err.Error())
		}
		old := w.state.Position
		c.raidOwnerRole, c.ownedRaidID = w.role.ID, team.Recruitment.ID
		// Failure removes precisely this newly allocated team; another owner's
		// team can never be removed by a failed request or another connection.
		fail := func(e error) dispatchAction {
			c.raidTeams.leave(w.role.ID, team.Recruitment.ID)
			w.state.Position = old
			w.raidWaiting = false
			w.enterArea()
			c.event(map[string]any{"kind": "raid_create_delivery_error", "error": e.Error()})
			return dispatchClose
		}
		list, err := protocol.RaidOwnedDetails(team.Recruitment)
		if err != nil {
			return fail(err)
		}
		if err = c.output.send(0, 578, list); err != nil {
			return fail(err)
		}
		if err = c.output.send(1, 656, protocol.RaidCreateSuccess(team.Recruitment.ID)); err != nil {
			return fail(err)
		}
		// Use the existing verified world placement notifications. Raid waiting
		// position is session-local and must not pollute the normal town save.
		x, y := rules.Waiting.Spawn()
		w.state.Position = database.WorldPosition{Town: rules.Waiting.TownID, Area: rules.Waiting.AreaID, X: x, Y: y}
		w.raidWaiting = true
		area, err := w.userAreaPayload()
		if err != nil {
			return fail(err)
		}
		if err = c.output.send(0, 23, area); err != nil {
			return fail(err)
		}
		w.enterArea()
		if err = w.introducePeers(c.output.send); err != nil {
			return fail(err)
		}
		scene, err := w.areaPayload()
		if err != nil {
			return fail(err)
		}
		if err = c.output.send(0, 24, scene); err != nil {
			return fail(err)
		}
		c.event(map[string]any{"kind": "raid_created_waiting", "raid_id": team.Recruitment.ID, "character_id": w.role.ID, "channel_type": w.channelType, "source": rules.Path, "position": w.state.Position, "member_max": rules.MemberMax, "start_minimum": rules.StartMinimum})
		return dispatchHandled
	case 1353:
		mode, id, err := protocol.DecodeRaidInfoRequest(q.plaintext)
		if err != nil {
			return refuse(err.Error())
		}
		team, ok := c.raidTeams.get(w.role.ID, c.channel)
		if !ok || team.Recruitment.ID != id || mode < 1 || mode > 2 {
			return refuse("raid info request does not name owned team")
		}
		var list []byte
		if mode == 1 {
			list, err = protocol.RaidMemberInfoSuccess(team.Recruitment)
		} else {
			list, err = protocol.RaidFullInfoSuccess(team.Recruitment)
		}
		if err != nil {
			return refuse(err.Error())
		}
		if c.output.send(1, 1353, list) != nil {
			return dispatchClose
		}
		// POST_CHARAC_INFO reader 146d5a3c0 consumes one byte and calls
		// 144cdb100 to stamp raid-manager +200. Without it, 144cf40e0
		// keeps the menu in its loading guard before any new query is sent.
		if c.output.send(0, 1220, []byte{0}) != nil {
			return dispatchClose
		}
		c.event(map[string]any{"kind": "raid_owned_details_sent", "id": 1353, "raid_id": id, "actor": w.role.WireID, "members": 1, "position": team.Recruitment.MemberPosition, "mode": mode, "plain_bytes": len(list)})
		return dispatchHandled
	case 657:
		// Sender 144cfb300 writes a client context u16 through 146d76180.
		// The observed value is 2 even on channel 82; it is not our channel ID.
		// Authorize leave against the authenticated role's connection-owned team.
		_, err := protocol.DecodeRaidLeaveRequest(q.plaintext)
		if err != nil {
			return refuse(fmt.Sprintf("invalid leave raid body: %d", len(q.plaintext)))
		}
		team, ok := c.raidTeams.get(w.role.ID, c.channel)
		if !ok {
			return refuse("no owned raid")
		}
		// 144cda220 reads an additional u8 on success (zero = normal leave).
		if c.output.send(1, 657, protocol.RaidLeaveSuccess()) != nil {
			return dispatchClose
		}
		c.raidTeams.leave(w.role.ID, team.Recruitment.ID)
		c.raidOwnerRole, c.ownedRaidID = 0, 0
		w.raidWaiting = false
		w.bakalOpening, w.bakalRules = nil, nil
		w.bakalTown = 0
		w.state.Position = team.Origin
		area, err := w.userAreaPayload()
		if err != nil || c.output.send(0, 23, area) != nil {
			return dispatchClose
		}
		w.enterArea()
		if w.introducePeers(c.output.send) != nil {
			return dispatchClose
		}
		scene, err := w.areaPayload()
		if err != nil || c.output.send(0, 24, scene) != nil {
			return dispatchClose
		}
		list, _ := protocol.RaidRecruitmentList(nil)
		if c.output.send(0, 577, list) != nil {
			return dispatchClose
		}
		return dispatchHandled
	case 658, 2089:
		var choice byte
		var decodeErr error
		if q.frame.ID == 2089 {
			choice, decodeErr = protocol.DecodeRaidStartVote(q.plaintext)
		} else {
			decodeErr = protocol.DecodeRaidStartRequest(q.plaintext)
		}
		if decodeErr != nil {
			return refuse(decodeErr.Error())
		}
		team, owned := c.raidTeams.get(w.role.ID, c.channel)
		rules, bound := c.raidEntrances[w.channelType]
		if !owned || !bound || !w.raidWaiting || w.channelType != 82 || rules.Bakal == nil || team.Recruitment.Create.Kind != 8 || team.Recruitment.Leader != w.role.WireID || team.Recruitment.MemberCount != 1 || w.activeDungeon != nil || w.bakalOpening != nil {
			return refuse("normal Bakal start requires its owned real waiting-room member and native phase rules")
		}
		if len(rules.Bakal.Events) > 0 {
			if err := bakalRaidAdmission(w.role, rules.Bakal, time.Now()); err != nil {
				return refuse(err.Error())
			}
		}
		var plan []outboundPacket
		if q.frame.ID == 2089 {
			if choice != 0 {
				return refuse("no owned start vote to answer")
			}
			// Native leader UI hides both vote buttons and marks the initiator
			// agreed (1419c1131). With exactly one real owned member, the
			// start request itself completes quorum; no second request follows.
			vote, err := protocol.RaidSoloStartVote(w.role.WireID, 0)
			if err != nil {
				return refuse(err.Error())
			}
			accepted, err := protocol.RaidSoloStartVote(w.role.WireID, 1)
			if err != nil {
				return refuse(err.Error())
			}
			plan = append(plan, outboundPacket{"bakal_vote_start", 0, 2343, vote}, outboundPacket{"bakal_vote_state", 0, 2344, accepted})
		}
		if team.Recruitment.MemberPosition == 0 {
			var err error
			team, err = c.raidTeams.assign(w.role.ID, c.channel, w.role.WireID, 1, rules.MemberMax)
			if err != nil {
				return refuse(err.Error())
			}
			update, err := protocol.RaidAssignmentUpdate(team.Recruitment)
			if err != nil {
				return refuse(err.Error())
			}
			plan = append(plan, outboundPacket{"bakal_real_member_assigned", 0, 578, update})
		}
		opening, err := raid.PrepareBakalOpening(rules, []raid.Member{{Actor: w.role.WireID, Position: uint32(team.Recruitment.MemberPosition)}}, time.Now())
		if err != nil {
			return refuse(err.Error())
		}
		// The native preparation state is 1; registered NOTI581 starts
		// the source-defined countdown. Activation happens in the serial loop.
		plan = append(plan, outboundPacket{"bakal_preparing", 0, 574, []byte{1, 0}}, outboundPacket{"bakal_countdown", 0, 581, []byte{}}, outboundPacket{"bakal_start_ack", 1, q.frame.ID, []byte{1}})
		if err := c.sendPlan(plan, c.logCharacterResponseBody); err != nil {
			return dispatchClose
		}
		w.bakalOpening, w.bakalRules = opening, rules.Bakal
		if err := c.raidTeams.transition(w.role.ID, team.Recruitment.ID, 0, 1); err != nil {
			return dispatchClose
		}
		w.bakalParty = uint32(team.Recruitment.MemberPosition)
		w.bakalTown = rules.Waiting.TownID
		c.event(map[string]any{"kind": "bakal_prepared", "raid_id": team.Recruitment.ID, "members": 1, "party": w.bakalParty, "ready_at": opening.ReadyAt(), "source": rules.Path})
		return dispatchHandled
	}
	return dispatchNext
}

// Repeated candidate validation must not reset earned weekly rewards or the
// character's progress. Only the weekly entry cap can be bypassed, and only
// by an explicit diagnostic profile. Source reward eligibility is unchanged.
func bakalRaidAdmission(role database.Character, rules *catalog.BakalRaidRules, now time.Time) error {
	err := workflow.BakalRaidAdmission(role, rules, now)
	if errors.Is(err, workflow.ErrBakalWeeklyClearLimit) && os.Getenv("DFO_RAID_OPEN_EVENTS_PROBE") == "1" && os.Getenv("DFO_BAKAL_MODE") == "unlimited" {
		return nil
	}
	return err
}
