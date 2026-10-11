package workflow

import (
	"context"
	cryptorand "crypto/rand"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"math/big"
	"time"
)

// Auction plans are separate from automatic clear rewards. No gateway may
// publish these until the native bidding notification contract is verified.
const bakalBiddingKey = "bakal_bidding_plans"

type BakalBiddingLot struct {
	Award loot.Award `json:"award"`
	Sold  bool       `json:"sold"`
	Price uint32     `json:"price"`
}
type BakalBiddingPlan struct {
	Run     string            `json:"run"`
	Source  string            `json:"source"`
	Content string            `json:"content"`
	Hard    bool              `json:"hard"`
	ReadyAt time.Time         `json:"ready_at"`
	Lots    []BakalBiddingLot `json:"lots"`
}

func readBakalBidding(state json.RawMessage) (map[string]json.RawMessage, map[string]BakalBiddingPlan, error) {
	var top map[string]json.RawMessage
	if len(state) > 0 {
		if e := json.Unmarshal(state, &top); e != nil || top == nil {
			return nil, nil, fmt.Errorf("invalid bidding character state")
		}
	}
	if top == nil {
		top = map[string]json.RawMessage{}
	}
	plans := map[string]BakalBiddingPlan{}
	if raw := top[bakalBiddingKey]; len(raw) > 0 {
		if e := json.Unmarshal(raw, &plans); e != nil {
			return nil, nil, e
		}
	}
	if plans == nil {
		plans = map[string]BakalBiddingPlan{}
	}
	return top, plans, nil
}
func saveBakalBidding(top map[string]json.RawMessage, plans map[string]BakalBiddingPlan) (json.RawMessage, error) {
	b, e := json.Marshal(plans)
	if e != nil {
		return nil, e
	}
	top[bakalBiddingKey] = b
	return json.Marshal(top)
}
func drawBakalBidding(weights []catalog.BakalBiddingRate) (int, error) {
	var sum int64
	for _, r := range weights {
		if r.Weight < 0 {
			return 0, fmt.Errorf("negative bidding weight")
		}
		sum += int64(r.Weight)
	}
	if sum <= 0 {
		return 0, fmt.Errorf("empty source bidding pool")
	}
	n, e := cryptorand.Int(cryptorand.Reader, big.NewInt(sum))
	if e != nil {
		return 0, e
	}
	v := n.Int64()
	for _, r := range weights {
		if v < int64(r.Weight) {
			return r.Grade, nil
		}
		v -= int64(r.Weight)
	}
	return 0, fmt.Errorf("invalid bidding weights")
}
func biddingCount(rows []catalog.BakalBiddingHard, hard bool) (int, error) {
	difficulty := 0
	if hard {
		difficulty = 1
	}
	for _, r := range rows {
		if r.Hard == difficulty {
			return drawBakalBidding(r.Rates)
		}
	}
	return 0, fmt.Errorf("source bidding difficulty missing")
}

func (s *BakalRewardService) drawBiddingItems(hard bool) ([]catalog.BakalRewardEntry, error) {
	b := s.Rules.Bidding
	count, e := biddingCount(b.Hards, hard)
	if e != nil {
		return nil, e
	}
	difficulty := 0
	if hard {
		difficulty = 1
	}
	var pool []catalog.BakalRewardEntry
	var weights []catalog.BakalBiddingRate
	for _, item := range b.Items {
		if item.Difficulty == difficulty {
			weights = append(weights, catalog.BakalBiddingRate{Grade: len(pool), Weight: int(item.Weight)})
			pool = append(pool, item)
		}
	}
	var items []catalog.BakalRewardEntry
	for i := 0; i < count; i++ {
		pick, e := drawBakalBidding(weights)
		if e != nil {
			return nil, e
		}
		items = append(items, pool[pick])
	}
	weekly, e := biddingCount(b.WeeklyCounts, hard)
	if e != nil {
		return nil, e
	}
	for i := 0; i < weekly; i++ {
		group, e := drawBakalBidding(b.WeeklyRates)
		if e != nil {
			return nil, e
		}
		p := b.WeeklyGroups[group]
		weights = nil
		for j, row := range p {
			weights = append(weights, catalog.BakalBiddingRate{Grade: j, Weight: row.Weight})
		}
		pick, e := drawBakalBidding(weights)
		if e != nil {
			return nil, e
		}
		row := p[pick]
		// Nonzero trailing values require the native equipment/auction decoder.
		if row.Values != [5]int{} {
			return nil, fmt.Errorf("unbound native weekly bidding item values")
		}
		items = append(items, catalog.BakalRewardEntry{Category: "weekly_bidding", Template: row.Template, Weight: uint32(row.Weight), Amount: 1, Difficulty: difficulty})
	}
	return items, nil
}

