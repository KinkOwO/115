package main

import (
	"bytes"
	"context"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

func (client *gameConnection) dispatchStoryAndAdvancement(requestData *clientRequest) dispatchAction {
	if requestData.frame.Type == 1 && requestData.frame.ID == 806 && client.bootstrapped && requestData.verified && client.worldState != nil {
		// CMD806 共用送礼(p[0]=0)与剧情角色染色(p[0]=1)。giveFavor
		// 内部按 p[0] 分流：送礼扣材料+加点数；染色回 0x01+请求体
		// 回显，客户端据此本地写角色颜色（不弹好感度窗）。
		plan, favorErr := client.worldState.giveFavor(requestData.plaintext)
		if favorErr != nil {
			client.event(map[string]any{"kind": "npc_favor_refused", "character_id": client.selectedCharacterID, "reason": favorErr.Error(), "plain_hex": hex.EncodeToString(requestData.plaintext)})
			if e := client.output.send(1, 806, protocol.Refusal(4)); e != nil {
				return dispatchClose
			}
			return dispatchHandled
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 2079 && client.bootstrapped && requestData.verified && client.characters != nil {
		id, err := protocol.DecodeSynopsisRead(requestData.plaintext)
		var payload []byte
		if err == nil {
			payload, err = saveSynopsisRead(client.gameStore, client.worldState, id)
		}
		if err != nil {
			client.event(map[string]any{"kind": "synopsis_read_refused", "error": err.Error()})
			return dispatchHandled
		}
		if err = client.output.send(0, 2310, payload); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "synopsis_table_info_sent", "character_id": client.worldState.role.ID, "synopsis_id": id, "attempt": "2/3", "plain_hex": hex.EncodeToString(payload)})
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 1438 && client.bootstrapped && requestData.verified && client.characters != nil {
		// CMD1438 STORY_DIGEST_UPDATE: the client reports the opening
		// recap movie finished (empty payload). Must be matched before
		// the 1417 branch: it shares the same request family, and a
		// later placement would let 1417 swallow the report so the
		// digest level never advances.
		if err := saveStoryDigest(client.gameStore, client.worldState, requestData.plaintext); err != nil {
			client.event(map[string]any{"kind": "story_digest_save_error", "error": err.Error()})
		} else {
			client.event(map[string]any{"kind": "story_digest_saved", "character_id": client.worldState.role.ID, "level": client.worldState.level})
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 1417 && client.bootstrapped && requestData.verified && client.characters != nil {
		if err := cinematicSkip(client.gameStore, client.worldState, requestData.plaintext); err != nil {
			client.event(map[string]any{"kind": "cinematic_skip_refused", "error": err.Error()})
		} else {
			client.event(map[string]any{"kind": "cinematic_skip_saved", "character_id": client.worldState.role.ID})
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && requestData.frame.ID == 2177 && client.bootstrapped && requestData.verified && client.characters != nil {
		packets, err := awakenCharacter(client.characters, client.worldState, requestData.plaintext, client.keys)
		if err != nil {
			client.event(map[string]any{"kind": "awakening_refused", "error": err.Error()})
			if err = client.output.send(1, 2177, protocol.Refusal(4)); err != nil {
				return dispatchClose
			}
		} else if err = client.output.writePrepared(packets, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if requestData.frame.Type == 1 && (requestData.frame.ID == 1881 || requestData.frame.ID == 777) && client.bootstrapped && requestData.verified && client.characters != nil {
		// 1881 = CHANGE_GROW_TYPE (首次转职), 777 = RE_GROWUP_CHANGE (随时更换职业).
		// 同一 grow-type 家族，请求体与响应格式一致，仅响应 opcode 不同。
		packets, err := changeGrowType(client.characters, client.worldState, requestData.plaintext, client.keys, requestData.frame.ID)
		if err != nil {
			client.event(map[string]any{"kind": "advancement_refused", "id": requestData.frame.ID, "error": err.Error()})
			if err = client.output.send(1, requestData.frame.ID, protocol.Refusal(advancementRefusalCode(err))); err != nil {
				return dispatchClose
			}
		} else if err = client.output.writePrepared(packets, func(p preparedPacket) {
			client.event(map[string]any{"kind": p.Name, "id": p.ID, "plain_hex": hex.EncodeToString(p.Payload)})
		}); err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchSessionTransitions(requestData *clientRequest) dispatchAction {
	if client.bootstrapped && (requestData.frame.ID == 3 || requestData.frame.ID == 7 || requestData.frame.ID == 1301) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "menu_rejected", "id": requestData.frame.ID, "reason": "checksum failed"})
			return dispatchHandled
		}
		option, e := protocol.DecodeMenuRequest(requestData.frame.ID, requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "menu_rejected", "id": requestData.frame.ID, "reason": e.Error()})
			return dispatchHandled
		}
		payload := protocol.MenuLeaveSuccess()
		if requestData.frame.ID == 1301 {
			payload, e = client.worldState.returnDestination()
			if e != nil {
				client.event(map[string]any{"kind": "village_return_refused", "reason": e.Error()})
				// Native refusal clears the manager wait at143ca6b3f.
				payload = protocol.Refusal(4)
			}
		}
		if e = client.output.send(1, requestData.frame.ID, payload); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "menu_response", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "option": option, "plain_hex": hex.EncodeToString(payload)})
		if requestData.frame.ID == 3 || requestData.frame.ID == 7 {
			client.selectedCharacterID = 0
			client.selectedBasic, client.selectedAddition = nil, nil
			if client.worldState != nil {
				client.worldState.departArea()
			}
			clearSelectedWorld(client.worldState)
			if requestData.frame.ID == 3 {
				client.bootstrapped = false
				client.event(map[string]any{"kind": "menu_exit_session_closed", "peer": client.peer, "option": option})
				return dispatchClose
			}
		}
		return dispatchHandled
	}
	// The exit button emits CMD1302 and CMD2285 in the same
	// millisecond; 1302 is an upload with no receive handler, 2285
	// is the content-briefing report whose own handler fills the
	// exit window. Answering it is what gives the in-game exit
	// button something to present; the client used to get silence.

	return dispatchNext
}

func (client *gameConnection) dispatchCharacterSkills(requestData *clientRequest) dispatchAction {
	if client.bootstrapped && requestData.frame.ID == 2285 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "content_briefing_rejected", "reason": "checksum failed"})
			return dispatchHandled
		}
		payload := protocol.ExitContentBriefingDefaults()
		if err := client.output.send(1, 2285, payload); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "content_briefing_response", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "bytes": len(payload), "plain_hex": hex.EncodeToString(payload)})
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 331 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "skill_commands_rejected", "reason": "checksum failed", "character_id": client.selectedCharacterID})
			return dispatchHandled
		}
		if client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID {
			client.event(map[string]any{"kind": "skill_commands_rejected", "reason": "character selection mismatch", "character_id": client.selectedCharacterID})
			return dispatchHandled
		}
		count, e := client.skillState.saveCommands(client.characters, client.worldState, requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "skill_commands_rejected", "reason": e.Error(), "character_id": client.selectedCharacterID})
		} else {
			client.event(map[string]any{"kind": "skill_commands_saved", "character_id": client.selectedCharacterID, "count": count})
			restore, restoreErr := client.characters.EntrySkills(client.worldState.role)
			if restoreErr != nil {
				client.event(map[string]any{"kind": "skill_commands_refresh_failed", "reason": restoreErr.Error(), "character_id": client.selectedCharacterID})
			} else {
				if e = client.output.send(0, 19, restore); e != nil {
					return dispatchClose
				}
				client.event(map[string]any{"kind": "skill_commands_refreshed", "character_id": client.selectedCharacterID, "id": 19})
				preset, presetErr := client.characters.SkillPresetInfo(client.worldState.role)
				if presetErr != nil {
					client.event(map[string]any{"kind": "skill_preset_refresh_failed", "reason": presetErr.Error(), "character_id": client.selectedCharacterID})
				} else if len(preset) > 0 {
					if e = client.output.send(0, 2758, preset); e != nil {
						return dispatchClose
					}
					client.event(map[string]any{"kind": "skill_preset_restored_after_commands", "character_id": client.selectedCharacterID, "id": 2758})
				}
				combo, comboErr := client.characters.ComboSkillInfoNotify(client.worldState.role)
				if comboErr != nil {
					client.event(map[string]any{"kind": "combo_skill_info_refresh_failed", "character_id": client.selectedCharacterID, "error": comboErr.Error()})
				} else if len(combo) > 0 {
					if e = client.output.send(0, 433, combo); e != nil {
						return dispatchClose
					}
					client.event(map[string]any{"kind": "combo_skill_info_restored_after_commands", "character_id": client.selectedCharacterID, "type": 0, "id": 433, "plain_hex": hex.EncodeToString(combo)})
				}
			}
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 1421 {
		if !requestData.verified || client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID {
			client.event(map[string]any{"kind": "buff_enhancement_rejected", "character_id": client.selectedCharacterID, "reason": "checksum or character selection mismatch"})
			return dispatchHandled
		}
		notify, saveErr := client.buffEnhancementState.save(client.characters, client.worldState, requestData.plaintext)
		if saveErr != nil {
			client.event(map[string]any{"kind": "buff_enhancement_rejected", "character_id": client.selectedCharacterID, "reason": saveErr.Error()})
			if e := client.output.send(1, 1421, protocol.Refusal(0)); e != nil {
				return dispatchClose
			}
			// CMD1421 failure rolls back the optimistic item edit; restore
			// the authoritative complete selection after that rollback.
			var restoreErr error
			notify, restoreErr = client.characters.BuffEnhancementRestore(client.worldState.role)
			if restoreErr != nil {
				return dispatchHandled
			}
		} else {
			client.event(map[string]any{"kind": "buff_enhancement_saved", "character_id": client.selectedCharacterID, "plain_hex": hex.EncodeToString(requestData.plaintext)})
			if e := client.output.send(1, 1421, []byte{1}); e != nil {
				return dispatchClose
			}
		}
		if e := client.output.send(0, 1361, notify); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "buff_enhancement_replied", "character_id": client.selectedCharacterID, "type": 0, "id": 1361, "plain_hex": hex.EncodeToString(notify)})
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && (requestData.frame.ID == 500 || requestData.frame.ID == 502) {
		if !requestData.verified || client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID {
			client.event(map[string]any{"kind": "combo_skill_info_rejected", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "reason": "checksum or character selection mismatch"})
			return dispatchHandled
		}
		req, saveErr := client.comboState.save(client.characters, client.worldState, requestData.frame.ID, requestData.plaintext)
		if saveErr != nil {
			client.event(map[string]any{"kind": "combo_skill_info_rejected", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "reason": saveErr.Error()})
			return dispatchHandled
		}
		client.event(map[string]any{"kind": "combo_skill_info_saved", "id": requestData.frame.ID, "character_id": client.selectedCharacterID, "cells": req.Cells, "plain_hex": hex.EncodeToString(requestData.plaintext)})
		if requestData.frame.ID == 500 {
			notify, encodeErr := protocol.EncodeComboSkillInfoNotify(req)
			if encodeErr != nil {
				client.event(map[string]any{"kind": "combo_skill_info_reply_failed", "character_id": client.selectedCharacterID, "error": encodeErr.Error()})
				return dispatchHandled
			}
			if !bytes.Equal(notify, client.comboState.lastNotify) {
				if sendErr := client.output.send(0, 433, notify); sendErr != nil {
					return dispatchClose
				}
				client.comboState.lastNotify = notify
				client.event(map[string]any{"kind": "combo_skill_info_replied", "character_id": client.selectedCharacterID, "type": 0, "id": 433, "plain_hex": hex.EncodeToString(notify)})
			}
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 527 {
		if !requestData.verified || client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID {
			return dispatchHandled
		}
		ack, saveErr := client.cubeContractState.save(client.worldState, requestData.plaintext)
		if saveErr != nil {
			client.event(map[string]any{"kind": "cube_contract_selection_rejected", "character_id": client.selectedCharacterID, "reason": saveErr.Error()})
			ack = protocol.Refusal(0)
		}
		if e := client.output.send(1, 527, ack); e != nil {
			return dispatchClose
		}
		if saveErr == nil {
			client.event(map[string]any{"kind": "cube_contract_selection_saved", "character_id": client.selectedCharacterID, "selection": ack[2]})
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && (requestData.frame.ID == 28 || requestData.frame.ID == 29 || requestData.frame.ID == 483 || requestData.frame.ID == 2179 || requestData.frame.ID == 2346 || requestData.frame.ID == 2347) {
		if !requestData.verified {
			return dispatchHandled
		}
		plan, e := client.skillState.handle(client.characters, client.worldState, requestData.frame.ID, requestData.plaintext, requestData.frame.Raw)
		if e != nil {
			client.event(map[string]any{"kind": "skill_refused", "id": requestData.frame.ID, "reason": e.Error()})
			plan = []outboundPacket{{"skill_refused_response", 1, requestData.frame.ID, protocol.Refusal(4)}}
		}
		if client.sendPlan(plan, client.logCharacterResponseBody) != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	// Teaching/notice read reports. CMD469 is tree 1 (INFORM_NOTICE,
	// payload = u32 notice id); CMD495 is tree 2 (INFORM_NOTICE_2ND)
	// with two shapes: the bulletin-board opcode 0x3e (62) and the
	// teaching frame's (u32 id, u8 value). 62 doubles as the
	// "player chose Manual Setup" mark, so persisting it here is what
	// stops the third-awakening teaching frame from re-popping once the
	// guide has been answered. Both are acknowledged with {1,0}.
	if client.characters != nil && client.bootstrapped && client.selectedCharacterID != 0 && (requestData.frame.ID == 469 || requestData.frame.ID == 495) {
		if !requestData.verified {
			return dispatchHandled
		}
		if client.worldState == nil || client.worldState.role.ID != client.selectedCharacterID {
			client.event(map[string]any{"kind": "notice_seen_rejected", "reason": "character selection mismatch", "character_id": client.selectedCharacterID})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var persistErr error
		switch requestData.frame.ID {
		case 469:
			if len(requestData.plaintext) < 4 {
				persistErr = fmt.Errorf("short notice-seen report")
				break
			}
			nid := binary.LittleEndian.Uint32(requestData.plaintext[:4])
			if nid > 255 {
				persistErr = fmt.Errorf("notice id out of wire range")
				break
			}
			persistErr = client.gameStore.MarkCharacterNotice(ctx, client.developmentAccount, client.selectedCharacterID, 1, uint16(nid), true)
			if persistErr == nil {
				client.event(map[string]any{"kind": "notice_seen_persisted", "character_id": client.selectedCharacterID, "tree": 1, "notice_id": nid})
			}
		case 495:
			if len(requestData.plaintext) >= 4 && binary.LittleEndian.Uint32(requestData.plaintext[:4]) == 0x3e {
				// Bulletin-board opcode 62. Persisting 62 into tree 2
				// also covers the Manual Setup teaching mark; the
				// season-5 Anton quest chain (pre-req 3223 -> NPC15
				// 3226) is a separate flow and is not implemented here.
				persistErr = client.gameStore.MarkCharacterNotice(ctx, client.developmentAccount, client.selectedCharacterID, 2, 62, true)
				if persistErr == nil {
					client.event(map[string]any{"kind": "notice_2nd_persisted", "character_id": client.selectedCharacterID, "notice_id": 62})
				}
				break
			}
			if len(requestData.plaintext) < 5 {
				persistErr = fmt.Errorf("short notice-seen report")
				break
			}
			nid := binary.LittleEndian.Uint32(requestData.plaintext[:4])
			value := requestData.plaintext[4]
			if nid > 255 {
				persistErr = fmt.Errorf("notice id out of wire range")
				break
			}
			persistErr = client.gameStore.MarkCharacterNotice(ctx, client.developmentAccount, client.selectedCharacterID, 2, uint16(nid), value != 0)
			if persistErr == nil {
				if value != 0 {
					client.event(map[string]any{"kind": "notice_2nd_seen_persisted", "character_id": client.selectedCharacterID, "notice_id": nid})
				} else {
					client.event(map[string]any{"kind": "notice_2nd_seen_removed", "character_id": client.selectedCharacterID, "notice_id": nid})
				}
			}
		}
		cancel()
		if persistErr != nil {
			client.event(map[string]any{"kind": "notice_seen_rejected", "character_id": client.selectedCharacterID, "reason": persistErr.Error()})
			return dispatchHandled
		}
		if e := client.output.send(1, requestData.frame.ID, []byte{1, 0}); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "notice_seen_ack", "character_id": client.selectedCharacterID, "id": requestData.frame.ID})
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchCharacterNotice(requestData *clientRequest) dispatchAction {
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 143 {
		if !requestData.verified || client.selectedCharacterID == 0 {
			client.event(map[string]any{"kind": "tutorial_rejected", "reason": "invalid checksum or no selected character"})
			return dispatchHandled
		}
		r, e := protocol.DecodeTutorialChange(requestData.plaintext)
		if e != nil {
			client.event(map[string]any{"kind": "tutorial_rejected", "reason": e.Error()})
			return dispatchHandled
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		e = client.gameStore.SaveTutorialFlag(ctx, client.developmentAccount, client.selectedCharacterID, r.Index, r.Completed)
		cancel()
		if e != nil {
			client.event(map[string]any{"kind": "tutorial_save_error", "error": e.Error()})
			return dispatchHandled
		}
		if e = client.output.send(1, 143, protocol.TutorialChangeSaved()); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "tutorial_flag_saved", "character_id": client.selectedCharacterID, "index": r.Index, "completed": r.Completed, "rewards_granted": false})
		return dispatchHandled
	}

	return dispatchNext
}

func (client *gameConnection) dispatchRoster(requestData *clientRequest) dispatchAction {
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 295 {
		if !requestData.verified {
			client.event(map[string]any{"kind": "character_slot_rejected", "error": "request checksum or cipher rejected"})
			return dispatchClose
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var err error
		if client.selectedCharacterID != 0 {
			err = errors.New("character slot change requires character selection screen")
		} else {
			err = client.characters.ChangeSlot(ctx, client.developmentAccount, requestData.plaintext)
		}
		cancel()
		payload := protocol.CharacterSlotSuccess()
		if err != nil {
			client.event(map[string]any{"kind": "character_slot_rejected", "error": err.Error()})
			payload = protocol.Refusal(19)
		} else {
			client.event(map[string]any{"kind": "character_slot_saved", "account_id": client.developmentAccount, "plain_hex": hex.EncodeToString(requestData.plaintext)})
		}
		if e := client.output.send(1, 295, payload); e != nil {
			return dispatchClose
		}
		// The native drag already updates its local maps. Do not reload
		// the roster between the packets of a multi-cell drag operation.
		// A refused operation leaves those maps ahead of storage; close
		// this session so it cannot select/archive a different character.
		if err != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 637 {
		if requestData.verified && len(requestData.plaintext) == 0 && client.selectedCharacterID == 0 {
			// 145250040 reads the count of pending delayed deletions.
			// Local archival is immediate, so no pending timer rows.
			if e := client.output.send(1, 637, []byte{1, 0}); e != nil {
				return dispatchClose
			}
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && (requestData.frame.ID == 433 || requestData.frame.ID == 848) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "roster_followup_rejected", "id": requestData.frame.ID, "error": "checksum or cipher rejected"})
			return dispatchHandled
		}
		var payload []byte
		var e error
		if requestData.frame.ID == 433 {
			payload, e = protocol.EmptyMercenaryInfo(requestData.plaintext)
		} else {
			payload, e = protocol.ProbeRosterCounters(requestData.plaintext)
		}
		if e != nil {
			client.event(map[string]any{"kind": "roster_followup_rejected", "id": requestData.frame.ID, "error": e.Error()})
			return dispatchHandled
		}
		encrypted, e := wire.EncryptPayload(client.keys, requestData.frame.ID, payload)
		if e != nil {
			client.event(map[string]any{"kind": "roster_followup_error", "error": e.Error()})
			return dispatchClose
		}
		response, e := wire.ServerFrame(1, requestData.frame.ID, encrypted)
		if e != nil {
			return dispatchClose
		}
		if e = client.output.writeRaw(response); e != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "roster_followup_response", "id": requestData.frame.ID, "hex": hex.EncodeToString(response)})
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && requestData.frame.ID == 1725 {
		if !requestData.verified || client.selectedCharacterID != 0 {
			client.event(map[string]any{"kind": "roster_background_rejected", "error": "背景选择需要有效校验及选角状态"})
			return dispatchHandled
		}
		req, e := character.DecodeRosterBackgroundSelect(requestData.plaintext)
		if e == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			_, e = client.gameStore.SelectRosterBackground(ctx, client.developmentAccount, req.Page, req.Background)
			cancel()
		}
		if e != nil {
			client.event(map[string]any{"kind": "roster_background_rejected", "error": e.Error()})
		} else {
			client.event(map[string]any{"kind": "roster_background_selected", "page": req.Page, "category": req.Background.Category, "background_id": req.Background.ID})
		}
		// 原生按钮会乐观应用选择；拒绝时也恢复账号的权威状态，不编造未知 ACK。
		if e = restoreRosterBackgrounds(client.gameStore, client.developmentAccount, client.output.send, client.event); e != nil {
			return dispatchClose
		}
		return dispatchHandled
	}
	if client.characters != nil && client.bootstrapped && (requestData.frame.ID == 5 || requestData.frame.ID == 6 || requestData.frame.ID == 684 || requestData.frame.ID == 8) {
		if !requestData.verified {
			client.event(map[string]any{"kind": "character_rejected", "id": requestData.frame.ID, "error": "request checksum or cipher unsupported"})
			return dispatchClose
		}
		var userInfoMode byte
		if requestData.frame.ID == 8 {
			uid, mode, e := protocol.DecodeUserInfoRequest(requestData.plaintext)
			if e != nil {
				client.event(map[string]any{"kind": "userinfo_rejected", "error": e.Error()})
				return dispatchHandled
			}
			userInfoMode = mode
			if !((mode == 2 && uid == 0xffff) || (mode == 0 && uid == 0xffff && len(client.selectedBasic) > 0) || (mode == 1 && uid == 0xffff && len(client.selectedAddition) > 0)) {
				client.event(map[string]any{"kind": "userinfo_mode_pending", "mode": mode, "uid": uid, "selected_character_id": client.selectedCharacterID})
				return dispatchHandled
			}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		var payload []byte
		created := false
		var err error
		kind, id := byte(1), requestData.frame.ID
		switch requestData.frame.ID {
		case 6:
			var req protocol.DeleteCharacterRequest
			var deletedID int64
			req, err = protocol.DecodeDeleteCharacter(requestData.plaintext)
			if err == nil && client.selectedCharacterID != 0 {
				err = fmt.Errorf("delete requires character selection screen")
			}
			if err == nil {
				deletedID, err = client.gameStore.DeleteCharacter(ctx, client.developmentAccount, req.Slot, req.Name)
			}
			if err == nil {
				created = true // refresh complete roster after the native removal callback
				payload = protocol.DeleteCharacterSuccess(req.Slot)
				client.event(map[string]any{"kind": "character_archived", "character_id": deletedID, "slot": req.Slot, "name": req.Name})
			}
		case 684:
			payload, err = client.characters.CheckName(ctx, requestData.plaintext)
		case 5:
			var role storage.Character
			role, err = client.characters.Create(ctx, client.developmentAccount, requestData.plaintext)
			if err == nil {
				var slot uint16
				slot, err = client.characters.RosterSlot(ctx, client.developmentAccount, role.ID)
				if err == nil {
					created = true
					payload = protocol.CreateSuccess(slot, role.Name)
					// Only a character created from here on owes a
					// starting route; every earlier character was
					// backfilled as already finished.
					owed := "not tracked"
					if client.tutorialRoutes != nil {
						if e := client.gameStore.StartBirth(ctx, client.developmentAccount, role.ID); e != nil {
							owed = "record failed: " + e.Error()
						} else {
							owed = "pending"
						}
					}
					client.event(map[string]any{"kind": "character_committed", "id": role.WireID, "slot": slot, "name": role.Name, "profession": role.Profession, "config_version": role.ConfigVersion, "starting_route": owed})
				}
			}
		case 8:
			kind, id = 0, 2
			if userInfoMode == 0 {
				payload = client.selectedBasic
				if client.worldState != nil && client.worldState.role.ID == client.selectedCharacterID {
					payload, err = client.characters.EntryBasicProbe(client.worldState.role, [2]byte{})
				}
			} else if userInfoMode == 1 {
				payload = client.selectedAddition
				if client.worldState != nil && client.worldState.role.ID == client.selectedCharacterID {
					payload, err = client.characters.EntryAddition(client.worldState.role)
				}
			} else {
				payload, err = client.characters.ListWithFatigue(ctx, client.developmentAccount, client.fatigueService, time.Now())
			}
		}
		cancel()
		if err != nil {
			client.event(map[string]any{"kind": "character_rejected", "id": requestData.frame.ID, "error": err.Error()})
			// Both native creation/name handlers explicitly handle code 2
			// and restore input state. Never drop a valid connection for
			// a business refusal; other code meanings remain unverified.
			kind, id = 1, requestData.frame.ID
			payload = protocol.Refusal(2)
		}
		ciphertext, err := wire.EncryptPayload(client.keys, id, payload)
		if err != nil {
			client.event(map[string]any{"kind": "character_error", "error": err.Error()})
			return dispatchClose
		}
		response, err := wire.ServerFrame(kind, id, ciphertext)
		if err != nil {
			return dispatchClose
		}
		if err = client.output.writeRaw(response); err != nil {
			return dispatchClose
		}
		client.event(map[string]any{"kind": "character_response", "id": id, "bytes": len(response), "hex": hex.EncodeToString(response)})
		// NOTI2 先建立选角管理器，再由 NOTI1759 初始化背景列表和五页选择。
		if requestData.frame.ID == 8 && userInfoMode == 2 && kind == 0 && id == 2 {
			if err = restoreRosterBackgrounds(client.gameStore, client.developmentAccount, client.output.send, client.event); err != nil {
				return dispatchClose
			}
		}
		if created {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			list, e := client.characters.ListWithFatigue(ctx, client.developmentAccount, client.fatigueService, time.Now())
			cancel()
			if e != nil {
				client.event(map[string]any{"kind": "character_list_error", "error": e.Error()})
				return dispatchHandled
			}
			encrypted, e := wire.EncryptPayload(client.keys, 2, list)
			if e != nil {
				return dispatchClose
			}
			notification, e := wire.ServerFrame(0, 2, encrypted)
			if e != nil {
				return dispatchClose
			}
			if e = client.output.writeRaw(notification); e != nil {
				return dispatchClose
			}
			client.event(map[string]any{"kind": "character_list_after_mutation", "request": requestData.frame.ID, "id": 2, "bytes": len(notification)})
			if e = restoreRosterBackgrounds(client.gameStore, client.developmentAccount, client.output.send, client.event); e != nil {
				return dispatchClose
			}
		}
		return dispatchHandled
	}

	return dispatchNext
}
