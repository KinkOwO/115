package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventureelite"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"fmt"
)

// 角色选择列表按Store.Characters的顺序下发。精锐页14021ADF0从该列表
// 查询界面索引；存档保存稳定ID，重登时重新投影，不能持久化易变的索引。
func (w *worldSession) adventureElitePayload(ctx context.Context, profile database.AccountAdventure) ([]byte, error) {
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	return adventureElitePayloadForRoles(w.eliteProfileView(profile), roles)
}

func adventureElitePayloadForRoles(profile database.AccountAdventure, roles []database.Character) ([]byte, error) {
	slots := make(map[int64]int32, len(roles))
	for slot, role := range roles {
		slots[role.ID] = int32(slot)
	}
	var rows []protocol.AdventureEliteSelection
	for mode := uint16(1); mode <= 4; mode++ {
		ids, ok := profile.Data.EliteSelections[mode]
		if !ok {
			continue
		}
		row := protocol.AdventureEliteSelection{Mode: mode, Slots: [3]int32{-1, -1, -1}}
		seen := map[int64]bool{}
		for i, id := range ids {
			if slot, exists := slots[id]; exists && !seen[id] {
				row.Slots[i] = slot
				row.SkillUsage[i] = profile.Data.EliteSkillUsage[mode][id]
				seen[id] = true
			}
		}
		rows = append(rows, row)
	}
	return protocol.AdventureEliteSelections(rows)
}

