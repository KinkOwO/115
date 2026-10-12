package legion

import (
	"bytes"
	"dfolan/internal/catalog"
	"encoding/binary"
	"testing"
	"time"
)

func TestCloisterSourceProjectionMatchesNativeCapture(t *testing.T) {
	r, err := NewNativeRun(nativeRunFixture("Dimension Cloister"), "normal", [3]int{3, 2, 1}, time.Time{}, func(n int64) (int64, error) { return 0, nil })
	if err != nil {
		t.Fatal(err)
	}
	p, err := r.Info(1, 0)
	if err != nil || !bytes.Equal(p, dimCloisterInfoHallInitial[:107]) {
		t.Fatalf("initial projection %x: %v", p, err)
	}
	r.Ready(time.Now())
	r.Offers = []catalog.NativeLegionCard{{Index: 1, Pool: 0, Parameter: 37}, {Index: 4, Pool: 1, Parameter: 40}, {Index: 7, Pool: 2, Parameter: 150}}
	p, err = r.Info(6, 0)
	// +104 is native alignment padding; the capture's stale byte is not a
	// gameplay field. Everything consumed before that byte must match.
	if err != nil || !bytes.Equal(p[:104], dimCloisterInfoHallOptions[:104]) {
		t.Fatalf("source offers projection %x: %v", p, err)
	}
	r.Offers[1].Parameter = 35
	p, err = r.Info(6, 0)
	if err != nil || binary.LittleEndian.Uint16(p[77:]) != 35 {
		t.Fatal("source operand not projected", err)
	}
	r.Selected = 1
	p, err = r.Info(2, 0)
	if err != nil || p[2] != 1 {
		t.Fatal("selection must be an offered slot, not operation id")
	}
}

func TestDuskySourceProjectionKeepsHistorySeparateFromOffers(t *testing.T) {
	r, err := NewNativeRun(nativeRunFixture("DuskyIsland"), "normal", [3]int{0, 1, 2}, time.Time{}, func(n int64) (int64, error) { return 0, nil })
	if err != nil {
		t.Fatal(err)
	}
	r.Ready(time.Now())
	r.Phase = 1
	c := catalog.NativeLegionCard{Index: 23, Pool: 2, Parameter: 17}
	r.History[0] = &c
	r.Selected = 3
	p, err := r.Info(2, 0)
	if err != nil || len(p) != 133 {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint16(p[59:]) != 23 || binary.LittleEndian.Uint16(p[69:]) != r.Offers[3].Index || binary.LittleEndian.Uint16(p[89:]) != r.Offers[0].Index {
		t.Fatal("history/offers overlap")
	}
	if binary.LittleEndian.Uint32(p[27:]) != 1 || binary.LittleEndian.Uint32(p[39:]) != 0 || binary.LittleEndian.Uint32(p[51:]) != 2 {
		t.Fatal("source stage progress lost")
	}
}
