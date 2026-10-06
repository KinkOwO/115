package database

import (
	"context"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
)

const MaxLevelRewardMailEvent = "max-level-reward-mail-v1"

// One mail per character, ever: the event key is the receipt, so a replay
// inside the same lock reports applied=false. Existing rows and tables only —
// no save migration is needed for characters already at the cap.
func (s *Store) CommitMaxLevelRewardMail(ctx context.Context, account, id int64, version, title, text string,
	apply func(Character) (json.RawMessage, json.RawMessage, json.RawMessage, error)) (Character, bool, error) {
	if apply == nil {
		return Character{}, false, fmt.Errorf("max level reward callback missing")
	}
	if title == "" || text == "" {
		return Character{}, false, fmt.Errorf("max level reward mail wording missing")
	}
	return s.CommitCharacterEventTx(ctx, account, id, version, MaxLevelRewardMailEvent, MaxLevelRewardMailEvent, func(tx *Tx, role Character) (json.RawMessage, json.RawMessage, error) {
		state, proof, item, err := apply(role)
		if err != nil {
			return nil, nil, err
		}
		var attachment inventory.MailItem
		if err = json.Unmarshal(item, &attachment); err != nil {
			return nil, nil, err
		}
		if _, err = attachment.Row(); err != nil {
			return nil, nil, err
		}
		mailID, err := insertSystemMailTx(ctx, tx.queries, id, title, text, []MailAsset{{Item: item}})
		if err != nil {
			return nil, nil, err
		}
		var outcome map[string]json.RawMessage
		if err = json.Unmarshal(proof, &outcome); err != nil {
			return nil, nil, err
		}
		outcome["mail_id"], _ = json.Marshal(mailID)
		proof, err = json.Marshal(outcome)
		return state, proof, err
	})
}
