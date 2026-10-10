package legion

import (
	"bytes"
	"testing"
)

// Current14069A4D0 maps body107..122 to managerD3..E2. The client uses
// each real party seat, including empty seats; compacting this array moves a
// role onto the wrong character. This is a field contract, not live acceptance.
func TestLegionInfoRoleProjectionPreservesPartySeatsAndOtherState(t *testing.T) {
	state := WaitingLegionInfoState()
	state.OperationChoice, state.Stage = 2, 4
	baseline := LegionInfo(ApocalypseContent, state)
	state.Roles = [4]uint32{1, 0, 2, 1}
	got := LegionInfo(ApocalypseContent, state)
	want := []byte{1, 0, 0, 0, 0, 0, 0, 0, 2, 0, 0, 0, 1, 0, 0, 0}
	if !bytes.Equal(got[109:125], want) {
		t.Fatalf("native role-seat view: %x", got[109:125])
	}
	copy(got[109:125], baseline[109:125])
	if !bytes.Equal(got, baseline) {
		t.Fatal("role projection overwrote difficulty, stage, destinations or shortcut state")
	}
}
