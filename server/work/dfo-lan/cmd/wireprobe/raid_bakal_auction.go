package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"math"
	"time"
)

func (w *worldSession) bakalAuctionService() *workflow.BakalAuctionService {
	if w.store == nil || w.loot == nil {
		return nil
	}
	return &workflow.BakalAuctionService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}, Boxes: w.loot.RewardBoxes}
}
func (w *worldSession) beginBakalAuction(now time.Time) error {
	if w.bakalAuction != nil {
		return nil
	}
	// Unbound wire fixtures have no auction authority. Real source raids carry
	// both the bidding distributions and the shared COS price operands.
	if w.bakalRules == nil {
		return nil
	}
	s := w.bakalAuctionService()
	if s == nil {
		return fmt.Errorf("source auction service missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, e := s.Freeze(ctx, w.role, w.bakalRules, w.bakalSettledRun, now)
	if saved.ID != 0 {
		w.role = saved
	}
	if e != nil {
		return e
	}
	p, e := s.Snapshot(w.role, w.bakalSettledRun)
	if e != nil {
		return e
	}
	w.bakalAuction = &p
	return nil
}
func (w *worldSession) bakalAuctionListPlan() ([]outboundPacket, error) {
	p := w.bakalAuction
	if p == nil || p.Finished {
		return nil, nil
	}
	var templates []uint32
	for _, lot := range p.Lots[p.Index:] {
		templates = append(templates, lot.Template)
	}
	body, e := protocol.RaidAuctionItems115(templates)
	if e != nil {
		return nil, e
	}
	w.bakalAuctionBase = p.Index
	return []outboundPacket{{"bakal_auction_items", 0, 2060, body}, {"bakal_auction_join", 0, 2061, protocol.RaidAuctionJoin115(w.role.WireID, true)}}, nil
}
func auctionRemainingMillis(p *workflow.BakalAuctionPlan, now time.Time) uint32 {
	millis := p.Deadline.Sub(now).Milliseconds()
	if millis < 0 {
		return 0
	}
	if millis > math.MaxInt32 {
		return math.MaxInt32
	}
	return uint32(millis)
}

// Serialized connection ticks own the auction clock, just like the raid clock.
func (c *gameConnection) tickBakalAuction(now time.Time) error {
	w := c.worldState
	if w == nil || w.role.ID == 0 || w.channelType != 82 || now.Before(w.bakalAuctionRetryAt) {
		return nil
	}
	s := w.bakalAuctionService()
	if s == nil {
		return nil
	}
	if w.bakalAuction == nil {
		pending, e := s.Pending(w.role)
		if e != nil {
			return e
		}
		if len(pending) == 0 {
			return nil
		}
		p := pending[0]
		w.bakalAuction = &p
	}
	p := w.bakalAuction
	if p.Finished {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var plan []outboundPacket
	if !w.bakalAuctionListed {
		var e error
		plan, e = w.bakalAuctionListPlan()
		if e != nil {
			return e
		}
		w.bakalAuctionListed = true
		if !p.Deadline.IsZero() {
			plan = append(plan, outboundPacket{"bakal_auction_resumed", 0, 2059, protocol.RaidAuctionStart115(p.Lots[p.Index].Template, p.MinimumPrice)})
			update, e := protocol.RaidAuctionUpdate115(w.role.WireID, w.role.Name, 0, p.Lots[p.Index].Price, 0, auctionRemainingMillis(p, now))
			if e != nil {
				return e
			}
			plan = append(plan, outboundPacket{"bakal_auction_resumed_price", 1, 1902, update})
		}
	}
	if p.Deadline.IsZero() {
		if !now.Before(p.ReadyAt) {
			saved, _, e := s.Open(ctx, w.role, p.Run, p.Index, now)
			if saved.ID != 0 {
				w.role = saved
			}
			if e != nil {
				return e
			}
			next, e := s.Snapshot(w.role, p.Run)
			if e != nil {
				return e
			}
			w.bakalAuction = &next
			p = &next
			plan = append(plan, outboundPacket{"bakal_auction_started", 0, 2059, protocol.RaidAuctionStart115(p.Lots[p.Index].Template, p.MinimumPrice)})
		}
	} else if !now.Before(p.Deadline) {
		index := p.Index
		lot := p.Lots[index]
		saved, _, e := s.Settle(ctx, w.role, p.Run, index, now)
		if saved.ID != 0 {
			w.role = saved
		}
		if e != nil {
			w.bakalAuctionRetryAt = now.Add(5 * time.Second)
			c.event(map[string]any{"kind": "bakal_auction_award_pending", "run": p.Run, "index": index, "reason": e.Error()})
			if len(plan) > 0 {
				return c.sendPlan(plan, c.logCharacterResponseBody)
			}
			return nil
		}
		next, e := s.Snapshot(w.role, p.Run)
		if e != nil {
			return e
		}
		w.bakalAuction = &next
		result := byte(2)
		if lot.Price > 0 {
			result = 1
		}
		update, e := protocol.RaidAuctionUpdate115(w.role.WireID, w.role.Name, byte(index-w.bakalAuctionBase), lot.Price, result, 0)
		if e != nil {
			return e
		}
		bag, e := w.loot.Bootstrap(workflow.LootRole(w.role))
		if e != nil {
			return e
		}
		plan = append(plan, outboundPacket{"bakal_auction_result", 1, 1902, update}, outboundPacket{"bakal_auction_inventory", 0, 13, bag})
		if next.Finished {
			plan = append(plan, outboundPacket{"bakal_auction_left", 0, 2061, protocol.RaidAuctionJoin115(w.role.WireID, false)}, outboundPacket{"bakal_rewards_finished", 0, 574, []byte{0, 0}})
		}
		c.event(map[string]any{"kind": "bakal_auction_lot_settled", "run": p.Run, "index": index, "price": lot.Price, "result": result})
	}
	if c.sendPlan(plan, c.logCharacterResponseBody) != nil {
		return fmt.Errorf("auction delivery failed")
	}
	return c.releaseEndedBakalOpening()
}

func (w *worldSession) bakalAuctionBidPlan(request protocol.RaidAuctionBid115, now time.Time) ([]outboundPacket, error) {
	p := w.bakalAuction
	s := w.bakalAuctionService()
	if p == nil || s == nil || p.Finished || !w.bakalAuctionListed || p.Deadline.IsZero() {
		return nil, fmt.Errorf("no owned active auction lot")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, e := s.Bid(ctx, w.role, p.Run, p.Index, request.Amount, request.Pass, now)
	if saved.ID != 0 {
		w.role = saved
	}
	if e != nil {
		return nil, e
	}
	next, e := s.Snapshot(w.role, p.Run)
	if e != nil {
		return nil, e
	}
	w.bakalAuction = &next
	// Project the latest durable lot even if the bid event receipt is older.
	// Replaying a previous amount must not rewind either the UI price or clock.
	index := receipt.Index
	update, e := protocol.RaidAuctionUpdate115(w.role.WireID, w.role.Name, byte(index-w.bakalAuctionBase), next.Lots[index].Price, 0, auctionRemainingMillis(&next, now))
	if e != nil {
		return nil, e
	}
	bag, e := w.loot.Bootstrap(workflow.LootRole(w.role))
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"bakal_auction_bid_ack", 1, 1902, update}, {"bakal_auction_wallet", 0, 13, bag}}, nil
}
