package workflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

const delezieRewardModel = "delezie-source-reward-v1"

var ErrDelezieWeeklyLimit = errors.New("狄瑞吉本周通关或奖励次数已用完")

// This is a server-side lottery result, expressed as exact source row indices.
// The battle owner must establish a genuine clear and resolve shared draws,
// PC-room eligibility and pity first. Never construct it from client claims.
type DelezieRewardSelection struct {
	Rows, CommonRows []int
	PCRoom           bool
}

type DelezieRewardPlan struct {
	Mode                                  catalog.DelezieDifficulty
	Account, Role                         int64
	Run, Source, Content, Week, Selection string
	Rows                                  []catalog.DelezieRewardRow
	Common                                []catalog.DelezieCommonRewardRow
	Products                              []loot.Award
	PersonalClaimed, AuctionPending       bool
}

type delezieProgress struct {
	Week            string
	Clears, Rewards uint32
	Pending         map[string]DelezieRewardPlan
}

type DelezieRewardService struct {
	Store   equipmentEventStore
	Awarder *inventory.Awarder
	Boxes   loot.RewardBoxSource
}

func readDelezieProgress(raw json.RawMessage) (map[string]json.RawMessage, delezieProgress, error) {
	var state map[string]json.RawMessage
	var p delezieProgress
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, p, fmt.Errorf("invalid character state for Delezie rewards")
	}
	if b := state["delezie_raid_rewards"]; len(b) > 0 {
		if err := json.Unmarshal(b, &p); err != nil {
			return nil, p, err
		}
	}
	if p.Pending == nil {
		p.Pending = map[string]DelezieRewardPlan{}
	}
	return state, p, nil
}

func saveDelezieProgress(state map[string]json.RawMessage, p delezieProgress) (json.RawMessage, error) {
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	state["delezie_raid_rewards"] = b
	return json.Marshal(state)
}

func DelezieRaidAdmission(role database.Character, rules *catalog.DelezieRaidRules, now time.Time) error {
	clears, rewards, err := rules.RewardLimits()
	if err != nil {
		return err
	}
	if clears == 0 || rewards == 0 {
		return fmt.Errorf("Delezie zero weekly-limit semantics not established")
	}
	_, p, err := readDelezieProgress(role.State)
	if err != nil {
		return err
	}
	if p.Week == database.IspinsWeekStart(now).Format(time.RFC3339) && (p.Clears >= clears || p.Rewards >= rewards) {
		return ErrDelezieWeeklyLimit
	}
	return nil
}

type delezieRewardPool struct {
	kind  string
	group int32
}

func resolveDelezieSelection(program catalog.DelezieModeRewards, selection DelezieRewardSelection) (DelezieRewardPlan, error) {
	var plan DelezieRewardPlan
	rows := map[int]catalog.DelezieRewardRow{}
	pools := map[delezieRewardPool]bool{}
	for _, row := range program.Rewards {
		switch row.Kind {
		case "bidding_item":
			// An auction prize is not a personal clear reward. Keep the
			// auction obligation pending; this service cannot choose a winner.
			plan.AuctionPending = true
			continue
		case "party_card", "squad_item", "special_item":
		case "pcroom_card":
			if !selection.PCRoom {
				continue
			}
		default:
			return plan, fmt.Errorf("unsupported Delezie reward category %q", row.Kind)
		}
		if _, duplicate := rows[row.Row]; duplicate || row.Weight == 0 || row.Template == 0 || row.Template < -1 {
			return plan, fmt.Errorf("invalid Delezie source reward row %d", row.Row)
		}
		rows[row.Row] = row
		pools[delezieRewardPool{row.Kind, row.Group}] = false
	}
	for _, index := range selection.Rows {
		row, ok := rows[index]
		if !ok {
			return plan, fmt.Errorf("unowned/ineligible Delezie reward row %d", index)
		}
		pool := delezieRewardPool{row.Kind, row.Group}
		if pools[pool] {
			return plan, fmt.Errorf("duplicate Delezie result for %s/%d", row.Kind, row.Group)
		}
		pools[pool] = true
		plan.Rows = append(plan.Rows, row)
	}
	if len(pools) == 0 {
		return plan, fmt.Errorf("empty Delezie personal reward program")
	}
	for pool, selected := range pools {
		if !selected {
			return plan, fmt.Errorf("missing Delezie result for %s/%d", pool.kind, pool.group)
		}
	}
	common := map[int]catalog.DelezieCommonRewardRow{}
	commonPools := map[string]bool{}
	for _, row := range program.Common {
		if row.Kind != "squad_item" {
			return plan, fmt.Errorf("unsupported Delezie common reward category %q", row.Kind)
		}
		if _, duplicate := common[row.Row]; duplicate || row.Kind == "" || row.Weight == 0 || row.Template == 0 || row.Template < -1 {
			return plan, fmt.Errorf("invalid Delezie common source row %d", row.Row)
		}
		common[row.Row], commonPools[row.Kind] = row, false
	}
	for _, index := range selection.CommonRows {
		row, ok := common[index]
		if !ok || commonPools[row.Kind] {
			return plan, fmt.Errorf("unowned/duplicate Delezie common result %d", index)
		}
		commonPools[row.Kind] = true
		plan.Common = append(plan.Common, row)
	}
	for kind, selected := range commonPools {
		if !selected {
			return plan, fmt.Errorf("missing Delezie shared result for %s", kind)
		}
	}
	sort.Slice(plan.Rows, func(i, j int) bool { return plan.Rows[i].Row < plan.Rows[j].Row })
	sort.Slice(plan.Common, func(i, j int) bool { return plan.Common[i].Row < plan.Common[j].Row })
	encoded, err := json.Marshal(struct {
		Rows   []catalog.DelezieRewardRow
		Common []catalog.DelezieCommonRewardRow
		PCRoom bool
	}{plan.Rows, plan.Common, selection.PCRoom})
	if err != nil {
		return plan, err
	}
	digest := sha256.Sum256(encoded)
	plan.Selection = hex.EncodeToString(digest[:])
	return plan, nil
}

