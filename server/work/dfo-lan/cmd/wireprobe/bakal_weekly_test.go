package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/binary"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestBakalWeeklySyncBeforeRaidAndAfterLedgerChange(t *testing.T) {
	now := time.Date(2026, 10, 7, 5, 0, 0, 0, time.UTC)
	w := &worldSession{channelType: 82, bakalRules: &catalog.BakalRaidRules{RaidID: 8, WeeklyClearLimit: 1, WeeklyRewardLimit: 1}, role: database.Character{ID: 7, State: json.RawMessage(`{}`)}}
	var bodies [][]byte
	failed := true
	send := func(kind byte, id uint16, body []byte) error {
		if kind != 0 || id != 1434 {
			t.Fatal("quota routed as wrong notification")
		}
		if failed {
			return errors.New("write failure")
		}
		bodies = append(bodies, body)
		return nil
	}
	event := func(map[string]any) {}
	if w.syncBakalWeeklyQuota(now, send, event) == nil {
		t.Fatal("failed quota send committed cache")
	}
	failed = false
	if err := w.syncBakalWeeklyQuota(now, send, event); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 1 || binary.LittleEndian.Uint32(bodies[0][6:]) != 0 || binary.LittleEndian.Uint32(bodies[0][20:]) != 0 {
		t.Fatal("pre-create fresh quota not restored")
	}
	w.syncBakalWeeklyQuota(now, send, event)
	if len(bodies) != 1 {
		t.Fatal("unchanged quota sent on every client frame")
	}
	week := database.IspinsWeekStart(now).Format(time.RFC3339)
	w.role.State = json.RawMessage(`{"bakal_raid_rewards":{"week":"` + week + `","clears":1,"rewards":1}}`)
	if err := w.syncBakalWeeklyQuota(now, send, event); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 || binary.LittleEndian.Uint32(bodies[1][6:]) != 1 || binary.LittleEndian.Uint32(bodies[1][20:]) != 1 {
		t.Fatal("post-clear quota retained pre-clear UI values")
	}
	if err := w.syncBakalWeeklyQuota(now.Add(7*24*time.Hour), send, event); err != nil || len(bodies) != 3 || binary.LittleEndian.Uint32(bodies[2][6:]) != 0 {
		t.Fatal("new week did not restore quota")
	}
	w.bakalQuotaBody = nil // same-character re-selection clears native cache
	if err := w.syncBakalWeeklyQuota(now.Add(7*24*time.Hour), send, event); err != nil || len(bodies) != 4 {
		t.Fatal("re-selection did not restore identical quota")
	}
	w.channelType = 81
	w.bakalQuotaBody = nil
	w.syncBakalWeeklyQuota(now, send, event)
	if len(bodies) != 4 {
		t.Fatal("Bakal sync replaced unrelated raid cache")
	}
}
