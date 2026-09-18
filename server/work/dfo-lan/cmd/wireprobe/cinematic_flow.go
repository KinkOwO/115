package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"time"
)

func cinematicRestore(raw json.RawMessage) ([]byte, error) {
	var state struct {
		Scenes []uint16 `json:"cinematic_skipped"`
	}
	if err := json.Unmarshal(raw, &state); err != nil {
		return nil, err
	}
	return protocol.CinematicSkippedScenes(state.Scenes)
}

func cinematicSkip(store *storage.Store, w *worldSession, p []byte) error {
	if w == nil || w.role.ID == 0 {
		return fmt.Errorf("cinematic requires character")
	}
	id, err := protocol.DecodeCinematicSkip(p)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, fmt.Sprintf("cinematic-skip:%d", id), "cinematic-skip-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		var fields map[string]json.RawMessage
		if e := json.Unmarshal(current.State, &fields); e != nil {
			return nil, nil, e
		}
		var ids []uint16
		if value := fields["cinematic_skipped"]; value != nil {
			if e := json.Unmarshal(value, &ids); e != nil {
				return nil, nil, e
			}
		}
		found := false
		for _, v := range ids {
			found = found || v == id
		}
		if !found {
			ids = append(ids, id)
		}
		if _, e := protocol.CinematicSkippedScenes(ids); e != nil {
			return nil, nil, e
		}
		fields["cinematic_skipped"], _ = json.Marshal(ids)
		raw, e := json.Marshal(fields)
		receipt, _ := json.Marshal(id)
		return raw, receipt, e
	})
	if err == nil {
		saved.WireID = w.role.WireID
		w.role = saved
	}
	return err
}
