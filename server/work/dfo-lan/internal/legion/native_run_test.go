package legion

import (
	"dfolan/internal/catalog"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"
)

func nativeRunFixture(name string) catalog.LegionContent {
	limit := int64(1)
	c := catalog.LegionContent{Name: name, LastPhase: 2, SelectLimit: 17, ReselectCount: &limit, RetryCount: &limit,
		NativeOperations: &catalog.NativeLegionOperations{Modes: map[string][]uint16{}, Definitions: map[uint16]catalog.NativeLegionOperation{}}}
	for i := 0; i < 5; i++ {
		c.Dungeons = append(c.Dungeons, catalog.LegionDungeon{Dungeon: uint32(100 + i), Boss: uint32(200 + i)})
	}
	pools := []uint32{0, 1, 2, 3}
	if name == "DuskyIsland" {
		pools = []uint32{0, 1, 2, 4, 5, 6}
	}
	for i, p := range pools {
		id := uint16(i + 1)
		c.NativeOperations.Modes["normal"] = append(c.NativeOperations.Modes["normal"], id)
		c.NativeOperations.Definitions[id] = catalog.NativeLegionOperation{SelectPool: p, MemberLimit: "party", Probability: 1,
			Ranges: []catalog.NativeLegionRange{{Party: "party", Difficulty: "normal", Minimum: 10, Maximum: 10}}}
	}
	return c
}

func TestNativeRunExpiryAndRetryKeepOwnedOffers(t *testing.T) {
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		t.Run(name, func(t *testing.T) {
			now := time.Unix(100, 0)
			r, err := NewNativeRun(nativeRunFixture(name), "normal", [3]int{0, 1, 2}, now, func(int64) (int64, error) { return 0, nil })
			if err != nil {
				t.Fatal(err)
			}
			r.Ready(now)
			if err = r.Open(false, now); err != nil {
				t.Fatal(err)
			}
			deadline := r.SelectionDeadline
			if r.Expire(deadline.Add(-time.Nanosecond)) {
				t.Fatal("early expiration")
			}
			if !r.Expire(deadline) || r.State != NativeWaiting || r.Selected != -1 {
				t.Fatal("empty selection expiry")
			}
			if err = r.Open(false, deadline); err != nil {
				t.Fatal(err)
			}
			if err = r.Select(r.Offers[1].Index, deadline); err != nil {
				t.Fatal(err)
			}
			frozen := append([]catalog.NativeLegionCard(nil), r.Offers...)
			if err = r.Open(true, deadline); err != nil {
				t.Fatal(err)
			}
			if !r.Expire(r.SelectionDeadline) || r.Selected != 1 || r.State != NativeReady {
				t.Fatal("lost previous choice on expiration")
			}
			if err = r.Enter(0, r.Stage().Dungeon, "first"); err != nil {
				t.Fatal(err)
			}
			if err = r.Retry("foreign"); err == nil {
				t.Fatal("foreign retry")
			}
			if err = r.Retry("first"); err != nil {
				t.Fatal(err)
			}
			if r.Retries != 1 || r.State != NativeReady || r.Selected != 1 || r.Phase != 0 || !reflect.DeepEqual(frozen, r.Offers) {
				t.Fatal("retry changed frozen content")
			}
			if err = r.Enter(0, r.Stage().Dungeon, "second"); err != nil {
				t.Fatal(err)
			}
			if err = r.Clear(r.Stage().Dungeon, "first", true); err == nil {
				t.Fatal("stale attempt cleared retry")
			}
			if err = r.Retry("second"); err == nil {
				t.Fatal("source retry budget ignored")
			}
			r.Abandon()
			if err = r.Clear(r.Stage().Dungeon, "second", true); err == nil {
				t.Fatal("abandoned clear")
			}
		})
	}
}

