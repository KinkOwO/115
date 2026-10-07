package main

import (
	"bytes"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

// Only the selected Bakal channel publishes its owned weekly cache. N1434
// replaces the raid table; do not invent empty rows for unrelated raids.
func (w *worldSession) syncBakalWeeklyQuota(now time.Time, send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w == nil || w.channelType != legion.BakalChannelType || w.bakalRules == nil || w.role.ID == 0 {
		return nil
	}
	if w.bakalRules.RaidID <= 0 || w.bakalRules.RaidID > 255 {
		return fmt.Errorf("Bakal source raid kind outside weekly wire range")
	}
	usage, err := workflow.BakalWeeklyUsage(w.role, now)
	if err != nil {
		return err
	}
	body, err := protocol.RaidWeeklyClearInfo115(byte(w.bakalRules.RaidID), usage.Clears, usage.Rewards)
	if err != nil {
		return err
	}
	if w.bakalQuotaRole == w.role.ID && bytes.Equal(w.bakalQuotaBody, body) {
		return nil
	}
	if err := send(0, 1434, body); err != nil {
		return err
	}
	w.bakalQuotaRole, w.bakalQuotaBody = w.role.ID, body
	event(map[string]any{"kind": "bakal_weekly_quota_synchronized", "character_id": w.role.ID,
		"raid_kind": w.bakalRules.RaidID, "clears_used": usage.Clears, "rewards_used": usage.Rewards,
		"clear_limit": w.bakalRules.WeeklyClearLimit, "reward_limit": w.bakalRules.WeeklyRewardLimit,
		"plain_hex": fmt.Sprintf("%x", body)})
	return nil
}
