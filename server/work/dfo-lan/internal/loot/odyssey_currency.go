package loot

import (
	"dfolan/internal/catalog"
	"dfolan/internal/inventory"
	"encoding/json"
	"fmt"
	"os"
)

// Rates are the operator-approved permanent compatibility policy, not PVF rates.
type OdysseyCurrency struct {
	Source      string                      `json:"source"`
	Model       string                      `json:"model"`
	Denominator uint32                      `json:"denominator"`
	Rates       [4]uint32                   `json:"rates_by_rank"`
	Templates   [4]uint32                   `json:"templates_by_rank"`
	Items       map[uint32]catalog.LootItem `json:"items"`
}

func LoadOdysseyCurrency(path string) (*OdysseyCurrency, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var r OdysseyCurrency
	if e = json.Unmarshal(b, &r); e != nil {
		return nil, e
	}
	return NewOdysseyCurrency(r)
}

// NewOdysseyCurrency shares validation and runtime index construction across native and JSON sources.
func NewOdysseyCurrency(r OdysseyCurrency) (*OdysseyCurrency, error) {
	if r.Source != catalog.OdysseySource || r.Model != "operator-odyssey-coins-v1" || r.Denominator != 10000 || len(r.Items) != 2 {
		return nil, fmt.Errorf("invalid Odyssey currency policy")
	}
	for rank, rate := range r.Rates {
		id := r.Templates[rank]
		item := r.Items[id]
		if rate > r.Denominator || (id != 10418036 && id != 10418035) || item.ID != id || item.Kind != "stackable" || item.StackableType != "[unlimited waste]" || len(item.Script.SHA256) != 64 {
			return nil, fmt.Errorf("invalid Odyssey currency row")
		}
	}
	return &r, nil
}

func (r *OdysseyCurrency) Roll(seed uint32, rank byte) ([]Award, uint32, error) {
	if r == nil || rank > 3 || r.Denominator == 0 {
		return nil, seed, fmt.Errorf("invalid Odyssey currency rank")
	}
	rng := RNG{seed}
	roll := rng.Next(r.Denominator)
	if roll < r.Rates[rank] {
		return []Award{{r.Templates[rank], 1}}, rng.Seed, nil
	}
	return nil, rng.Seed, nil
}

// Storage/pickup overlays never enter the ordinary random item candidate pool.
func (r *OdysseyCurrency) StorageCatalog(c catalog.LootCatalog) catalog.LootCatalog {
	items := make(map[uint32]catalog.LootItem, len(c.Items)+len(r.Items))
	for id, v := range c.Items {
		items[id] = v
	}
	for id, v := range r.Items {
		items[id] = v
	}
	c.Items = items
	return c
}

func (r *OdysseyCurrency) BagRules(b inventory.BagRules) inventory.BagRules {
	slots := make(map[string][2]uint16, len(b.Slots)+1)
	for k, v := range b.Slots {
		slots[k] = v
	}
	slots["[unlimited waste]"] = [2]uint16{65, 120}
	b.Slots = slots
	return b
}
