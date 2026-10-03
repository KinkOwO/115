package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"dfolan/internal/workflow"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"math/big"
	"os"
	"strings"
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

// lotterySourcePVFSHA256 是抽奖目录的来源身份令牌。
//
// 2026-10-01（next146）：直读模式下这些目录由当次内层 PVF 直接派生，
// SourcePVFSHA256 自然等于当次内层 checksum，而不再是编译期写死的
// "7ef2db59…"（那是旧 client-build/Script.inner.pvf 的哈希）。它**不是**
// 运行时不变量（不像 WearRules.Source / OdysseySource 会被写进存档），
// 因此策略与其它来源身份门禁一致：留空 = 接受并派生为当次值；非空且
// 不一致 = 仍硬拒绝（保留手工钉版本的意图）。
var lotterySourcePVFSHA256 = "7ef2db59331f7e5b18b2f250b8b907526bf2c94b17a7312036cf599644d88e80"

// SetLotterySource 由直读目录准备阶段调用，把来源身份令牌切到当次内层 checksum。
// 只接受 64 位十六进制，否则忽略（避免把垃圾值灌进来源令牌）。
func SetLotterySource(checksum string) {
	if len(checksum) != 64 {
		return
	}
	for i := 0; i < len(checksum); i++ {
		c := checksum[i]
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return
		}
	}
	lotterySourcePVFSHA256 = checksum
}

// lotterySourceAccepts 判定一份目录的来源身份是否可接受。
// 留空或与当前令牌一致 = 接受；其它一概拒绝。
func lotterySourceAccepts(source string) bool {
	return source == "" || source == lotterySourcePVFSHA256
}

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
	return newLotteryItemCatalog(c, index)
}

func newLotteryItemCatalog(c lotteryItemCatalog, index map[uint32]ItemIndexInfo) (*lotteryItemCatalog, error) {
	return buildLotteryItemCatalog(c, index, true)
}

