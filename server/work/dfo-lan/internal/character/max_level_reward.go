package character

import (
	"context"
	"dfolan/internal/inventory"
	"dfolan/internal/savecontract"
	"encoding/json"
	"fmt"
)

type maxLevelRewardStore interface {
	CommitMaxLevelRewardMail(context.Context, int64, int64, string, string, string, func(Character) (json.RawMessage, json.RawMessage, json.RawMessage, error)) (Character, bool, error)
}

// ApplyMaxLevelRewardMail is pure: the store re-checks it inside the character
// lock. The level cap comes from the growth rules the same service already uses
// to stop experience, so nothing here declares a second cap.
func (s *ProgressionService) ApplyMaxLevelRewardMail(role Character) (json.RawMessage, json.RawMessage, json.RawMessage, error) {
	if s.MaxLevelReward == nil || role.ConfigVersion != savecontract.Identity() {
		return nil, nil, nil, fmt.Errorf("invalid max level reward recipient")
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return nil, nil, nil, err
	}
	if s.Rules.LevelCap < 2 || state.Level < s.Rules.LevelCap {
		return nil, nil, nil, fmt.Errorf("max level reward requires level %d", s.Rules.LevelCap)
	}
	r := s.MaxLevelReward
	mail := inventory.MailItem{Stack: &inventory.BagItem{Template: r.Template, Amount: r.Count, ExpireTime: inventory.GrantExpireTime}}
	if _, err := mail.Row(); err != nil {
		return nil, nil, nil, err
	}
	item, err := json.Marshal(mail)
	if err != nil {
		return nil, nil, nil, err
	}
	proof, err := json.Marshal(map[string]any{"template": r.Template, "count": r.Count, "level": state.Level, "level_cap": s.Rules.LevelCap, "definition": r.Definition.SHA256, "source": r.Source})
	if err != nil {
		return nil, nil, nil, err
	}
	return role.State, proof, item, nil
}

// MaxLevelRewardMail delivers the source's [maxlevel reward] box once per
// character. The durable event key makes reconnects, town returns and dungeon
// settlement retries harmless, so characters already at the cap are
// compensated on their next login without a save migration.
func (s *ProgressionService) MaxLevelRewardMail(ctx context.Context, role Character) (Character, bool, error) {
	if s.MaxLevelReward == nil {
		return role, false, nil
	}
	var state State
	if err := json.Unmarshal(role.State, &state); err != nil {
		return role, false, err
	}
	if s.Rules.LevelCap < 2 || state.Level < s.Rules.LevelCap {
		return role, false, nil
	}
	store, ok := s.Store.(maxLevelRewardStore)
	if !ok {
		return role, false, fmt.Errorf("max level reward mail store missing")
	}
	next, applied, err := store.CommitMaxLevelRewardMail(ctx, role.AccountID, role.ID, savecontract.Identity(), s.MaxLevelReward.Title, s.MaxLevelReward.Text, s.ApplyMaxLevelRewardMail)
	if err != nil {
		return role, false, err
	}
	next.WireID = role.WireID
	return next, applied, nil
}
