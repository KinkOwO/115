package legion

import (
	"dfolan/internal/catalog"
	"testing"
)

// phaseFixture mirrors the release table: six (phase, seconds) pairs and four
// operations, of which only the first carries [allow coin].
func phaseFixture() *catalog.ApocalypseCatalog {
	op := func(index int64, allowCoin []int64) catalog.ApocalypseOperation {
		i := index
		return catalog.ApocalypseOperation{
			Row:          int(10 + index*14),
			Index:        &i,
			Type:         &i,
			AllowCoin:    allowCoin,
			GateSchedule: []int64{900, 1, 0, 0, 0},
			GateFlow:     []int64{1, 2, 3, -1, -1},
			MemberLimit:  "party",
			Reward:       &catalog.ApocalypseReward{Label: "normal", Values: []float64{10421367, 1, 1000000, 1, 1}},
		}
	}
	return &catalog.ApocalypseCatalog{
		PhaseClock: []catalog.ApocalypsePhase{
			{Phase: 0, Seconds: 90},
			{Phase: 1, Seconds: 300},
			{Phase: 2, Seconds: 300},
			{Phase: 3, Seconds: 300},
			{Phase: 4, Seconds: 600},
			{Phase: 5, Seconds: 600},
		},
		Operations: []catalog.ApocalypseOperation{
			op(1, []int64{-1, 8}),
			op(2, nil),
			op(3, nil),
			op(5, nil),
		},
	}
}

func TestApocalypseClockFollowsTheTableOrder(t *testing.T) {
	clock, err := NewApocalypseClock(phaseFixture())
	if err != nil {
		t.Fatal(err)
	}
	if clock.Len() != 6 {
		t.Fatalf("clock has %d phases", clock.Len())
	}
	order := clock.Order()
	want := []int64{0, 1, 2, 3, 4, 5}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("phase order %v, want %v", order, want)
		}
	}
	if got := clock.TotalSeconds(); got != 2190 {
		t.Fatalf("total %g, want 2190", got)
	}
	if s, ok := clock.Seconds(0); !ok || s != 90 {
		t.Fatalf("phase 0 = %g %v", s, ok)
	}
	next, ok := clock.Next(4)
	if !ok || next != 5 {
		t.Fatalf("next after 4 = %d %v", next, ok)
	}
	if _, ok := clock.Next(5); ok {
		t.Fatal("phase 5 is the last one")
	}
}

func TestNewApocalypseClockRejectsIncompleteTables(t *testing.T) {
	if _, err := NewApocalypseClock(nil); err == nil {
		t.Fatal("nil catalog must be refused")
	}
	empty := &catalog.ApocalypseCatalog{}
	if _, err := NewApocalypseClock(empty); err == nil {
		t.Fatal("a table without a clock must be refused")
	}
}

func TestBuildRunPlanCarriesOnlyDeclaredFacts(t *testing.T) {
	cat := phaseFixture()
	clock, err := NewApocalypseClock(cat)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := BuildRunPlan(cat, clock, 1)
	if err != nil {
		t.Fatal(err)
	}
	if plan.OperationID != 1 || plan.Row != 24 {
		t.Fatalf("plan for operation 1: id=%d row=%d", plan.OperationID, plan.Row)
	}
	if !plan.AllowCoinConfigured || len(plan.AllowCoin) != 2 {
		t.Fatalf("operation 1 must carry [allow coin], got %v", plan.AllowCoin)
	}
	if plan.TotalSeconds != 2190 {
		t.Fatalf("plan total %g", plan.TotalSeconds)
	}
	if plan.MemberLimitClass != "party" {
		t.Fatalf("member limit class %q", plan.MemberLimitClass)
	}
	if plan.RewardLabel != "normal" || len(plan.RewardValues) != 5 {
		t.Fatalf("reward %q %v", plan.RewardLabel, plan.RewardValues)
	}

	// Operation 2 exists but configures no [allow coin]; absence must survive as
	// absence, not as a zeroed column.
	second, err := BuildRunPlan(cat, clock, 2)
	if err != nil {
		t.Fatal(err)
	}
	if second.AllowCoinConfigured || second.AllowCoin != nil {
		t.Fatalf("operation 2 must not claim [allow coin]: %v", second.AllowCoin)
	}
}

func TestBuildRunPlanRefusesUndeclaredOperation(t *testing.T) {
	cat := phaseFixture()
	clock, _ := NewApocalypseClock(cat)
	// 4 is the id the release table deliberately skips, so a client offering it
	// means the two sides disagree about the table.
	if _, err := BuildRunPlan(cat, clock, 4); err == nil {
		t.Fatal("operation 4 must be refused")
	}
	if _, err := BuildRunPlan(cat, clock, 0); err == nil {
		t.Fatal("operation 0 must be refused")
	}
	if _, err := BuildRunPlan(nil, clock, 1); err == nil {
		t.Fatal("a nil catalog must be refused")
	}
	if _, err := BuildRunPlan(cat, nil, 1); err == nil {
		t.Fatal("a nil clock must be refused")
	}
}
