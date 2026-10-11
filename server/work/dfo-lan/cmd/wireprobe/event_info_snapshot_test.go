package main

import (
	"bytes"
	"testing"
)

func TestEventSnapshotRetainsChallengeAcrossEntryAndReturn(t *testing.T) {
	t.Setenv("DFO_EVENT_INFO_VARIANT", "")
	for _, challenge := range []bool{false, true} {
		body, ok := buildTownEventInfoTable(challenge)
		if !ok {
			t.Fatal("snapshot construction")
		}
		runtime := &gatewayRuntime{boostEventInfo: body}
		selection := eventInfoSnapshot(runtime)
		returned := eventInfoSnapshot(runtime)
		var entry []byte
		frames := 0
		for _, p := range (entryPayloads{ChannelEventInfo: selection}).packets() {
			if p.ID == 108 {
				entry, frames = p.Payload, frames+1
			}
		}
		if frames != 1 || !bytes.Equal(selection, entry) || !bytes.Equal(entry, returned) {
			t.Fatal("selection, entry and return diverged")
		}
		ids := decodeOpenEventKeys(t, entry)
		// 官方切片 23 行 + 本仓构造的两行活动行（战令 516/10005，见
		// event_info_activity_constructed.go）；challenge 行另计一行。
		want := 25
		if challenge {
			want++
		}
		if len(ids) != want {
			t.Fatalf("challenge=%t rows=%d want=%d", challenge, len(ids), want)
		}
		seen := map[uint16]bool{}
		for _, id := range ids {
			if seen[id] {
				t.Fatalf("duplicate activity %d", id)
			}
			seen[id] = true
		}
		for _, id := range []uint16{2449, 414, 776, 627, 641, 899, 1007, 10017, 10018, 662} {
			if !seen[id] {
				t.Fatalf("lost gate or activity %d", id)
			}
		}
		if seen[665] != challenge {
			t.Fatal("challenge activity lost or enabled without authorization")
		}
		selection[2] ^= 1
		entry[3] ^= 1
		returned[4] ^= 1
		if !bytes.Equal(runtime.boostEventInfo, body) || !bytes.Equal(eventInfoSnapshot(runtime), body) {
			t.Fatal("caller mutated the owned snapshot")
		}
	}
}

func TestEventSnapshotDiagnosticVariantsAreConsistent(t *testing.T) {
	challenge, ok := buildTownEventInfoTable(true)
	if !ok {
		t.Fatal("challenge snapshot")
	}
	runtime := &gatewayRuntime{boostEventInfo: challenge}
	for _, tc := range []struct {
		variant string
		body    []byte
	}{{"plain", eventInfoTable}, {"legacy", eventInfoTableLegacy}} {
		t.Setenv("DFO_EVENT_INFO_VARIANT", tc.variant)
		selection := eventInfoSnapshot(runtime)
		if !bytes.Equal(selection, tc.body) || !bytes.Equal(selection, (entryPayloads{ChannelEventInfo: selection}).eventInfoSnapshot()) || !bytes.Equal(selection, raidChannelEventInfo(nil, nil)) {
			t.Fatalf("%s variant differs between producers", tc.variant)
		}
	}
}
