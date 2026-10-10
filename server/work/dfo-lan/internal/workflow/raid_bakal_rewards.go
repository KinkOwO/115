package workflow

import (
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"time"
)

const bakalRewardModel = "bakal-source-reward-v1"

var ErrBakalWeeklyClearLimit = errors.New("巴卡尔本周通关次数已用完")

type BakalRewardPlan struct {
	Mode              catalog.BakalMode
	CardSlots         uint32
	Cards             []BakalFrozenCard
	Run, Source, Week string
	Content           string
	Items             []catalog.BakalRewardEntry
	// Products freezes the resolved source wrappers. Nil means a legacy v1
	// receipt and retains that receipt's original one-template grant contract.
	Products []loot.Award
	Eligible bool
}
type bakalProgress struct {
	Week            string
	Clears, Rewards uint32
	Pending         map[string]BakalRewardPlan
}
type BakalRewardService struct {
	NativeCards bool
	Store       equipmentEventStore
	Awarder     *inventory.Awarder
	Boxes       loot.RewardBoxSource
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
		if r.Mode != catalog.BakalNormal && r.Mode != catalog.BakalHard {
			return nil, empty, fmt.Errorf("invalid raid reward mode")
		}
		plan := BakalRewardPlan{Mode: r.Mode, Run: run, Source: current.ConfigVersion, Week: week, Content: s.Awarder.Catalog.Source.Checksum}
		if s.NativeCards {
			if r.RewardCardSlots == 0 || r.RewardCardSlots > 255 {
				return nil, empty, fmt.Errorf("invalid native raid card slots")
			}
			plan.CardSlots = r.RewardCardSlots
		}
		p.Clears++
		plan.Eligible = clears >= r.MinimumClearCount && (r.WeeklyRewardCount == 0 || p.Rewards < r.WeeklyRewardCount)
		if plan.Eligible {
			for _, kind := range []string{"party_card", "squad_item"} {
				if s.NativeCards {
					plan.Cards = append(plan.Cards, BakalFrozenCard{Kind: byte(len(plan.Cards)), Products: make([]loot.Award, 0)})
				}
				var pool []catalog.BakalRewardEntry
				var total uint64
				for _, item := range r.Rewards {
					if item.Kind == kind && (item.Template > 0 || item.Template == -1) {
						if item.Weight == 0 {
							return nil, empty, fmt.Errorf("native raid reward weight is zero")
						}
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
						if item.Template > 0 {
							plan.Items = append(plan.Items, item)
						}
						break
					}
					value -= uint64(item.Weight)
				}
			}
			plan.Products = make([]loot.Award, 0)
			for _, item := range plan.Items {
				award := loot.Award{Template: uint32(item.Template), Amount: 1}
				cardIndex := 0
				if item.Kind == "squad_item" {
					cardIndex = 1
				}
				if s.Boxes == nil {
					definition, ok := s.Awarder.Catalog.Items[award.Template]
					if !ok || definition.StackableType == "[booster]" || definition.StackableType == "[virtual]" {
						return nil, empty, fmt.Errorf("native raid reward wrapper resolver missing")
					}
					plan.Products = append(plan.Products, award)
					if s.NativeCards {
						plan.Cards[cardIndex].Products = append(plan.Cards[cardIndex].Products, award)
					}
					continue
				}
				seed, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 32))
				if err != nil {
					return nil, empty, err
				}
				products, _, skipped := loot.OpenRewardBoxes(uint32(seed.Uint64()), s.Boxes, []loot.Award{award})
				for _, why := range skipped {
					if why != "attunement_empty_prize_4294967295" {
						return nil, empty, fmt.Errorf("native raid reward wrapper unresolved: %s", why)
					}
				}
				plan.Products = append(plan.Products, products...)
				if s.NativeCards {
					plan.Cards[cardIndex].Products = append(plan.Cards[cardIndex].Products, products...)
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
		if len(plan.Cards) > 0 {
			return nil, nil, fmt.Errorf("native raid cards require individual selection")
		}
		if !exists || plan.Run != run || plan.Source != current.ConfigVersion || plan.Content != s.Awarder.Catalog.Source.Checksum || current.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
			return nil, nil, fmt.Errorf("no owned source raid reward plan")
		}
		next := current.State
		var receipts []inventory.AwardReceipt
		products := plan.Products
		if products == nil {
			// Existing pending v1 receipts keep their frozen legacy contents.
			for _, item := range plan.Items {
				if item.Template <= 0 {
					return nil, nil, fmt.Errorf("invalid persisted raid reward item")
				}
				products = append(products, loot.Award{Template: uint32(item.Template), Amount: 1})
			}
		}
		for _, item := range products {
			if item.Template == 0 || item.Amount == 0 {
				return nil, nil, fmt.Errorf("invalid persisted raid reward item")
			}
			var receipt inventory.AwardReceipt
			next, receipt, err = s.Awarder.Grant(next, item.Template, item.Amount)
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
		if len(p.Pending[run].Cards) > 0 {
			for _, card := range p.Pending[run].Cards {
				if card.Claimed {
					continue
				}
				index := byte(0)
				if card.Selection != nil {
					index = *card.Selection
				}
				saved, _, e := s.SelectCard(ctx, role, run, card.Kind, index)
				if saved.ID != 0 {
					role = saved
				}
				if e != nil {
					return role, e
				}
				saved, _, e = s.ClaimCard(ctx, role, run, card.Kind)
				if saved.ID != 0 {
					role = saved
				}
				if e != nil {
					return role, e
				}
			}
			continue
		}
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
