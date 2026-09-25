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
	"fmt"
	"math/big"
	"os"
	"time"
)

// The catalog is a checked export of the current client's template 7772 PVF
// entry. Other lottery templates are deliberately not inferred from it.
type lotteryItemPool struct {
	SourceItem         uint32                   `json:"source_item"`
	SourceScript       string                   `json:"source_script"`
	SourceScriptSHA256 string                   `json:"source_script_sha256"`
	Candidates         []BoosterRewardCandidate `json:"candidates"`
	total              int64
}

const lottery7772ScriptHash = "418da31021ef6ad9dffec621542adc963e87cb52eb33f9b9e049f18093aa1810"

func loadLotteryItemPool(path string, index map[uint32]ItemIndexInfo) (*lotteryItemPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var p lotteryItemPool
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.SourceItem != 7772 || p.SourceScript != "stackable/ect/uplegacy_card01.stk" || p.SourceScriptSHA256 != lottery7772ScriptHash || len(p.Candidates) != 209 {
		return nil, fmt.Errorf("lottery item 7772 source identity mismatch")
	}
	if source, ok := index[p.SourceItem]; !ok || source.Path != p.SourceScript || source.StackableType != "[upgradable legacy]" {
		return nil, fmt.Errorf("lottery source missing from current item index")
	}
	seen := make(map[uint32]bool, len(p.Candidates))
	for _, row := range p.Candidates {
		def, ok := index[row.Template]
		if !ok || def.Kind != "stackable" || row.Template == 0 || row.Weight == 0 || row.Count != 1 || seen[row.Template] {
			return nil, fmt.Errorf("lottery reward %d is invalid or duplicated", row.Template)
		}
		seen[row.Template] = true
		p.total += int64(row.Weight)
	}
	if p.total != 98904 {
		return nil, fmt.Errorf("lottery weight total changed: %d", p.total)
	}
	return &p, nil
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
	RewardTemplate uint32                                 `json:"reward_template"`
	RewardSlot     uint16                                 `json:"reward_slot"`
	GrantCount     uint32                                 `json:"grant_count"`
	Updates        [][protocol.CurrentItemRecordSize]byte `json:"updates"`
}

func (w *worldSession) openLotteryItem(ctx context.Context, store lotteryItemStore, pool *lotteryItemPool, index map[uint32]ItemIndexInfo, request, raw []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.activeDungeon != nil || w.loot == nil || pool == nil {
		return nil, fmt.Errorf("lottery item requires selected character in town and loaded catalog")
	}
	sourceSlot, err := protocol.DecodeLotteryItemUse(request)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("lottery storage unavailable")
	}
	roll, err := rand.Int(rand.Reader, big.NewInt(pool.total))
	if err != nil {
		return nil, err
	}
	reward, err := pool.pick(roll.Int64())
	if err != nil {
		return nil, err
	}
	def, ok := index[reward.Template]
	if !ok || def.Kind != "stackable" {
		return nil, fmt.Errorf("lottery reward metadata absent")
	}
	// Bag.Add needs only the chosen item. Keep the shared drop catalog immutable.
	awardCatalog := catalog.LootCatalog{Source: w.loot.Catalog.Source, Items: map[uint32]catalog.LootItem{
		reward.Template: {ID: reward.Template, Kind: def.Kind, StackableType: def.StackableType, StackLimit: def.StackLimit},
	}}
	key := fmt.Sprintf("lottery-item-7772:%d:%x", w.role.ID, sha256.Sum256(raw))
	saved, _, err := store.CommitCharacterEvent(ctx, w.role.AccountID, w.role.ID, w.role.ConfigVersion, key, "lottery-item-7772-v1", func(current storage.Character) (json.RawMessage, json.RawMessage, error) {
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
		if source == nil || source.Template != pool.SourceItem || source.Amount == 0 {
			return nil, nil, fmt.Errorf("slot %d does not hold lottery item 7772", sourceSlot)
		}
		if protocol.StoredItemExpired(source.ExpireTime, time.Now().Unix()) {
			return nil, nil, fmt.Errorf("lottery item 7772 has expired")
		}
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
		updates := make([][protocol.CurrentItemRecordSize]byte, 0, 2)
		if sourceSlot != rewardSlot {
			row, _ := bag.RowAt(sourceSlot)
			if row == ([protocol.CurrentItemRecordSize]byte{}) {
				row = protocol.EmptyOrdinaryItem(sourceSlot)
			}
			updates = append(updates, row)
		}
		rewardAfter, ok := bag.RowAt(rewardSlot)
		if !ok {
			return nil, nil, fmt.Errorf("awarded slot missing after inventory update")
		}
		updates = append(updates, rewardAfter)
		receipt, err := json.Marshal(lotteryItemReceipt{RewardTemplate: reward.Template, RewardSlot: rewardSlot, GrantCount: reward.Count, Updates: updates})
		return nextState, receipt, err
	})
	if err != nil {
		return nil, err
	}
	data, err := store.CharacterEventReceipt(ctx, w.role.AccountID, w.role.ID, key)
	if err != nil {
		return nil, err
	}
	var receipt lotteryItemReceipt
	if err := json.Unmarshal(data, &receipt); err != nil {
		return nil, err
	}
	if receipt.RewardTemplate == 0 || receipt.GrantCount == 0 || len(receipt.Updates) == 0 {
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
