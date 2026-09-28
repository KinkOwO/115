package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"os"
	"testing"
	"time"
)

// Real-source offline route test. No game launch or database access.
// The input is supplied explicitly, never a developer's private path.
func TestMoonSoloSourceLifecycle(t *testing.T) {
	path := os.Getenv("MOON_TEST_DUNGEONS")
	if path == "" {
		t.Skip("set MOON_TEST_DUNGEONS to a same-client complete export")
	}
	c, e := catalog.LoadDungeons(path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1800000000, 0)
	r, e := NewMoonSolo(c, 115, 7, 0, now)
	if e != nil {
		t.Fatal(e)
	}
	firstRun := r.Stamp()
	if _, e = r.AdvanceMoonFloor(r.Stamp(), 7, c, 1, now); e == nil {
		t.Fatal("early portal")
	}
	clear := func() {
		t.Helper()
		if e := r.Loaded(now); e != nil {
			t.Fatal(e)
		}
		if _, e := r.MoonEnterRoom(r.Stamp(), 7, now); e != nil {
			t.Fatal(e)
		}
		for round := 0; round < 4; round++ {
			live := r.Session().LivingMonsters()
			killed := 0
			for _, v := range live {
				if v.NonCombat || v.APC {
					continue
				}
				changed, e := r.Session().ConfirmDeath(uint32(v.Entity), 7, 7)
				if e != nil || !changed {
					t.Fatal(e)
				}
				if e = r.MoonScoreDeath(r.Stamp(), 7, v.Entity); e != nil {
					t.Fatal(e)
				}
				if _, e = r.MoonAfterDeath(r.Stamp(), 7); e != nil {
					t.Fatal(e)
				}
				killed++
			}
			if killed == 0 {
				return
			}
		}
		t.Fatal("unbounded respawn")
	}
	for y := byte(0); y < 5; y++ {
		if y > 0 {
			if e = r.Move(c, protocol.DungeonRoomTransition{Position: [2]byte{0, y}}, uint32(y), now); e != nil {
				t.Fatal("floor1 move", y, e)
			}
		}
		clear()
	}
	if !r.Session().MoonPortalReady() || r.Session().Definition.ID != 100004136 {
		t.Fatal("gate did not wait for portal")
	}
	before := r.Session().NextEntity
	if _, e = r.AdvanceMoonFloor(r.Stamp(), 7, c, 10, now); e != nil {
		t.Fatal(e)
	}
	if r.Stamp().RunID != firstRun.RunID || r.Session().NextEntity < before {
		t.Fatal("floor replaced reward/entity identity")
	}
	if _, e = r.MoonProgress(firstRun, 7); e == nil {
		t.Fatal("stale stamp accepted")
	}
	if _, e = r.MoonProgress(r.Stamp(), 8); e == nil {
		t.Fatal("foreign actor")
	}
	for i, pos := range moonSecondFloorRoute {
		if i > 0 {
			if e = r.Move(c, protocol.DungeonRoomTransition{Position: pos}, uint32(i), now); e != nil {
				t.Fatal("floor2 move", pos, e)
			}
		}
		clear()
	}
	if !r.Session().Completed() || r.Session().CompletionTarget() == 0 {
		t.Fatal("real final death did not complete without C117")
	}
	if r.Session().Definition.ID != 100004137 {
		t.Fatal("wrong final")
	}
}

func TestMoonSoloStampAndLoadDeadline(t *testing.T) {
	r := &MoonSoloOwner{session: &Session{RunID: "run"}, leader: 7, room: 4, phase: moonSoloLoading, deadline: time.Unix(100, 0)}
	if !r.LoadExpired(time.Unix(101, 0)) {
		t.Fatal("missing deadline")
	}
	if e := r.Loaded(time.Unix(101, 0)); e == nil {
		t.Fatal("late loading release")
	}
	if e := r.validate(MoonSoloStamp{"old", 4}, 7); e == nil {
		t.Fatal("old run")
	}
	if e := r.validate(MoonSoloStamp{"run", 3}, 7); e == nil {
		t.Fatal("old room")
	}
	if e := r.validate(r.Stamp(), 8); e == nil {
		t.Fatal("foreign actor")
	}
}
