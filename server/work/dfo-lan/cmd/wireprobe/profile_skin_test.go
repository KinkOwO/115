package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/profileskin"
	"testing"
)

func TestProfileSkinEntryOrder(t *testing.T) {
	cargo, selected, err := protocol.ProfileSkinRestore(profileskin.Defaults())
	if err != nil {
		t.Fatal(err)
	}
	p := entryPayloads{ProfileSkinCargo: cargo, ProfileSkinSelection: selected}
	owned, choice, world := -1, -1, -1
	for i, packet := range p.packets() {
		if packet.ID == 1545 && packet.Kind == 0 {
			owned = i
		}
		if packet.ID == 1546 && packet.Kind == 0 {
			choice = i
		}
		if packet.ID == 124 && packet.Kind == 0 {
			world = i
		}
	}
	if owned < 0 || choice != owned+1 || world <= choice {
		t.Fatalf("invalid initialization order: %d %d %d", owned, choice, world)
	}
}
