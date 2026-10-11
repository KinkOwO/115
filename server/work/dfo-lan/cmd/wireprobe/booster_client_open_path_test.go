package main

import (
	"dfolan/internal/catalog"
	"testing"
)

func TestBoosterClientOpenPathOnlyChangesDroppedWrapperMode(t *testing.T) {
	cat := &BoosterCatalog{Definitions: map[uint32]BoosterDefinition{10: {Template: 10, ClientOpenPath: true, Pools: []catalog.BoosterRewardPool{{DrawCount: 1, Candidates: []catalog.BoosterRewardCandidate{{Template: 20, Weight: 1, Count: 1}}}}}, 11: {Template: 11, Pools: []catalog.BoosterRewardPool{{DrawCount: 1, Candidates: []catalog.BoosterRewardCandidate{{Template: 20, Weight: 1, Count: 1}}}}}}, Items: map[uint32]ItemIndexInfo{10: {ID: 10, Kind: "stackable"}, 11: {ID: 11, Kind: "stackable"}, 20: {ID: 20, Kind: "stackable"}}}
	ordinary := boosterBoxSource{catalog: cat}
	dropped := boosterBoxSource{catalog: cat, serverUnwrap: true}
	if _, ok := ordinary.RewardBox(10); !ok {
		t.Fatal("explicit event auto-open changed")
	}
	if _, ok := dropped.RewardBox(10); ok || dropped.Container(10) || !dropped.Item(10) {
		t.Fatal("client-openable item would be expanded or suppressed")
	}
	if _, ok := dropped.RewardBox(11); !ok || !dropped.Container(11) {
		t.Fatal("box with no client path became unopenable")
	}
}
