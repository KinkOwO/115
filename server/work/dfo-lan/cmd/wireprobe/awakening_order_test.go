package main

import "testing"

// id2177 is the client's switch point for enabling the VP system: any id29
// variation frame must land after it, or the client discards it and the panel
// reads 0. preparePackets skips empty payloads, so the plan is compared the
// same way.
func TestOrderAwakeningPacketsPlacesVariationAfterCompletion(t *testing.T) {
	restored := []outboundPacket{
		{"appearance_restored", 0, 2, []byte{0xAA}},
		{"skill_variation_response", 1, 29, []byte{0xBB}},
		{"avatar_restored", 0, 13, []byte{0xCC}},
	}
	plan := orderAwakeningPackets([]byte{0x01}, restored, []byte{0x02}, []byte{0x03})
	var ids []uint16
	for _, p := range plan {
		if len(p.Payload) == 0 {
			continue
		}
		ids = append(ids, p.ID)
	}
	want := []uint16{2, 2, 13, 19, 2758, 2177, 29}
	if len(ids) != len(want) {
		t.Fatalf("plan ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("plan ids = %v, want %v", ids, want)
		}
	}
}

// A stage-1/2 build carries no id29 (its payload is empty and preparePackets
// drops it); ordering must not fabricate a variation frame.
func TestOrderAwakeningPacketsWithoutVariation(t *testing.T) {
	restored := []outboundPacket{{"avatar_restored", 0, 13, []byte{0xCC}}}
	plan := orderAwakeningPackets([]byte{0x01}, restored, []byte{0x02}, nil)
	var ids []uint16
	for _, p := range plan {
		if len(p.Payload) == 0 {
			continue
		}
		ids = append(ids, p.ID)
	}
	want := []uint16{2, 13, 19, 2177}
	if len(ids) != len(want) {
		t.Fatalf("plan ids = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("plan ids = %v, want %v", ids, want)
		}
	}
}
