package loot

import (
	"context"
	"dfolan/internal/cashshop"
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"os"
	"strconv"
	"time"
)

// BoxReward is one prize template as imported from the source .cos table.
// Slots is the bag band its item family uses; a nil band means the family has
// no verified placement, and the prize is refused instead of guessed.
type BoxReward struct {
	Path          string     `json:"path"`
	StackableType string     `json:"stackable_type"`
	StackLimit    uint32     `json:"stack_limit"`
	Slots         *[2]uint16 `json:"slots"`
}

// BoxEntry is one source lot row: tier, prize template, count and weight.
type BoxEntry struct {
	Group    int32  `json:"group"`
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
	Weight   uint32 `json:"weight"`
}

// BoxRoll 保留一次开启中不同 UI 区域的奖励来源。Main 始终只有一项；
// Bonus 来自满 10 次的额外抽取；Section 来自 25/50/75/100 里程节点。
type BoxRoll struct {
	Main    ConsumeGrant
	Bonus   []ConsumeGrant
	Section []ConsumeGrant
}

// BoxSectionReward is one [section reward] row: the point threshold that hands
// out the template, and how many copies it gives.
type BoxSectionReward struct {
	Group     int32  `json:"group"`
	Threshold uint32 `json:"threshold"`
	Template  uint32 `json:"template"`
	Count     uint32 `json:"count"`
}

// BoxPointStack is one [point stack] block. The counter grows by Gain on every
// open: a bonus stack draws RewardParam's lot group once it reaches Max, and a
// section stack grants every SectionReward threshold the open crosses, resetting
// at Max.
type BoxPointStack struct {
	Type          string             `json:"type"`
	Gain          uint32             `json:"gain"`
	Max           uint32             `json:"max"`
	RewardType    string             `json:"reward_type"`
	RewardParam   int32              `json:"reward_param"`
	SectionReward []BoxSectionReward `json:"section_reward"`
}

// BoxTable is one imported .cos content script.
type BoxTable struct {
	Table         string                `json:"table"`
	Rate          uint32                `json:"rate"`
	MaterialCount uint32                `json:"material_count"`
	MainGroup     int32                 `json:"main_group"`
	SpecialGroup  int32                 `json:"special_group"`
	Groups        map[string][]BoxEntry `json:"groups"`
	PointStacks   []BoxPointStack       `json:"point_stacks"`
	// ChangeStack is the accumulated open count at which the box opens in its
	// special form: that open draws SpecialGroup instead of MainGroup, which is
	// what the client's normal-to-special open animation shows. Zero means the
	// source declares no such change.
	ChangeStack uint32 `json:"change_stack"`
}

// BoxCatalog maps a box template to its source table and to every prize
// template those tables can hand out.
type BoxCatalog struct {
	Source  string               `json:"source"`
	Tables  map[string]BoxTable  `json:"tables"`
	Rewards map[string]BoxReward `json:"rewards"`
}

// boxPointsKey is the character-state key holding the per-box pity counters.
// They live in the character state so they commit inside the same transaction
// that spends the box, and no separate schema is needed for them.
const boxPointsKey = "box_points"

// LoadBoxes reads the imported open-box tables. A table naming a lot group the
// file does not carry, or a point stack this build cannot honour, is rejected
// rather than drawn from nothing.
func LoadBoxes(path string) (*BoxCatalog, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var c BoxCatalog
	if e = json.Unmarshal(b, &c); e != nil {
		return nil, e
	}
	if len(c.Tables) == 0 {
		return nil, fmt.Errorf("box catalog has no tables")
	}
	for id, t := range c.Tables {
		if t.Rate == 0 || len(t.Groups) == 0 {
			return nil, fmt.Errorf("box %s has no rate or lot groups", id)
		}
		if _, ok := t.Groups[strconv.Itoa(int(t.MainGroup))]; !ok {
			return nil, fmt.Errorf("box %s main lot group %d is absent", id, t.MainGroup)
		}
		if t.ChangeStack > 0 {
			if _, ok := t.Groups[strconv.Itoa(int(t.SpecialGroup))]; !ok {
				return nil, fmt.Errorf("box %s special lot group %d is absent", id, t.SpecialGroup)
			}
		}
		for _, p := range t.PointStacks {
			switch p.Type {
			case "bonus":
				if p.RewardType != "draw" {
					return nil, fmt.Errorf("box %s bonus reward %q is not implemented", id, p.RewardType)
				}
				if _, ok := t.Groups[strconv.Itoa(int(p.RewardParam))]; !ok {
					return nil, fmt.Errorf("box %s bonus draw group %d is absent", id, p.RewardParam)
				}
			case "section":
			default:
				return nil, fmt.Errorf("box %s point stack %q is not implemented", id, p.Type)
			}
		}
	}
	return &c, nil
}

func (c *BoxCatalog) TableCount() int {
	if c == nil {
		return 0
	}
	return len(c.Tables)
}

func (c *BoxCatalog) RewardCount() int {
	if c == nil {
		return 0
	}
	return len(c.Rewards)
}

