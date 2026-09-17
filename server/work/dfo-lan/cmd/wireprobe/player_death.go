package main

import (
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"fmt"
)

func (w *worldSession) playerDeath(p []byte, frames ...[]byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent {
		return nil, fmt.Errorf("player death requires owned loaded dungeon")
	}
	if _, err := protocol.DecodePlayerDeath(p); err != nil {
		return nil, err
	}
	body, err := protocol.PlayerDeathState(w.role.WireID)
	if err != nil {
		return nil, err
	}
	if w.pilotDeath == nil || w.pilotDeath.Run != w.activeDungeon.RunID {
		w.pilotDeath = &odysseyDeath{Run: w.activeDungeon.RunID, Frames: map[[32]byte]uint32{}, Revives: map[[32]byte]bool{}}
	}
	d := w.pilotDeath
	if len(frames) > 0 {
		key := sha256.Sum256(frames[0])
		if seq, seen := d.Frames[key]; seen {
			if !d.Dead || seq != d.Sequence {
				return []outboundPacket{{"player_death_replay_ack", 1, 40, []byte{1}}}, nil
			}
		} else {
			if len(d.Frames) >= 256 {
				return nil, fmt.Errorf("death report limit exceeded")
			}
			if !d.Dead {
				d.Sequence++
			}
			d.Frames[key] = d.Sequence
		}
	} else if !d.Dead {
		d.Sequence++
	}
	d.Dead = true
	// Repeated reports project the same death state, never rewards or charges.
	return []outboundPacket{{"player_death_ack", 1, 40, []byte{1}}, {"player_death_state", 0, 32, body}}, nil
}
