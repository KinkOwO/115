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

type comboSkillSession struct {
	nonce       [16]byte
	initialized bool
	sequence    uint64
	lastNotify  []byte
}

func appendComboSkillRestore(plan []outboundPacket, cs *character.Service, role storage.Character) ([]outboundPacket, error) {
	notify, err := cs.ComboSkillInfoNotify(role)
	if err != nil {
		return nil, err
	}
	if len(notify) > 0 {
		plan = append(plan, outboundPacket{"combo_skill_info_restored", 0, 433, notify})
	}
	return plan, nil
}

// Each accepted edit gets a new event key. A body hash would mistake A -> B -> A
// for a replay of the first edit and leave B in the database.
func (s *comboSkillSession) nextKey() (string, error) {
	if !s.initialized {
		if _, err := rand.Read(s.nonce[:]); err != nil {
			return "", err
		}
		s.initialized = true
	}
	s.sequence++
	return fmt.Sprintf("combo-skill-info-v1:%x:%d", s.nonce, s.sequence), nil
}

func (s *comboSkillSession) save(cs *character.Service, w *worldSession, id uint16, body []byte) (protocol.ComboSkillInfo, error) {
	var req protocol.ComboSkillInfo
	if cs == nil || w == nil || w.role.ID == 0 || w.role.Profession != 9 {
		return req, fmt.Errorf("combo skill request requires selected dark knight")
	}
	var err error
	switch id {
	case 500:
		req, err = protocol.DecodeComboSkillInfo(body)
	case 502:
		// The bare C2S command is block-encrypted by this client, so the
		// decrypted body is one alignment block of zeros, not zero bytes.
		// 2026-09-30 live frame: 29 wire bytes, 16 decrypted zero bytes.
		if len(body) > 16 || !zeroComboPadding(body) {
			err = fmt.Errorf("combo reset must have no body")
		}
	default:
		err = fmt.Errorf("unsupported combo command %d", id)
	}
	if err != nil {
		return req, err
	}
	key, err := s.nextKey()
	if err != nil {
		return req, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var saved storage.Character
	if id == 502 {
		saved, _, err = cs.ClearComboSkillInfo(ctx, w.role, key)
	} else {
		saved, _, err = cs.SaveComboSkillInfo(ctx, w.role, key, req)
	}
	if err != nil {
		return req, err
	}
	w.role = saved
	if id == 502 {
		s.lastNotify = nil
	}
	return req, nil
}

func zeroComboPadding(body []byte) bool {
	for _, b := range body {
		if b != 0 {
			return false
		}
	}
	return true
}
