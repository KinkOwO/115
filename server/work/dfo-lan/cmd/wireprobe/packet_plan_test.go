package main

import (
	"errors"
	"reflect"
	"testing"

	"dfolan/internal/game/wire"
)

func TestSendPacketPlanStopsBeforeLoggingFailedPacket(t *testing.T) {
	plan := []outboundPacket{{"first", 1, 80, []byte{1}}, {"second", 0, 14, []byte{2}}, {"third", 0, 14, []byte{3}}}
	failure := errors.New("write failed")
	var sent, logged []uint16
	err := sendPacketPlan(plan, func(kind byte, id uint16, payload []byte) error {
		sent = append(sent, id)
		if !reflect.DeepEqual(plan[len(sent)-1], outboundPacket{plan[len(sent)-1].Name, kind, id, payload}) {
			t.Fatal("packet changed during dispatch")
		}
		if len(sent) == 2 {
			return failure
		}
		return nil
	}, func(packet outboundPacket) { logged = append(logged, packet.ID) })
	if !errors.Is(err, failure) || !reflect.DeepEqual(sent, []uint16{80, 14}) || !reflect.DeepEqual(logged, []uint16{80}) {
		t.Fatalf("err=%v sent=%v logged=%v", err, sent, logged)
	}
}

func TestSendPacketPlanWithoutLogging(t *testing.T) {
	var sent []byte
	err := sendPacketPlan([]outboundPacket{{Payload: []byte{1}}, {Payload: []byte{2}}}, func(_ byte, _ uint16, payload []byte) error {
		sent = append(sent, payload...)
		return nil
	}, nil)
	if err != nil || !reflect.DeepEqual(sent, []byte{1, 2}) {
		t.Fatalf("err=%v sent=%v", err, sent)
	}
}

// TestPreparePacketsEmptyBodyDelivery guards the Ispins stage-settlement N1658
// frame (next79 §25): official s4 frame 493 is a bare 16-byte s2c header with
// an empty body and checksum 0x18. preparePackets used to drop every empty
// payload silently, so the client never received the packet the official
// chain delivers between N2255 and N115. A non-nil zero-length slice now means
// "genuine empty frame"; nil stays a skipped placeholder.
func TestPreparePacketsEmptyBodyDelivery(t *testing.T) {
	keys := make([]byte, wire.SessionKeyBytes)
	prepared, err := preparePackets(keys, []outboundPacket{
		{"ispins_req_dungeon_clear_info", 0, 1658, []byte{}},
		{"placeholder_skipped", 0, 20, nil},
	})
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if len(prepared) != 1 || prepared[0].ID != 1658 {
		t.Fatalf("prepared=%d want exactly the 1658 frame", len(prepared))
	}
	raw := prepared[0].Raw
	if len(raw) != 16 {
		t.Fatalf("frame length=%d want 16 (header only)", len(raw))
	}
	if raw[0] != 0 || raw[1] != 0x7a || raw[2] != 0x06 {
		t.Fatalf("kind/id bytes=% x want 00 7a 06", raw[:3])
	}
	if raw[11] != 0x18 {
		t.Fatalf("checksum=%#x want 0x18 (official s4 frame 493)", raw[11])
	}
	for i, b := range raw[12:16] {
		if b != 0 {
			t.Fatalf("trailing header byte %d=%#x want 0", i+12, b)
		}
	}
}
