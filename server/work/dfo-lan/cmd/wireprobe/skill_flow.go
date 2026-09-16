package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"fmt"
	"time"
)

type skillSession struct {
	nonce       [16]byte
	initialized bool
}

func (s *skillSession) handle(cs *character.Service, w *worldSession, id uint16, p, raw []byte) ([]outboundPacket, error) {
	if cs == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("skill request requires owned selected character")
	}
	if !s.initialized {
		if _, e := rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	h := sha256.Sum256(raw)
	key := fmt.Sprintf("skill:%x:%x", s.nonce, h)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var saved storage.Character
	var applied bool
	var body []byte
	var e error
	switch id {
	case 28:
		var r protocol.SkillMove
		r, e = protocol.DecodeSkillMove(p)
		if e == nil {
			saved, applied, e = cs.MoveSkill(ctx, w.role, key, r)
		}
		if e == nil {
			body = protocol.SkillMoveSuccess(r)
		}
	case 29:
		var r protocol.SkillPurchase
		r, e = protocol.DecodeSkillPurchase(p)
		if e == nil {
			saved, applied, e = cs.Learn(ctx, w.role, key, r)
		}
		if e == nil {
			body, e = cs.LearningResponse(saved, r)
		}
	default:
		return nil, fmt.Errorf("unsupported skill mutation")
	}
	if e != nil {
		return nil, e
	}
	w.role = saved
	plan := []outboundPacket{{"skill_committed_response", 1, id, body}}
	if id == 29 || !applied {
		restore, e := cs.EntrySkills(saved)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"skill_state_restored", 0, 19, restore})
	}
	return plan, nil
}
