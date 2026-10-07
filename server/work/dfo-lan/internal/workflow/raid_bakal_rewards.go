package workflow

import (
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"
)

const bakalRewardModel = "bakal-source-reward-v1"

var ErrBakalWeeklyClearLimit = errors.New("普通巴卡尔本周通关次数已用完")

type BakalRewardPlan struct {
	Run, Source, Week string
	Content           string
	Items             []catalog.BakalRewardEntry
	Eligible          bool
}
type bakalProgress struct {
	Week            string
	Clears, Rewards uint32
	Pending         map[string]BakalRewardPlan
}
type BakalRewardService struct {
	Store   equipmentEventStore
	Awarder *inventory.Awarder
}

func readBakalProgress(raw json.RawMessage) (map[string]json.RawMessage, bakalProgress, error) {
	var state map[string]json.RawMessage
	var p bakalProgress
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, p, fmt.Errorf("invalid character state for raid rewards")
	}
	if b := state["bakal_raid_rewards"]; len(b) > 0 {
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, p, err
		}
	}
	if p.Pending == nil {
		p.Pending = map[string]BakalRewardPlan{}
	}
	return state, p, nil
}
func saveBakalProgress(state map[string]json.RawMessage, p bakalProgress) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	state["bakal_raid_rewards"] = b
	return json.Marshal(state)
}

func BakalRaidAdmission(role database.Character, r *catalog.BakalRaidRules, now time.Time) error {
	if r == nil {
		return fmt.Errorf("native raid rules missing")
	}
	_, p, err := readBakalProgress(role.State)
	if err != nil {
		return err
	}
	if r.WeeklyClearCount > 0 && p.Week == database.IspinsWeekStart(now).Format(time.RFC3339) && p.Clears >= r.WeeklyClearCount {
		return ErrBakalWeeklyClearLimit
	}
	return nil
}

// Freeze records the clear and its exact source lottery before touching a bag.
// Full bags retain Pending; retries/reconnects cannot reroll or award twice.
func (s *BakalRewardService) Freeze(ctx context.Context, role database.Character, r *catalog.BakalRaidRules, run string, clears uint32, now time.Time) (database.Character, BakalRewardPlan, error) {
	var empty BakalRewardPlan
	if s == nil || s.Store == nil || s.Awarder == nil || r == nil || run == "" || len(r.Rewards) == 0 || role.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
		return role, empty, fmt.Errorf("raid reward source/owner missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "bakal-clear:"+run, bakalRewardModel, func(current database.Character) (json.RawMessage, BakalRewardPlan, error) {
		state, p, err := readBakalProgress(current.State)
		if err != nil {
			return nil, empty, err
		}
		// All US raid services use the existing Tuesday 09:00 UTC reset policy.
		week := database.IspinsWeekStart(now).Format(time.RFC3339)
		if p.Week != week {
			p.Week = week
			p.Clears, p.Rewards = 0, 0
		}
		plan := BakalRewardPlan{Run: run, Source: current.ConfigVersion, Week: week, Content: s.Awarder.Catalog.Source.Checksum}
		p.Clears++
		plan.Eligible = clears >= r.MinimumClearCount && (r.WeeklyRewardCount == 0 || p.Rewards < r.WeeklyRewardCount)
		if plan.Eligible {
			for _, kind := range []string{"party_card", "squad_item"} {
				var pool []catalog.BakalRewardEntry
				var total uint64
				for _, item := range r.Rewards {
					if item.Kind == kind && item.Template > 0 {
						pool = append(pool, item)
						total += uint64(item.Weight)
					}
				}
				if total == 0 {
					return nil, empty, fmt.Errorf("native raid reward category %s is empty", kind)
				}
				draw, err := rand.Int(rand.Reader, new(big.Int).SetUint64(total))
				if err != nil {
					return nil, empty, err
				}
				value := draw.Uint64()
				for _, item := range pool {
					if value < uint64(item.Weight) {
						plan.Items = append(plan.Items, item)
						break
					}
					value -= uint64(item.Weight)
				}
			}
			p.Rewards++
			p.Pending[run] = plan
		}
		next, err := saveBakalProgress(state, p)
		return next, plan, err
	})
}

func (s *BakalRewardService) Claim(ctx context.Context, role database.Character, run string) (database.Character, []inventory.AwardReceipt, error) {
	if s == nil || s.Store == nil || s.Awarder == nil {
		return role, nil, fmt.Errorf("raid reward service missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "bakal-grant:"+run, bakalRewardModel, func(current database.Character) (json.RawMessage, []inventory.AwardReceipt, error) {
		_, p, err := readBakalProgress(current.State)
		if err != nil {
			return nil, nil, err
		}
		plan, exists := p.Pending[run]
		if !exists || plan.Run != run || plan.Source != current.ConfigVersion || plan.Content != s.Awarder.Catalog.Source.Checksum || current.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
			return nil, nil, fmt.Errorf("no owned source raid reward plan")
		}
		next := current.State
		var receipts []inventory.AwardReceipt
		for _, item := range plan.Items {
			if item.Template <= 0 {
				return nil, nil, fmt.Errorf("invalid persisted raid reward item")
			}
			var receipt inventory.AwardReceipt
			next, receipt, err = s.Awarder.Grant(next, uint32(item.Template), 1)
			if err != nil {
				return nil, nil, err
			}
			receipts = append(receipts, receipt)
		}
		state, _, err := readBakalProgress(next)
		if err != nil {
			return nil, nil, err
		}
		delete(p.Pending, run)
		next, err = saveBakalProgress(state, p)
		return next, receipts, err
	})
}

func (s *BakalRewardService) Recover(ctx context.Context, role database.Character) (database.Character, error) {
	_, p, err := readBakalProgress(role.State)
	if err != nil {
		return role, err
	}
	var runs []string
	for run := range p.Pending {
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
