package main

import (
	"dfolan/internal/legion"
	"fmt"
)

// Current PE 141c3cf60 sends C2293 with no application body; 141c3cb80
// consumes only the common success byte. CN's C2294 is a different dialect.
func (w *worldSession) venusEndAtPhase4(p []byte) ([]outboundPacket, []map[string]any, error) {
	if len(p) != 0 && len(p) != 8 {
		return nil, nil, fmt.Errorf("Venus phase-four stop has no request body")
	}
	for _, b := range p {
		if b != 0 {
			return nil, nil, fmt.Errorf("Venus phase-four stop has nonzero transport padding")
		}
	}
	r := w.venus
	if r == nil {
		return nil, nil, fmt.Errorf("Venus phase-four stop without a run")
	}
	ack := outboundPacket{"venus_phase4_end_ack", 1, legion.CmdVenusEndAtPhase4, []byte{1}}
	if r.downshift && r.rewardSent {
		return []outboundPacket{ack}, nil, nil
	}
	if r.choice != 2 || !r.entered || r.stage != 3 || r.finalDone || r.cleared[3] || !r.cleared[0] || !r.cleared[1] || !r.cleared[2] || w.activeDungeon == nil || w.activeDungeon.Definition.ID != legion.VenusStageDungeons[3] {
		return nil, nil, fmt.Errorf("Venus phase-four stop requires the owned descent phase after three clears")
	}
	// Retain a pending stop on bag/storage failure, so later combat cannot
	// replace its already frozen expert reward with a different payout.
	r.downshift = true
	packets, err := w.completeVenusStage()
	if err != nil {
		return nil, nil, err
	}
	if !r.rewardSent {
		return nil, nil, fmt.Errorf("Venus phase-four reward has not committed")
	}
	return append([]outboundPacket{ack}, packets...), []map[string]any{{"kind": "venus_phase4_downgraded", "character_id": w.role.ID, "reward_stage": 2, "reward_choice": 1}}, nil
}