func buildLotteryItemCatalog(c lotteryItemCatalog, index map[uint32]ItemIndexInfo, historicalBaseline bool) (*lotteryItemCatalog, error) {
	// PVF catalogs keep their actual source hash; pool counts and particular
	// historical content vectors only apply to the old JSON baseline loader.
	if !lotterySourceAccepts(c.SourcePVFSHA256) {
		return nil, fmt.Errorf("lottery catalog source identity or pool count mismatch")
	}
	if historicalBaseline && c.SourcePVFSHA256 != "" && len(c.Pools) != 276 {
		return nil, fmt.Errorf("lottery catalog source identity or pool count mismatch")
	}
	if c.SourcePVFSHA256 == "" {
		c.SourcePVFSHA256 = lotterySourcePVFSHA256
	}
	c.byTemplate = make(map[uint32]*lotteryItemPool, len(c.Pools))
	original := c.Pools
	c.Pools = make([]*lotteryItemPool, len(original))
	for i, source := range original {
		if source == nil {
			return nil, fmt.Errorf("nil lottery pool")
		}
		p := new(lotteryItemPool)
		*p = *source
		p.Candidates = append([]BoosterRewardCandidate(nil), source.Candidates...)
		p.total = 0
		c.Pools[i] = p
		hash, hashErr := hex.DecodeString(p.SourceScriptSHA256)
		if p.SourceItem == 0 || hashErr != nil || len(hash) != 32 || len(p.Candidates) == 0 {
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
	if p := c.byTemplate[7772]; historicalBaseline && (p == nil || p.SourceScriptSHA256 != "b6f59a8a3193ae4f6c0796e156f90d48317cbf2e16af2c309bacbdc63322e54c" || len(p.Candidates) != 209 || p.total != 98904) {
		return nil, fmt.Errorf("lottery item 7772 regression")
	}
	if p := c.byTemplate[10306598]; historicalBaseline && (p == nil || p.SourceScriptSHA256 != "a97094b202704c3cfb0f1e1e53a86bc9986813cddf071acc078c98b937414063" || len(p.Candidates) != 1 || p.Candidates[0] != (BoosterRewardCandidate{Template: 0, Weight: 10000, Count: 1000000})) {
		return nil, fmt.Errorf("lottery gold pot 10306598 regression")
	}
	return &c, nil
}

// Equipment-containing pools use compact numeric triples to keep the checked
// PVF export reviewable. Import the whole pool or none of it: removing a row
// would change the source lottery odds.
func loadLotteryEquipmentPools(path string, index map[uint32]ItemIndexInfo, catalog *lotteryItemCatalog) (int, error) {
	if catalog == nil || !lotterySourceAccepts(catalog.SourcePVFSHA256) {
		return 0, fmt.Errorf("lottery base catalog unavailable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var source lotteryEquipmentSource
	if err := json.Unmarshal(data, &source); err != nil {
		return 0, err
	}
	return applyLotteryEquipmentPools(source, index, catalog)
}

type lotteryEquipmentSource = catalog.LotteryPoolCatalog

func applyLotteryEquipmentPools(source lotteryEquipmentSource, index map[uint32]ItemIndexInfo, catalog *lotteryItemCatalog) (int, error) {
	return bindLotteryEquipmentPools(source, index, catalog, true)
}

func bindLotteryEquipmentPools(source lotteryEquipmentSource, index map[uint32]ItemIndexInfo, catalog *lotteryItemCatalog, historicalBaseline bool) (int, error) {
	if catalog == nil || !lotterySourceAccepts(catalog.SourcePVFSHA256) {
		return 0, fmt.Errorf("lottery base catalog unavailable")
	}
	if !lotterySourceAccepts(source.SourcePVFSHA256) {
		return 0, fmt.Errorf("equipment lottery source identity or pool count mismatch")
	}
	if historicalBaseline && source.SourcePVFSHA256 != "" && len(source.Pools) != 2477 {
		return 0, fmt.Errorf("equipment lottery source identity or pool count mismatch")
	}
	additions := make(map[uint32]*lotteryItemPool, len(source.Pools))
	for _, row := range source.Pools {
		meta, ok := index[row.SourceItem]
		hash, hashErr := hex.DecodeString(row.SourceScriptSHA256)
		if !ok || row.SourceItem == 0 || meta.Path != row.SourceScript || meta.Kind != "stackable" || meta.StackableType != "[upgradable legacy]" || hashErr != nil || len(hash) != 32 || len(row.Candidates) == 0 || catalog.byTemplate[row.SourceItem] != nil || additions[row.SourceItem] != nil {
			return 0, fmt.Errorf("equipment lottery source %d invalid", row.SourceItem)
		}
		p := &lotteryItemPool{SourceItem: row.SourceItem, SourceScript: row.SourceScript, SourceScriptSHA256: row.SourceScriptSHA256}
		hasEquipment := false
		for _, triple := range row.Candidates {
			template, weight, count := triple[0], triple[1], triple[2]
			if weight == 0 || count == 0 || p.total > int64(^uint64(0)>>1)-int64(weight) {
				return 0, fmt.Errorf("equipment lottery source %d has invalid triple", row.SourceItem)
			}
			if template != 0 {
				item, ok := index[template]
				if !ok || (item.Kind != "stackable" && item.Kind != "equipment" && item.Kind != "avatar") || (item.Kind != "stackable" && count != 1) || (item.Kind == "equipment" && strings.Contains(item.Path, "equipment/creature/")) {
					return 0, fmt.Errorf("equipment lottery source %d reward %d unsupported", row.SourceItem, template)
				}
				if item.Kind == "equipment" || item.Kind == "avatar" {
					hasEquipment = true
				}
			}
			p.Candidates = append(p.Candidates, BoosterRewardCandidate{Template: template, Weight: weight, Count: count})
			p.total += int64(weight)
		}
		if !hasEquipment {
			return 0, fmt.Errorf("equipment lottery source %d has no equipment", row.SourceItem)
		}
		additions[row.SourceItem] = p
	}
	if p := additions[7213]; historicalBaseline && (p == nil || p.SourceScriptSHA256 != "995df495602d0ede8df426e8ae3f3b200ef0a740ee9e2411a8794b4c24dd01ee" || len(p.Candidates) != 78) {
		return 0, fmt.Errorf("Pokin armor pot 7213 regression")
	}
	for id, p := range additions {
		catalog.byTemplate[id] = p
	}
	return len(additions), nil
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
	ResultRow      *[protocol.CurrentItemRecordSize]byte  `json:"result_row,omitempty"`
	HasExtra       bool                                   `json:"has_extra,omitempty"`
	SpecialRefresh []byte                                 `json:"special_refresh,omitempty"`
}

func (w *worldSession) openLotteryItem(ctx context.Context, store lotteryItemStore, pools *lotteryItemCatalog, index map[uint32]ItemIndexInfo, request, raw []byte, wear ...*workflow.WearService) ([]outboundPacket, error) {
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
				return nil, nil, fmt.Errorf("lottery item %d has no verified pool", source.Template)
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
			kind := "gold"
			if reward.Template != 0 {
				def, ok := index[reward.Template]
				if !ok {
					return nil, nil, fmt.Errorf("lottery reward metadata absent")
				}
				kind = def.Kind
				if kind == "stackable" {
					awardCatalog.Items[reward.Template] = catalog.LootItem{ID: reward.Template, Kind: kind, StackableType: def.StackableType, StackLimit: def.StackLimit}
				} else if kind != "equipment" && kind != "avatar" {
					return nil, nil, fmt.Errorf("lottery reward %d has unsupported kind %s", reward.Template, kind)
				}
			}
			if kind == "equipment" || kind == "avatar" {
				if len(wear) == 0 || wear[0] == nil || wear[0].Catalog == nil || reward.Count != 1 {
					return nil, nil, fmt.Errorf("equipment lottery grant unavailable")
				}
			}
			before := bag
			bag, _, err = bag.Consume(awardCatalog, sourceSlot, pool.SourceItem)
			if err != nil {
				return nil, nil, err
			}
			var rewardSlot uint16
			var resultRow [protocol.CurrentItemRecordSize]byte
			var specialRefresh []byte
			hasExtra := false
			switch kind {
			case "equipment":
				var slots []uint16
				bag, slots, err = bag.AddEquipment(wear[0].Catalog, wear[0].BagRules.EquipmentSlots, reward.Template, 1)
				if err != nil {
					return nil, nil, err
				}
				rewardSlot = slots[0]
				resultRow, _ = bag.RowAt(rewardSlot)
				var typ int32
				typ, err = wear[0].Catalog.RewardType(reward.Template)
				if err != nil {
					return nil, nil, err
				}
				hasExtra = typ <= 11
			case "avatar":
				// Every avatar in the pinned export has numeric PVF type 0.
				// The equipment catalog indexes ordinary .equ files, not avatars.
				hasExtra = true
				used := make(map[uint16]bool)
				for _, item := range bag.Special[1] {
					used[item.Slot] = true
				}
				found := false
				for slot := uint16(0); slot < protocol.AvatarInventorySlots(bag.AvatarExpansion); slot++ {
					if !used[slot] {
						rewardSlot = slot
						found = true
						break
					}
				}
				if !found {
					return nil, nil, fmt.Errorf("avatar bag is full")
				}
				if bag.Special == nil {
					bag.Special = make(map[byte][]inventory.BagEquipment)
				}
				item := inventory.BagEquipment{Slot: rewardSlot, Template: reward.Template}
				bag.Special[1] = append(bag.Special[1], item)
				resultRow = inventory.EquipmentRow(item)
				specialRefresh, err = inventory.EquipmentPayload(1, bag.Special[1], false)
				if err != nil {
					return nil, nil, err
				}
			default:
				bag, rewardSlot, err = bag.Add(awardCatalog, w.loot.BagRules, reward.Template, reward.Count)
				if err != nil {
					return nil, nil, err
				}
				resultRow = protocol.OrdinaryItem(rewardSlot, reward.Template, reward.Count)
			}
			nextState, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, err
			}
			updates := inventory.ChangedItemRows(before, bag)
			if len(updates) == 0 {
				return nil, nil, fmt.Errorf("lottery inventory produced no change")
			}
			receipt, err := json.Marshal(lotteryItemReceipt{SourceTemplate: pool.SourceItem, RewardTemplate: reward.Template, RewardSlot: rewardSlot, GrantCount: reward.Count, Updates: updates, ResultRow: &resultRow, HasExtra: hasExtra, SpecialRefresh: specialRefresh})
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
	resultRow := protocol.OrdinaryItem(receipt.RewardSlot, receipt.RewardTemplate, receipt.GrantCount)
	if receipt.ResultRow != nil {
		resultRow = *receipt.ResultRow
	}
	ack := protocol.LotteryItemSuccess(sourceSlot, resultRow)
	if receipt.HasExtra {
		ack = append(ack, 0, 0, 0, 0)
	}
	refresh, err := protocol.InventoryUpdate(receipt.Updates)
	if err != nil {
		return nil, err
	}
	packets := []outboundPacket{{"lottery_item_open", 1, 27, ack}, {"lottery_item_inventory", 0, 14, refresh}}
	if len(receipt.SpecialRefresh) != 0 {
		packets = append(packets, outboundPacket{"lottery_item_avatar", 0, 14, receipt.SpecialRefresh})
	}
	return packets, nil
}
