package main

import (
	"context"
	"crypto/rand"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"time"
)

type cubeContractSession struct {
	nonce       [16]byte
	initialized bool
	sequence    uint64
}

func cubeContractRestore(raw json.RawMessage) ([]byte, error) {
	var state struct {
		Selection *byte `json:"cube_contract_selection"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	selection := byte(0xff)
	if state.Selection != nil {
		selection = *state.Selection
	}
	return protocol.CubeContractSelectionInfo(selection)
}

func (s *cubeContractSession) save(w *worldSession, p []byte) ([]byte, error) {
	if w == nil || w.role.ID == 0 || w.characters == nil || w.store == nil {
		return nil, fmt.Errorf("晶体契约设置需要已选中的所属角色")
	}
	selection, err := protocol.DecodeCubeContractSelection(p)
	if err != nil {
		return nil, err
	}
	ack, err := protocol.CubeContractSelectionReply(selection)
	if err != nil {
		return nil, err
	}
	if !s.initialized {
		if _, err := rand.Read(s.nonce[:]); err != nil {
			return nil, err
		}
		s.initialized = true
	}
	// 不能只用选择值作幂等键，否则从 A 切到 B 再切回 A 会被误判为旧请求。
	s.sequence++
	key := fmt.Sprintf("cube-contract-selection:%x:%d", s.nonce, s.sequence)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "cube-contract-selection-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(current.State, &fields); err != nil {
			return nil, nil, err
		}
		if fields == nil {
			return nil, nil, fmt.Errorf("角色存档不是有效对象")
		}
		value, err := json.Marshal(selection)
		if err != nil {
			return nil, nil, err
		}
		fields["cube_contract_selection"] = value
		next, err := json.Marshal(fields)
		return next, json.RawMessage(`{}`), err
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return ack, nil
}