func (s *DelezieRewardService) resolveWrapper(template int32) ([]loot.Award, error) {
	if template == -1 {
		return nil, nil
	}
	award := loot.Award{Template: uint32(template), Amount: 1}
	if s.Boxes == nil {
		item, exists := s.Awarder.Catalog.Items[award.Template]
		if !exists || item.StackableType == "[booster]" || item.StackableType == "[virtual]" {
			return nil, fmt.Errorf("Delezie reward wrapper resolver missing for %d", template)
		}
		return []loot.Award{award}, nil
	}
	seed, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 32))
	if err != nil {
		return nil, err
	}
	products, _, skipped := loot.OpenRewardBoxes(uint32(seed.Uint64()), s.Boxes, []loot.Award{award})
	for _, why := range skipped {
		if why != "attunement_empty_prize_4294967295" {
			return nil, fmt.Errorf("unresolved Delezie reward wrapper: %s", why)
		}
	}
	return products, nil
}

// FreezeResolvedAwards atomically records an authoritative clear's resolved
// personal outcome and expands its source wrappers exactly once. It does not
// implement the missing battle end predicate, pity/shared lottery or auction.
func (s *DelezieRewardService) FreezeResolvedAwards(ctx context.Context, role database.Character, rules *catalog.DelezieRaidRules, mode *catalog.DelezieModeRules, run string, selection DelezieRewardSelection, now time.Time) (database.Character, DelezieRewardPlan, error) {
	var empty DelezieRewardPlan
	if s == nil || s.Store == nil || s.Awarder == nil || rules == nil || run == "" || role.ID <= 0 || role.AccountID <= 0 || rules.Source.Checksum == "" || rules.Source.Checksum != s.Awarder.Catalog.Source.Checksum || role.ConfigVersion != rules.Source.SaveIdentity() {
		return role, empty, fmt.Errorf("Delezie reward source/owner missing")
	}
	program, err := rules.RewardProgram(mode)
	if err != nil {
		return role, empty, err
	}
	selected, err := resolveDelezieSelection(program, selection)
	if err != nil {
		return role, empty, err
	}
	clears, rewards, err := rules.RewardLimits()
	if err != nil || clears == 0 || rewards == 0 || now.IsZero() {
		return role, empty, fmt.Errorf("invalid Delezie source weekly limits/time: %v", err)
	}
	saved, plan, err := commitEquipmentEvent(ctx, s.Store, role, "delezie-clear:"+run, delezieRewardModel, func(current database.Character) (json.RawMessage, DelezieRewardPlan, error) {
		state, p, err := readDelezieProgress(current.State)
		if err != nil {
			return nil, empty, err
		}
		week := database.IspinsWeekStart(now).Format(time.RFC3339)
		if p.Week != week {
			p.Week, p.Clears, p.Rewards = week, 0, 0
		}
		if p.Clears >= clears || p.Rewards >= rewards || p.Clears == math.MaxUint32 || p.Rewards == math.MaxUint32 {
			return nil, empty, ErrDelezieWeeklyLimit
		}
		plan := selected
		plan.Mode, plan.Account, plan.Role = mode.Difficulty, current.AccountID, current.ID
		plan.Run, plan.Source, plan.Content, plan.Week = run, current.ConfigVersion, rules.Source.Checksum, week
		plan.Products = make([]loot.Award, 0)
		for _, row := range plan.Rows {
			products, err := s.resolveWrapper(row.Template)
			if err != nil {
				return nil, empty, err
			}
			plan.Products = append(plan.Products, products...)
		}
		for _, row := range plan.Common {
			products, err := s.resolveWrapper(row.Template)
			if err != nil {
				return nil, empty, err
			}
			plan.Products = append(plan.Products, products...)
		}
		p.Clears++
		p.Rewards++
		p.Pending[run] = plan
		next, err := saveDelezieProgress(state, p)
		return next, plan, err
	})
	if err == nil && (plan.Mode != mode.Difficulty || plan.Selection != selected.Selection || plan.Content != rules.Source.Checksum || plan.Account != role.AccountID || plan.Role != role.ID) {
		return saved, empty, fmt.Errorf("conflicting Delezie clear replay")
	}
	return saved, plan, err
}

