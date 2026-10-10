package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func (w *worldSession) useJumpLevelCapsule(ctx context.Context, p, raw []byte, prefix string, event func(map[string]any)) ([]outboundPacket, error) {
	slot, err := protocol.DecodeJumpLevelCapsule(p)
	if err != nil {
		return nil, err
	}
	if w == nil || w.role.ID == 0 || w.role.AccountID != w.account || w.progression == nil || w.loot == nil || w.service == nil || w.characters == nil || prefix == "" || len(raw) == 0 {
		return nil, fmt.Errorf("capsule requires an owned entered character and source catalogs")
	}
	if w.activeDungeon != nil || w.inTutorial || w.odyssey {
		return nil, fmt.Errorf("请在普通模式的赛丽亚房间使用胶囊")
	}
	area, ok := w.service.Catalog.Areas[catalog.AreaKey(w.state.Position.Town, w.state.Position.Area)]
	if !ok || w.service.Catalog.Source.Checksum != w.loot.Catalog.Source.Checksum {
		return nil, fmt.Errorf("capsule world source missing")
	}
	key := fmt.Sprintf("jump-level-capsule:%s:%x", prefix, sha256.Sum256(raw))
	previousLevel := w.level
	saved, applied, err := w.progression.UseLevelCapsule(ctx, w.role, w.loot.Catalog, slot, area.SeriaReturnWarp, key, time.Now())
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	experience, err := character.ExperiencePayload(saved)
	if err != nil {
		return nil, err
	}
	bag, err := inventory.ReadBag(saved.State)
	if err != nil {
		return nil, err
	}
	row := protocol.EmptyOrdinaryItem(slot)
	if owned, ok := bag.RowAt(slot); ok {
		row = owned
	}
	update, err := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if err != nil {
		return nil, err
	}
	w.role, w.level = saved, experience[0]
	// The popup already closes on the native request. Reuse the existing
	// absolute inventory/EXP readers; no speculative CMD507 acknowledgement.
	plan := []outboundPacket{{"升级胶囊库存同步", 0, 14, update}}
	if applied && previousLevel != w.level {
		// NOTI37 updates EXP/SP, but the town's character detail and actor
		// projection can retain their entry snapshot (attempt-1 live report).
		// Reuse the proven reward actor rebuild: mode0, mode1, then worn
		// rows. Re-send EXP afterwards because mode0 resets actor state.
		refresh, err := w.unlockRefresh(saved)
		if err != nil {
			return nil, err
		}
		plan = append(plan, refresh...)
		if bag.TitleSkin != 0 {
			// Keep the confirmed title-book appearance and applied marker
			// after rebuilding its equipped-title object, as on town entry.
			plan = append(plan, outboundPacket{"升级胶囊称号外观恢复", 1, 560, protocol.TitleAppearanceSuccess(bag.TitleSkin)})
		}
	}
	plan = append(plan, outboundPacket{"升级胶囊等级同步", 0, 37, experience})
	if applied && previousLevel != w.level {
		skills, err := w.automaticSkillRefresh()
		if err != nil {
			return nil, err
		}
		plan = append(plan, skills...)
		if w.quests != nil {
			available, err := w.availableQuestPayload(ctx)
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"升级胶囊可接任务同步", 0, 21, available})
		}
	}
	if event != nil {
		event(map[string]any{"kind": "jump_level_capsule_committed", "character_id": saved.ID, "slot": slot, "before_level": previousLevel, "level": w.level, "applied": applied})
	}
	return plan, nil
}
