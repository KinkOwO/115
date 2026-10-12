package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/legion"
	"dfolan/internal/loot"
	"encoding/binary"
	"fmt"
	"math"
)

// The owner selects the COS basic wrapper. Subsequent wrappers open only when
// their same-source scripts declare [instantly open]. User-openable boxes,
// including the fragment pot, remain boxes. The CTP star box never enters here.
type apocalypseBundleSource struct {
	loot.RewardBoxSource
	catalog catalog.LootCatalog
	root    uint32
	roots   map[uint32]bool
	err     error
}

func (s *apocalypseBundleSource) RewardBox(id uint32) (loot.RewardBox, bool) {
	if id != s.root && !s.roots[id] {
		if _, ok := s.catalog.Items[id]; !ok {
			return loot.RewardBox{}, false
		}
		script, err := s.catalog.ItemScript(id)
		if err != nil {
			s.err = err
			return loot.RewardBox{}, false
		}
		instant := false
		for _, cell := range script.Cells {
			if cell.Text == "[instantly open]" {
				instant = true
				break
			}
		}
		if !instant {
			return loot.RewardBox{}, false
		}
	}
	box, ok := s.RewardBoxSource.RewardBox(id)
	if !ok {
		s.err = fmt.Errorf("apocalypse instant reward wrapper %d has no parsed pools", id)
	}
	return box, ok
}

func (s *apocalypseBundleSource) Container(id uint32) bool {
	if id == s.root || s.roots[id] {
		return true
	}
	// A box left intact above is an actual payable item, not an unresolved
	// wrapper. Instant wrappers with no parsed pool are refused.
	_, open := s.RewardBox(id)
	if open {
		return true
	}
	item, ok := s.catalog.Items[id]
	return ok && item.StackableType == "[virtual]"
}

func apocalypseBasicProducts(contents catalog.LegionContents, cat catalog.LootCatalog, boxes loot.RewardBoxSource, difficulty, run string) ([]legion.ClearReward, error) {
	basic, err := legion.GuaranteedBasicClearReward(contents, cat.Source.Checksum, difficulty)
	if err != nil {
		return nil, err
	}
	if boxes == nil || run == "" {
		return nil, fmt.Errorf("apocalypse basic reward bundle source unavailable")
	}
	if _, ok := boxes.RewardBox(basic.Template); !ok {
		return nil, fmt.Errorf("apocalypse basic reward bundle has no parsed pools")
	}
	src := &apocalypseBundleSource{RewardBoxSource: boxes, catalog: cat, root: basic.Template}
	// Stable across retry/disconnect. Freeze persists the exact products before
	// any inventory grant, including probabilistic coins and equipment.
	sum := sha256.Sum256([]byte(run + ":" + cat.Source.Checksum + ":" + contents.SHA256))
	awards, _, skipped := loot.OpenRewardBoxes(binary.LittleEndian.Uint32(sum[:]), src, []loot.Award{{Template: basic.Template, Amount: basic.Amount}})
	if src.err != nil {
		return nil, src.err
	}
	for _, why := range skipped {
		if why != "attunement_empty_prize_4294967295" {
			return nil, fmt.Errorf("apocalypse basic reward unresolved: %s", why)
		}
	}
	var out []legion.ClearReward
	at := map[uint32]int{}
	for _, a := range awards {
		if a.Template == 0 || a.Amount == 0 {
			return nil, fmt.Errorf("invalid apocalypse basic reward product")
		}
		if i, ok := at[a.Template]; ok {
			total := uint64(out[i].Amount) + uint64(a.Amount)
			if total > math.MaxUint32 {
				return nil, fmt.Errorf("apocalypse reward amount overflow")
			}
			out[i].Amount = uint32(total)
		} else {
			at[a.Template] = len(out)
			out = append(out, legion.ClearReward{Template: a.Template, Amount: a.Amount})
		}
	}
	if len(out) == 0 || len(out) > 35 {
		return nil, fmt.Errorf("apocalypse basic reward products exceed native list")
	}
	return out, nil
}
