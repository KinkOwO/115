package workflow

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"sort"
	"time"
)

type BakalAuctionLot struct {
	Template        uint32
	Products        []loot.Award
	Price           uint32
	Bids            uint32
	Passed, Settled bool
}
type BakalAuctionPlan struct {
	Run, Source, Content                           string
	Mode                                           catalog.BakalMode
	Lots                                           []BakalAuctionLot
	Index                                          int
	ReadyAt, Deadline                              time.Time
	MinimumPrice                                   uint32
	IncreaseRatio                                  float64
	Time, BreakTime, LowestTime, DecreaseAfterBids uint32
	DecreasePerBid                                 float32
	Finished                                       bool
}
type BakalAuctionService struct {
	Store   equipmentEventStore
	Awarder *inventory.Awarder
	Boxes   loot.RewardBoxSource
}

func readBakalAuctions(raw json.RawMessage) (map[string]json.RawMessage, map[string]BakalAuctionPlan, error) {
	var state map[string]json.RawMessage
	if err := json.Unmarshal(raw, &state); err != nil || state == nil {
		return nil, nil, fmt.Errorf("invalid auction owner state")
	}
	plans := map[string]BakalAuctionPlan{}
	if b := state["bakal_raid_auctions"]; len(b) > 0 {
		if err := json.Unmarshal(b, &plans); err != nil || plans == nil {
			return nil, nil, fmt.Errorf("invalid persisted auctions")
		}
	}
	return state, plans, nil
}
func saveBakalAuction(state map[string]json.RawMessage, plans map[string]BakalAuctionPlan, p BakalAuctionPlan) (json.RawMessage, error) {
	plans[p.Run] = p
	b, err := json.Marshal(plans)
	if err != nil {
		return nil, err
	}
	state["bakal_raid_auctions"] = b
	return json.Marshal(state)
}
func weightedBakalNumber(pool []catalog.BakalWeightedNumber, fixed *uint64) (uint32, error) {
	var total uint64
	for _, row := range pool {
		total += uint64(row.Weight)
	}
	if total == 0 {
		return 0, fmt.Errorf("empty source auction probability pool")
	}
	var value uint64
	if fixed != nil {
		value = *fixed % total
	} else {
		n, e := rand.Int(rand.Reader, new(big.Int).SetUint64(total))
		if e != nil {
			return 0, e
		}
		value = n.Uint64()
	}
	for _, row := range pool {
		if value < uint64(row.Weight) {
			return row.Value, nil
		}
		value -= uint64(row.Weight)
	}
	return 0, fmt.Errorf("invalid auction pool")
}
func (s *BakalAuctionService) valid(role database.Character) bool {
	return s != nil && s.Store != nil && s.Awarder != nil && s.Awarder.Catalog.Source.Checksum != "" && s.Boxes != nil && role.ID > 0 && role.AccountID > 0 && role.ConfigVersion == s.Awarder.Catalog.Source.SaveIdentity()
}

func (s *BakalAuctionService) Snapshot(role database.Character, run string) (BakalAuctionPlan, error) {
	_, _, p, e := s.owned(role, run)
	return p, e
}
func (s *BakalAuctionService) Pending(role database.Character) ([]BakalAuctionPlan, error) {
	if !s.valid(role) {
		return nil, fmt.Errorf("auction source/owner missing")
	}
	_, plans, e := readBakalAuctions(role.State)
	if e != nil {
		return nil, e
	}
	var out []BakalAuctionPlan
	for run := range plans {
		p, e := s.Snapshot(role, run)
		if e != nil {
			return nil, e
		}
		if !p.Finished {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Run < out[j].Run })
	return out, nil
}

