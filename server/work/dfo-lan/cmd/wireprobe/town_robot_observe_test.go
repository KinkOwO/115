package main

import "testing"

func TestRobotUDPObservationIsOptInBoundedAndEpochScoped(t *testing.T) {
	var events []map[string]any
	tr := &townRobotTransport{event: func(r map[string]any) { events = append(events, r) }}
	peer := &townRobotUDP{}
	tr.observeUDP(peer, 1, []byte{2, 0, 3})
	if len(events) != 0 {
		t.Fatal("observation must be opt-in")
	}
	t.Setenv("DFO_TOWN_ROBOT_UDP_OBSERVE", "r12-observe1")
	tr.observeUDP(peer, 1, nil)
	tr.observeUDP(peer, 1, make([]byte, 4097))
	if len(events) != 0 {
		t.Fatal("unbounded or empty record")
	}
	body := []byte{2, 0, 3}
	for i := 0; i < 30; i++ {
		tr.observeUDP(peer, 1, body)
	}
	body[0] = 99
	if len(events) != 24 || events[0]["cipher_hex"] != "020003" {
		t.Fatal("epoch budget or retained receive buffer")
	}
	for epoch := 0; epoch < 8; epoch++ {
		room := [2]uint16{uint16(epoch), 0}
		tr.setRoom(&room)
		for i := 0; i < 30; i++ {
			tr.observeUDP(peer, 1, body)
		}
	}
	if len(events) != 96 || peer.observeTotal != 96 {
		t.Fatal("session record budget")
	}
	before := len(events)
	tr.closed = true
	tr.observeUDP(peer, 1, body)
	if len(events) != before {
		t.Fatal("closed endpoint observed")
	}
}
