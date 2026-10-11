package workflow

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/legion"
	"encoding/json"
	"fmt"
	"sort"
)

const apocalypseRewardModel = "apocalypse-source-clear-v1"

type ApocalypseRewardPlan struct {
	Run, Source, Content, Rules string
	Operation                   uint32
	Phase                       byte
	ElapsedMillis               uint64
	Reward                      legion.ClearReward
	Additional                  []legion.ClearReward `json:",omitempty"`
	AdditionalRules             string               `json:",omitempty"`
}

// Embedding retains the v1 single-box receipt fields. Existing clear/grant
// receipts and pending clears continue to award exactly their frozen contents.
type ApocalypseAwardReceipt struct {
	inventory.AwardReceipt
	Additional []inventory.AwardReceipt `json:",omitempty"`
}

func (r ApocalypseAwardReceipt) Matches(plan ApocalypseRewardPlan) bool {
	if r.Template != plan.Reward.Template || r.Amount != plan.Reward.Amount || len(r.Slots) == 0 || len(r.Additional) != len(plan.Additional) {
		return false
	}
	for i, item := range plan.Additional {
		g := r.Additional[i]
		if g.Template != item.Template || g.Amount != item.Amount || len(g.Slots) == 0 {
			return false
		}
	}
	return true
}

type ApocalypseRewardService struct {
	Store   equipmentEventStore
	Awarder *inventory.Awarder
}

func apocalypsePending(raw json.RawMessage) (map[string]json.RawMessage, map[string]ApocalypseRewardPlan, error) {
	var state map[string]json.RawMessage
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, nil, fmt.Errorf("invalid apocalypse reward character state")
	}
	pending := map[string]ApocalypseRewardPlan{}
	if b := state["apocalypse_pending_rewards"]; len(b) != 0 {
		if err := json.Unmarshal(b, &pending); err != nil {
			return nil, nil, err
		}
	}
	if pending == nil {
		pending = map[string]ApocalypseRewardPlan{}
	}
	return state, pending, nil
}

func saveApocalypsePending(state map[string]json.RawMessage, pending map[string]ApocalypseRewardPlan) (json.RawMessage, error) {
	b, err := json.Marshal(pending)
	if err != nil {
		return nil, err
	}
	state["apocalypse_pending_rewards"] = b
	return json.Marshal(state)
}

// Freeze is called only by the owned full-clear executor. The key is the
// navigation run identity, shared by all combat rooms. The user's testing
// policy permits each NEW full clear, without a weekly reward-count guard.
// Pending survives a full bag/disconnect; the same clear never rerolls.
func (s *ApocalypseRewardService) Freeze(ctx context.Context, role database.Character, plan ApocalypseRewardPlan) (database.Character, ApocalypseRewardPlan, error) {
	if s == nil || s.Store == nil || s.Awarder == nil || plan.Run == "" || len(plan.Rules) != 64 || plan.Operation == 0 || plan.Phase == 0 || plan.Source != role.ConfigVersion || plan.Source != s.Awarder.Catalog.Source.SaveIdentity() || plan.Content != s.Awarder.Catalog.Source.Checksum || plan.Reward.Template == 0 || plan.Reward.Amount == 0 {
		return role, ApocalypseRewardPlan{}, fmt.Errorf("apocalypse full-clear reward identity missing")
	}
	if len(plan.Additional) > 35 || (len(plan.Additional) > 0 && len(plan.AdditionalRules) != 64) {
		return role, ApocalypseRewardPlan{}, fmt.Errorf("apocalypse basic reward source identity missing")
	}
	for _, item := range plan.Additional {
		if item.Template == 0 || item.Amount == 0 {
			return role, ApocalypseRewardPlan{}, fmt.Errorf("invalid apocalypse basic reward product")
		}
	}
	return commitEquipmentEvent(ctx, s.Store, role, "apocalypse-clear:"+plan.Run, apocalypseRewardModel, func(current database.Character) (json.RawMessage, ApocalypseRewardPlan, error) {
		state, pending, err := apocalypsePending(current.State)
		if err != nil {
			return nil, ApocalypseRewardPlan{}, err
		}
		pending[plan.Run] = plan
		next, err := saveApocalypsePending(state, pending)
		return next, plan, err
	})
}

func (s *ApocalypseRewardService) Claim(ctx context.Context, role database.Character, run string) (database.Character, ApocalypseAwardReceipt, error) {
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" {
		return role, ApocalypseAwardReceipt{}, fmt.Errorf("apocalypse reward service missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "apocalypse-grant:"+run, apocalypseRewardModel, func(current database.Character) (json.RawMessage, ApocalypseAwardReceipt, error) {
		_, pending, err := apocalypsePending(current.State)
		plan, ok := pending[run]
		if err != nil || !ok || plan.Run != run || plan.Source != current.ConfigVersion || plan.Source != s.Awarder.Catalog.Source.SaveIdentity() || plan.Content != s.Awarder.Catalog.Source.Checksum {
			return nil, ApocalypseAwardReceipt{}, fmt.Errorf("owned persisted apocalypse clear required")
		}
		next, box, err := s.Awarder.Grant(current.State, plan.Reward.Template, plan.Reward.Amount)
		receipt := ApocalypseAwardReceipt{AwardReceipt: box}
		if err != nil {
			return nil, receipt, err
		}
		for _, item := range plan.Additional {
			var grant inventory.AwardReceipt
			next, grant, err = s.Awarder.Grant(next, item.Template, item.Amount)
			if err != nil {
				return nil, receipt, err
			}
			receipt.Additional = append(receipt.Additional, grant)
		}
		state, _, err := apocalypsePending(next)
		if err != nil {
			return nil, receipt, err
		}
		delete(pending, run)
		next, err = saveApocalypsePending(state, pending)
		return next, receipt, err
	})
}

func (s *ApocalypseRewardService) Granted(ctx context.Context, role database.Character, run string) (ApocalypseRewardPlan, error) {
	var plan ApocalypseRewardPlan
	var grant ApocalypseAwardReceipt
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" {
		return plan, fmt.Errorf("apocalypse reward service missing")
	}
	for key, value := range map[string]any{"apocalypse-clear:" + run: &plan, "apocalypse-grant:" + run: &grant} {
		raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
		if err != nil {
			return plan, err
		}
		if err = json.Unmarshal(raw, value); err != nil {
			return plan, err
		}
	}
	if plan.Run != run || plan.Source != role.ConfigVersion || plan.Source != s.Awarder.Catalog.Source.SaveIdentity() || plan.Content != s.Awarder.Catalog.Source.Checksum || !grant.Matches(plan) {
		return ApocalypseRewardPlan{}, fmt.Errorf("apocalypse clear/grant receipt mismatch")
	}
	return plan, nil
}

// Recover retries only already frozen clears. It never treats logging in or
// opening the legion UI as a new clear. Changed sources retain their pending
// plan; a failed grant cannot overwrite unrelated saved fields.
func (s *ApocalypseRewardService) Recover(ctx context.Context, role database.Character) (database.Character, error) {
	_, pending, err := apocalypsePending(role.State)
	if err != nil {
		return role, err
	}
	var runs []string
	for run := range pending {
		runs = append(runs, run)
	}
	sort.Strings(runs)
	for _, run := range runs {
		saved, _, err := s.Claim(ctx, role, run)
		if saved.ID != 0 {
			role = saved
		}
		if err != nil {
			return role, err
		}
	}
	return role, nil
}
