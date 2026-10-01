package character

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"

	"errors"
	"testing"
)

func TestNativeOdysseyRoutesAvoidEmbeddedReadForEveryProgressPrefix(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	source, err := EmbeddedOdysseyJournalRoutes()
	if err != nil {
		t.Fatal(err)
	}
	direct := *s
	direct.JournalRoutes, err = catalog.NewOdysseyJournalRoutes(*source)
	if err != nil {
		t.Fatal(err)
	}
	type query struct {
		role    Character
		request protocol.AreaChangeRequest
		want    bool
	}
	queries := []query{}
	addQueries := func() {
		for _, node := range source.Nodes {
			for _, offset := range []uint16{0, 17} {
				r := protocol.AreaChangeRequest{Town: node.Destination[0], Area: node.Destination[1], X: uint16(node.Destination[2]) + offset, Y: uint16(node.Destination[3]) + offset, Flag: 5, TailFlags: [2]byte{0, 2}}
				queries = append(queries, query{role, r, s.OdysseyJournalTeleport(role, r)})
			}
		}
	}
	addQueries()
	for _, node := range source.Nodes {
		for _, id := range node.Dungeons {
			role.State, err = s.saveOdysseyCompletion(role, id)
			if err != nil {
				t.Fatal(err)
			}
			addQueries()
		}
	}
	original := loadEmbeddedOdysseyJournalRoutes
	loadEmbeddedOdysseyJournalRoutes = func() (*catalog.OdysseyJournalRoutes, error) { return nil, errors.New("embedded routes unavailable") }
	defer func() { loadEmbeddedOdysseyJournalRoutes = original }()
	for _, q := range queries {
		if direct.OdysseyJournalTeleport(q.role, q.request) != q.want {
			t.Fatal("native route eligibility changed", q.request)
		}
	}
	r := queries[0].request
	r.TailFlags = [2]byte{5, 0}
	if direct.OdysseyJournalTeleport(role, r) {
		t.Fatal("map-selector tail admitted")
	}
	role.ConfigVersion = "other-source"
	if direct.OdysseyJournalTeleport(role, queries[0].request) {
		t.Fatal("cross-source character admitted")
	}
	t.Log("native eligibility checks", len(queries))
}
