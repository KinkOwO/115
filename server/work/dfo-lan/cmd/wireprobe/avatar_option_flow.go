package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"time"
)

func avatarOption(service *workflow.WearService, w *worldSession, p, keys []byte) ([]preparedPacket, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("avatar selection requires character")
	}
	r, e := protocol.DecodeAvatarOption(p)
	if e != nil {
		return nil, e
	}
	// Prepare the native acknowledgment before committing any state.
	packets, e := preparePackets(keys, []outboundPacket{{"avatar_option_selected", 1, 451, protocol.AvatarOptionSuccess(r)}})
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	hash := sha256.Sum256(p)
	saved, _, e := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("avatar-option:%x", hash), "avatar-option-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		raw, e := service.SelectAvatarOption(workflow.InventoryRole(current), r)
		if e != nil {
			return nil, nil, e
		}
		receipt, e := json.Marshal(r)
		return raw, receipt, e
	})
	if e != nil {
		return nil, e
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return packets, nil
}
