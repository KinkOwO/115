package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"time"
)

// Run on the connection's serialized ticker, using the same native fail and
// verified town-return packets as existing dungeon timeouts. No reward/weekly
// receipt is created for a failed attempt; cleared earlier stages survive.
func (w *worldSession) ispinsTimeout(now time.Time) ([]outboundPacket, error) {
	if w == nil || w.ispins == nil || w.activeDungeon == nil || w.completionSent || w.ispins.deadline.IsZero() || now.Before(w.ispins.deadline) {
		return nil, nil
	}
	leave, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"ispins_time_limit_failed", 0, 33, protocol.DungeonFailClear(100)}}
	// This is a server-owned timeout, not an invented client GIVEUP request.
	for _, p := range leave {
		if p.Name != "dungeon_leave_ack" {
			packets = append(packets, p)
		}
	}
	w.activeDungeon = nil
	w.ispins.confirmed = false
	w.ispins.deadline = time.Time{}
	w.ispins.operationIndex = 0
	w.ispinsRetryPending = true
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.pilotDeath = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.resetCards()
	return packets, nil
}

// N33 enters a native failure/death scene even without CMD40. Restore actor
// state and the accepted waiting UI only after the town scene reports ready.
func (w *worldSession) ispinsRetryRestorePackets(keepChallenge bool) ([]outboundPacket, error) {
	if w == nil || !w.ispinsRetryPending || w.activeDungeon != nil || w.role.ID == 0 || w.channelType != 81 {
		return nil, nil
	}
	revive, err := protocol.PlayerDeathState(w.role.WireID)
	if err != nil {
		return nil, err
	}
	revive[2] = 1 // Same native alive state as leaveDungeon's town_actor_revived.
	packets := []outboundPacket{{"ispins_timeout_actor_restored", 0, 32, revive}}
	if keepChallenge && w.ispins != nil {
		wait, err := w.ispinsWaitInfo()
		if err != nil {
			return nil, err
		}
		return append(packets, outboundPacket{"ispins_timeout_wait_restored", 0, legion.NotiIspinsInfo, wait}), nil
	}
	entry, err := w.ispinsStandbyQuotaInfo()
	if err != nil {
		return nil, err
	}
	return append(packets,
		outboundPacket{"ispins_timeout_entry_restored", 0, legion.NotiIspinsEntryCharacterInfo, entry},
		outboundPacket{"ispins_timeout_weekly_user_restored", 0, 781, weeklyDifficultyInfoUserStandby},
		outboundPacket{"ispins_timeout_weekly_character_restored", 0, 782, weeklyDifficultyInfoCharacStandby}), nil
}
