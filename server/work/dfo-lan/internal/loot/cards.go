package loot

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

type CardRules struct {
	Model           string    `json:"model"`
	Reference       string    `json:"reference_sha256"`
	GoldNumerator   uint32    `json:"gold_numerator"`
	GoldDenominator uint32    `json:"gold_denominator"`
	Difficulty      []float64 `json:"difficulty_bonus"`
}

func LoadCardRules(path string) (CardRules, error) {
	var r CardRules
	b, e := os.ReadFile(path)
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, e
	}
	if r.Model != "reference90-free-gold-v1" || len(r.Reference) != 64 || r.GoldNumerator == 0 || r.GoldDenominator == 0 || len(r.Difficulty) != 5 {
		return r, fmt.Errorf("invalid card policy")
	}
	for _, n := range r.Difficulty {
		if n <= 0 || math.IsNaN(n) || math.IsInf(n, 0) {
			return r, fmt.Errorf("invalid card multiplier")
		}
	}
	return r, nil
}

type CardPlan struct {
	Run, Source, Model string
	Gold               uint32
	Level              byte
	// 固定长度保持旧回执可比较；旧存档缺省为空，不改变普通翻牌。
	Items [8]Award `json:",omitempty"`
	// 独立领主奖励与翻牌同事务冻结，但分别领取，不能在翻牌时再次发放。
	BossItems [3]Award `json:",omitempty"`
	BossModel string   `json:",omitempty"`
}
type CardReceipt struct {
	Plan  CardPlan
	Index byte
}

func CardGold(t Tables, r CardRules, seed uint32, level, difficulty byte) (uint32, error) {
	if r.GoldDenominator == 0 || int(difficulty) >= len(r.Difficulty) {
		return 0, fmt.Errorf("invalid card formula")
	}
	for i := 0; i+2 < len(t.Gold); i += 3 {
		if t.Gold[i] != float64(level) {
			continue
		}
		base, pct := t.Gold[i+1], t.Gold[i+2]
		if base <= 0 || pct < 0 || pct > 100 || math.Trunc(base) != base || math.Trunc(pct) != pct {
			return 0, fmt.Errorf("invalid source card gold row")
		}
		n := math.Floor(base * float64(r.GoldNumerator) / float64(r.GoldDenominator))
		if n < 1 {
			n = 1
		}
		n = math.Floor(n * r.Difficulty[difficulty])
		if n < 1 {
			n = 1
		}
		if n > math.MaxUint32 {
			return 0, fmt.Errorf("card gold overflow")
		}
		amount := int64(n)
		rng := RNG{seed}
		if pct > 0 {
			amount += (int64(rng.Next(uint32(pct*2+1))) - int64(pct)) * amount / 100
		}
		if amount < 1 {
			amount = 1
		}
		if amount > math.MaxUint32 {
			return 0, fmt.Errorf("card gold overflow")
		}
		return uint32(amount), nil
	}
	return 0, fmt.Errorf("source card gold level absent")
}

// Freeze is a durable plan, not an award. No reward is granted until PickCard.
func (s *Service) PlanCards(role Role, d *dungeon.Session, r CardRules, seed uint32) (CardPlan, error) {
	var p CardPlan
	if d == nil || !d.Completed() || role.ConfigVersion != s.Catalog.Source.SaveIdentity() {
		return p, fmt.Errorf("card plan before owned completion")
	}
	level := byte(0)
	for _, m := range d.Monsters {
		if !m.NonCombat && m.Level > level {
			level = m.Level
		}
	}
	if level == 0 {
		for _, monsters := range d.Visited {
			for _, m := range monsters {
				if !m.NonCombat && m.Level > level {
					level = m.Level
				}
			}
		}
	}
	if level == 0 && d.Definition.BasisLevel > 0 {
		level = byte(d.Definition.BasisLevel)
	}
	if level == 0 {
		level = 1
	}
	gold, e := CardGold(s.Tables, r, seed, level, 0)
	if e != nil {
		return p, e
	}
	p = CardPlan{Run: d.RunID, Source: s.Catalog.Source.SaveIdentity(), Model: r.Model, Gold: gold, Level: level}
	return p, nil
}
func (s *Service) PrepareFrozenCard(current Role, p CardPlan, index byte) (json.RawMessage, json.RawMessage, error) {
	var receipt CardReceipt
	bag, e := inventory.ReadBag(current.State)
	if e != nil {
		return nil, nil, e
	}
	if p.Gold > 0 {
		bag, _, e = bag.Add(s.Catalog, s.BagRules, 0, p.Gold)
		if e != nil {
			return nil, nil, e
		}
	}
	for _, item := range p.Items {
		if item == (Award{}) {
			continue
		}
		if item.Template == 0 || item.Amount == 0 {
			return nil, nil, fmt.Errorf("冻结翻牌物品无效")
		}
		bag, _, e = bag.Add(s.Catalog, s.BagRules, item.Template, item.Amount, inventory.GrantExpireTime)
		if e != nil {
			return nil, nil, e
		}
	}
	state, e := inventory.SaveBag(current.State, bag)
	if e != nil {
		return nil, nil, e
	}
	receipt = CardReceipt{p, index}
	data, e := json.Marshal(receipt)
	return state, data, e
}
