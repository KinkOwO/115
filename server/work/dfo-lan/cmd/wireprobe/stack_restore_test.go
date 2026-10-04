package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"testing"
)

func TestStackMoveRestoreNeverAcquires(t *testing.T) {
	role := database.Character{State: json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","items":[{"slot":76,"Template":14,"Amount":5}]}}`)}
	r := protocol.ItemMoveRequest{SourceSlot: 76, DestinationSlot: 68, DestinationItem: 14, Selection: 0xffffffff}
	for _, applied := range []bool{true, false} {
		plan, e := stackMovePackets(role, r, applied)
		if e != nil {
			t.Fatal(e)
		}
		want := 1
		if applied {
			want = 2
		}
		if len(plan) != want {
			t.Fatal("replay repeats ACK", plan)
		}
		for _, p := range plan {
			if p.ID == 14 {
				t.Fatal("movement emits acquisition packet")
			}
		}
		p := plan[len(plan)-1]
		if p.ID != 13 || p.Payload[0] != 0 || p.Payload[1] != 0 || p.Payload[2] != 0 {
			t.Fatal("not full bag replacement")
		}
	}
}
