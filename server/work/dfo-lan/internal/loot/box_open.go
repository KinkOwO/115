package loot

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
)

// BoxOpenReceipt records one settled radiant box open.
type BoxOpenReceipt struct {
	Premiums   []storage.CashPremium `json:"premiums,omitempty"`
	Box        uint32                `json:"box"`
	Opened     uint32                `json:"opened"`
	Granted    []ConsumeGrant        `json:"granted"`
	Results    []ConsumeGrant        `json:"results"`
	Bonus      []ConsumeGrant        `json:"bonus,omitempty"`
	Milestones []ConsumeGrant        `json:"milestones,omitempty"`
	Points     map[string]uint32     `json:"points,omitempty"`
	Source     string                `json:"source"`
}

// boxOpensKey is the reserved counter inside box_points. It gives every open its
// own idempotency key without a second schema: a retried request repeats the
// current value and returns the original receipt, while the next deliberate open
// sees the incremented one.
const boxOpensKey = "opens"

// BoxOpenMaterial reports which owned stack holds the box material. The event
// request names no slot, so the bag decides what the player is opening.
func BoxOpenMaterial(bag inventory.Bag, box uint32) (uint16, uint32, bool) {
	for _, row := range bag.Items {
		if row.Template == box {
			return row.Slot, row.Amount, true
		}
	}
	return 0, 0, false
}

// NOTI2551 的两张表共用数值键，但语义不同。第一表的 0/1/2 是额外奖励、
// 变换和里程计数器；第二表中 key1 是 1/10 个主结果，key3 是单个额外奖励，
// key2/key4 分别描述里程奖励和已经取得的里程奖励。
const (
	boxBonusCounterKey   = 0
	boxChangeCounterKey  = 1
	boxSectionCounterKey = 2

	boxBonusStateKey    = 0
	boxMainResultKey    = 1
	boxSectionStateKey  = 2
	boxBonusResultKey   = 3
	boxEarnedSectionKey = 4
)

// BoxStateRows 生成额外奖励进度和里程奖励状态；主结果由独立 key1 行承载。
func (t BoxTable) BoxStateRows(points map[string]uint32) []BoxStateRow {
	var rows []BoxStateRow
	for i, stack := range t.PointStacks {
		current := points[strconv.Itoa(i)]
		switch stack.Type {
		case "bonus":
			total := stack.Max
			if total == 0 {
				total = 10
			}
			for slot := uint32(0); slot < total; slot++ {
				flag := uint32(0)
				if slot < current {
					flag = 1
				}
				rows = append(rows, BoxStateRow{Key: boxBonusStateKey, Flag: flag})
			}
		case "section":
			for _, reward := range stack.SectionReward {
				flag := uint32(0)
				if current >= reward.Threshold {
					flag = 1
				}
				rows = append(rows, BoxStateRow{
					Key: boxSectionStateKey, Flag: flag,
					Template: reward.Template, Count: reward.Count, Threshold: reward.Threshold,
				})
				if flag == 1 {
					rows = append(rows, BoxStateRow{
						Key: boxEarnedSectionKey, Flag: 1,
						Template: reward.Template, Count: reward.Count, Threshold: reward.Threshold,
					})
				}
			}
		}
	}
	return rows
}

// boxResultRows 保留每次抽取的独立行。客户端用 key1 行数区分单开与十连开。
func boxResultRows(key uint32, grants []ConsumeGrant) []BoxStateRow {
	rows := make([]BoxStateRow, 0, len(grants))
	for _, g := range grants {
		group := uint32(1)
		if g.Group > 0 {
			group = uint32(g.Group)
		}
		rows = append(rows, BoxStateRow{Key: key, Flag: group, Template: g.Template, Count: g.Count})
	}
	return rows
}

// BoxMainResultRows 生成中央“结果”区域读取的 key1 行。
func BoxMainResultRows(grants []ConsumeGrant) []BoxStateRow {
	return boxResultRows(boxMainResultKey, grants)
}

// BoxBonusResultRows 生成单个 BONUS REWARD 区域读取的 key3 行。
func BoxBonusResultRows(grants []ConsumeGrant) []BoxStateRow {
	return boxResultRows(boxBonusResultKey, grants)
}

// BoxStateRow is one row of the notice's second list: the stack it belongs to,
// its filled marker, and the slot fields the window draws from it.
type BoxStateRow struct {
	Key       uint32
	Flag      uint32
	Template  uint32
	Count     uint32
	Threshold uint32
}

// BoxNoticeEntry is one {key,value} pair of the notice's first list: a prize uses
// its template as the key, and the stacks use the reserved counter keys.
type BoxNoticeEntry struct {
	Key   uint32
	Value uint32
}

