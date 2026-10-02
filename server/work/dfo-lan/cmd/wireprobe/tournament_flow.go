package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/json"
	"fmt"
	"math"
	"time"
)

func (w *worldSession) tournamentClear() ([]byte, error) {
	if w.activeDungeon == nil || w.activeDungeon.Tournament == nil || !w.activeDungeon.Completed() {
		return nil, fmt.Errorf("tournament is not complete")
	}
	run := w.activeDungeon.Tournament
	if !run.RewardReady {
		if w.loot == nil || w.loot.CardPolicy == nil {
			return nil, fmt.Errorf("tournament card source unavailable")
		}
		rate, items, err := dungeon.TournamentPrizeRules(w.activeDungeon.Definition)
		if err != nil {
			return nil, err
		}
		gold, err := loot.CardGold(w.loot.Tables, *w.loot.CardPolicy, run.Seed, byte(w.activeDungeon.Definition.BasisLevel), 0)
		if err != nil {
			return nil, err
		}
		if uint64(gold)*uint64(rate) > math.MaxUint32 {
			return nil, fmt.Errorf("tournament gold overflow")
		}
		for i := 0; i < 2; i++ {
			run.Rewards[0][i] = protocol.TournamentCard{Amount: gold * rate}
		}
		var total uint32
		for _, item := range items {
			total += item[1]
		}
		if total == 0 {
			return nil, fmt.Errorf("empty tournament item rates")
		}
		// The bracket seed freezes both hidden cards for the entire run.
		rng := loot.RNG{Seed: run.Seed}
		for i := 0; i < 2; i++ {
			roll := rng.Next(total)
			for _, item := range items {
				if roll < item[1] {
					run.Rewards[1][i] = protocol.TournamentCard{Template: item[0], Amount: item[2]}
					break
				}
				roll -= item[1]
			}
			if _, ok := w.loot.Catalog.Items[run.Rewards[1][i].Template]; !ok {
				return nil, fmt.Errorf("tournament item missing from loot catalog")
			}
		}
		run.Selected = [2]byte{255, 255}
		run.RewardReady = true
	}
	return protocol.TournamentClearReward(4, run.Rewards)
}

func (w *worldSession) tournamentSelectState(p []byte) ([]outboundPacket, error) {
	if len(p) != 0 || w.activeDungeon == nil || w.activeDungeon.Tournament == nil || !w.activeDungeon.Tournament.RewardReady {
		return nil, fmt.Errorf("tournament reward state unavailable")
	}
	return []outboundPacket{{"tournament_select_state_ack", 1, 449, protocol.TournamentSelectState()}}, nil
}

func (w *worldSession) tournamentSelect(p []byte) ([]outboundPacket, error) {
	cardType, cardIndex, err := decodeTournamentChoice(p)
	if err != nil || w.activeDungeon == nil || w.activeDungeon.Tournament == nil || !w.activeDungeon.Tournament.RewardReady {
		return nil, fmt.Errorf("invalid tournament card choice: %v", err)
	}
	run := w.activeDungeon.Tournament
	if run.Selected[cardType] < 2 {
		return []outboundPacket{{"tournament_select_ack", 1, 450, protocol.TournamentSelection(run.Selected)}}, nil
	}
	card := run.Rewards[cardType][cardIndex]
	if w.loot == nil || w.store == nil {
		return nil, fmt.Errorf("tournament award storage unavailable")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("tournament-card:%s:%d", w.activeDungeon.RunID, cardType)
	saved, _, err := w.store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "tournament-card-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
		award := inventory.Awarder{Catalog: w.loot.Catalog, Rules: w.loot.BagRules}
		state, _, err := award.Grant(current.State, card.Template, card.Amount)
		if err != nil {
			return nil, nil, err
		}
		receipt, err := json.Marshal(struct {
			Run   string                  `json:"run"`
			Type  byte                    `json:"type"`
			Index byte                    `json:"index"`
			Card  protocol.TournamentCard `json:"card"`
		}{w.activeDungeon.RunID, cardType, cardIndex, card})
		return state, receipt, err
	})
	if err != nil {
		return nil, err
	}
	saved.WireID = w.role.WireID
	w.role = saved
	run.Selected[cardType] = cardIndex
	bag, err := w.items.Bootstrap(workflow.InventoryRole(saved))
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"tournament_reward_inventory", 0, 13, bag}, {"tournament_select_ack", 1, 450, protocol.TournamentSelection(run.Selected)}}, nil
}

func decodeTournamentChoice(p []byte) (byte, byte, error) {
	// The native sender writes two choice bytes into its fixed 16-byte game
	// command record. Live solo CMD450 bodies were 00 01 / 01 00 followed by
	// fourteen zero bytes, not a two-byte transport body.
	if len(p) != 16 || p[0] > 1 || p[1] > 1 {
		return 0, 0, fmt.Errorf("invalid tournament card choice length or index")
	}
	for _, reserved := range p[2:] {
		if reserved != 0 {
			return 0, 0, fmt.Errorf("nonzero tournament card reserved bytes")
		}
	}
	return p[0], p[1], nil
}