func (w *worldSession) setAdventureElite(ctx context.Context, p, raw []byte, prefix string) ([]outboundPacket, error) {
	if w.blackPurgatory.prepared {
		return nil, fmt.Errorf("黑鸦挑战已开始，请回到等待区后再修改精锐名单")
	}
	req, err := protocol.DecodeAdventureEliteSelection(p)
	if err != nil {
		return nil, err
	}
	// N1754 can release/reload the complete native mode-2 team, even when
	// saving another mode. Never send that update during an ordinary run or
	// unfinished scene transition. Native special-channel rules stay separate.
	if adventureelite.Enabled() && !adventureEliteChannel(w.channelType) &&
		(w.activeDungeon != nil || w.selectingDungeon || w.pendingTownArrival != nil ||
			w.bleedingMineStart != nil || w.specialWarpPending) {
		return nil, fmt.Errorf("请完成回城后再修改精锐名单或技能设置")
	}
	if _, err = w.prepareAdventure(ctx); err != nil {
		return nil, err
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	var selected [3]int64
	usage := map[int64][30]int32{}
	seen := map[int64]bool{}
	for i, slot := range req.Slots {
		if slot == -1 {
			continue
		}
		if int64(slot) >= int64(len(roles)) {
			return nil, fmt.Errorf("精锐角色不在当前账号角色列表内")
		}
		role := roles[slot]
		if role.ID == w.role.ID || seen[role.ID] {
			return nil, fmt.Errorf("精锐角色不能重复或选择当前出战角色")
		}
		var state character.State
		if err = json.Unmarshal(role.State, &state); err != nil {
			return nil, fmt.Errorf("读取精锐角色存档：%w", err)
		}
		// 业主指定的兼容策略：仅开关启用时取消资格，始终保留真实存档。
		if !adventureelite.Enabled() && (state.Level < 100 || state.Awakening < 2) {
			return nil, fmt.Errorf("精锐角色需要达到100级并完成二次觉醒或自我觉醒")
		}
		selected[i] = role.ID
		usage[role.ID], err = w.characters.AdventureEliteSkillUsage(role, req.SkillUsage[i])
		if err != nil {
			return nil, err
		}
		seen[role.ID] = true
	}
	key := fmt.Sprintf("adventure-elite:%s:%x", prefix, sha256.Sum256(raw))
	_, profile, _, err := w.store.CommitAdventure(ctx, w.account, w.role.ID, key,
		func(role database.Character, profile *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			if profile.Data.EliteSelections == nil {
				profile.Data.EliteSelections = map[uint16][3]int64{}
			}
			profile.Data.EliteSelections[req.Mode] = selected
			if profile.Data.EliteSkillUsage == nil {
				profile.Data.EliteSkillUsage = map[uint16]map[int64][30]int32{}
			}
			profile.Data.EliteSkillUsage[req.Mode] = usage
			receipt, e := json.Marshal(map[string]any{"mode": req.Mode, "characters": selected, "skill_usage": usage})
			return role.State, receipt, e
		})
	if err != nil {
		return nil, err
	}
	body, err := w.adventureElitePayload(ctx, profile)
	if err != nil {
		return nil, err
	}
	w.adventureEliteSnapshot = sha256.Sum256(body)
	if prepared := w.adventureElitePrepared; prepared != nil {
		view := w.eliteProfileView(profile).Data.EliteSelections[2]
		// Native142E5ABFC compares slot identities: changed slots release the
		// actors at142E5AE64 and request1811 at142E5AED6. Clear only the stale
		// preparation so that original request can build the new roster.
		// Unchanged slots retain actors and apply skills via142E653A0 without
		// requesting1811; keep their identity and refresh its settings hash.
		if view == ([3]int64{}) || view != prepared.Selected || prepared.Owner != w.role.ID || prepared.Channel != w.channelType {
			w.adventureElitePrepared = nil
		} else {
			prepared.Settings = w.adventureEliteSnapshot
		}
	}
	// 成功ACK只弹成功提示，不更新设置map；先发NOTI1754，界面才会回显。
	return []outboundPacket{{"精锐角色设置同步", 0, 1754, body}, {"精锐角色保存完成", 1, 1719, []byte{1}}}, nil
}

// 原生142E60CF0在这些频道选择类型2的精锐容器；普通频道和矿区走类型0。
// 另一个分支依赖活动662，尚未接入，不伪造活动或临时更改玩家频道身份。
func adventureEliteChannel(channelType uint32) bool {
	switch channelType {
	case 68, 73, 74, 76, 78:
		return true
	}
	return false
}

func (w *worldSession) loadAdventureElite(ctx context.Context, p []byte) ([]outboundPacket, error) {
	mode, err := protocol.DecodeAdventureEliteLoad(p)
	if err != nil {
		return nil, err
	}
	if w == nil || (!adventureEliteChannel(w.channelType) && !w.ordinaryElitePreparationAllowed()) {
		return nil, fmt.Errorf("当前频道未启用客户端精锐同伴系统，请在支持精锐的频道加载")
	}
	if w.activeDungeon != nil || w.selectingDungeon || w.bleedingMineStart != nil || w.pendingTownArrival != nil {
		return nil, fmt.Errorf("请在城镇加载精锐资料，不能重建正在出战的同伴")
	}
	// 当前原版mycharacters_apc_contents.etc仅保留contents key=2。
	// 旧模式和活动模式4有不同消费分支，不套用当前账号同伴资料。
	if mode != 2 {
		return nil, fmt.Errorf("当前客户端资源未启用此精锐模式：%d", mode)
	}
	ordinary := !adventureEliteChannel(w.channelType)
	if ordinary && w.adventureElitePrepared != nil {
		return nil, fmt.Errorf("精锐资料已经准备，请勿重复加载")
	}
	profile, err := w.prepareAdventure(ctx)
	if err != nil {
		return nil, err
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]int, len(roles))
	for slot, role := range roles {
		byID[role.ID] = slot
	}
	var slots []byte
	var companions []protocol.TagCharacter
	seen := map[int64]bool{}
	effectiveProfile := w.eliteProfileView(profile)
	for _, id := range effectiveProfile.Data.EliteSelections[mode] {
		if id == 0 {
			continue
		}
		slot, exists := byID[id]
		if !exists {
			// 已删除角色不再下发，保留其他槽位及账号存档。
			continue
		}
		role := roles[slot]
		if role.AccountID != w.account || id == w.role.ID || seen[id] || slot >= 255 {
			return nil, fmt.Errorf("精锐角色归属、槽位或选择已变化，请重新保存名单")
		}
		seen[id] = true
		var state character.State
		if err = json.Unmarshal(role.State, &state); err != nil {
			return nil, fmt.Errorf("读取精锐角色%s：%w", role.Name, err)
		}
		// 不为低等级精锐升级或补觉醒；TagCharacterSnapshot投影真实状态。
		if !adventureelite.Enabled() && (state.Level < 100 || state.Awakening < 2) {
			return nil, fmt.Errorf("精锐角色%s未达到100级二次觉醒条件", role.Name)
		}
		_, err := w.characters.AdventureEliteSkillUsage(role, profile.Data.EliteSkillUsage[mode][id])
		if err != nil {
			return nil, fmt.Errorf("精锐角色%s的技能设置需要重新保存：%w", role.Name, err)
		}
		snapshot, err := w.characters.TagCharacterSnapshot(role)
		if err != nil {
			return nil, fmt.Errorf("读取精锐角色%s的装备技能：%w", role.Name, err)
		}
		slots = append(slots, byte(slot))
		companions = append(companions, snapshot)
	}
	if len(companions) == 0 {
		return nil, fmt.Errorf("请先保存至少一个有效精锐角色")
	}
	characters, err := protocol.AdventureEliteCharacterInfo(w.role.WireID, slots, companions)
	if err != nil {
		return nil, fmt.Errorf("精锐装备技能资料无法完整编码：%w", err)
	}
	settings, err := adventureElitePayloadForRoles(effectiveProfile, roles)
	if err != nil {
		return nil, err
	}
	done := make([]byte, 2)
	binary.LittleEndian.PutUint16(done, mode)
	var packets []outboundPacket
	// 先确保客户端选择与本次同一份账号角色顺序一致。只在内容变化时发送，
	// 避免1754自动发出的1811形成循环；本次仍完成资料加载以解除等待状态。
	if signature := sha256.Sum256(settings); signature != w.adventureEliteSnapshot {
		w.adventureEliteSnapshot = signature
		packets = append(packets, outboundPacket{"精锐角色索引变更同步", 0, 1754, settings})
	}
	if ordinary {
		w.adventureElitePrepared = &adventureElitePreparation{Channel: w.channelType, Owner: w.role.ID,
			Selected: effectiveProfile.Data.EliteSelections[mode], Settings: sha256.Sum256(settings)}
	}
	// 原生 layout attempt 1/3；普通事务适配 attempt 2/3。1382建立真实对象，1879克隆为AI同伴并应用技能开关。
	return append(packets,
		outboundPacket{"精锐真实装备技能加载（attempt 1/3）", 0, 1382, characters},
		outboundPacket{"精锐同伴资料加载完成", 0, 1879, done},
	), nil
}
