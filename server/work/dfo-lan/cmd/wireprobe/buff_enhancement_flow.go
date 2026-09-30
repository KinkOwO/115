package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

type buffEnhancementSession struct {
	nonce       [16]byte
	initialized bool
	sequence    uint64
}

// USERINFO's 14563C1F0 clears the actor-specific registration map. Scene
// transitions rebuild actors and worn instances; replay only after the final
// actor/item updates so 1360 resolves the current objects, not stale pointers.
func appendBuffEnhancementRestore(plan []outboundPacket, cs *character.Service, role storage.Character, name string) ([]outboundPacket, error) {
	if cs == nil || role.ID == 0 || len(role.State) == 0 {
		return plan, nil
	}
	notify, err := cs.BuffEnhancementRestore(role)
	if err != nil {
		return nil, err
	}
	return append(plan, outboundPacket{name, 0, 1361, notify}), nil
}

func (s *buffEnhancementSession) save(cs *character.Service, w *worldSession, body []byte) ([]byte, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("buff enhancement requires selected character")
	}
	req, err := protocol.DecodeBuffEnhancement(body)
	if err != nil {
		return nil, err
	}
	if !s.initialized {
		if _, err = rand.Read(s.nonce[:]); err != nil {
			return nil, err
		}
		s.initialized = true
	}
	s.sequence++
	key := fmt.Sprintf("buff-enhancement-v1:%x:%d", s.nonce, s.sequence)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := cs.SaveBuffEnhancement(ctx, w.role, key, req)
	if err != nil {
		return nil, err
	}
	w.role = saved
	return cs.BuffEnhancementRestore(saved)
}
