package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func synopsisRestore(raw json.RawMessage) ([]byte, error) {
	var state struct {
		Read []uint32 `json:"synopsis_read"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return protocol.SynopsisTableInfo(state.Read)
}

// Store only IDs acknowledged by this character's own CMD2079. Existing
// saves need no migration: the absent key is the empty read set. Preserve all
// other JSON fields and use the existing row-locked event transaction.
func synopsisReadState(raw json.RawMessage, id uint32) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, err
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	var ids []uint32
	if value := fields["synopsis_read"]; value != nil {
		if err := json.Unmarshal(value, &ids); err != nil {
			return nil, err
		}
	}
	found := false
	for _, v := range ids {
		found = found || v == id
	}
	if !found {
		ids = append(ids, id)
	}
	if _, err := protocol.SynopsisTableInfo(ids); err != nil {
		return nil, err
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	fields["synopsis_read"], _ = json.Marshal(ids)
	return json.Marshal(fields)
}

func saveSynopsisRead(store *database.Store, w *worldSession, id uint32) ([]byte, error) {
	if w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("synopsis requires character")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion,
		fmt.Sprintf("synopsis-read:%d", id), "synopsis-read-v1", func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			state, err := synopsisReadState(current.State, id)
			receipt, _ := json.Marshal(id)
			return state, receipt, err
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	return synopsisRestore(saved.State)
}
