package workflow

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"encoding/json"
	"testing"
)

func TestBoostMainlineEligibilityUsesSelectedSourceGoal(t *testing.T) {
	for _, tc := range []struct {
		name                string
		activated, finished bool
		level, goal         byte
		wantError           bool
	}{
		{"ordinary", false, false, 115, 115, false},
		{"inactive-finished", false, true, 115, 115, false},
		{"still-training", true, false, 115, 115, false},
		{"below-source-goal", true, true, 115, 116, true},
		{"missing-source-goal", true, true, 115, 0, true},
		{"graduated-requires-service", true, true, 115, 115, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, _ := json.Marshal(map[string]any{"level": tc.level, "unknown": "preserved"})
			raw, err := boostup.WriteState(raw, boostup.State{Version: 1, Activated: tc.activated, Training: boostup.Training{Finished: tc.finished}})
			if err != nil {
				t.Fatal(err)
			}
			role := database.Character{ID: 5, WireID: 7, State: raw}
			out, err := (&QuestService{}).ReconcileBoostMainline(context.Background(), role, &boostup.Catalog{GoalLevel: tc.goal})
			if (err != nil) != tc.wantError {
				t.Fatalf("error=%v", err)
			}
			if !tc.wantError && (out.Role.ID != role.ID || string(out.Role.State) != string(raw) || len(out.Settled) > 0) {
				t.Fatal("ineligible role changed")
			}
		})
	}
}