// A durable eligible clear is required. The lottery and all resolved wrapper
// products are committed once, before any lot can reserve this owner's gold.
func (s *BakalAuctionService) Freeze(ctx context.Context, role database.Character, r *catalog.BakalRaidRules, run string, now time.Time) (database.Character, BakalAuctionPlan, error) {
	var empty BakalAuctionPlan
	if !s.valid(role) || r == nil || run == "" || r.Bidding.MinimumPrice == 0 || r.Bidding.IncreaseRatio <= 1 || r.Bidding.Time == 0 {
		return role, empty, fmt.Errorf("auction source/owner missing")
	}
	b, err := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, "bakal-clear:"+run)
	var clear BakalRewardPlan
	if err != nil || json.Unmarshal(b, &clear) != nil || !clear.Eligible || clear.Run != run || clear.Source != role.ConfigVersion || clear.Content != s.Awarder.Catalog.Source.Checksum || clear.Mode != r.Mode {
		return role, empty, fmt.Errorf("auction requires owned eligible source clear")
	}
	return commitEquipmentEvent(ctx, s.Store, role, "bakal-auction-freeze:"+run, "bakal-auction-source-v1", func(current database.Character) (json.RawMessage, BakalAuctionPlan, error) {
		state, plans, e := readBakalAuctions(current.State)
		if e != nil {
			return nil, empty, e
		}
		if existing, ok := plans[run]; ok {
			return nil, empty, fmt.Errorf("auction already frozen without its event receipt: %s", existing.Run)
		}
		b := r.Bidding
		p := BakalAuctionPlan{Run: run, Source: current.ConfigVersion, Content: s.Awarder.Catalog.Source.Checksum, Mode: r.Mode, ReadyAt: now.Add(time.Duration(b.StartDelay) * time.Second), MinimumPrice: b.MinimumPrice, IncreaseRatio: b.IncreaseRatio, Time: b.Time, BreakTime: b.BreakTime, LowestTime: b.LowestTime, DecreaseAfterBids: b.DecreaseAfterBids, DecreasePerBid: b.DecreasePerBid}
		count, e := weightedBakalNumber(b.Count, nil)
		if e != nil {
			return nil, empty, e
		}
		weekly, e := weightedBakalNumber(b.WeeklyCount, nil)
		if e != nil {
			return nil, empty, e
		}
		if uint64(count)+uint64(weekly) > 255 {
			return nil, empty, fmt.Errorf("source auction exceeds native byte index")
		}
		var regular []catalog.BakalWeightedNumber
		for _, row := range r.Rewards {
			if row.Kind == "bidding_item" {
				template := uint32(row.Template)
				if row.Template == -1 {
					template = 0
				}
				regular = append(regular, catalog.BakalWeightedNumber{Value: template, Weight: row.Weight})
			}
		}
		week := database.IspinsWeekStart(now).Format(time.RFC3339)
		h := sha256.Sum256([]byte(p.Content + ":" + week))
		seed := binary.LittleEndian.Uint64(h[:8])
		group, e := weightedBakalNumber(b.WeeklyGroups, &seed)
		if e != nil {
			return nil, empty, e
		}
		var weeklyPool []catalog.BakalWeightedNumber
		for _, row := range b.WeeklyItems[group] {
			weeklyPool = append(weeklyPool, catalog.BakalWeightedNumber{Value: row.Template, Weight: row.Weight})
		}
		for i := uint32(0); i < count+weekly; i++ {
			pool := regular
			if i >= count {
				pool = weeklyPool
			}
			template, e := weightedBakalNumber(pool, nil)
			if e != nil {
				return nil, empty, e
			}
			if template == 0 {
				continue
			}
			item, ok := s.Awarder.Catalog.Items[template]
			if !ok || item.Kind != "stackable" {
				return nil, empty, fmt.Errorf("auction preview requires source wrapper record %d", template)
			}
			n, e := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 32))
			if e != nil {
				return nil, empty, e
			}
			products, _, skipped := loot.OpenRewardBoxes(uint32(n.Uint64()), s.Boxes, []loot.Award{{Template: template, Amount: 1}})
			for _, why := range skipped {
				if why != "attunement_empty_prize_4294967295" {
					return nil, empty, fmt.Errorf("unresolved auction wrapper: %s", why)
				}
			}
			for _, award := range products {
				definition, ok := s.Awarder.Catalog.Items[award.Template]
				if !ok || award.Amount == 0 || definition.StackableType == "[virtual]" || definition.StackableType == "[booster]" {
					return nil, empty, fmt.Errorf("invalid auction product")
				}
			}
			p.Lots = append(p.Lots, BakalAuctionLot{Template: template, Products: products})
		}
		p.Finished = len(p.Lots) == 0
		next, e := saveBakalAuction(state, plans, p)
		return next, p, e
	})
}

