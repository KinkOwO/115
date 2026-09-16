package protocol

import (
	"encoding/hex"
	"testing"
)

func TestAreaUsersNativeReadContract(t *testing.T) {
	// Native 0x1452fc679/683/755: u32 town, u32 area, u16 count.
	// 0x1452fcfc9..d014: actor/x/y u16 and three u8 values.
	p, e := AreaUsers(38, 0, []AreaUser{{ActorServerID: 3, X: 561, Y: 234, Flags: [3]byte{0, 1, 1}}})
	if e != nil {
		t.Fatal(e)
	}
	if hex.EncodeToString(p) != "2600000000000000010003003102ea00000101" {
		t.Fatalf("native field mismatch: %x", p)
	}
	if _, e = AreaUsers(38, 0, []AreaUser{{ActorServerID: 3}, {ActorServerID: 3}}); e == nil {
		t.Fatal("duplicate scene actor accepted")
	}
}
