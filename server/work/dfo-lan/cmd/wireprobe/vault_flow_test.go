package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"encoding/binary"
	"encoding/json"
	"testing"
)

func TestVaultMovePacketsClearSourceAndRestoreZeroSlot(t *testing.T) {
	role := storage.Character{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","gold":0,"items":[]}}`)}
	v := storage.VaultState{Slots: 8, Items: json.RawMessage(`[{"slot":0,"Template":15,"Amount":10}]`)}
	r := protocol.ItemMoveRequest{DestinationList: 2, SourceSlot: 65, SourceItem: 15, Count: 10, Selection: 0xffffffff}
	plan, e := vaultMovePackets(role, v, r)
	if e != nil {
		t.Fatal(e)
	}
	if len(plan) != 3 || plan[0].ID != 19 || plan[1].ID != 13 || plan[2].ID != 13 {
		t.Fatal(plan)
	}
	if binary.LittleEndian.Uint32(plan[0].Payload[4:]) != 10 {
		t.Fatal("ACK count differs from transfer")
	}
	if plan[1].Payload[0] != 0 || plan[2].Payload[0] != 2 || len(plan[2].Payload) != 186 {
		t.Fatal("restore spaces/row size")
	}
	if _, e = (&worldSession{}).moveVault("fixture", r); e == nil {
		t.Fatal("unselected character")
	}
}