func (s *BakalAuctionService) owned(role database.Character, run string) (map[string]json.RawMessage, map[string]BakalAuctionPlan, BakalAuctionPlan, error) {
	state, plans, e := readBakalAuctions(role.State)
	if e != nil {
		return nil, nil, BakalAuctionPlan{}, e
	}
	p, ok := plans[run]
	if !s.valid(role) || !ok || p.Run != run || p.Source != role.ConfigVersion || p.Content != s.Awarder.Catalog.Source.Checksum || p.Index < 0 || p.Index > len(p.Lots) {
		return nil, nil, p, fmt.Errorf("no owned source auction")
	}
	return state, plans, p, nil
}
func (s *BakalAuctionService) Open(ctx context.Context, role database.Character, run string, index int, now time.Time) (database.Character, BakalAuctionPlan, error) {
	var empty BakalAuctionPlan
	if !s.valid(role) {
		return role, empty, fmt.Errorf("auction source/owner missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-auction-open:%s:%d", run, index), "bakal-auction-source-v1", func(current database.Character) (json.RawMessage, BakalAuctionPlan, error) {
		state, plans, p, e := s.owned(current, run)
		if e != nil {
			return nil, empty, e
		}
		if p.Finished || p.Index != index || index >= len(p.Lots) || now.Before(p.ReadyAt) {
			return nil, empty, fmt.Errorf("auction lot not ready")
		}
		p.Deadline = now.Add(time.Duration(p.Time) * time.Second)
		next, e := saveBakalAuction(state, plans, p)
		return next, p, e
	})
}

// Current Bakal execution owns a one-member raid. Its bid reserves only the
// difference from this owner's prior bid; no other player's wallet is touched.
func (s *BakalAuctionService) Bid(ctx context.Context, role database.Character, run string, index int, amount uint32, pass bool, now time.Time) (database.Character, BakalAuctionPlan, error) {
	var empty BakalAuctionPlan
	if !s.valid(role) {
		return role, empty, fmt.Errorf("auction source/owner missing")
	}
	key := fmt.Sprintf("bakal-auction-bid:%s:%d:%d:%t", run, index, amount, pass)
	return commitEquipmentEvent(ctx, s.Store, role, key, "bakal-auction-source-v1", func(current database.Character) (json.RawMessage, BakalAuctionPlan, error) {
		state, plans, p, e := s.owned(current, run)
		if e != nil {
			return nil, empty, e
		}
		if p.Finished || p.Index != index || index >= len(p.Lots) || p.Deadline.IsZero() || !now.Before(p.Deadline) {
			return nil, empty, fmt.Errorf("auction lot expired or foreign")
		}
		lot := &p.Lots[index]
		if lot.Settled || lot.Passed {
			return nil, empty, fmt.Errorf("auction lot already decided")
		}
		if pass {
			if amount != 0 || lot.Price != 0 {
				return nil, empty, fmt.Errorf("cannot pass an owned reserved bid")
			}
			lot.Passed = true
			p.Deadline = now
		} else {
			minimum := uint64(p.MinimumPrice)
			if lot.Price > 0 {
				minimum = uint64(math.Ceil(float64(lot.Price) * p.IncreaseRatio))
				if minimum < uint64(lot.Price)+uint64(p.MinimumPrice) {
					minimum = uint64(lot.Price) + uint64(p.MinimumPrice)
				}
			}
			if uint64(amount) < minimum || amount > math.MaxInt32 {
				return nil, empty, fmt.Errorf("auction bid below source minimum or native price bound")
			}
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, empty, e
			}
			cost := amount - lot.Price
			if bag.Gold < cost {
				return nil, empty, fmt.Errorf("insufficient auction gold")
			}
			bag.Gold -= cost
			current.State, e = inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, empty, e
			}
			state, _, e = readBakalAuctions(current.State)
			if e != nil {
				return nil, empty, e
			}
			lot.Price = amount
			lot.Bids++
			seconds := float64(p.Time)
			if lot.Bids > p.DecreaseAfterBids {
				seconds -= float64(lot.Bids-p.DecreaseAfterBids) * float64(p.DecreasePerBid)
			}
			if seconds < float64(p.LowestTime) {
				seconds = float64(p.LowestTime)
			}
			p.Deadline = now.Add(time.Duration(seconds * float64(time.Second)))
		}
		next, e := saveBakalAuction(state, plans, p)
		return next, p, e
	})
}