// BoxNoticeEntries folds the granted prizes into template->count entries and adds
// the stack counters the window reads back for its gauges. Key1 is the change
// counter the [change box stack] block counts, which the window pairs with the
// table's own limit.
func BoxNoticeEntries(t BoxTable, points map[string]uint32, grants []ConsumeGrant) []BoxNoticeEntry {
	entries := make([]BoxNoticeEntry, 0, len(grants)+3)
	for i, stack := range t.PointStacks {
		current := points[strconv.Itoa(i)]
		switch stack.Type {
		case "bonus":
			entries = append(entries, BoxNoticeEntry{Key: boxBonusCounterKey, Value: current})
		case "section":
			entries = append(entries, BoxNoticeEntry{Key: boxSectionCounterKey, Value: current})
		}
	}
	if t.ChangeStack > 0 {
		entries = append(entries, BoxNoticeEntry{Key: boxChangeCounterKey, Value: points["change"]})
	}
	sum := map[uint32]uint32{}
	for _, g := range grants {
		sum[g.Template] += g.Count
	}
	templates := make([]int, 0, len(sum))
	for t := range sum {
		templates = append(templates, int(t))
	}
	sort.Ints(templates)
	for _, t := range templates {
		entries = append(entries, BoxNoticeEntry{Key: uint32(t), Value: sum[uint32(t)]})
	}
	return entries
}

// OpenBoxes settles one radiant box request: it spends the material and hands out
// every rolled prize inside the same character transaction, so a retried request
// cannot spend a second stack or advance pity twice.
func (s *Service) OpenBoxes(ctx context.Context, role storage.Character, box, count uint32) (storage.Character, BoxOpenReceipt, bool, error) {
	var out BoxOpenReceipt
	fail := func(e error) (storage.Character, BoxOpenReceipt, bool, error) {
		return role, out, false, e
	}
	if role.ConfigVersion != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("box open source mismatch"))
	}
	table, known := s.Boxes.Table(box)
	if !known {
		return fail(fmt.Errorf("box %d has no imported content table", box))
	}
	if count == 0 || count > 100 {
		return fail(fmt.Errorf("box open count %d is out of range", count))
	}
	opens, e := readBoxCounter(role.State, box, boxOpensKey)
	if e != nil {
		return fail(e)
	}
	key := fmt.Sprintf("boxopen:%d:%d:%d", box, count, opens)
	saved, applied, e := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID,
		s.Catalog.Source.Checksum, key, s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, nil, e
			}
			slot, held, ok := BoxOpenMaterial(bag, box)
			if !ok {
				return nil, nil, nil, fmt.Errorf("no owned %d to open", box)
			}
			if held < count {
				return nil, nil, nil, fmt.Errorf("only %d of %d owned", held, box)
			}
			material := catalog.LootCatalog{
				Source: s.Catalog.Source,
				Items: map[uint32]catalog.LootItem{
					box: {ID: box, Kind: "stackable", StackLimit: 1000},
				},
			}
			for i := uint32(0); i < count; i++ {
				if bag, _, e = bag.Consume(material, slot, box); e != nil {
					return nil, nil, nil, e
				}
			}
			counters, e := readBoxPoints(current.State, box)
			if e != nil {
				return nil, nil, nil, e
			}
			seed, e := boxSeed()
			if e != nil {
				return nil, nil, nil, e
			}
			rng := rand.New(rand.NewSource(seed))
			var granted []ConsumeGrant
			var results []ConsumeGrant
			var bonus []ConsumeGrant
			var milestones []ConsumeGrant
			var grantedPremiums []storage.CashPremiumActivation
			for i := uint32(0); i < count; i++ {
				var roll BoxRoll
				roll, counters, e = table.Open(rng, counters)
				if e != nil {
					return nil, nil, nil, e
				}
				prizes := []ConsumeGrant{roll.Main}
				prizes = append(prizes, roll.Bonus...)
				prizes = append(prizes, roll.Section...)
				for _, prize := range prizes {
					var premium *storage.CashPremiumActivation
					if bag, premium, e = s.grantBoxPrize(bag, prize); e != nil {
						return nil, nil, nil, e
					}
					if premium != nil {
						grantedPremiums = append(grantedPremiums, *premium)
					}
				}
				granted = append(granted, prizes...)
				results = append(results, roll.Main)
				bonus = append(bonus, roll.Bonus...)
				milestones = append(milestones, roll.Section...)
			}
			bag, premiums, e := s.settleBoxRewardBag(bag)
			if e != nil {
				return nil, nil, nil, e
			}
			premiums = append(premiums, grantedPremiums...)
			updated, e := inventory.SaveBag(current.State, bag)
			if e != nil {
				return nil, nil, nil, e
			}
			counters[boxOpensKey] = opens + 1
			if updated, e = saveBoxPoints(updated, box, counters); e != nil {
				return nil, nil, nil, e
			}
			delete(counters, boxOpensKey)
			out = BoxOpenReceipt{Box: box, Opened: count, Granted: granted,
				Results: results, Bonus: bonus, Milestones: milestones,
				Points: counters, Source: s.Catalog.Source.Checksum}
			receipt, e := json.Marshal(out)
			return updated, receipt, premiums, e
		})
	if e != nil {
		return fail(e)
	}
	receipt, e := s.Store.CharacterEventReceipt(ctx, role.AccountID, role.ID, key)
	if e != nil {
		return fail(e)
	}
	if e = json.Unmarshal(receipt, &out); e != nil {
		return fail(e)
	}
	if out.Box != box || out.Source != s.Catalog.Source.Checksum {
		return fail(fmt.Errorf("box open receipt conflict"))
	}
	saved.WireID = role.WireID
	return saved, out, applied, nil
}

// readBoxCounter reads one reserved counter from a box's point state.
func readBoxCounter(state json.RawMessage, box uint32, name string) (uint32, error) {
	points, e := readBoxPoints(state, box)
	if e != nil {
		return 0, e
	}
	return points[name], nil
}
