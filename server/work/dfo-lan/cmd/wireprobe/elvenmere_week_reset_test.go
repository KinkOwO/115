package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"testing"
	"time"
)

func TestElvenmereResetTerminatesOldRunWithoutRewardOrReceipt(t *testing.T) {
	w, f := elvenmereRewardFixture(t, false)
	boundary := time.Unix(dungeon.ElvenmereWeekStart(time.Now()), 0).AddDate(0, 0, 7)
	w.state = database.WorldState{Position: database.WorldPosition{Town: 146, Area: 0, X: 700, Y: 300}}
	w.activeDungeon.ElvenmereWeekStart = dungeon.ElvenmereWeekStart(boundary.Add(-time.Second))
	before := string(w.role.State)
	if plan, err := w.elvenmereWeekReset(boundary.Add(-time.Nanosecond)); err != nil || len(plan) != 0 || w.activeDungeon == nil {
		t.Fatal("reset fired early", err)
	}
	plan, err := w.elvenmereWeekReset(boundary)
	if err != nil || w.activeDungeon != nil || len(plan) < 3 {
		t.Fatal("missing town return", plan, err)
	}
	for _, p := range plan {
		if p.Kind == 1 || p.ID == 14 || p.ID == 31 || p.ID == 37 {
			t.Fatal("reset fabricated request or reward", p)
		}
	}
	if string(w.role.State) != before {
		t.Fatal("reset rewrote character state")
	}
	if n, err := f.EventCount(context.Background(), w.role.ID); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if plan, err := w.elvenmereWeekReset(boundary.Add(time.Second)); err != nil || len(plan) != 0 {
		t.Fatal("repeated reset", plan, err)
	}
}
