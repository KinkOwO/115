package main

import (
	"dfolan/internal/game/protocol"
	"testing"
)

func TestEquipmentAppearanceRebuildDeferredDuringDungeon(t *testing.T) {
	cases := []struct {
		name      string
		inDungeon bool
		request   protocol.ItemMoveRequest
		want      bool
	}{
		{name: "town worn move", request: protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, want: true},
		{name: "dungeon worn move", inDungeon: true, request: protocol.ItemMoveRequest{SourceList: 1, DestinationList: 3}, want: false},
		{name: "town bag move", request: protocol.ItemMoveRequest{SourceList: 1, DestinationList: 1}, want: false},
		{name: "dungeon bag move", inDungeon: true, request: protocol.ItemMoveRequest{SourceList: 1, DestinationList: 1}, want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldSendEquipmentAppearanceRebuild(tc.inDungeon, tc.request); got != tc.want {
				t.Fatalf("shouldSendEquipmentAppearanceRebuild() = %v, want %v", got, tc.want)
			}
		})
	}
}
