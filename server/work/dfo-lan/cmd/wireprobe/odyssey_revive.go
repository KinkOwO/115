package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/savecontract"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

var errOdysseyCreditsExhausted = errors.New("test revive credits exhausted")

type odysseyDeath struct {
	Run      string
	Sequence uint32
	Dead     bool
	Frames   map[[32]byte]uint32
	Revives  map[[32]byte]bool
}

const odysseyCreditField = "odyssey_pilot_revive_credits"
const odysseyCreditGrant = "odyssey-pilot-revive-10-user-approved-20260917-v1"

func changeOdysseyCredits(role database.Character, grant bool) (json.RawMessage, json.RawMessage, error) {
	if !isOdysseyRewardRole(role) || role.ConfigVersion != savecontract.Identity() {
		return nil, nil, fmt.Errorf("test revive credits require Odyssey source role")
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(role.State, &fields); e != nil {
		return nil, nil, e
	}
	if fields == nil {
		return nil, nil, fmt.Errorf("missing role state")
	}
	var before uint32
	if v, ok := fields[odysseyCreditField]; ok {
		if e := json.Unmarshal(v, &before); e != nil {
			return nil, nil, e
		}
	}
	after := before
	if grant {
		if before != 0 {
			return nil, nil, fmt.Errorf("existing test credits require manual reconciliation")
		}
		after = 10
	} else {
		if before == 0 {
			return nil, nil, errOdysseyCreditsExhausted
		}
		after--
	}
	fields[odysseyCreditField], _ = json.Marshal(after)
	raw, e := json.Marshal(fields)
	if e != nil {
		return nil, nil, e
	}
	receipt, e := json.Marshal(map[string]uint32{"before": before, "after": after})
	return raw, receipt, e
}
func grantOdysseyCredits(ctx context.Context, s *database.Store, role database.Character) (database.Character, bool, error) {
	if !isOdysseyRewardRole(role) {
		return role, false, nil
	}
	return s.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, odysseyCreditGrant, "odyssey-test-credits-v1", func(r database.Character) (json.RawMessage, json.RawMessage, error) {
		return changeOdysseyCredits(r, true)
	})
}
func (w *worldSession) pilotReviveAllowed(p []byte) error {
	if w == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded || !w.activeDungeon.Definition.Odyssey || w.resultSent || !isOdysseyRewardRole(w.role) {
		return fmt.Errorf("test revive requires owned loaded Odyssey run")
	}
	if len(p) != 8 || w.role.WireID == 0 || w.role.WireID == 65535 || binary.LittleEndian.Uint16(p) != w.role.WireID {
		return fmt.Errorf("revive actor mismatch")
	}
	for _, v := range p[2:] {
		if v != 0 {
			return fmt.Errorf("unsupported coin request options")
		}
	}
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID || !w.pilotDeath.Dead {
		return fmt.Errorf("revive requires confirmed player death")
	}
	if w.dungeons == nil {
		return fmt.Errorf("missing map revive rules")
	}
	script, err := w.dungeons.MapScript(w.activeDungeon.Room.Map)
	if err != nil {
		return err
	}
	for _, v := range script.Cells {
		if v.Type == 3 && v.Text == "[cannot use coin map]" {
			return fmt.Errorf("source map forbids coin revival")
		}
	}
	return nil
}
func (w *worldSession) pilotRevive(ctx context.Context, s dungeonReviveStore, p, frame []byte) ([]outboundPacket, error) {
	if w != nil && w.activeDungeon != nil && w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID && w.pilotDeath.Revives[sha256.Sum256(frame)] {
		return nil, nil
	}
	if e := w.pilotReviveAllowed(p); e != nil {
		return nil, e
	}
	key := fmt.Sprintf("odyssey-pilot-revive:%s:%d", w.pilotDeath.Run, w.pilotDeath.Sequence)
	saved, _, e := s.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "odyssey-test-revive-v1", func(r database.Character) (json.RawMessage, json.RawMessage, error) {
		return changeOdysseyCredits(r, false)
	})
	if e != nil {
		return nil, e
	}
	death, e := protocol.PlayerDeathState(w.role.WireID)
	if e != nil {
		return nil, e
	}
	// Native1452aadea restores full HP/MP for state1; state2 restores one third.
	death[2] = 1
	ack := []byte{1}
	ack = binary.LittleEndian.AppendUint16(ack, w.role.WireID)
	w.role = saved
	w.pilotDeath.Dead = false
	w.pilotDeath.Revives[sha256.Sum256(frame)] = true
	return []outboundPacket{{"odyssey_test_revive_ack", 1, 41, ack}, {"odyssey_test_revived", 0, 32, death}}, nil
}
