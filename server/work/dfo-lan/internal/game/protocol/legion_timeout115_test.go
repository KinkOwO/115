package protocol

import (
	"encoding/binary"
	"testing"
	"time"
)

func TestLegionTimeoutNativeClockPairAndBounds(t *testing.T) {
	start := time.Unix(1800000000, 0)
	p, e := LegionDungeonTimeout115(start, 300*time.Second)
	if e != nil || len(p) != 8 {
		t.Fatal(p, e)
	}
	limit, base := binary.LittleEndian.Uint32(p), binary.LittleEndian.Uint32(p[4:])
	if limit != 300 || base != 1800000000 || uint64(base)+uint64(limit) != 1800000300 {
		t.Fatal("swapped relative/absolute clock", limit, base)
	}
	// Native frozen-clear formula: deadline - (start + elapsedMS/1000).
	if (uint64(base)+uint64(limit))-(uint64(base)+uint64(12345)/1000) != 288 {
		t.Fatal("millisecond conversion")
	}
	for _, x := range []struct {
		start time.Time
		limit time.Duration
	}{
		{time.Time{}, time.Second}, {time.Unix(-1, 0), time.Second},
		{start, 0}, {start, time.Millisecond}, {start, 2 * time.Hour}, {time.Unix(0x7ffffffe, 0), 3 * time.Second},
	} {
		if _, e := LegionDungeonTimeout115(x.start, x.limit); e == nil {
			t.Fatal("invalid clock accepted", x)
		}
	}
}