// FreezeBidding authenticates against the durable eligible clear receipt.
// Draws and resolved products commit once; reconnect/retry never redraws.
func (s *BakalRewardService) FreezeBidding(ctx context.Context, role database.Character, run string, hard bool, now time.Time) (BakalBiddingPlan, database.Character, error) {
	if s == nil || s.Store == nil || s.Rules == nil || s.Loot == nil || run == "" {
		return BakalBiddingPlan{}, role, fmt.Errorf("bidding source missing")
	}
	raw, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "bakal-clear:"+run)
	if e != nil {
		return BakalBiddingPlan{}, role, e
	}
	var clear BakalRewardPlan
	if e = json.Unmarshal(raw, &clear); e != nil {
		return BakalBiddingPlan{}, role, e
	}
	if !clear.Eligible || clear.Run != run || clear.Hard != hard || clear.Source != role.ConfigVersion || clear.Content != s.Content {
		return BakalBiddingPlan{}, role, fmt.Errorf("bidding requires owned eligible clear")
	}
	saved, plan, e := commitEquipmentEvent(ctx, s.Store, role, "bakal-bidding-freeze:"+run, "bakal-bidding-v1", func(current database.Character) (json.RawMessage, BakalBiddingPlan, error) {
		top, plans, e := readBakalBidding(current.State)
		if e != nil {
			return nil, BakalBiddingPlan{}, e
		}
		items, e := s.drawBiddingItems(hard)
		if e != nil {
			return nil, BakalBiddingPlan{}, e
		}
		products, e := s.openProducts(items)
		if e != nil {
			return nil, BakalBiddingPlan{}, e
		}
		plan := BakalBiddingPlan{Run: run, Source: current.ConfigVersion, Content: s.Content, Hard: hard, ReadyAt: now.Add(time.Duration(s.Rules.Bidding.StartDelaySecs) * time.Second)}
		for _, product := range products {
			if product.Template == 0 || product.Amount == 0 {
				return nil, plan, fmt.Errorf("invalid bidding product")
			}
			plan.Lots = append(plan.Lots, BakalBiddingLot{Award: product})
		}
		plans[run] = plan
		state, e := saveBakalBidding(top, plans)
		return state, plan, e
	})
	if e == nil && (plan.Source != role.ConfigVersion || plan.Content != s.Content || plan.Hard != hard) {
		e = fmt.Errorf("bidding receipt source/mode conflict")
	}
	return plan, saved, e
}

// PurchaseBiddingLot is the single-owner payout transaction, not a packet
// handler or a timer policy. The future verified controller selects the won
// lot/price. Gold and the source reward commit together; sold lots cannot be
// bought again, and event replay restores the original payout.
func (s *BakalRewardService) PurchaseBiddingLot(ctx context.Context, role database.Character, run string, lot int, price uint32, now time.Time) (database.Character, BakalBiddingLot, error) {
	if s == nil || s.Store == nil || s.Loot == nil || s.Rules == nil || run == "" || lot < 0 || price == 0 {
		return role, BakalBiddingLot{}, fmt.Errorf("invalid bidding purchase")
	}
	raw, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "bakal-bidding-freeze:"+run)
	if err != nil {
		return role, BakalBiddingLot{}, err
	}
	var frozen BakalBiddingPlan
	if err = json.Unmarshal(raw, &frozen); err != nil {
		return role, BakalBiddingLot{}, err
	}
	if frozen.Run != run || frozen.Source != role.ConfigVersion || frozen.Content != s.Content || lot >= len(frozen.Lots) {
		return role, BakalBiddingLot{}, fmt.Errorf("bidding freeze identity conflict")
	}
	return commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-bidding-purchase:%s:%d", run, lot), "bakal-bidding-v1", func(current database.Character) (json.RawMessage, BakalBiddingLot, error) {
		top, plans, e := readBakalBidding(current.State)
		if e != nil {
			return nil, BakalBiddingLot{}, e
		}
		plan, ok := plans[run]
		if !ok || plan.Source != current.ConfigVersion || plan.Content != s.Content || lot >= len(plan.Lots) || now.Before(plan.ReadyAt) {
			return nil, BakalBiddingLot{}, fmt.Errorf("bidding lot unavailable")
		}
		row := plan.Lots[lot]
		if row.Sold {
			return nil, row, fmt.Errorf("bidding lot already sold")
		}
		bag, e := inventory.ReadBag(current.State)
		if e != nil {
			return nil, row, e
		}
		if bag.Gold < price {
			return nil, row, fmt.Errorf("insufficient bidding gold")
		}
		bag.Gold -= price
		state, e := inventory.SaveBag(current.State, bag)
		if e != nil {
			return nil, row, e
		}
		awarder := inventory.Awarder{Catalog: s.Loot.Catalog, Rules: s.Loot.BagRules, Equipment: s.Loot.Equipment}
		state, _, e = awarder.Grant(state, row.Award.Template, row.Award.Amount)
		if e != nil {
			return nil, row, e
		}
		top, _, e = readBakalBidding(state)
		if e != nil {
			return nil, row, e
		}
		row.Sold, row.Price = true, price
		plan.Lots[lot] = row
		plans[run] = plan
		state, e = saveBakalBidding(top, plans)
		return state, row, e
	})
}
