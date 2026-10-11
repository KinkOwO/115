package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

func (w *worldSession) bakalCardSelectionPlan(selection protocol.RaidCardSelection115) ([]outboundPacket, error) {
	p := w.bakalCardPlan
	service := w.bakalRewardService()
	if p == nil || service == nil || !p.Eligible || p.Source != w.role.ConfigVersion || p.Run != w.bakalSettledRun || uint32(selection.Index) >= p.CardSlots {
		return nil, fmt.Errorf("no owned native Bakal card")
	}
	// PC-room entitlement is not granted by an offline raid clear. The native
	// optional third request still receives its own empty list, releasing its
	// category without adding a paid entitlement or consuming another pool.
	if selection.Kind == 2 {
		body, err := protocol.RaidRewardList115(2, w.role.WireID, w.role.Name, nil)
		if err != nil {
			return nil, err
		}
		return []outboundPacket{{"bakal_no_pcroom_reward", 0, 587, body}}, nil
	}
	index := -1
	for i, card := range p.Cards {
		if card.Kind == selection.Kind {
			index = i
		}
	}
	if index < 0 {
		return nil, fmt.Errorf("native card category not in frozen plan")
	}
	// Encode before committing any grant: a display overflow must leave the
	// player's choice and award pending rather than pay an undisplayable card.
	var items []protocol.RaidRewardItem115
	for _, frozen := range p.Cards {
		for _, product := range frozen.Products {
			left := product.Amount
			for left > 0 {
				count := left
				if count > 32767 {
					count = 32767
				}
				items = append(items, protocol.RaidRewardItem115{Template: product.Template, Amount: uint16(count)})
				left -= count
			}
		}
	}
	// The native sender submits both choices. The observed N587 category0
	// publishes the combined owned card items after those decisions complete.
	body, err := protocol.RaidRewardList115(0, w.role.WireID, w.role.Name, items)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, _, err := service.SelectCard(ctx, w.role, p.Run, selection.Kind, selection.Index)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, err
	}
	saved, card, err := service.ClaimCard(ctx, w.role, p.Run, selection.Kind)
	if saved.ID != 0 {
		w.role = saved
	}
	if err != nil {
		return nil, err
	}
	p.Cards[index] = card
	inventory, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return nil, err
	}
	plan := []outboundPacket{{"bakal_card_inventory", 0, 13, inventory}}
	complete := true
	for _, card := range p.Cards {
		complete = complete && card.Claimed
	}
	if complete {
		if err := w.beginBakalAuction(time.Now()); err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_native_reward_list", 0, 587, body}, outboundPacket{"bakal_native_reward_decision", 0, 2302, protocol.RaidRewardDecision115()})
		if w.bakalAuction == nil || w.bakalAuction.Finished {
			plan = append(plan, outboundPacket{"bakal_rewards_finished", 0, 574, []byte{0, 0}})
		}
	}
	return plan, nil
}
