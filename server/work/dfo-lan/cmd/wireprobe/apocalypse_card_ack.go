package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"time"
)

// Source settlement has already granted every item atomically. C71 records
// only the presentation choice; the ordinary card grant would pay twice.
func (w *worldSession) apocalypseAcknowledgePaidCard(index byte) ([]outboundPacket, error) {
	if index > 3 || w.apocalypse == nil || !w.apocalypse.Rewarded || w.activeDungeon == nil || !w.activeDungeon.Completed() || w.cardPlan == nil || w.cardPlan.Run != w.activeDungeon.RunID || w.store == nil || w.loot == nil {
		return nil, fmt.Errorf("apocalypse card choice before owned paid settlement")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	svc := &workflow.ApocalypseRewardService{Store: w.store, Awarder: &inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules, Equipment: w.loot.Equipment}}
	if _, err := svc.Granted(ctx, w.role, w.apocalypse.ID); err != nil {
		return nil, err
	}
	plan := *w.cardPlan
	raw, err := w.store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, "cardplan:"+plan.Run)
	if err != nil {
		return nil, err
	}
	var frozen loot.CardPlan
	if err = json.Unmarshal(raw, &frozen); err != nil {
		return nil, err
	}
	if frozen != plan {
		return nil, fmt.Errorf("apocalypse presentation plan mismatch")
	}
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, plan.Source, "cardpick:"+plan.Run, plan.Model,
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			raw, err := json.Marshal(loot.CardReceipt{Plan: plan, Index: index})
			return current.State, raw, err
		})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	raw, err = w.store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, "cardpick:"+plan.Run)
	if err != nil {
		return nil, err
	}
	var receipt loot.CardReceipt
	if err = json.Unmarshal(raw, &receipt); err != nil {
		return nil, err
	}
	if receipt.Plan != plan || receipt.Index > 3 {
		return nil, fmt.Errorf("apocalypse card receipt mismatch")
	}
	w.cardReceipt = &receipt
	w.cardAutoPickAt = time.Time{}
	bag, err := w.loot.Bootstrap(workflow.LootRole(w.role))
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"card_inventory_committed", 0, 13, bag}, {"card_selection_ack", 1, 71, w.cardSnapshot()}}, nil
}