// Table reports the source table for one box template.
func (c *BoxCatalog) Table(template uint32) (BoxTable, bool) {
	if c == nil {
		return BoxTable{}, false
	}
	t, ok := c.Tables[strconv.FormatUint(uint64(template), 10)]
	return t, ok
}

func consumeGrant(template, count uint32, group int32) ConsumeGrant {
	if count == 0 {
		count = 1
	}
	return ConsumeGrant{Template: template, Count: count, Group: group}
}

// drawRows picks one row by the source weights.
func drawRows(rng *rand.Rand, rows []BoxEntry) (BoxEntry, error) {
	var total uint32
	for _, r := range rows {
		total += r.Weight
	}
	if total == 0 {
		return BoxEntry{}, fmt.Errorf("lot group carries no weight")
	}
	pick := rng.Uint32() % total
	for _, r := range rows {
		if pick < r.Weight {
			return r, nil
		}
		pick -= r.Weight
	}
	return rows[len(rows)-1], nil
}

// Draw picks one prize from the table's declared main lot group.
func (t BoxTable) Draw(rng *rand.Rand) (BoxEntry, error) {
	return drawRows(rng, t.Groups[strconv.Itoa(int(t.MainGroup))])
}

// Open performs one box use: the main lot group — or the special lot group on
// the open that completes [change box stack] — then every [point stack] this
// open completes. Counters are passed in and returned so the caller persists
// them in the same transaction that spent the box, which keeps a replayed
// request from advancing pity twice.
func (t BoxTable) Open(rng *rand.Rand, counters map[string]uint32) (BoxRoll, map[string]uint32, error) {
	next := make(map[string]uint32, len(counters)+len(t.PointStacks)+1)
	for k, v := range counters {
		next[k] = v
	}
	// "change" is the reserved key for the [change box stack] counter, kept
	// beside the [point stack] counters and persisted with them.
	var entry BoxEntry
	var e error
	if t.ChangeStack > 0 && next["change"]+1 >= t.ChangeStack {
		next["change"] = 0
		special := t.Groups[strconv.Itoa(int(t.SpecialGroup))]
		if len(special) == 0 {
			return BoxRoll{}, nil, fmt.Errorf("box special lot group %d is absent", t.SpecialGroup)
		}
		entry, e = drawRows(rng, special)
	} else {
		if t.ChangeStack > 0 {
			next["change"]++
		}
		entry, e = t.Draw(rng)
	}
	if e != nil {
		return BoxRoll{}, nil, e
	}
	roll := BoxRoll{Main: consumeGrant(entry.Template, entry.Count, entry.Group)}
	for i, stack := range t.PointStacks {
		key := strconv.Itoa(i)
		gain := stack.Gain
		if gain == 0 {
			gain = 1
		}
		before, after := next[key], next[key]+gain
		switch stack.Type {
		case "bonus":
			if stack.Max > 0 && after >= stack.Max {
				next[key] = 0
				bonus, e := drawRows(rng, t.Groups[strconv.Itoa(int(stack.RewardParam))])
				if e != nil {
					return BoxRoll{}, nil, e
				}
				roll.Bonus = append(roll.Bonus, consumeGrant(bonus.Template, bonus.Count, bonus.Group))
			} else {
				next[key] = after
			}
		case "section":
			for _, r := range stack.SectionReward {
				if before < r.Threshold && after >= r.Threshold {
					roll.Section = append(roll.Section, consumeGrant(r.Template, r.Count, r.Group))
				}
			}
			if stack.Max > 0 && after >= stack.Max {
				next[key] = 0
			} else {
				next[key] = after
			}
		}
	}
	return roll, next, nil
}

// readBoxPoints loads one box's pity counters from the character state.
func readBoxPoints(state json.RawMessage, box uint32) (map[string]uint32, error) {
	out := map[string]uint32{}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	raw, ok := fields[boxPointsKey]
	if !ok {
		return out, nil
	}
	var all map[string]map[string]uint32
	if e := json.Unmarshal(raw, &all); e != nil {
		return nil, e
	}
	for k, v := range all[strconv.FormatUint(uint64(box), 10)] {
		out[k] = v
	}
	return out, nil
}

// saveBoxPoints writes one box's counters back without disturbing any other
// character-state key.
func saveBoxPoints(state json.RawMessage, box uint32, points map[string]uint32) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	all := map[string]map[string]uint32{}
	if raw, ok := fields[boxPointsKey]; ok {
		if e := json.Unmarshal(raw, &all); e != nil {
			return nil, e
		}
	}
	all[strconv.FormatUint(uint64(box), 10)] = points
	v, e := json.Marshal(all)
	if e != nil {
		return nil, e
	}
	fields[boxPointsKey] = v
	return json.Marshal(fields)
}

