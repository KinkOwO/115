package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/json"
	"fmt"
	"time"
)

type RoutineService struct {
	Store *database.Store
	Rules activity.RoutineRules
	Items catalog.ItemIndex
}
type RoutineResult struct {
	Progress    activity.RoutineProgress
	MailID      int64
	CreatedMail bool
}

func NewRoutineService(store *database.Store, rules activity.RoutineRules, items catalog.ItemIndex) (*RoutineService, error) {
	s := &RoutineService{Store: store, Rules: rules, Items: items}
	return s, s.validate()
}
func (s *RoutineService) validate() error {
	if s == nil || s.Store == nil || s.Rules.SourceChecksum != s.Items.Source.Checksum {
		return fmt.Errorf("routine requires selected archive and storage")
	}
	if e := s.Rules.Validate(); e != nil {
		return e
	}
	for _, r := range s.Rules.Rewards() {
		i, ok := s.Items.Items[r.Item]
		if !ok || i.Kind != "stackable" || i.Path == "" || (i.StackLimit > 0 && r.Count > i.StackLimit) {
			return fmt.Errorf("routine reward %d is not a supported source stackable", r.Item)
		}
	}
	return nil
}

// RoutineAction is provided by verified gateway execution. From/Now delimit
// connected server time; ProperClearKey is the existing committed clear identity.
type RoutineAction struct {
	From, Now      time.Time
	GameDay        string
	ResetHour      int
	ClaimKind      string
	ClaimIndex     uint32
	ProperClearKey string
	ObservationKey string
}

func routineProgress(p database.AccountAdventure, campaign string) (activity.RoutineProgress, error) {
	var out activity.RoutineProgress
	if raw := p.Data.Attendance[campaign]; len(raw) > 0 {
		if string(raw) == "null" {
			return out, fmt.Errorf("null routine save")
		}
		if e := json.Unmarshal(raw, &out); e != nil {
			return out, e
		}
	}
	return out, nil
}

// Retain fields written by other compatible versions of the campaign save.
// Only this service's known progress fields are updated inside the transaction.
func mergeRoutineProgress(previous json.RawMessage, progress activity.RoutineProgress) (json.RawMessage, error) {
	fields := map[string]json.RawMessage{}
	if len(previous) != 0 {
		if e := json.Unmarshal(previous, &fields); e != nil || fields == nil {
			return nil, fmt.Errorf("invalid routine save object")
		}
	}
	for _, key := range []string{"version", "high_water", "day", "sequence_day", "daily_millis", "total_millis", "period_millis", "clears", "observed", "claimed"} {
		delete(fields, key)
	}
	raw, e := json.Marshal(progress)
	if e != nil {
		return nil, e
	}
	var known map[string]json.RawMessage
	if e = json.Unmarshal(raw, &known); e != nil {
		return nil, e
	}
	for key, value := range known {
		fields[key] = value
	}
	return json.Marshal(fields)
}

func (s *RoutineService) Run(ctx context.Context, role database.Character, a RoutineAction) (RoutineResult, error) {
	var result RoutineResult
	if e := s.validate(); e != nil {
		return result, e
	}
	if role.AccountID <= 0 || role.ID <= 0 || role.ConfigVersion != s.Items.Source.SaveIdentity() {
		return result, fmt.Errorf("owned source-compatible routine character required")
	}
	campaign := s.Rules.CampaignKey()
	key := "routine:" + campaign + ":" + a.ObservationKey
	if a.ObservationKey == "" {
		return result, fmt.Errorf("routine transaction identity required")
	}
	if a.ProperClearKey != "" {
		key = "routine-clear:" + campaign + ":" + a.ProperClearKey
	}
	// Claim receipts are observation-scoped. Account progress supplies permanent
	// claim identity across roles, days and sessions; this avoids stale receipts
	// returning an old progress snapshot after another account action.
	_, profile, receipt, e := s.Store.CommitAdventureTx(ctx, role.AccountID, role.ID, key, func(tx *database.Tx, current database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		if current.ConfigVersion != role.ConfigVersion {
			return nil, nil, fmt.Errorf("routine character source changed")
		}
		var state struct {
			Level uint32 `json:"level"`
		}
		if e := json.Unmarshal(current.State, &state); e != nil {
			return nil, nil, e
		}
		progress, e := routineProgress(*p, campaign)
		if e != nil {
			return nil, nil, e
		}
		progress, e = s.Rules.Observe(progress, a.From, a.Now, a.GameDay, a.ResetHour, state.Level)
		if e != nil {
			return nil, nil, e
		}
		if a.ProperClearKey != "" {
			progress = s.Rules.ProperClear(progress, a.Now)
		}
		if e := s.Rules.ValidateProgress(progress, a.Now.Location(), a.ResetHour); e != nil {
			return nil, nil, e
		}
		var mailID int64
		if a.ClaimKind != "" {
			var r activity.Reward
			var grant bool
			progress, r, grant, e = s.Rules.Claim(progress, a.Now, a.ClaimKind, a.ClaimIndex)
			if e != nil {
				return nil, nil, e
			}
			if grant {
				assets, e := database.SystemMailAssets(0, []database.GrantItem{{Template: r.Item, Amount: r.Count}})
				if e != nil {
					return nil, nil, e
				}
				mailID, e = tx.SendSystemMail(ctx, r.MailTitle, r.MailMessage, assets)
				if e != nil {
					return nil, nil, e
				}
				result.CreatedMail = true
			}
		}
		raw, e := mergeRoutineProgress(p.Data.Attendance[campaign], progress)
		if e != nil {
			return nil, nil, e
		}
		if p.Data.Attendance == nil {
			p.Data.Attendance = map[string]json.RawMessage{}
		}
		p.Data.Attendance[campaign] = raw
		proof, e := json.Marshal(struct {
			MailID int64 `json:"mail_id"`
		}{mailID})
		return current.State, proof, e
	})
	if e != nil {
		return RoutineResult{}, e
	}
	result.Progress, e = routineProgress(profile, campaign)
	if e != nil {
		return RoutineResult{}, e
	}
	var proof struct {
		MailID int64 `json:"mail_id"`
	}
	if e = json.Unmarshal(receipt, &proof); e != nil {
		return RoutineResult{}, e
	}
	result.MailID = proof.MailID
	return result, nil
}
