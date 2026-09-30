package main

import (
	"errors"
	"reflect"
	"testing"
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