func (s *BakalAuctionService) Settle(ctx context.Context, role database.Character, run string, index int, now time.Time) (database.Character, BakalAuctionPlan, error) {
	var empty BakalAuctionPlan
	if !s.valid(role) {
		return role, empty, fmt.Errorf("auction source/owner missing")
	}
	return commitEquipmentEvent(ctx, s.Store, role, fmt.Sprintf("bakal-auction-settle:%s:%d", run, index), "bakal-auction-source-v1", func(current database.Character) (json.RawMessage, BakalAuctionPlan, error) {
		_, plans, p, e := s.owned(current, run)
		if e != nil {
			return nil, empty, e
		}
		if p.Finished || p.Index != index || index >= len(p.Lots) || p.Deadline.IsZero() || now.Before(p.Deadline) {
			return nil, empty, fmt.Errorf("auction lot not settled yet")
		}
		lot := &p.Lots[index]
		next := current.State
		if lot.Price > 0 {
			for _, award := range lot.Products {
				next, _, e = s.Awarder.Grant(next, award.Template, award.Amount)
				if e != nil {
					return nil, empty, e
				}
			}
		}
		state, _, e := readBakalAuctions(next)
		if e != nil {
			return nil, empty, e
		}
		lot.Settled = true
		p.Index++
		p.ReadyAt = p.Deadline.Add(time.Duration(p.BreakTime) * time.Second)
		p.Deadline = time.Time{}
		p.Finished = p.Index == len(p.Lots)
		next, e = saveBakalAuction(state, plans, p)
		return next, p, e
	})
}

// Disconnected auctions have no new bidder. Settle a previously reserved bid
// after its deadline, retaining it on full bags; unbid lots expire without gold.
func (s *BakalAuctionService) Recover(ctx context.Context, role database.Character, now time.Time) (database.Character, error) {
	if !s.valid(role) {
		return role, fmt.Errorf("auction source/owner missing")
	}
	_, plans, e := readBakalAuctions(role.State)
	if e != nil {
		return role, e
	}
	var runs []string
	for run := range plans {
		if !plans[run].Finished {
			runs = append(runs, run)
		}
	}
	sort.Strings(runs)
	for _, run := range runs {
		for {
			_, _, p, e := s.owned(role, run)
			if e != nil {
				return role, e
			}
			if p.Finished {
				break
			}
			if p.Deadline.IsZero() {
				if now.Before(p.ReadyAt) {
					break
				}
				if now.Before(p.ReadyAt.Add(time.Duration(p.Time) * time.Second)) {
					break
				}
				role, p, e = s.Open(ctx, role, run, p.Index, p.ReadyAt)
			} else if now.Before(p.Deadline) {
				break
			}
			if e != nil {
				return role, e
			}
			role, _, e = s.Settle(ctx, role, run, p.Index, now)
			if e != nil {
				return role, e
			}
		}
	}
	return role, nil
}
