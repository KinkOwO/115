package main

import "testing"

func TestBorderGradePrecedesClientLoadedScripts(t *testing.T) {
	plan := []outboundPacket{{Name: "ack", ID: 37}, {Name: "state", ID: 3}, {Name: "loaded", ID: 30}, {Name: "oath", ID: 2838}}
	border := []outboundPacket{{Name: "border", ID: 2756}}
	got := insertBorderBeforeLoaded(plan, border)
	for i, want := range []uint16{37, 3, 2756, 30, 2838} {
		if got[i].ID != want {
			t.Fatal("Border announced after ACT started")
		}
	}
	if len(plan) != 4 || plan[2].ID != 30 {
		t.Fatal("input plan mutated")
	}
	got = insertBorderBeforeLoaded(plan, nil)
	for i, p := range plan {
		if got[i].ID != p.ID {
			t.Fatal("other dungeon packet order changed")
		}
	}
}
