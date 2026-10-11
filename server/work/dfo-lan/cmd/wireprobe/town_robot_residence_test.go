package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/world"
	"testing"
)

func TestTownRobotResidenceIsUniqueDistributedAndSourceBounded(t *testing.T) {
	c := catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{}}
	for town := uint32(1); town <= 10; town++ {
		for area := uint32(0); area < 3; area++ {
			c.Areas[catalog.AreaKey(town, area)] = catalog.WorldArea{Town: town, Area: area, Kind: "[normal]", Walkable: [][4]int32{{0, 0, 900, 500}}}
		}
	}
	c.Areas["11/0"] = catalog.WorldArea{Town: 11, SeriaReturnWarp: true, Walkable: [][4]int32{{0, 0, 900, 500}}}
	c.Areas["12/0"] = catalog.WorldArea{Town: 12, MinimumLevel: 116, Walkable: [][4]int32{{0, 0, 900, 500}}}
	c.Areas["13/0"] = catalog.WorldArea{Town: 13, Kind: "[dungeon gate]", Walkable: [][4]int32{{0, 0, 900, 500}}}
	seen := map[int]bool{}
	for town := uint32(1); town <= 13; town++ {
		for area := uint32(0); area < 3; area++ {
			residents := robotResidentIDs(c, 69, town, area)
			if len(residents) > 7 {
				t.Fatal("one town monopolized roster")
			}
			if area != 0 {
				continue
			} // Other public streets share the same town IDs.
			for _, id := range residents {
				if seen[id] {
					t.Fatal("duplicate career", id)
				}
				seen[id] = true
				if town > 10 || area != 0 {
					t.Fatal("nonpublic home")
				}
			}
		}
	}
	if len(seen) != 69 {
		t.Fatal("career omitted", len(seen))
	}
}

func TestTownRobotPartySurvivesTownTravelAndPrivateRoom(t *testing.T) {
	c, f, _, bot := townPartyFixture(t)
	if c.dispatchTownRobotParty(townInviteRequest()) != dispatchHandled {
		t.Fatal("invite")
	}
	w := c.worldState
	p := f.parties[w.peer]
	transport := p.transport
	stored := bytes.Clone(f.manager.records[1].Role.State)
	w.service = &world.Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"39/0": {Town: 39, Walkable: [][4]int32{{0, 0, 900, 500}}},
		"39/1": {Town: 39, Area: 1, SeriaReturnWarp: true, Walkable: [][4]int32{{0, 0, 900, 500}}},
	}}}
	// In-flight retirement must not be interpreted as member departure.
	f.hub.retire(w.peer)
	f.tick()
	if f.parties[w.peer] != p {
		t.Fatal("in-flight travel dissolved party")
	}
	for _, area := range []uint32{0, 1, 0} {
		w.state.Position = database.WorldPosition{Town: 39, Area: area, X: 450, Y: 250}
		w.enterArea()
		f.tick()
		if f.parties[w.peer] != p || p.transport != transport || len(p.bots) != 1 || p.bots[0] != bot {
			t.Fatal("travel replaced membership or endpoints")
		}
		if area == 1 {
			if bot.peer.placed {
				t.Fatal("private room published teammate")
			}
		} else if !bot.peer.placed || bot.peer.town != 39 || bot.peer.area != 0 {
			t.Fatal("teammate did not follow scene")
		}
		if !bytes.Equal(stored, f.manager.records[1].Role.State) {
			t.Fatal("travel changed owned gear")
		}
	}
	w.departArea()
	f.tick()
	if f.parties[w.peer] != nil {
		t.Fatal("account departure retained party")
	}
}
