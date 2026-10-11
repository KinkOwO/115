package dungeon

import (
	"dfolan/internal/catalog"
	"fmt"
)

// Rewards resolve from the current PVF catalog. The native floor vector is 100.
func ElvenmereFloorReward(rules *catalog.ElvenmereRules, floor byte) (catalog.ElvenmereFloorRule, error) {
	if rules == nil || floor == 0 || floor > rules.MaxFloor {
		return catalog.ElvenmereFloorRule{}, fmt.Errorf("elvenmere source floor rules unavailable")
	}
	r, ok := rules.Floors[floor]
	if !ok {
		return r, fmt.Errorf("elvenmere source floor missing")
	}
	return r, nil
}
