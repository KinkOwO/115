package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type MembershipService struct {
	Store    *database.Store
	Rules    activity.MembershipRules
	Items    catalog.ItemIndex
	Location *time.Location
	Now      func() time.Time
}
type MembershipResult struct {
	Progress    activity.MembershipProgress
	CreatedMail bool
}

func NewMembershipService(store *database.Store, rules activity.MembershipRules, items catalog.ItemIndex, loc *time.Location) (*MembershipService, error) {
	s := &MembershipService{Store: store, Rules: rules, Items: items, Location: loc, Now: time.Now}
	return s, s.validate()
}
func (s *MembershipService) validate() error {
	if s == nil || s.Store == nil || s.Location == nil || s.Rules.SourceChecksum != s.Items.Source.Checksum {
		return fmt.Errorf("membership requires source, account storage and calendar")
	}
	if e := s.Rules.Validate(); e != nil {
		return e
	}
	for _, r := range s.Rules.Rewards() {
		i, ok := s.Items.Items[r.Item]
		if !ok || i.Kind != "stackable" || i.Path == "" || (i.StackLimit > 0 && r.Count > i.StackLimit) {
			return fmt.Errorf("unsupported membership reward %d", r.Item)
		}
	}
	return nil
}
func (s *MembershipService) progress(p database.AccountAdventure) (activity.MembershipProgress, error) {
	var out activity.MembershipProgress
	raw := p.Data.Attendance[s.Rules.CampaignKey()]
	if len(raw) > 0 {
		if string(raw) == "null" {
			return out, fmt.Errorf("null membership save")
		}
		if e := json.Unmarshal(raw, &out); e != nil {
			return out, e
		}
	}
	return out, s.Rules.ValidateProgress(out)
}
func (s *MembershipService) save(p *database.AccountAdventure, progress activity.MembershipProgress) error {
	key := s.Rules.CampaignKey()
	fields := map[string]json.RawMessage{}
	if raw := p.Data.Attendance[key]; len(raw) > 0 {
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			return fmt.Errorf("invalid membership object")
		}
	}
	raw, e := json.Marshal(progress)
	if e != nil {
		return e
	}
	var known map[string]json.RawMessage
	if e = json.Unmarshal(raw, &known); e != nil {
		return e
	}
	for _, k := range []string{"version", "spent", "high_water", "claimed", "repeat_claimed"} {
		delete(fields, k)
	}
	for k, v := range known {
		fields[k] = v
	}
	raw, e = json.Marshal(fields)
	if e != nil {
		return e
	}
	if p.Data.Attendance == nil {
		p.Data.Attendance = map[string]json.RawMessage{}
	}
	p.Data.Attendance[key] = raw
	return nil
}
func (s *MembershipService) Bind() error {
	if e := s.validate(); e != nil {
		return e
	}
	if e := s.Store.RegisterCashAccountObserver(func(role database.Character, receipt database.CashReceipt, p *database.AccountAdventure) (bool, error) {
		if receipt.Charged == 0 {
			return false, nil
		}
		if role.ConfigVersion != s.Items.Source.SaveIdentity() {
			return false, fmt.Errorf("membership purchase source mismatch")
		}
		progress, e := s.progress(*p)
		if e != nil {
			return false, e
		}
		progress, e = s.Rules.Observe(progress, s.Now().In(s.Location), receipt.Charged)
		if errors.Is(e, activity.ErrRoutineInactive) {
			return false, nil
		}
		if e != nil {
			return false, e
		}
		return true, s.save(p, progress)
	}); e != nil {
		return e
	}
	return s.Store.RegisterCharacterAccountTxObserver(func(ctx context.Context, tx *database.Tx, role database.Character, key string, outcome json.RawMessage, p *database.AccountAdventure) (bool, error) {
		if !strings.HasPrefix(key, "clear:") {
			return false, nil
		}
		var receipt character.ClearReceipt
		if e := json.Unmarshal(outcome, &receipt); e != nil {
			return false, e
		}
		var benefit *activity.MembershipPayback
		for i := range s.Rules.Paybacks {
			if s.Rules.Paybacks[i].Dungeon == receipt.DungeonID {
				benefit = &s.Rules.Paybacks[i]
				break
			}
		}
		if benefit == nil {
			return false, nil
		}
		if receipt.Source != s.Items.Source.SaveIdentity() || role.ConfigVersion != receipt.Source || receipt.Run == "" || key != "clear:"+receipt.Run || receipt.CompletedAt.IsZero() {
			return false, fmt.Errorf("membership payback requires owned clear receipt")
		}
		progress, e := s.progress(*p)
		if e != nil {
			return false, e
		}
		// Delayed clear receipts may arrive after a newer purchase. Eligibility
		// uses committed account grade, while activity dates use the clear time.
		if !s.Rules.Active(receipt.CompletedAt.In(s.Location)) {
			return false, nil
		}
		reward, ok := benefit.Grades[s.Rules.Grade(progress.Spent)]
		if !ok {
			return false, nil
		}
		if e = sendMembershipBundle(ctx, tx, []activity.Reward{reward}); e != nil {
			return false, e
		}
		return false, nil // mailbox is committed by the owning clear transaction
	})
}
func sendMembershipBundle(ctx context.Context, tx *database.Tx, rewards []activity.Reward) error {
	if len(rewards) == 0 {
		return nil
	}
	grants := make([]database.GrantItem, 0, len(rewards))
	for _, r := range rewards {
		grants = append(grants, database.GrantItem{Template: r.Item, Amount: r.Count})
	}
	assets, e := database.SystemMailAssets(0, grants)
	if e != nil {
		return e
	}
	_, e = tx.SendSystemMail(ctx, rewards[0].MailTitle, rewards[0].MailMessage, assets)
	return e
}
func (s *MembershipService) Run(ctx context.Context, role database.Character, now time.Time, group string, index uint32, observation string) (MembershipResult, error) {
	var result MembershipResult
	if e := s.validate(); e != nil {
		return result, e
	}
	if role.ID <= 0 || role.AccountID <= 0 || role.ConfigVersion != s.Items.Source.SaveIdentity() || observation == "" {
		return result, fmt.Errorf("owned membership session required")
	}
	_, profile, _, e := s.Store.CommitAdventureTx(ctx, role.AccountID, role.ID, fmt.Sprintf("membership:%s:%s:%d:%s", s.Rules.CampaignKey(), group, index, observation), func(tx *database.Tx, current database.Character, p *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
		if current.ConfigVersion != role.ConfigVersion {
			return nil, nil, fmt.Errorf("membership source changed")
		}
		progress, e := s.progress(*p)
		if e != nil {
			return nil, nil, e
		}
		progress, e = s.Rules.Observe(progress, now.In(s.Location), 0)
		if e != nil {
			return nil, nil, e
		}
		if group != "" {
			var rewards []activity.Reward
			progress, rewards, e = s.Rules.Claim(progress, group, index)
			if e != nil {
				return nil, nil, e
			}
			if e = sendMembershipBundle(ctx, tx, rewards); e != nil {
				return nil, nil, e
			}
			result.CreatedMail = len(rewards) > 0
		}
		if e = s.save(p, progress); e != nil {
			return nil, nil, e
		}
		return current.State, json.RawMessage(`{}`), nil
	})
	if e != nil {
		return MembershipResult{}, e
	}
	result.Progress, e = s.progress(profile)
	return result, e
}
