package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
)

// 角色选择列表按Store.Characters的顺序下发。精锐页14021ADF0从该列表
// 查询界面索引；存档保存稳定ID，重登时重新投影，不能持久化易变的索引。
func (w *worldSession) adventureElitePayload(ctx context.Context, profile storage.AccountAdventure) ([]byte, error) {
	roles, err := w.characters.Store.Characters(ctx, w.account)
	if err != nil {
		return nil, err
	}
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
	req, err := protocol.DecodeAdventureEliteSelection(p)
	if err != nil {
		return nil, err
	}
	if _, err = w.prepareAdventure(ctx); err != nil {
		return nil, err
	}
	roles, err := w.characters.Store.Characters(ctx, w.account)
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
		if state.Level < 100 || state.Awakening < 2 {
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
	_, profile, _, err := w.characters.Store.CommitAdventure(ctx, w.account, w.role.ID, key,
		func(role storage.Character, profile *storage.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
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
	// 成功ACK只弹成功提示，不更新设置map；先发NOTI1754，界面才会回显。
	return []outboundPacket{{"精锐角色设置同步", 0, 1754, body}, {"精锐角色保存完成", 1, 1719, []byte{1}}}, nil
}

func (w *worldSession) loadAdventureElite(p []byte) ([]outboundPacket, error) {
	if _, err := protocol.DecodeAdventureEliteLoad(p); err != nil {
		return nil, err
	}
	// 142E5A340的失败分支会清除等待标记，不改动已恢复的设置map。
	// 本次保存偏好不生成战斗APC；不能用1879空完成通知冒充已加载角色战斗数据。
	return nil, fmt.Errorf("精锐选择已恢复；战斗APC资料尚未接入")
}
