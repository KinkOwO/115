package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/world"
	"testing"
)

func TestPrevVillageReturnsToStampedOrigin(t *testing.T) {
	stamp := &database.WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}
	inside := database.WorldPosition{Town: 38, Area: 1, X: 550, Y: 340, Return: stamp}
	if _, err := previousVillageRequest(inside, []byte{1}, false, false); err == nil {
		t.Fatal("non-empty body accepted")
	}
	if _, err := previousVillageRequest(inside, nil, true, false); err == nil {
		t.Fatal("dungeon request accepted")
	}
	if _, err := previousVillageRequest(inside, nil, false, true); err == nil {
		t.Fatal("dungeon selection request accepted")
	}
	unstamped := inside
	unstamped.Return = nil
	if _, err := previousVillageRequest(unstamped, nil, false, false); err == nil {
		t.Fatal("missing return stamp accepted")
	}
	request, err := previousVillageRequest(inside, nil, false, false)
	if err != nil || request.Town != stamp.Town || request.Area != stamp.Area || request.X != stamp.X || request.Y != stamp.Y || request.PreviousTown != 38 || request.PreviousArea != 1 {
		t.Fatalf("incorrect previous village request: %+v %v", request, err)
	}
	if !observedGameRequest(1418) {
		t.Fatal("CMD1418 body is not retained for diagnosis")
	}
	w := &worldSession{service: &world.Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"41/1": {Town: 41, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{350, 150, 100, 100}}},
	}}, Rules: world.Rules{RequirePortalProximity: true}}, level: 1, state: database.WorldState{Position: inside}}
	out, err := w.areaTransition(request)
	if err != nil || out.Town != stamp.Town || out.Area != stamp.Area || out.X != stamp.X || out.Y != stamp.Y || out.Return != nil {
		t.Fatalf("previous village transition failed: %+v %v", out, err)
	}
}

// CMD 1418 uses an empty body and the saved Return stamp to identify the
// destination. The handler must preserve that stamp and reject unsafe contexts.
func TestPrevVillageUsesStampedOrigin(t *testing.T) {
	w := &worldSession{role: database.Character{ID: 7, WireID: 10}}
	w.state.Position = database.WorldPosition{
		Town: 38, Area: 1, X: 557, Y: 340,
		Return: &database.WorldReturn{Town: 41, Area: 1, X: 404, Y: 197},
	}
	r, err := w.prevVillage(nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Town != 41 || r.Area != 1 || r.X != 404 || r.Y != 197 {
		t.Fatalf("destination is not the stamped origin: %+v", r)
	}
	if r.PreviousTown != 38 || r.PreviousArea != 1 {
		t.Fatalf("source area is not the room the character stands in: %+v", r)
	}
}

func TestPrevVillageRefusesWithoutEvidence(t *testing.T) {
	stamped := func() *worldSession {
		w := &worldSession{role: database.Character{ID: 7, WireID: 10}}
		w.state.Position = database.WorldPosition{
			Town: 38, Area: 1, X: 557, Y: 340,
			Return: &database.WorldReturn{Town: 41, Area: 1, X: 404, Y: 197},
		}
		return w
	}

	w := stamped()
	if _, err := w.prevVillage([]byte{0}); err == nil {
		t.Fatal("non-empty body accepted")
	}

	w = &worldSession{role: database.Character{ID: 7, WireID: 10}}
	w.state.Position = database.WorldPosition{Town: 38, Area: 1, X: 557, Y: 340}
	if _, err := w.prevVillage(nil); err == nil {
		t.Fatal("prev village without a return stamp accepted")
	}

	w = stamped()
	w.activeDungeon = &dungeon.Session{}
	if _, err := w.prevVillage(nil); err == nil {
		t.Fatal("prev village inside a dungeon accepted")
	}
}
