package quest

import (
	"dfolan/internal/catalog"
	"reflect"
	"testing"
)

// The Silent City (寂静城) faction choice is a [collision quest] trio: quests
// 3923/3924/3925 are the Luke_01_01/02/03 branches and must be mutually
// exclusive, or a character can pick a faction three times and re-clear the
// same maps on every branch.
func TestCollisionQuestsParsedFromSource(t *testing.T) {
	c, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	x := BuildIndex(c)
	want := map[uint32][]uint32{
		3923: {3924, 3925},
		3924: {3923, 3925},
		3925: {3923, 3924},
	}
	for id, collisions := range want {
		e := x.Entries[id]
		if e == nil || !reflect.DeepEqual(e.Collisions, collisions) {
			t.Fatalf("quest %d collision set changed: %+v", id, e)
		}
	}
}

func TestCollisionsBlocked(t *testing.T) {
	if collisionsBlocked(nil, nil) {
		t.Fatal("empty collision set must never block")
	}
	status := map[uint32]string{3923: "accepted"}
	if !collisionsBlocked([]uint32{3923, 3924}, status) {
		t.Fatal("accepted peer must block the sibling branch")
	}
	if collisionsBlocked([]uint32{3924, 3925}, status) {
		t.Fatal("unrelated peer must not block")
	}
	if collisionsBlocked([]uint32{3923}, map[uint32]string{}) {
		t.Fatal("peer without character state must not block the choice")
	}
	status = map[uint32]string{3923: "completed"}
	if !collisionsBlocked([]uint32{3923}, status) {
		t.Fatal("completed peer must block re-selection")
	}
}
