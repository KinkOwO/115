package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math/big"
	"os"
	"time"
)

// One checked [upgradable legacy] entry from the current client PVF.
type lotteryItemPool struct {
	SourceItem         uint32                   `json:"source_item"`
	SourceScript       string                   `json:"source_script"`
	SourceScriptSHA256 string                   `json:"source_script_sha256"`
	Candidates         []BoosterRewardCandidate `json:"candidates"`
	total              int64
}

const lotterySourcePVFSHA256 = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

type lotteryItemCatalog struct {
	SourcePVFSHA256 string             `json:"source_pvf_sha256"`
	Pools           []*lotteryItemPool `json:"pools"`
	byTemplate      map[uint32]*lotteryItemPool
}

func loadLotteryItemCatalog(path string, index map[uint32]ItemIndexInfo) (*lotteryItemCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c lotteryItemCatalog
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.SourcePVFSHA256 != lotterySourcePVFSHA256 || len(c.Pools) != 276 {
		return nil, fmt.Errorf("lottery catalog source identity or pool count mismatch")
	}
	c.byTemplate = make(map[uint32]*lotteryItemPool, len(c.Pools))
	for _, p := range c.Pools {
		if p == nil || p.SourceItem == 0 || len(p.SourceScriptSHA256) != 64 || len(p.Candidates) == 0 {
			return nil, fmt.Errorf("invalid lottery pool identity")
		}
		if source, ok := index[p.SourceItem]; !ok || source.Path != p.SourceScript || source.Kind != "stackable" || source.StackableType != "[upgradable legacy]" {
			return nil, fmt.Errorf("lottery source %d missing from current item index", p.SourceItem)
		}
		if c.byTemplate[p.SourceItem] != nil {
			return nil, fmt.Errorf("duplicate lottery source %d", p.SourceItem)
		}
		for _, row := range p.Candidates {
			if row.Weight == 0 || row.Count == 0 || p.total > int64(^uint64(0)>>1)-int64(row.Weight) {
				return nil, fmt.Errorf("lottery reward in source %d has invalid weight or count", p.SourceItem)
			}
			if row.Template != 0 {
				def, ok := index[row.Template]
				if !ok || def.Kind != "stackable" {
					return nil, fmt.Errorf("lottery reward %d is not a source stackable", row.Template)
				}
			}
			p.total += int64(row.Weight)
		}
		c.byTemplate[p.SourceItem] = p
	}
	if p := c.byTemplate[7772]; p == nil || p.SourceScriptSHA256 != "b6f59a8a3193ae4f6c0796e156f90d48317cbf2e16af2c309bacbdc63322e54c" || len(p.Candidates) != 209 || p.total != 98904 {
		return nil, fmt.Errorf("lottery item 7772 regression")
	}
	if p := c.byTemplate[10306598]; p == nil || p.SourceScriptSHA256 != "a97094b202704c3cfb0f1e1e53a86bc9986813cddf071acc078c98b937414063" || len(p.Candidates) != 1 || p.Candidates[0] != (BoosterRewardCandidate{Template: 0, Weight: 10000, Count: 1000000}) {
		return nil, fmt.Errorf("lottery gold pot 10306598 regression")
	}
	return &c, nil
}

func (p *lotteryItemPool) pick(draw int64) (BoosterRewardCandidate, error) {
	if p == nil || draw < 0 || draw >= p.total {
		return BoosterRewardCandidate{}, fmt.Errorf("lottery draw out of range")
	}
	for _, row := range p.Candidates {
		if draw < int64(row.Weight) {
			return row, nil
		}
		draw -= int64(row.Weight)
	}
	return BoosterRewardCandidate{}, fmt.Errorf("lottery pool underflow")
}

type lotteryItemStore interface {
	CommitCharacterEvent(context.Context, int64, int64, string, string, string, func(storage.Character) (json.RawMessage, json.RawMessage, error)) (storage.Character, bool, error)
	CharacterEventReceipt(context.Context, int64, int64, string) (json.RawMessage, error)
}

type lotteryItemReceipt struct {
	SourceTemplate uint32                                 `json:"source_template"`
	RewardTemplate uint32                                 `json:"reward_template"`
	RewardSlot     uint16                                 `json:"reward_slot"`
	GrantCount     uint32                                 `json:"grant_count"`
	Updates        [][protocol.CurrentItemRecordSize]byte `json:"updates"`
}

