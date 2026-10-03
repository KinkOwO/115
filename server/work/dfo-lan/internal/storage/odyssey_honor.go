package storage

import (
	"context"
	"dfolan/internal/db"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

const OdysseyHonorMailEvent = "odyssey-honor-mail-v1"

// Uses existing mailbox and event tables; no player schema or save migration.
func (s *Store) CommitOdysseyHonorMail(ctx context.Context, account, id int64, version string,
	apply func(Character, bool) (json.RawMessage, json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	if apply == nil {
		return Character{}, false, fmt.Errorf("Odyssey honor callback missing")
	}
	return s.CommitCharacterEventTx(ctx, account, id, version, OdysseyHonorMailEvent, OdysseyHonorMailEvent, func(tx db.Tx, role Character) (json.RawMessage, json.RawMessage, error) {
		var paid bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_events WHERE character_id=$1 AND event_key='odyssey-graduate-reward-v1')`, id).Scan(&paid); err != nil {
			return nil, nil, err
		}
		state, proof, item, err := apply(role, paid)
		if err != nil {
			return nil, nil, err
		}
		if !paid {
			var attachment inventory.MailItem
			if err = json.Unmarshal(item, &attachment); err != nil {
				return nil, nil, err
			}
			if _, err = attachment.Row(); err != nil {
				return nil, nil, err
			}
			mailID, err := insertSystemMailTx(ctx, tx, id, "Arad Odyssey", "Congratulations on reaching Level 115! Please claim your Arad Odyssey Honor Reward Box.", []MailAsset{{Item: item}})
			if err != nil {
				return nil, nil, err
			}
			var outcome map[string]json.RawMessage
			if err = json.Unmarshal(proof, &outcome); err != nil {
				return nil, nil, err
			}
			outcome["mail_id"], _ = json.Marshal(mailID)
			proof, err = json.Marshal(outcome)
			if err != nil {
				return nil, nil, err
			}
		}
		return state, proof, nil
	})
}
