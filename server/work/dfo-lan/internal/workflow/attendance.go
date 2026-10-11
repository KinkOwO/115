package workflow

import (
	"context"
	"dfolan/internal/activity"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"
)

var ErrAttendanceInactive = errors.New("attendance is outside its source calendar or level limit")

func NewAttendanceService(store *database.Store, rules activity.Login, items catalog.ItemIndex) (*AttendanceService, error) {
	s := &AttendanceService{Store: store, Rules: rules, Items: items}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return s, nil
}

// AttendanceService executes the parsed cumulative login source. It has no
// packet, timer, alternate reward table or fallback source of its own.
type AttendanceService struct {
	Store *database.Store
	Rules activity.Login
	Items catalog.ItemIndex
}

type AttendanceResult struct {
	Progress    activity.LoginProgress
	MailID      int64
	CreatedMail bool
}

func (s *AttendanceService) validate() error {
	if s == nil || s.Store == nil || len(s.Items.Source.Checksum) != 64 || s.Rules.SourceChecksum != s.Items.Source.Checksum || s.Rules.MailTextTitle == "" || s.Rules.MailTextMessage == "" {
		return fmt.Errorf("attendance requires store, selected source and resolved mail text")
	}
	for _, day := range s.Rules.Days {
		for _, reward := range day.Rewards {
			item, ok := s.Items.Items[reward.Item]
			// This campaign's direct grants are stackable boxes/tickets. Equipment
			// and avatar mail construction requires their own metadata contract.
			if !ok || item.ID != reward.Item || item.Path == "" || item.Kind != "stackable" || reward.Count == 0 {
				return fmt.Errorf("attendance reward %d is not a prepared stackable", reward.Item)
			}
			if item.StackLimit > 0 && reward.Count > item.StackLimit {
				return fmt.Errorf("attendance reward exceeds source stack limit")
			}
		}
	}
	return nil
}

func attendanceProgress(p database.AccountAdventure, campaign string) (activity.LoginProgress, error) {
	var progress activity.LoginProgress
	if raw := p.Data.Attendance[campaign]; len(raw) > 0 {
		if string(raw) == "null" {
			return progress, fmt.Errorf("null attendance save")
		}
		if err := json.Unmarshal(raw, &progress); err != nil {
			return progress, err
		}
	}
	return progress, nil
}

func (s *AttendanceService) Observe(ctx context.Context, role database.Character, now time.Time, gameDay string) (AttendanceResult, error) {
	return s.run(ctx, role, now, gameDay, nil)
}

func (s *AttendanceService) Claim(ctx context.Context, role database.Character, now time.Time, gameDay string, day uint32) (AttendanceResult, error) {
	return s.run(ctx, role, now, gameDay, &day)
}

func (s *AttendanceService) run(ctx context.Context, role database.Character, now time.Time, gameDay string, claim *uint32) (AttendanceResult, error) {
	var result AttendanceResult
	if err := s.validate(); err != nil {
		return result, err
	}
	if role.AccountID <= 0 || role.ID <= 0 || role.ConfigVersion != s.Items.Source.SaveIdentity() {
		return result, fmt.Errorf("owned attendance character required")
	}
	campaign := s.Rules.CampaignKey()
	key := "attendance-login:" + campaign + ":" + gameDay
	if claim != nil {
		key = "attendance-claim:" + campaign + ":" + strconv.FormatUint(uint64(*claim), 10)
	}
	_, profile, receipt, err := s.Store.CommitAdventureTx(ctx, role.AccountID, role.ID, key,
		func(tx *database.Tx, current database.Character, profile *database.AccountAdventure) (json.RawMessage, json.RawMessage, error) {
			if current.ConfigVersion != role.ConfigVersion {
				return nil, nil, fmt.Errorf("attendance character source changed")
			}
			var base struct {
				Level uint32 `json:"level"`
			}
			if err := json.Unmarshal(current.State, &base); err != nil {
				return nil, nil, err
			}
			progress, err := attendanceProgress(*profile, campaign)
			if err != nil {
				return nil, nil, err
			}
			progress, active, err := s.Rules.Observe(progress, now, gameDay, base.Level)
			if err != nil {
				return nil, nil, err
			}
			if !active {
				return nil, nil, ErrAttendanceInactive
			}
			var mailID int64
			if claim != nil {
				var rewards []activity.Reward
				var grant bool
				progress, rewards, grant, err = s.Rules.Claim(progress, *claim)
				if err != nil {
					return nil, nil, err
				}
				if grant {
					grants := make([]database.GrantItem, 0, len(rewards))
					for _, r := range rewards {
						grants = append(grants, database.GrantItem{Template: r.Item, Amount: r.Count})
					}
					assets, err := database.SystemMailAssets(0, grants)
					if err != nil {
						return nil, nil, err
					}
					mailID, err = tx.SendSystemMail(ctx, s.Rules.MailTextTitle, s.Rules.MailTextMessage, assets)
					if err != nil {
						return nil, nil, err
					}
					result.CreatedMail = true
				}
			}
			raw, err := json.Marshal(progress)
			if err != nil {
				return nil, nil, err
			}
			if profile.Data.Attendance == nil {
				profile.Data.Attendance = map[string]json.RawMessage{}
			}
			profile.Data.Attendance[campaign] = raw
			proof, err := json.Marshal(struct {
				MailID int64 `json:"mail_id"`
			}{mailID})
			return current.State, proof, err
		})
	if err != nil {
		return AttendanceResult{}, err
	}
	result.Progress, err = attendanceProgress(profile, campaign)
	if err != nil {
		return AttendanceResult{}, err
	}
	var proof struct {
		MailID int64 `json:"mail_id"`
	}
	if err = json.Unmarshal(receipt, &proof); err != nil {
		return AttendanceResult{}, err
	}
	result.MailID = proof.MailID
	return result, nil
}
