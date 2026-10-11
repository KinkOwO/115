package protocol

import (
	"encoding/binary"
	"testing"
)

func TestGoldReinforcementDestructionUsesNativeBranch(t *testing.T) {
	for _, space := range []byte{0, 3} {
		r := ReinforcementRequest{Mode: 0, EquipmentSpace: space, EquipmentSlot: 12, TicketSlot: 78}
		body, err := ReinforcementGoldReply(r, 99, 12, 12, 3)
		if err != nil {
			t.Fatal(err)
		}
		if len(body) != 36 || body[1] != 10 || body[12] != 3 || body[14] != space || binary.LittleEndian.Uint16(body[15:]) != 12 || body[17] != 0 || binary.LittleEndian.Uint16(body[18:]) != 0xffff {
			t.Fatalf("invalid native destruction: %x", body)
		}
	}
}