func TestNativeRunOwnedSequentialThreeStages(t *testing.T) {
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		t.Run(name, func(t *testing.T) {
			now := time.Unix(100, 0)
			r, err := NewNativeRun(nativeRunFixture(name), "normal", [3]int{4, 0, 2}, now, func(n int64) (int64, error) { return 0, nil })
			if err != nil {
				t.Fatal(err)
			}
			if r.Ready(now.Add(-time.Second)) || !r.Ready(now) {
				t.Fatal("countdown gate")
			}
			for phase := 0; phase < 3; phase++ {
				if r.Phase != phase || r.State != NativeWaiting {
					t.Fatal("phase skipped", r)
				}
				wantOffers := 3
				if name == "DuskyIsland" || phase == 2 {
					wantOffers = 4
				}
				if len(r.Offers) != wantOffers {
					t.Fatal("wrong phase offers", r.Offers)
				}
				if err := r.Advance(); err == nil {
					t.Fatal("advanced before clear")
				}
				if err := r.Enter(phase, r.Stage().Dungeon, "foreign"); err == nil {
					t.Fatal("entered without selection")
				}
				if err := r.Open(false, now); err != nil {
					t.Fatal(err)
				}
				deadline := r.SelectionDeadline
				if err := r.Open(false, now.Add(time.Second)); err != nil || r.SelectionDeadline != deadline {
					t.Fatal("reopen extended deadline", err)
				}
				if err := r.Select(32767, now); err == nil {
					t.Fatal("accepted unoffered operation")
				}
				if err := r.Select(r.Offers[0].Index, now); err != nil {
					t.Fatal(err)
				}
				frozen := append([]catalog.NativeLegionCard(nil), r.Offers...)
				if err := r.Open(true, now); err != nil || r.Reselections != 1 || !reflect.DeepEqual(frozen, r.Offers) {
					t.Fatal("reselection rerolled source", err)
				}
				if err := r.Select(r.Offers[0].Index, now); err != nil {
					t.Fatal(err)
				}
				if err := r.Open(true, now); err == nil {
					t.Fatal("exceeded source reselection budget")
				}
				dgn := r.Stage().Dungeon
				attempt := fmt.Sprint("attempt-", phase)
				if err := r.Enter(phase+1, dgn, attempt); err == nil {
					t.Fatal("accepted claimed later phase")
				}
				if err := r.Enter(phase, dgn, attempt); err != nil {
					t.Fatal(err)
				}
				for _, x := range []struct {
					d    uint32
					a    string
					done bool
				}{{dgn, "foreign", true}, {dgn + 1, attempt, true}, {dgn, attempt, false}} {
					if err := r.Clear(x.d, x.a, x.done); err == nil {
						t.Fatal("accepted foreign/incomplete clear")
					}
				}
				if err := r.Clear(dgn, attempt, true); err != nil {
					t.Fatal(err)
				}
				if err := r.Clear(dgn, attempt, true); err != nil {
					t.Fatal("duplicate clear", err)
				}
				if err := r.Advance(); err == nil {
					t.Fatal("advanced without durable reward acknowledgement")
				}
				if err := r.Settle("foreign"); err == nil {
					t.Fatal("foreign settlement accepted")
				}
				if err := r.Settle(attempt); err != nil {
					t.Fatal(err)
				}
				if err := r.Advance(); err != nil {
					t.Fatal(err)
				}
			}
			if r.State != NativeComplete {
				t.Fatal("not complete")
			}
			for _, c := range r.History {
				if c == nil {
					t.Fatal("lost phase history")
				}
			}
			if err := r.Open(false, now); err == nil {
				t.Fatal("reopened completed run")
			}
		})
	}
}

func TestNativeRunUsesCurrentArchive(t *testing.T) {
	p := os.Getenv("DFO_REPAIR_PVF")
	if p == "" {
		t.Skip("native PVF required")
	}
	a, err := catalog.OpenTestArchiveCached(p, "")
	if err != nil {
		t.Fatal(err)
	}
	c, err := catalog.ImportLegionContents(a)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"Dimension Cloister", "DuskyIsland"} {
		for mode := range c.Contents[name].NativeOperations.Modes {
			t.Run(name+"/"+mode, func(t *testing.T) {
				for _, last := range []bool{false, true} {
					r, err := NewNativeRun(c.Contents[name], mode, [3]int{0, 1, 2}, time.Time{}, func(n int64) (int64, error) {
						if last {
							return n - 1, nil
						}
						return 0, nil
					})
					if err != nil {
						t.Fatal(err)
					}
					for phase := 0; phase < 3; phase++ {
						if _, err := r.drawOffers(phase); err != nil {
							t.Fatalf("phase %d: %v", phase, err)
						}
					}
				}
			})
		}
	}
}
