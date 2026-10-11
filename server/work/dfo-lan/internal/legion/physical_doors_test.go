package legion

import (
	"dfolan/internal/catalog"
	"testing"
	"time"
)

func TestApocalypsePhysicalDoorsMatchActualCTPRoutes(t *testing.T) {
	// Small typed test inputs only; production bindings come from OBJ/ACT.
	doors := []catalog.ApocalypseDoor{{Slot: 0, Target: 2, Object: 1, Dungeon: 20}, {Slot: 1, Target: 3, Object: 2, Dungeon: 30}, {Slot: 2, Target: 4, Object: 3, Dungeon: 40}, {Slot: 3, Target: 5, Object: 4, Dungeon: 50}}
	c, e := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	clock, e := NewApocalypseClock(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, choice := range []byte{0, 1, 2, 4} {
		plan, e := PlanForChoice(c, clock, choice)
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range plan.Gates.Schedule {
			for _, door := range doors {
				path, e := plan.Gates.PhysicalDoorPath(row.FlowKeys, door)
				if row.FlowKeys[door.Slot] == 0 {
					if e == nil {
						t.Fatal("closed physical door accepted")
					}
					continue
				}
				if e != nil || len(path) == 0 || path[0] != door.Target {
					t.Fatal(choice, row, door, path, e)
				}
			}
		}
	}
	p, e := PlanForChoice(c, clock, 2)
	if e != nil {
		t.Fatal(e)
	}
	fast, _ := p.Gates.AtElapsed(time.Second)
	for _, d := range doors {
		if _, e := p.Gates.PhysicalDoorPath(fast, d); e != nil {
			t.Fatal("fast clear lost player choice", d, e)
		}
	}
	slow, _ := p.Gates.AtElapsed(121 * time.Second)
	if _, e := p.Gates.PhysicalDoorPath(slow, doors[3]); e == nil {
		t.Fatal("bottom shortcut reopened after timeout")
	}
	bad := fast
	bad[0] = 4
	if _, e := p.Gates.PhysicalDoorPath(bad, doors[0]); e == nil {
		t.Fatal("right door silently redirected to final boss")
	}
}

func TestPhysicalDoorSlotAndDestinationFollowSourceBinding(t *testing.T) {
	rules, err := CompileGateRules([]int64{10, 0, 0, 1, 0}, []int64{1, 1, 3, -1, -1})
	if err != nil {
		t.Fatal(err)
	}
	door := catalog.ApocalypseDoor{Slot: 2, Target: 1, Object: 50, Dungeon: 60}
	board, _ := rules.AtElapsed(0)
	path, err := rules.PhysicalDoorPath(board, door)
	if err != nil || len(path) != 2 || path[0] != 1 {
		t.Fatal("source slot was replaced by target-2", path, err)
	}
	door.Slot = 0
	if _, err := rules.PhysicalDoorPath(board, door); err == nil {
		t.Fatal("closed source slot accepted")
	}
}
