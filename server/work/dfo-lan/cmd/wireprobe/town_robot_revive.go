package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// AI acknowledgement extension: game-frame time, three monotonic death
// sequences, dead mask. Worker heartbeats cannot renew game-frame evidence.
// All fields are local IPC; the network uses the existing N32/C41 protocol.
type robotNativeDeath struct {
	death    odysseyDeath
	notified bool
}

// Called only after the full lease/generation/actor vector has been validated,
// under transport.mu. Never changes the owner's death state or starts a timer.
func (t *townRobotTransport) observeNativeDeathsLocked(ack []byte, now time.Time) {
	if len(ack) != 104 || t.closed || t.roomLoading || !t.completed || t.room == nil || len(t.peers) > 3 {
		return
	}
	sample := binary.LittleEndian.Uint64(ack[80:])
	ms := uint64(now.UnixMilli())
	mask := binary.LittleEndian.Uint32(ack[100:])
	if sample > ms || ms-sample > 2500 || mask>>uint(len(t.peers)) != 0 {
		return
	}
	for i, p := range t.peers {
		seq := binary.LittleEndian.Uint32(ack[88+4*i:])
		state := &t.nativeAI.deaths[i]
		if mask&(1<<uint(i)) == 0 || seq == 0 || seq < state.death.Sequence {
			continue
		}
		if seq > state.death.Sequence {
			// Bound the per-room history; reject skipped or forged transitions.
			if seq != state.death.Sequence+1 || seq > 256 || state.death.Dead {
				continue
			}
			state.death.Run = fmt.Sprintf("%016x", t.nativeAI.generation)
			state.death.Sequence = seq
			state.death.Dead = true
			state.notified = false
		}
		// A retained old dead snapshot must not undo an accepted resurrection.
		if !state.death.Dead || state.notified || t.send == nil {
			continue
		}
		body, err := protocol.PlayerDeathState(p.actor)
		if err == nil {
			err = t.send(0, 32, body)
		}
		if err == nil {
			state.notified = true
			t.event(map[string]any{"kind": "town_robot_death", "actor": p.actor, "sequence": seq, "generation": t.nativeAI.generation})
		}
	}
}

func (w *worldSession) reviveTownRobot(ctx context.Context, store ceraReviveStore, p, frame []byte) ([]outboundPacket, bool, error) {
	if w == nil || len(p) != 8 || binary.LittleEndian.Uint16(p) == w.role.WireID {
		return nil, false, nil
	}
	// A non-self target never falls through to the player's own resurrection.
	refuse := func(reason string) ([]outboundPacket, bool, error) {
		return nil, true, fmt.Errorf("robot revive: %s", reason)
	}
	if w.hub == nil || w.hub.townRobots == nil || w.peer == nil || w.activeDungeon == nil || !w.activeDungeon.Loaded {
		return refuse("no active ordinary party")
	}
	f := w.hub.townRobots
	f.mu.Lock()
	defer f.mu.Unlock()
	party := f.parties[w.peer]
	if party == nil || !party.dungeon || party.transport == nil {
		return refuse("party is not in dungeon")
	}
	t := party.transport
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed || t.roomLoading || t.room == nil || !t.completed || t.ownerActor != w.role.WireID || t.nativeAI.phase != 2 || time.Since(t.nativeAI.lastAck) > 2500*time.Millisecond {
		return refuse("inactive room controller")
	}
	target := binary.LittleEndian.Uint16(p)
	for i, peer := range t.peers {
		if peer.actor != target || i >= 3 {
			continue
		}
		state := &t.nativeAI.deaths[i]
		if !state.notified {
			return refuse("death not confirmed")
		}
		v := coinReviveTarget{actor: target, death: &state.death, scope: fmt.Sprintf("%016x", t.nativeAI.generation)}
		plan, err := w.useRobotCoinRevive(ctx, store, p, frame, v)
		if err == nil && len(plan) > 0 {
			t.event(map[string]any{"kind": "town_robot_revived", "actor": target, "sequence": state.death.Sequence, "generation": t.nativeAI.generation})
		}
		return plan, true, err
	}
	return refuse("target is not a current companion")
}

func (w *worldSession) useRobotCoinRevive(ctx context.Context, store ceraReviveStore, p, frame []byte, v coinReviveTarget) ([]outboundPacket, error) {
	if err := w.odysseyReviveGate(); err != nil {
		return nil, err
	}
	if err := w.checkIspinsCoinUse(); err != nil {
		return nil, err
	}
	plan, err := w.lifeTokenReviveFor(ctx, store, p, frame, v)
	if err == nil || !errors.Is(err, inventory.ErrCoinStackEmpty) {
		return w.afterAzureCoinRevive(plan, err)
	}
	return w.afterAzureCoinRevive(w.ceraReviveFor(ctx, store, p, frame, v))
}
