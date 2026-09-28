package legion

import (
	"dfolan/internal/catalog"
	"testing"
	"time"
)

func TestApocalypseGateTablesRetainSourceRows(t *testing.T) {
	c, e := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	clock, e := NewApocalypseClock(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, choice := range []byte{0, 1, 2, 4} {
		p, e := PlanForChoice(c, clock, choice)
		if e != nil {
			t.Fatal(choice, e)
		}
		if len(p.Gates.Schedule) == 0 {
			t.Fatal("missing parsed source schedule", choice)
		}
		flow, ok := p.Gates.Flow(1)
		if !ok {
			t.Fatal("missing flow1", choice)
		}
		if choice == 0 || choice == 4 {
			if flow.Targets != [4]int8{2, 3, -1, -1} || p.Gates.Schedule[0].Seconds != 900 {
				t.Fatal(flow, p.Gates.Schedule)
			}
		} else {
			if flow.Targets != [4]int8{3, 4, 5, -1} || p.Gates.Schedule[0].Seconds != 300 {
				t.Fatal(flow, p.Gates.Schedule)
			}
		}
	}
}

func TestApocalypseShortcutThresholdsUseEachDifficultySource(t *testing.T) {
	c, e := catalog.LoadApocalypseCatalog("../../configs/apocalypse.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	clock, e := NewApocalypseClock(c)
	if e != nil {
		t.Fatal(e)
	}
	for _, v := range []struct {
		choice  byte
		elapsed time.Duration
		want    [4]byte
	}{
		{1, 59 * time.Second, [4]byte{3, 1, 2, 0}},
		{1, 60 * time.Second, [4]byte{3, 1, 0, 0}},
		{1, 120 * time.Second, [4]byte{3, 0, 0, 0}},
		{2, 45*time.Second - time.Nanosecond, [4]byte{3, 1, 2, 4}},
		{2, 45 * time.Second, [4]byte{3, 1, 2, 0}},
		{2, 60 * time.Second, [4]byte{3, 1, 0, 0}},
		{2, 120 * time.Second, [4]byte{3, 0, 0, 0}},
		{2, 300 * time.Second, [4]byte{}},
		{0, 899 * time.Second, [4]byte{1, 0, 0, 0}},
	} {
		p, e := PlanForChoice(c, clock, v.choice)
		if e != nil {
			t.Fatal(e)
		}
		got, e := p.Gates.AtElapsed(v.elapsed)
		if e != nil || got != v.want {
			t.Fatal(v, got, e)
		}
	}
}

func TestApocalypseGateRowsRejectBrokenSource(t *testing.T) {
	for _, v := range []struct{ s, f []int64 }{
		{[]int64{900, 1, 0, 0}, []int64{1, 2, 3, -1, -1}},
		{[]int64{900, 2, 0, 0, 0}, []int64{1, 2, 3, -1, -1}},
		{[]int64{900, 1, 0, 0, 0}, []int64{1, 2, 9, -1, -1}},
		{[]int64{900, 1, 0, 0, 0}, []int64{1, 2, 3, -1, -1, 1, 3, 4, -1, -1}},
	} {
		if _, e := CompileGateRules(v.s, v.f); e == nil {
			t.Fatal("invalid gates accepted", v)
		}
	}
}
