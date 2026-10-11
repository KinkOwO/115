package main

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

// Follow the native reader's variable-width cursor, rather than assuming
// every future record has the captured Ispins record's length.
func decodeOpenEventKeys(t *testing.T, body []byte) []uint16 {
	t.Helper()
	r := bytes.NewReader(body)
	read := func(v any) {
		if err := binary.Read(r, binary.LittleEndian, v); err != nil {
			t.Fatal(err)
		}
	}
	var count uint16
	read(&count)
	var ids []uint16
	for i := 0; i < int(count); i++ {
		var id uint16
		var present, flags [1]byte
		read(&id)
		read(&present)
		if present[0] != 1 {
			t.Fatal("missing complete open record")
		}
		read(&flags)
		read(&flags)
		str := func() {
			var n uint32
			read(&n)
			if uint64(n) > uint64(r.Len()) {
				t.Fatal("string crosses record body")
			}
			if _, err := io.CopyN(io.Discard, r, int64(n)); err != nil {
				t.Fatal(err)
			}
		}
		str()
		str()
		str()
		var start, end uint32
		read(&start)
		read(&end)
		if start >= end {
			t.Fatal("invalid availability window")
		}
		str()
		str()
		read(&flags)
		ids = append(ids, id)
	}
	// The captured table terminates with an empty external-data count.
	var external uint8
	read(&external)
	if external != 0 || r.Len() != 0 {
		t.Fatalf("invalid external tail: count=%d bytes=%d", external, r.Len())
	}
	return ids
}

func TestOfficialRaidGateTablePreservedAtEntry(t *testing.T) {
	t.Setenv("DFO_EVENT_INFO_VARIANT", "")
	before := append([]byte(nil), eventInfoTable...)
	body := raidChannelEventInfo(map[uint32]uint32{45: 45, 50: 50, 99: 99, 119: 119, 112: 112}, nil)
	keys := decodeOpenEventKeys(t, body)
	// 官方 24 行 + 构造的 516/10005 两行。
	if len(keys) != 26 {
		t.Fatalf("official records=%d", len(keys))
	}
	seen := map[uint16]bool{}
	for _, id := range keys {
		if seen[id] {
			t.Fatalf("duplicate gate %d", id)
		}
		seen[id] = true
	}
	for _, id := range []uint16{2449, 414, 776, 627, 641, 899, 1007} {
		if !seen[id] {
			t.Fatalf("missing native gate %d", id)
		}
	}
	frames := 0
	for _, p := range (entryPayloads{ChannelEventInfo: body}).packets() {
		if p.ID == 108 {
			frames++
			if p.Kind != 0 || len(p.Payload) < len(before) || !bytes.Equal(p.Payload[2:len(before)-1], before[2:len(before)-1]) {
				t.Fatal("official gate records altered or split")
			}
			mergedKeys := decodeOpenEventKeys(t, p.Payload)
			// 官方尾部 19..23 是 331/10017/10018/662/665；本仓在 331 之后插了构造的
			// 516/10005 两行 ⇒ 尾部整体后移两位。（见 event_info_activity_constructed.go）
			if len(mergedKeys) != 26 || mergedKeys[19] != 331 || mergedKeys[20] != 516 || mergedKeys[21] != 10005 || mergedKeys[22] != 10017 || mergedKeys[23] != 10018 ||
				mergedKeys[24] != 662 || mergedKeys[25] != 665 {
				t.Fatalf("boost activities replaced raid gates: %v", mergedKeys)
			}
		}
	}
	if frames != 1 || !bytes.Equal(eventInfoTable, before) {
		t.Fatal("duplicate frame or mutated gate table")
	}
}
