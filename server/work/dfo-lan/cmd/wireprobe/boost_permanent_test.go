package main

import (
	"bytes"
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"testing"
	"time"
)

// Validate the actual town announcement, including the replacement snapshot
// and its original channel gates, rather than only the calendar constant.
func TestBoostPermanentAnnouncementKeepsChannelGates(t *testing.T) {
	for _, challenge := range []bool{false, true} {
		full, ok := buildTownEventInfoTable(challenge)
		if !ok {
			t.Fatal("build event announcement")
		}
		base := eventInfoTable[2 : len(eventInfoTable)-1]
		if !bytes.Equal(full[2:2+len(base)], base) {
			t.Fatal("channel gates changed")
		}
		dates, e := protocol.BoostOpeningEvents115(0, boostup.EventEnd, challenge)
		if e != nil {
			t.Fatal(e)
		}
		added := int(binary.LittleEndian.Uint16(dates))
		if int(binary.LittleEndian.Uint16(full)) != int(binary.LittleEndian.Uint16(eventInfoTable))+added {
			t.Fatal("event row count")
		}
		rows := dates[2 : len(dates)-1]
		for row := 0; row < added; row++ {
			if len(rows) < 5 {
				t.Fatal("event row header")
			}
			id := binary.LittleEndian.Uint16(rows)
			if rows[2] != 1 {
				t.Fatal("event disabled", id)
			}
			rows = rows[5:]
			for i := 0; i < 3; i++ {
				if len(rows) < 4 {
					t.Fatal("string length")
				}
				n := int(binary.LittleEndian.Uint32(rows))
				if len(rows) < 4+n {
					t.Fatal("event string")
				}
				rows = rows[4+n:]
			}
			if len(rows) < 8 {
				t.Fatal("calendar")
			}
			start, end := binary.LittleEndian.Uint32(rows), binary.LittleEndian.Uint32(rows[4:])
			rows = rows[8:]
			if start != 0 || end != boostup.EventEnd || int64(end) <= time.Date(2034, 1, 1, 0, 0, 0, 0, time.UTC).Unix() {
				t.Fatal("old seasonal cutoff", id, start, end)
			}
			for i := 0; i < 2; i++ {
				if len(rows) < 4 {
					t.Fatal("string length")
				}
				n := int(binary.LittleEndian.Uint32(rows))
				if len(rows) < 4+n {
					t.Fatal("event string")
				}
				rows = rows[4+n:]
			}
			if len(rows) < 1 {
				t.Fatal("event flag")
			}
			rows = rows[1:]
		}
		if len(rows) != 0 {
			t.Fatal("trailing event data")
		}
	}
}
