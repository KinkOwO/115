package workflow

import (
	"context"
	"crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"math/big"
	"sort"
	"time"
)

type LegionRewardPlan struct {
	Run, Source, ContentChecksum, Content, Week string
	Dungeon                                     uint32
	Eligible                                    bool
	Basic, Additional                           []loot.Award
	Roots                                       []catalog.LegionOperationReward
}
type legionRewardProgress struct {
	Week    string
	Counts  map[string]uint32
	Pending map[string]LegionRewardPlan
}
type LegionRewardService struct {
	Store   equipmentEventStore
	Awarder *inventory.Awarder
	Boxes   loot.RewardBoxSource
	// Explicit operator policy; source quantities and pools still apply.
	Unlimited bool
	// Draw allows deterministic boundary verification. Production uses crypto/rand.
	Draw func(uint64) (uint64, error)
}

func readLegionRewards(raw json.RawMessage) (map[string]json.RawMessage, legionRewardProgress, error) {
	var state map[string]json.RawMessage
	var p legionRewardProgress
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, p, fmt.Errorf("invalid legion owner state")
	}
	if b := state["legion_phase_rewards"]; len(b) > 0 {
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, p, err
		}
	}
	if p.Counts == nil {
		p.Counts = map[string]uint32{}
	}
	if p.Pending == nil {
		p.Pending = map[string]LegionRewardPlan{}
	}
	return state, p, nil
}
func saveLegionRewards(state map[string]json.RawMessage, p legionRewardProgress) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	state["legion_phase_rewards"] = b
	return json.Marshal(state)
}
func (s *LegionRewardService) resolve(pool []catalog.LegionOperationReward, denominator uint32) (catalog.LegionOperationReward, []loot.Award, error) {
	var zero catalog.LegionOperationReward
	var total uint64
	for _, row := range pool {
		if row.Template == 0 || row.Amount == 0 || row.Weight == 0 {
			return zero, nil, fmt.Errorf("invalid source phase reward")
		}
		total += uint64(row.Weight)
	}
	if total == 0 {
		return zero, nil, fmt.Errorf("empty source phase pool")
	}
	limit := total
	if denominator != 0 {
		limit = uint64(denominator)
	}
	if total > limit {
		return zero, nil, fmt.Errorf("legion weights exceed source probability rate")
	}
	var value uint64
	var err error
	if s.Draw != nil {
		value, err = s.Draw(limit)
	} else {
		var draw *big.Int
		draw, err = rand.Int(rand.Reader, new(big.Int).SetUint64(limit))
		if err == nil {
			value = draw.Uint64()
		}
	}
	if err != nil {
		return zero, nil, err
	}
	if value >= limit {
		return zero, nil, fmt.Errorf("legion lottery outside source range")
	}
	if value >= total {
		return zero, nil, nil
	}
	for _, row := range pool {
		if value >= uint64(row.Weight) {
			value -= uint64(row.Weight)
			continue
		}
		seed, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 32))
		if err != nil {
			return zero, nil, err
		}
		products, _, skipped := loot.OpenRewardBoxes(uint32(seed.Uint64()), s.Boxes, []loot.Award{{Template: row.Template, Amount: row.Amount}})
		for _, reason := range skipped {
			if reason != "attunement_empty_prize_4294967295" {
				return zero, nil, fmt.Errorf("unresolved source legion wrapper: %s", reason)
			}
		}
		if len(products) == 0 {
			return zero, nil, fmt.Errorf("unresolved source legion wrapper: %v", skipped)
		}
		for _, a := range products {
			item, ok := s.Awarder.Catalog.Items[a.Template]
			if !ok || a.Amount == 0 || item.StackableType == "[virtual]" {
				return zero, nil, fmt.Errorf("invalid resolved legion product")
			}
		}
		return row, products, nil
	}
	return zero, nil, fmt.Errorf("invalid source lottery")
}