func (w *worldSession) openLotteryItem(ctx context.Context, store lotteryItemStore, pools *lotteryItemCatalog, index map[uint32]ItemIndexInfo, request, raw []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon != nil || w.loot == nil || pools == nil {
		return nil, fmt.Errorf("lottery item requires selected character in town and loaded catalog")
	}
	sourceSlot, err := protocol.DecodeLotteryItemUse(request)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("lottery storage unavailable")
	}
	// Preserve the v1 event key so a retransmitted 7772 request cannot spend a
	// second pot after the catalog upgrade. The raw frame keeps other slots apart.
	key := fmt.Sprintf("lottery-item-7772:%d:%x", w.role.ID, sha256.Sum256(raw))
	// A stored v1 or v2 receipt may carry a different model. Replay it before
	// committing so the storage model check never turns a retry into a failure.
	prior, err := store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, key)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	saved := w.role
	if len(prior) == 0 {
		saved, _, err = store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "lottery-item-v2", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, err
			}
			var source *inventory.BagItem
			for i := range bag.Items {
				if bag.Items[i].Slot == sourceSlot {
					source = &bag.Items[i]
					break
				}
			}
			if source == nil || source.Amount == 0 {
				return nil, nil, fmt.Errorf("slot %d has no lottery item", sourceSlot)
			}
			pool := pools.byTemplate[source.Template]
			if pool == nil {
				return nil, nil, fmt.Errorf("lottery item %d has no verified gold or stackable pool", source.Template)
			}
			if protocol.StoredItemExpired(source.ExpireTime, time.Now().Unix()) {
				return nil, nil, fmt.Errorf("lottery item %d has expired", source.Template)
			}
			roll, err := rand.Int(rand.Reader, big.NewInt(pool.total))
			if err != nil {
				return nil, nil, err
			}
			reward, err := pool.pick(roll.Int64())
			if err != nil {
				return nil, nil, err
			}
			awardCatalog := catalog.LootCatalog{Source: w.loot.Catalog.Source, Items: make(map[uint32]catalog.LootItem)}
			if reward.Template != 0 {
				def, ok := index[reward.Template]
				if !ok || def.Kind != "stackable" {
					return nil, nil, fmt.Errorf("lottery reward metadata absent")
				}
				awardCatalog.Items[reward.Template] = catalog.LootItem{ID: reward.Template, Kind: def.Kind, StackableType: def.StackableType, StackLimit: def.StackLimit}
			}
			before := bag
			bag, _, err = bag.Consume(awardCatalog, sourceSlot, pool.SourceItem)
			if err != nil {
				return nil, nil, err
			}
			bag, rewardSlot, err := bag.Add(awardCatalog, w.loot.BagRules, reward.Template, reward.Count)
			if err != nil {
				return nil, nil, err
			}
			nextState, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, err
			}
			updates := inventory.ChangedItemRows(before, bag)
			if len(updates) == 0 {
				return nil, nil, fmt.Errorf("lottery inventory produced no change")
			}
			receipt, err := json.Marshal(lotteryItemReceipt{SourceTemplate: pool.SourceItem, RewardTemplate: reward.Template, RewardSlot: rewardSlot, GrantCount: reward.Count, Updates: updates})
			return nextState, receipt, err
		})
		if err != nil {
			return nil, err
		}
	}
	data := prior
	if len(data) == 0 {
		data, err = store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, key)
	}
	if err != nil {
		return nil, err
	}
	var receipt lotteryItemReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	if receipt.SourceTemplate == 0 && receipt.RewardTemplate != 0 {
		receipt.SourceTemplate = 7772 // pre-upgrade receipts had no source field
	}
	if receipt.SourceTemplate == 0 || receipt.GrantCount == 0 || len(receipt.Updates) == 0 {
		return nil, fmt.Errorf("lottery receipt incomplete")
	}
	saved.WireID = w.role.WireID
	w.role = saved
	ack := protocol.LotteryItemSuccess(sourceSlot, protocol.OrdinaryItem(receipt.RewardSlot, receipt.RewardTemplate, receipt.GrantCount))
	refresh, err := protocol.InventoryUpdate(receipt.Updates)
	if err != nil {
		return nil, err
	}
	return []outboundPacket{{"lottery_item_open", 1, 27, ack}, {"lottery_item_inventory", 0, 14, refresh}}, nil
}
