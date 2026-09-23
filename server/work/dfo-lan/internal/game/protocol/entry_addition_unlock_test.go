package protocol

import (
	"bytes"
	"testing"
)

// The extended-slot unlock byte sits between the packed stats and the
// equipment block, at payload offset 360 (native 14563d692). It used to be a
// hardcoded zero, which is why an armoury could stay locked after the slot was
// already saved. Opening a slot must change that byte and nothing else.
func TestAdditionCarriesExpandedSlotUnlockByte(t *testing.T) {
	s := EntryAdditionProbe{ActorServerID: 3, Experience: 7, Stats: PackedEntryStats{HP: 5200, MP: 4800, BasePercent: 100}}
	locked, e := UserInfoAdditionProbe(s)
	if e != nil {
		t.Fatal(e)
	}
	if locked[360] != 0 {
		t.Fatalf("unlock byte with nothing opened: %d", locked[360])
	}
	for _, flags := range []byte{1, 2, 16, 1 | 2, 1 | 2 | 16} {
		s.ExpandEquipFlags = flags
		got, e := UserInfoAdditionProbe(s)
		if e != nil {
			t.Fatal(e)
		}
		if len(got) != len(locked) {
			t.Fatalf("flags %d changed the payload length: %d vs %d", flags, len(got), len(locked))
		}
		if got[360] != flags {
			t.Fatalf("unlock byte %d, want %d", got[360], flags)
		}
		if !bytes.Equal(got[:360], locked[:360]) || !bytes.Equal(got[361:], locked[361:]) {
			t.Fatalf("flags %d shifted a neighbouring field", flags)
		}
	}
}
