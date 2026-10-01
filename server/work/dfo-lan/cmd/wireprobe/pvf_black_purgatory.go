package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/gamedata"
	"dfolan/internal/loot"
	"encoding/json"
	"fmt"
	"log"
	"os"
)

// The legacy client hash is the exporter provenance, not a character save
// version. Normalize only this historical metadata field, after exact inner
// identity and all three native script hashes have been checked. No archive
// checksum alias is introduced for runtime or player saves.
func auditBlackPurgatory(legacy, direct loot.BlackPurgatoryRewards) error {
	if legacy.Source != direct.Source || direct.Source != catalog.OdysseySource || legacy.ScriptHash != direct.ScriptHash || legacy.Boss.GroupHash != direct.Boss.GroupHash || legacy.Boss.RoutingHash != direct.Boss.RoutingHash {
		return fmt.Errorf("Black Purgatory baseline source/definition mismatch")
	}
	if legacy.ClientSource != direct.ClientSource {
		if legacy.ClientSource != "2429b15aa4235be32c3f3b49676646a25b6bf42bd45d5d651dae7e6fd9186167" || direct.ClientSource != direct.Source {
			return fmt.Errorf("unknown Black Purgatory export provenance")
		}
		legacy.ClientSource = direct.ClientSource
	}
	return verifyPVFCatalog(legacy, direct)
}

func blackPurgatoryLookup(index *catalog.ItemIndex) func(uint32) (catalog.LootItem, bool) {
	return func(id uint32) (catalog.LootItem, bool) {
		entry, ok := index.Items[id]
		return catalog.LootItem{ID: id, Kind: entry.Kind, StackableType: entry.StackableType, StackLimit: entry.StackLimit, Script: catalog.ScriptRecord{Path: entry.Path}}, ok
	}
}

func preparePVFBlackPurgatory(c *pvfCoreCatalogs, s *gamedata.Source, selected map[string]bool, i pvfItemInputs) error {
	if !selected["black-purgatory"] {
		return nil
	}
	policy, err := readPVFContentPolicy(i.contentPolicyPath)
	if err != nil {
		return err
	}
	direct, err := s.BlackPurgatory(*c.items, policy.BlackPurgatory)
	if err != nil {
		return err
	}
	if c.boosters == nil {
		c.boosters, err = s.Boosters(*c.items)
		if err != nil {
			return err
		}
	}
	boxes := boosterBoxSource{catalog: &BoosterCatalog{Definitions: c.boosters, Items: c.items.Items}}
	bound, err := loot.NewBlackPurgatoryRewards(*direct, boxes, blackPurgatoryLookup(c.items))
	if err != nil {
		return err
	}
	if i.checksBaselines() {
		path := pvfOdysseyBaseline(i, i.blackPurgatoryPath, "", "black-purgatory-rewards.json")
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var legacy loot.BlackPurgatoryRewards
		if err := json.Unmarshal(raw, &legacy); err != nil {
			return err
		}
		if _, err := loot.NewBlackPurgatoryRewards(legacy, boxes, blackPurgatoryLookup(c.items)); err != nil {
			return err
		}
		if err := auditBlackPurgatory(legacy, *direct); err != nil {
			return fmt.Errorf("Black Purgatory: %w", err)
		}
	}
	c.blackPurgatory = bound
	log.Printf("PVF Black Purgatory prepared: card branches=%d VIP source-only=%d equipment groups=%d rates=%v/%d; operator probabilities retained", len(direct.Cards), len(direct.VIPSourceOnly), len(direct.Boss.Groups), direct.Boss.Rates, direct.Boss.Denominator)
	s.ReleaseReadCaches()
	return nil
}

func (c pvfCoreCatalogs) loadBlackPurgatory(path string, boxes loot.RewardBoxSource, lookup func(uint32) (catalog.LootItem, bool)) (*loot.BlackPurgatoryRewards, error) {
	if c.blackPurgatory != nil {
		return loot.NewBlackPurgatoryRewards(*c.blackPurgatory, boxes, lookup)
	}
	return loot.LoadBlackPurgatoryRewards(path, boxes, lookup)
}
