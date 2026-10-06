package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/database"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// boostCapsuleAction 是胶囊在 CMD507 上的 [action type] 编号（S-0904 实测）。
const boostCapsuleAction = 337

func (w *worldSession) useStackableAction(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	if len(p) >= 11 && binary.LittleEndian.Uint32(p[7:]) == boostCapsuleAction {
		return w.useBoostCapsule(p, event)
	}
	return w.recoverFatiguePotion(p)
}
func boostRosterForRoles(roles []database.Character, override database.Character) ([]byte, error) {
	var rows []protocol.BoostRosterRow115
	seen := map[int64]bool{}
	matched := override.ID == 0
	for i, role := range roles {
		if seen[role.ID] {
			return nil, fmt.Errorf("duplicate roster character")
		}
		seen[role.ID] = true
		if role.ID == override.ID {
			if role.AccountID != override.AccountID {
				return nil, fmt.Errorf("boost override owner mismatch")
			}
			matched = true
			role = override
		}
		state, e := boostup.ReadState(role.State)
		if e != nil {
			return nil, e
		}
		if !state.Activated {
			continue
		}
		if state.Training.Step == 0 || state.Training.Phase > 2 {
			return nil, fmt.Errorf("invalid persisted boost training")
		}
		mode := byte(0)
		if state.Training.Finished {
			mode = 2
		}
		rows = append(rows, protocol.BoostRosterRow115{Slot: uint32(i), Mode: mode})
	}
	if !matched {
		return nil, fmt.Errorf("boost override absent from roster")
	}
	return protocol.BoostRoster115(rows)
}
func boostCapsuleRefresh(w *worldSession, role database.Character, r loot.BoostCapsuleReceipt, roles []database.Character, replay bool) ([]outboundPacket, error) {
	bag, e := inventory.ReadBag(role.State)
	if e != nil {
		return nil, e
	}
	delta, e := protocol.InventoryUpdate(stackSlotRows(bag, r.Slot))
	if e != nil {
		return nil, e
	}
	skills, e := w.characters.EntrySkills(role)
	if e != nil {
		return nil, e
	}
	// Install awakening before the level-change notice to avoid a stale
	// First Awakening prompt. Do not replay an old level on a lost-ACK retry.
	prime := role
	var fields map[string]json.RawMessage
	if e = json.Unmarshal(role.State, &fields); e != nil {
		return nil, e
	}
	if !replay {
		fields["level"], _ = json.Marshal(r.BeforeLevel)
	}
	prime.State, e = json.Marshal(fields)
	if e != nil {
		return nil, e
	}
	before, e := w.characters.EntryBasicProbe(prime, w.characters.ChannelContext)
	if e != nil {
		return nil, e
	}
	xp, e := character.ExperiencePayload(role)
	if e != nil {
		return nil, e
	}
	after, e := w.characters.EntryBasicProbe(role, w.characters.ChannelContext)
	if e != nil {
		return nil, e
	}
	restore, e := appearanceRestore(w.characters, role)
	if e != nil {
		return nil, e
	}
	status, e := boostTrainingRestore(w.boostup, role)
	if e != nil {
		return nil, e
	}
	roster, e := boostRosterForRoles(roles, role)
	if e != nil {
		return nil, e
	}
	plan := []outboundPacket{{"boost_capsule_consumed", 0, 14, delta}, {"boost_capsule_training", 0, 2638, status}, {"boost_capsule_roster", 0, 2639, roster}, {"boost_capsule_skills", 0, 19, skills}}
	plan, e = appendSkillPresetRestore(plan, w.characters, role, "skill_preset_restored_after_boost_capsule")
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"boost_capsule_awakening_primed", 0, 2, before}, outboundPacket{"boost_capsule_experience", 0, 37, xp}, outboundPacket{"boost_capsule_actor", 0, 2, after})
	plan = append(plan, restore...)
	// Capture S-0904 is01/u16 slot/u8 space/u32 action (before transport tail).
	ack := []byte{1, byte(r.Slot), byte(r.Slot >> 8), 0, 0x51, 1, 0, 0}
	return append(plan, outboundPacket{"boost_capsule_ack", 1, 507, ack}), nil
}
func (w *worldSession) useBoostCapsule(p []byte, event func(map[string]any)) ([]outboundPacket, error) {
	r, e := protocol.DecodeBoostCapsule115(p)
	if e != nil {
		return nil, e
	}
	if w.boostup == nil || w.characters == nil || w.characters.Store == nil || w.service == nil || w.progression == nil || w.loot == nil || w.role.ID == 0 || w.account != w.role.AccountID {
		return nil, fmt.Errorf("capsule feature/owner unavailable")
	}
	if r.Space != 0 || w.activeDungeon != nil || w.inTutorial || w.selectingDungeon {
		return nil, fmt.Errorf("capsule requires ordinary bag in town")
	}
	a, ok := w.service.Catalog.Areas[catalog.AreaKey(w.state.Position.Town, w.state.Position.Area)]
	eventState, e := boostup.ReadState(w.role.State)
	if e != nil {
		return nil, e
	}
	if !ok || !a.SeriaReturnWarp && !(eventState.Activated && w.state.Position.Town == w.boostup.Town) {
		return nil, fmt.Errorf("capsule source requires Seria room")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	roles, e := w.characters.Store.Characters(ctx, w.account)
	if e != nil {
		return nil, e
	}
	origin, e := json.Marshal(w.state.Position)
	if e != nil {
		return nil, e
	}
	if eventState.Activated {
		origin = eventState.Origin
	}
	// 胶囊变体（普通/缓冲）由**物品模板在源里的变体号**决定，不吃包里的字段
	// （donor 基线的 r.Variant 在本树协议结构里不存在：客户端那格填什么都不作数）。
	bag, e := inventory.ReadBag(w.role.State)
	if e != nil {
		return nil, e
	}
	var capsule boostup.Capsule
	var found bool
	for _, item := range bag.Items {
		if item.Slot != r.Slot {
			continue
		}
		capsule, found = w.boostup.Capsules[item.Template]
		break
	}
	if !found {
		return nil, fmt.Errorf("slot %d is not a source boost capsule", r.Slot)
	}
	var plan []outboundPacket
	next, receipt, applied, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).UseBoostCapsule(ctx, w.role, w.boostup, r.Slot, capsule.Variant,
		func(role database.Character, target byte) (database.Character, error) {
			return w.characters.BoostLevel(role, w.progression, target)
		},
		func(role database.Character, receipt loot.BoostCapsuleReceipt) error {
			event, err := boostup.ReadState(role.State)
			if err != nil {
				return err
			}
			if _, _, err = boostWorldTarget(w.service, w.boostup, event, receipt.Level, w.ordinaryBoostChannel()); err != nil {
				return err
			}
			plan, err = boostCapsuleRefresh(w, role, receipt, roles, false)
			return err
		}, origin)
	if e != nil {
		return nil, e
	}
	if !applied {
		plan, e = boostCapsuleRefresh(w, next, receipt, roles, true)
		if e != nil {
			return nil, e
		}
	}
	w.role = next
	var state character.State
	if e = json.Unmarshal(next.State, &state); e != nil {
		return nil, e
	}
	w.level = state.Level
	town, e := w.boostTownRefresh(ctx)
	if e != nil {
		return nil, e
	}
	last := plan[len(plan)-1]
	plan = append(append(plan[:len(plan)-1], town...), last)
	if applied {
		// 开包同时置了副装备解锁位（见 workflow.UseBoostCapsule），
		// 挂锁只吃 USERINFO1，需按券/任务同款三连重发。
		unlock, e := w.unlockRefresh(next)
		if e != nil {
			return nil, e
		}
		plan = append(plan, unlock...)
		// 直升落地后把主线一次性标为完成（boost-story-skip-v2，幂等收据）。
		// 失败只由 boostStorySkipNow 记事件、不阻断本帧串：胶囊已经提交，
		// 下一次进城镇的登录钩子会按同一张收据重试。
		skip, skipErr := w.boostStorySkipNow(ctx, event)
		if skipErr == nil {
			plan = append(plan, skip...)
		}
	}
	//  donor 的 partyIdentityNetworkDirty/presenceForceRefresh 挂脏位在本树不存在：
	// boostTownRefresh 已经当场重发了城镇在场与状态包。
	return plan, nil
}