func (s *LegionRewardService) Freeze(ctx context.Context, role database.Character, r catalog.LegionPhaseRewards, run string, now time.Time) (database.Character, LegionRewardPlan, error) {
	var empty LegionRewardPlan
	if s == nil || s.Store == nil || s.Awarder == nil || s.Boxes == nil || role.ID <= 0 || role.AccountID <= 0 || run == "" || r.Content == "" || r.Dungeon == 0 || r.WeeklyLimit == 0 || r.Checksum != s.Awarder.Catalog.Source.Checksum || role.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
		return role, empty, fmt.Errorf("legion source/owner missing")
	}
	saved, frozen, err := commitEquipmentEvent(ctx, s.Store, role, "legion-phase-freeze:"+run, "legion-phase-source-v1", func(current database.Character) (json.RawMessage, LegionRewardPlan, error) {
		state, p, err := readLegionRewards(current.State)
		if err != nil {
			return nil, empty, err
		}
		week := database.IspinsWeekStart(now).Format(time.RFC3339)
		if p.Week != week {
			p.Week = week
			p.Counts = map[string]uint32{}
		}
		key := fmt.Sprintf("%s:%d", r.Content, r.Dungeon)
		plan := LegionRewardPlan{Run: run, Source: current.ConfigVersion, ContentChecksum: r.Checksum, Content: r.Content, Dungeon: r.Dungeon, Week: week, Eligible: s.Unlimited || p.Counts[key] < r.WeeklyLimit}
		if plan.Eligible {
			root, products, err := s.resolve(r.Basic, r.ProbabilityRate)
			if err != nil {
				return nil, empty, err
			}
			if root.Template != 0 {
				plan.Roots = append(plan.Roots, root)
			}
			plan.Basic = products
			root, products, err = s.resolve(r.Additional, r.ProbabilityRate)
			if err != nil {
				return nil, empty, err
			}
			if root.Template != 0 {
				plan.Roots = append(plan.Roots, root)
			}
			plan.Additional = products
			if len(r.GlobalClear) > 0 {
				root, products, err = s.resolve(r.GlobalClear, r.ProbabilityRate)
				if err != nil {
					return nil, empty, err
				}
				if root.Template != 0 {
					plan.Roots = append(plan.Roots, root)
				}
				plan.Basic = append(plan.Basic, products...)
			}
			// Current native consumers: 35 basic and 15 additional records per owner.
			if len(plan.Basic) > 35 || len(plan.Additional) > 15 {
				return nil, empty, fmt.Errorf("legion rewards exceed native display capacity")
			}
			if p.Counts[key] < ^uint32(0) {
				p.Counts[key]++
			}
			p.Pending[run] = plan
		}
		next, err := saveLegionRewards(state, p)
		return next, plan, err
	})
	if err == nil && (frozen.Run != run || frozen.Dungeon != r.Dungeon || frozen.Content != r.Content || frozen.ContentChecksum != r.Checksum || frozen.Source != role.ConfigVersion) {
		err = fmt.Errorf("legion frozen receipt belongs to another source phase")
	}
	return saved, frozen, err
}

func (s *LegionRewardService) Claim(ctx context.Context, role database.Character, run string) (database.Character, []inventory.AwardReceipt, error) {
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" {
		return role, nil, fmt.Errorf("legion reward service missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "legion-phase-grant:"+run, "legion-phase-source-v1", func(current database.Character) (json.RawMessage, []inventory.AwardReceipt, error) {
		_, p, err := readLegionRewards(current.State)
		if err != nil {
			return nil, nil, err
		}
		plan, ok := p.Pending[run]
		if !ok || !plan.Eligible || plan.Run != run || plan.Source != current.ConfigVersion || plan.ContentChecksum != s.Awarder.Catalog.Source.Checksum || current.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() {
			return nil, nil, fmt.Errorf("no owned frozen legion reward")
		}
		next := current.State
		var receipts []inventory.AwardReceipt
		for _, item := range append(append([]loot.Award{}, plan.Basic...), plan.Additional...) {
			if item.Template == 0 || item.Amount == 0 {
				return nil, nil, fmt.Errorf("invalid frozen legion product")
			}
			var receipt inventory.AwardReceipt
			next, receipt, err = s.Awarder.Grant(next, item.Template, item.Amount)
			if err != nil {
				return nil, nil, err
			}
			receipts = append(receipts, receipt)
		}
		state, _, err := readLegionRewards(next)
		if err != nil {
			return nil, nil, err
		}
		delete(p.Pending, run)
		next, err = saveLegionRewards(state, p)
		return next, receipts, err
	})
}
func (s *LegionRewardService) Recover(ctx context.Context, role database.Character) (database.Character, error) {
	_, p, err := readLegionRewards(role.State)
	if err != nil {
		return role, err
	}
	runs := make([]string, 0, len(p.Pending))
	for run := range p.Pending {
		runs = append(runs, run)
	}
	sort.Strings(runs)
	for _, run := range runs {
		saved, _, e := s.Claim(ctx, role, run)
		if saved.ID != 0 {
			role = saved
		}
		if e != nil {
			return role, e
		}
	}
	return role, nil
}
