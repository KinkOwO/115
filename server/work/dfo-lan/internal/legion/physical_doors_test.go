package legion

import (
	"dfolan/internal/catalog"
	"testing"
	"time"
)

func TestApocalypsePhysicalDoorsMatchActualCTPRoutes(t *testing.T) {
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
			for _, door := range ApocalypsePhysicalDoors() {
				path, e := plan.Gates.PhysicalDoorPath(row.FlowKeys, door.Target)
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
	for _, d := range ApocalypsePhysicalDoors() {
		if _, e := p.Gates.PhysicalDoorPath(fast, d.Target); e != nil {
			t.Fatal("fast clear lost player choice", d, e)
		}
	}
	slow, _ := p.Gates.AtElapsed(121 * time.Second)
	if _, e := p.Gates.PhysicalDoorPath(slow, 5); e == nil {
		t.Fatal("bottom shortcut reopened after timeout")
	}
	bad := fast
	bad[0] = 4
	if _, e := p.Gates.PhysicalDoorPath(bad, 2); e == nil {
		t.Fatal("right door silently redirected to final boss")
	}
}
