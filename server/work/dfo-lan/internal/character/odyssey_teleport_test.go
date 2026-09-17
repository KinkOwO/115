package character

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestOdysseyJournalTeleportCapturedRequest(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	b, _ := hex.DecodeString("280000000400000081032a01052600000001000000000000")
	r, err := protocol.DecodeAreaChangeRequest(b)
	if err != nil || r.Town != 40 || r.Area != 4 || r.X != 897 || r.Y != 298 {
		t.Fatal(r, err)
	}
	if s.OdysseyJournalTeleport(role, r) {
		t.Fatal("locked node accepted")
	}
	for _, id := range []uint32{100004934, 100004935, 100004936} {
		role.State, err = s.saveOdysseyCompletion(role, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !s.OdysseyJournalTeleport(role, r) {
		t.Fatal("confirmed prior node rejected")
	}
	for _, mutate := range []func(*protocol.AreaChangeRequest){func(r *protocol.AreaChangeRequest) { r.X++ }, func(r *protocol.AreaChangeRequest) { r.Town++ }, func(r *protocol.AreaChangeRequest) { r.Flag = 0 }, func(r *protocol.AreaChangeRequest) { r.TailFlags[0] = 1 }} {
		bad := r
		mutate(&bad)
		if s.OdysseyJournalTeleport(role, bad) {
			t.Fatal("altered journal target accepted", bad)
		}
	}
	role.Request = nil
	if s.OdysseyJournalTeleport(role, r) {
		t.Fatal("ordinary role admitted")
	}
}

func TestOdysseyJournalSourceIDs(t *testing.T) {
	s, _ := odysseyGrowthFixture(t)
	var nodes []odysseyJournalNode
	if err := json.Unmarshal(odysseyJournalRoutes, &nodes); err != nil || len(nodes) != 29 {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	for _, node := range nodes {
		for _, id := range node.Dungeons {
			if seen[id] || s.Odyssey.ClearLevels[id] == 0 {
				t.Fatal("journal does not match source progression", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 50 {
		t.Fatal("missing source journal nodes")
	}
}
