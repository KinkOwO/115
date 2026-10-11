package activity

import (
	"reflect"
	"testing"
	"time"
)

func TestLoginSourceProgressAndCatchup(t *testing.T) {
	c, err := ParseLogin(fixture(t, "login"))
	if err != nil {
		t.Fatal(err)
	}
	observe := func(p LoginProgress, date string) LoginProgress {
		t.Helper()
		now, _ := time.Parse("2006-01-02 15:04:05", date+" 08:00:00")
		next, active, err := c.Observe(p, now, date, 115)
		if err != nil || !active {
			t.Fatal(active, err)
		}
		return next
	}
	p := observe(LoginProgress{}, "2026-09-01")
	p = observe(p, "2026-09-01")
	if p.Visits != 1 {
		t.Fatal("reconnect counted twice")
	}
	p = observe(p, "2026-09-04")
	if p.Visits != 2 {
		t.Fatal("offline days fabricated", p)
	}
	states, err := c.DayStates(p)
	if err != nil || states[0] != 1 || states[1] != 1 || states[2] != 0 {
		t.Fatal(states, err)
	}
	next, rewards, applied, err := c.Claim(p, 0)
	if err != nil || !applied || !reflect.DeepEqual(rewards, c.Days[0].Rewards) {
		t.Fatal(rewards, applied, err)
	}
	if p.Claimed[0] {
		t.Fatal("claim mutated input before transaction commit")
	}
	_, rewards, applied, err = c.Claim(next, 0)
	if err != nil || applied || len(rewards) != 0 {
		t.Fatal("duplicate grant", err)
	}
	if _, _, _, err = c.Claim(next, 2); err == nil {
		t.Fatal("future day unlocked")
	}
	if _, _, err = c.Observe(next, time.Date(2026, 9, 2, 8, 0, 0, 0, time.UTC), "2026-09-02", 115); err == nil {
		t.Fatal("clock rollback accepted")
	}
	for d := 5; d <= 14; d++ {
		next = observe(next, time.Date(2026, 9, d, 0, 0, 0, 0, time.UTC).Format("2006-01-02"))
	}
	if next.Visits != uint32(len(c.Days)) {
		t.Fatal("login cap not source driven")
	}
}

func TestLoginExecutionRespectsSourceBoundsAndRewardChanges(t *testing.T) {
	c, err := ParseLogin(fixture(t, "login"))
	if err != nil {
		t.Fatal(err)
	}
	end, _ := time.Parse("2006-01-02 15:04:05", c.Dates.End)
	if _, active, err := c.Observe(LoginProgress{}, end, end.Format("2006-01-02"), 115); err != nil || active {
		t.Fatal("end boundary accepted", err)
	}
	start, _ := time.Parse("2006-01-02 15:04:05", c.Dates.Start)
	c.MinimumLevel = 116
	if _, active, err := c.Observe(LoginProgress{}, start, start.Format("2006-01-02"), 115); err != nil || active {
		t.Fatal("source level ignored", err)
	}
	c.MinimumLevel = 0
	p, _, err := c.Observe(LoginProgress{}, start, start.Format("2006-01-02"), 115)
	if err != nil {
		t.Fatal(err)
	}
	c.Days[0].Rewards[0].Count += 7
	_, rewards, _, err := c.Claim(p, 0)
	if err != nil || rewards[0].Count != c.Days[0].Rewards[0].Count {
		t.Fatal("parallel reward table", err)
	}
	c.Accumulate = false
	if _, _, err = c.Observe(p, start, start.Format("2006-01-02"), 115); err == nil {
		t.Fatal("unverified branch guessed")
	}
}

func TestLoginCorruptOrNewerProgressNeverGrants(t *testing.T) {
	c, err := ParseLogin(fixture(t, "login"))
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []LoginProgress{
		{Version: 2, Visits: 1, LastDay: "2026-09-01"},
		{Version: 1, Visits: 1},
		{Version: 0, Visits: 1, LastDay: "2026-09-01"},
		{Version: 1, Visits: 1, LastDay: "2026-09-01", Claimed: map[uint32]bool{4: true}},
		{Version: 1, Visits: 1, LastDay: "2026-09-01", Claimed: map[uint32]bool{0: false}},
	} {
		if _, _, applied, err := c.Claim(p, 0); err == nil || applied {
			t.Fatalf("corrupt state granted: %+v", p)
		}
		if _, err := c.DayStates(p); err == nil {
			t.Fatalf("corrupt state advertised: %+v", p)
		}
	}
}