func (s *DelezieRewardService) ClaimPersonal(ctx context.Context, role database.Character, run string) (database.Character, []inventory.AwardReceipt, error) {
	if s == nil || s.Store == nil || s.Awarder == nil || run == "" {
		return role, nil, fmt.Errorf("Delezie reward service/run missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "delezie-grant:"+run, delezieRewardModel, func(current database.Character) (json.RawMessage, []inventory.AwardReceipt, error) {
		_, p, err := readDelezieProgress(current.State)
		if err != nil {
			return nil, nil, err
		}
		plan, owned := p.Pending[run]
		if !owned || plan.Run != run || plan.Source != current.ConfigVersion || plan.Content != s.Awarder.Catalog.Source.Checksum || current.ConfigVersion != s.Awarder.Catalog.Source.SaveIdentity() || plan.Account != current.AccountID || plan.Role != current.ID || plan.Products == nil || plan.PersonalClaimed {
			return nil, nil, fmt.Errorf("no owned Delezie personal reward plan")
		}
		next := current.State
		var receipts []inventory.AwardReceipt
		for _, product := range plan.Products {
			if product.Template == 0 || product.Amount == 0 {
				return nil, nil, fmt.Errorf("invalid frozen Delezie product")
			}
			var receipt inventory.AwardReceipt
			next, receipt, err = s.Awarder.Grant(next, product.Template, product.Amount)
			if err != nil {
				return nil, nil, err
			}
			receipts = append(receipts, receipt)
		}
		state, _, err := readDelezieProgress(next)
		if err != nil {
			return nil, nil, err
		}
		if plan.AuctionPending {
			plan.PersonalClaimed = true
			p.Pending[run] = plan
		} else {
			delete(p.Pending, run)
		}
		next, err = saveDelezieProgress(state, p)
		return next, receipts, err
	})
}

// RecoverPersonal retries the already frozen parcel; it cannot reroll, award
// an auction prize or settle a different mode on reconnect.
func (s *DelezieRewardService) RecoverPersonal(ctx context.Context, role database.Character) (database.Character, error) {
	if s == nil || s.Store == nil || s.Awarder == nil {
		return role, fmt.Errorf("Delezie reward recovery service missing")
	}
	_, p, err := readDelezieProgress(role.State)
	if err != nil {
		return role, err
	}
	var runs []string
	for run, plan := range p.Pending {
		if !plan.PersonalClaimed {
			runs = append(runs, run)
		}
	}
	sort.Strings(runs)
	for _, run := range runs {
		var saved database.Character
		saved, _, err = s.ClaimPersonal(ctx, role, run)
		if saved.ID != 0 {
			role = saved
		}
		if err != nil {
			return role, err
		}
	}
	return role, nil
}
