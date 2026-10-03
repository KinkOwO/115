package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"time"
)

// 装备事务已提交时，名望刷新失败只记日志，不能把成功扣款回报成失败。
// 使用专用通知，不重建角色模型，也不打断强化动画及地下城对象。
func (w *worldSession) appendFameUpdate(plan []outboundPacket, event func(map[string]any)) []outboundPacket {
	if w == nil || w.characters == nil || w.characters.Equipment == nil || w.role.ID == 0 {
		return plan
	}
	detail, err := w.characters.EquipmentFameBreakdown(w.role.State)
	if err == nil && w.fameInitialized && w.lastFame == detail.Total {
		return plan
	}
	highest := detail.Total
	if err == nil && w.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		highest, err = w.store.RecordCharacterFame(ctx, w.role.AccountID, w.role.ID, detail.Total)
		cancel()
	}
	var payload []byte
	if err == nil {
		payload, err = protocol.CharacterFameValue(w.role.WireID, detail.Total, highest)
	}
	if err != nil {
		if event != nil {
			event(map[string]any{"kind": "fame_refresh_failed", "character_id": w.role.ID, "reason": err.Error()})
		}
		return plan
	}
	w.lastFame, w.fameInitialized = detail.Total, true
	if event != nil {
		event(map[string]any{"kind": "fame_calculated", "character_id": w.role.ID, "current": detail.Total, "highest": highest, "items": detail.Items, "sets": detail.Sets})
	}
	return append(plan, outboundPacket{"character_fame_updated", 0, 2257, payload})
}
