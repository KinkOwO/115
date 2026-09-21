package dungeon

import "testing"

func TestElvenmereFloorWeeklyReward(t *testing.T) {
	tests := []struct {
		floor    byte
		template uint32
		count    uint32
	}{
		{1, 10336354, 5},
		{34, 10336354, 5},
		{35, 10336354, 30},
		{36, 10336354, 7},
		{60, 10336354, 40},
		{61, 10332382, 15},
		{85, 10332382, 100},
		{86, 10332382, 20},
		{100, 10332382, 150},
		{0, 0, 0},
		{101, 0, 0},
	}

	for _, tc := range tests {
		tmpl, cnt := ElvenmereFloorWeeklyReward(tc.floor)
		if tmpl != tc.template || cnt != tc.count {
			t.Errorf("ElvenmereFloorWeeklyReward(%d) = (%d, %d), want (%d, %d)", tc.floor, tmpl, cnt, tc.template, tc.count)
		}
	}
}

func TestElvenmereFloorSeasonReward(t *testing.T) {
	if tmpl, cnt := ElvenmereFloorSeasonReward(5); tmpl != 10333027 || cnt != 1 {
		t.Errorf("expected season reward for floor 5, got (%d, %d)", tmpl, cnt)
	}
	if tmpl, cnt := ElvenmereFloorSeasonReward(25); tmpl != 10333719 || cnt != 5 {
		t.Errorf("expected season reward for floor 25, got (%d, %d)", tmpl, cnt)
	}
	if tmpl, cnt := ElvenmereFloorSeasonReward(100); tmpl != 10333022 || cnt != 1 {
		t.Errorf("expected season reward for floor 100, got (%d, %d)", tmpl, cnt)
	}
	if tmpl, cnt := ElvenmereFloorSeasonReward(1); tmpl != 0 || cnt != 0 {
		t.Errorf("expected no season reward for floor 1, got (%d, %d)", tmpl, cnt)
	}
}

func TestElvenmereFloorClearExp(t *testing.T) {
	if exp := ElvenmereFloorClearExp(1); exp != 90000 {
		t.Errorf("expected 90000 exp for floor 1, got %d", exp)
	}
	if exp := ElvenmereFloorClearExp(100); exp != 3060000 {
		t.Errorf("expected 3060000 exp for floor 100, got %d", exp)
	}
	if exp := ElvenmereFloorClearExp(0); exp != 0 {
		t.Errorf("expected 0 exp for floor 0, got %d", exp)
	}
}