// grantBoxPrize places one prize into the caller's bag inside the same
// character transaction that spent the box. Placement comes from the imported
// family band, never from a guess: an unknown family refuses the whole open
// rather than dropping the prize somewhere the client does not expect.
func (s *Service) grantBoxPrize(bag inventory.Bag, prize ConsumeGrant) (inventory.Bag, *storage.CashPremiumActivation, error) {
	reward, ok := s.Boxes.Rewards[strconv.FormatUint(uint64(prize.Template), 10)]
	if !ok {
		return bag, nil, fmt.Errorf("box prize %d has no imported item family", prize.Template)
	}
	if reward.Slots == nil {
		return bag, nil, fmt.Errorf("box prize %d family %s has no bag band", prize.Template, reward.StackableType)
	}
	amount := prize.Count
	if amount == 0 {
		amount = 1
	}
	// 契约直接累计账号时长，不临时占用背包，也不与旧的过期堆叠合并。
	if contract, ok := cashshop.ResolveContractItem(prize.Template); ok {
		if contract.DurationSecond <= 0 || int64(amount) > math.MaxInt64/contract.DurationSecond {
			return bag, nil, fmt.Errorf("土罐契约时长溢出：%d", prize.Template)
		}
		return bag, &storage.CashPremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond * int64(amount)}, nil
	}
	item := catalog.LootCatalog{
		Source: s.Catalog.Source,
		Items: map[uint32]catalog.LootItem{
			prize.Template: {
				ID: prize.Template, Kind: "stackable",
				StackableType: reward.StackableType, StackLimit: reward.StackLimit,
			},
		},
	}
	rules := inventory.BagRules{
		Source:            s.Catalog.Source.Checksum,
		Slots:             map[string][2]uint16{reward.StackableType: *reward.Slots},
		MissingStackLimit: 1000,
	}
	// 与商城发货保持一致，显式填写可表示的远期时间，避免限时模板将零值判为过期。
	updated, _, e := bag.Add(item, rules, prize.Template, amount, cashshop.MaxExpireTime)
	return updated, nil, e
}

// settleBoxRewardBag 补齐旧奖励的零期限，并将源契约别名兑换为账号时长。
// 契约占位物品与效果必须由调用者在同一事务提交，不能只删物品再单独续期。
func (s *Service) settleBoxRewardBag(bag inventory.Bag) (inventory.Bag, []storage.CashPremiumActivation, error) {
	if s.Boxes == nil {
		return bag, nil, nil
	}
	var premiums []storage.CashPremiumActivation
	items := make([]inventory.BagItem, 0, len(bag.Items))
	for _, row := range bag.Items {
		if _, ok := s.Boxes.Rewards[strconv.FormatUint(uint64(row.Template), 10)]; !ok {
			items = append(items, row)
			continue
		}
		if contract, ok := cashshop.ResolveContractItem(row.Template); ok && row.Amount > 0 && !protocol.StoredItemExpired(row.ExpireTime, time.Now().Unix()) {
			if contract.DurationSecond <= 0 || int64(row.Amount) > math.MaxInt64/contract.DurationSecond {
				return bag, nil, fmt.Errorf("土罐契约时长溢出：%d", row.Template)
			}
			premiums = append(premiums, storage.CashPremiumActivation{Type: contract.Type, DurationSecond: contract.DurationSecond * int64(row.Amount)})
			continue
		}
		if row.ExpireTime == 0 {
			row.ExpireTime = cashshop.MaxExpireTime
		}
		items = append(items, row)
	}
	bag.Items = items
	return bag, premiums, nil
}

// RepairBoxRewards 在登录背包还原前修复遗留奖励，重复登录不重复续期。
func (s *Service) RepairBoxRewards(ctx context.Context, role storage.Character) (storage.Character, bool, error) {
	if s.Boxes == nil {
		return role, false, nil
	}
	bag, err := inventory.ReadBag(role.State)
	if err != nil {
		return role, false, err
	}
	needed := false
	for _, row := range bag.Items {
		if _, ok := s.Boxes.Rewards[strconv.FormatUint(uint64(row.Template), 10)]; ok {
			_, contract := cashshop.ResolveContractItem(row.Template)
			if row.ExpireTime == 0 || (contract && row.Amount > 0 && !protocol.StoredItemExpired(row.ExpireTime, time.Now().Unix())) {
				needed = true
				break
			}
		}
	}
	if !needed {
		return role, false, nil
	}
	saved, applied, err := s.Store.CommitCharacterPremiumEvent(ctx, role.AccountID, role.ID, s.Catalog.Source.Checksum, "box-reward-repair-v1", s.Rules.Model,
		func(current storage.Character) (json.RawMessage, json.RawMessage, []storage.CashPremiumActivation, error) {
			bag, err := inventory.ReadBag(current.State)
			if err != nil {
				return nil, nil, nil, err
			}
			before, err := json.Marshal(bag.Items)
			if err != nil {
				return nil, nil, nil, err
			}
			bag, premiums, err := s.settleBoxRewardBag(bag)
			if err != nil {
				return nil, nil, nil, err
			}
			state, err := inventory.SaveBag(current.State, bag)
			if err != nil {
				return nil, nil, nil, err
			}
			outcome, err := json.Marshal(map[string]any{"previous_items": json.RawMessage(before), "items": bag.Items})
			return state, outcome, premiums, err
		})
	saved.WireID = role.WireID
	return saved, applied, err
}
