package character

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
)

type odysseyHonorStore interface {
	CommitOdysseyHonorMail(context.Context, int64, int64, string, func(Character, bool) (json.RawMessage, json.RawMessage, json.RawMessage, error)) (Character, bool, error)
}

// Rechecked inside the character lock. A legacy bag receipt suppresses mail.
func (s *ProgressionService) ApplyOdysseyHonorMail(role Character, paid bool) (json.RawMessage, json.RawMessage, json.RawMessage, error) {
	if s.CompletionRewards == nil || role.ConfigVersion != savecontract.Identity() || !CreatedAsOdyssey(role) {
		return nil, nil, nil, fmt.Errorf("invalid Odyssey honor recipient")
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, nil, nil, err
	}
	if state.Level < OdysseyGraduationLevel {
		return nil, nil, nil, fmt.Errorf("Odyssey honor requires level 115")
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &doc); err != nil {
		return nil, nil, nil, err
	}
	doc["odyssey_graduation_reward_owed"] = json.RawMessage(`0`)
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, nil, nil, err
	}
	r := s.CompletionRewards.Honor
	var item json.RawMessage
	if !paid {
		if s.CompletionAwarder == nil || s.CompletionAwarder.Catalog.Items[r.Template].Kind != "stackable" {
			return nil, nil, nil, fmt.Errorf("Odyssey honor item source missing")
		}
		mail := inventory.MailItem{Stack: &inventory.BagItem{Template: r.Template, Amount: r.Count, ExpireTime: inventory.GrantExpireTime}}
		if _, err = mail.Row(); err != nil {
			return nil, nil, nil, err
		}
		item, err = json.Marshal(mail)
		if err != nil {
			return nil, nil, nil, err
		}
	}
	proof, err := json.Marshal(map[string]any{"template": r.Template, "count": r.Count, "legacy_reward_paid": paid})
	return raw, proof, item, err
}

// Honour mail is independent of graduation and bag space. Reconnects and
// return-to-town retry failed delivery; receipt + mail + debt are atomic.
func (s *ProgressionService) OdysseyHonorMail(ctx context.Context, role Character) (Character, bool, error) {
	if s.CompletionRewards == nil || !CreatedAsOdyssey(role) {
		return role, false, nil
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return role, false, err
	}
	if state.Level < OdysseyGraduationLevel {
		return role, false, nil
	}
	store, ok := s.Store.(odysseyHonorStore)
	if !ok {
		return role, false, fmt.Errorf("Odyssey honor mail store missing")
	}
	next, applied, err := store.CommitOdysseyHonorMail(ctx, role.AccountID, role.ID, savecontract.Identity(), s.ApplyOdysseyHonorMail)
	if err != nil {
		return role, false, err
	}
	next.WireID = role.WireID
	return next, applied, nil
}
