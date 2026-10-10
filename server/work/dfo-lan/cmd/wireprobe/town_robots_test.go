package main

import (
	"bytes"
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/world"
	"fmt"
	"sync"
	"testing"
)

func townRobotWorld(t *testing.T, n int) (*worldSession, *townRobotFleet) {
	t.Helper()
	c, err := catalog.LoadCharacters("../../configs/characters.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	h := newLanHub()
	p := newTestPeer(1, 3)
	h.add(p)
	h.publish(p, 1, 38, 0, 500, 250, false)
	f := &townRobotFleet{hub: h, target: n, stop: make(chan struct{}), done: make(chan struct{})}
	h.townRobots = f
	w := &worldSession{hub: h, peer: p, role: database.Character{ID: 1}, characters: &character.Service{Catalog: c}, service: &world.Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		catalog.AreaKey(38, 0): {Town: 38, Area: 0, Walkable: [][4]int32{{100, 200, 800, 100}}},
	}}}, state: database.WorldState{Position: database.WorldPosition{Town: 38, Area: 0, X: 500, Y: 250}}}
	return w, f
}

func TestTownRobotPopulationLifecycle(t *testing.T) {
	for _, n := range []int{1, 5, 20, 50} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			w, f := townRobotWorld(t, n)
			var sent []capture
			w.peer.send = func(_ byte, id uint16, b []byte) error { sent = append(sent, capture{id, b}); return nil }
			w.ensureTownRobots()
			w.ensureTownRobots()
			if len(f.bots) != n || len(w.hub.roster(w.peer)) != n+1 {
				t.Fatalf("duplicate/missing bots: %d", len(f.bots))
			}
			if len(sent) != 0 {
				t.Fatal("sent before entering player's entry plan")
			}
			for _, b := range f.bots {
				if b.peer.roleID >= 0 || b.peer.send != nil || !b.peer.robot || len(b.peer.info) == 0 || len(b.peer.addition) == 0 {
					t.Fatal("temporary identity escaped")
				}
			}
			for range 12 {
				f.tick()
			}
			if countCapture(sent, 22) == 0 {
				t.Fatal("no movement projection")
			}
			for _, b := range f.bots {
				if !world.Contains(b.rect, b.peer.x, b.peer.y, 0) {
					t.Fatal("robot left source rectangle")
				}
			}
			w.hub.retire(w.peer)
			f.tick()
			if len(f.bots) != 0 || len(w.hub.peers) != 1 {
				t.Fatal("empty scene leaked robots")
			}
		})
	}
}

func TestTownRobotSceneIsolationAndRealIdentityWins(t *testing.T) {
	w, f := townRobotWorld(t, 5)
	w.ensureTownRobots()
	for _, spec := range []struct {
		channel, town, area uint32
		private             bool
	}{{2, 38, 0, false}, {1, 38, 1, false}, {1, 38, 0, true}} {
		p := newTestPeer(2, 4)
		w.hub.add(p)
		w.hub.publish(p, spec.channel, spec.town, spec.area, 500, 250, spec.private)
		if len(w.hub.shared(p)) != 0 {
			t.Fatal("robots visible across scene boundary")
		}
		w.hub.depart(p)
	}
	bot := f.bots[0].peer
	var out []capture
	observer := allPeer(3, 6, &out)
	w.hub.add(observer)
	w.hub.publish(observer, 1, 38, 0, 500, 250, false)
	real := newTestPeer(4, bot.actorID)
	w.hub.add(real)
	if _, ok := w.hub.peers[bot]; ok || bot.placed || countCapture(out, 6) != 1 {
		t.Fatal("real identity was blocked or left a ghost")
	}
	f.tick()
	if len(f.bots) != 4 {
		t.Fatal("evicted robot not pruned")
	}
	w.ensureTownRobots()
	if len(f.bots) != 5 {
		t.Fatal("fleet did not refill")
	}
	for _, b := range f.bots {
		if b.peer.actorID == real.actorID {
			t.Fatal("allocated unplaced player's identity")
		}
	}
}

func TestTownRobotGeometryAndScope(t *testing.T) {
	for _, rect := range [][4]int32{{-50, -20, 40, 10}, {100, 200, 1, 1}, {-40000, -40000, 50000, 50000}} {
		r, ok := robotRectangle(catalog.WorldArea{Walkable: [][4]int32{rect}})
		if !ok {
			t.Fatal("legal source rectangle rejected")
		}
		for seed := uint32(0); seed < 200; seed++ {
			x, y := robotPoint(r, seed)
			if !world.Contains(rect, x, y, 0) {
				t.Fatalf("coordinate outside source: %v %d %d", rect, x, y)
			}
		}
	}
	for _, name := range []string{"private", "isolated", "dungeon", "missing geometry"} {
		t.Run(name, func(t *testing.T) {
			w, f := townRobotWorld(t, 5)
			switch name {
			case "private":
				a := w.service.Catalog.Areas["38/0"]
				a.SeriaReturnWarp = true
				w.service.Catalog.Areas["38/0"] = a
			case "isolated":
				w.channelWorldIsolated = true
			case "dungeon":
				w.activeDungeon = &dungeon.Session{}
				w.hub.retire(w.peer)
			case "missing geometry":
				delete(w.service.Catalog.Areas, "38/0")
			}
			w.ensureTownRobots()
			if len(f.bots) != 0 {
				t.Fatal("unsupported scene populated")
			}
		})
	}
}

func TestTownRobotConcurrentEnsureAndShutdown(t *testing.T) {
	w, f := townRobotWorld(t, 20)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() { defer wg.Done(); w.ensureTownRobots() }()
	}
	wg.Wait()
	if len(f.bots) != 20 {
		t.Fatal("concurrent population duplicated")
	}
	// close is idempotent and removes only its own transient actors.
	close(f.done)
	f.close()
	f.close()
	if len(w.hub.peers) != 1 {
		t.Fatal("shutdown touched real peer or leaked bots")
	}
	w.ensureTownRobots()
	if len(f.bots) != 0 {
		t.Fatal("closed fleet restarted")
	}
}

func TestTownRobotProjectionDoesNotTouchPersistence(t *testing.T) {
	w, _ := townRobotWorld(t, 1)
	w.characters.ProfileFrames = func(_ context.Context, _ character.Character) ([2]uint32, error) {
		t.Fatal("accessed account profile")
		return [2]uint32{}, nil
	}
	before := append([]byte(nil), w.role.State...)
	w.ensureTownRobots()
	if len(w.hub.townRobots.bots) != 1 || !bytes.Equal(before, w.role.State) {
		t.Fatal("projection changed player's role")
	}
}

func TestTownRobotCountValidation(t *testing.T) {
	for _, raw := range []string{"", "0", "1", "20", "50", "69", "100"} {
		if _, err := townRobotCount(raw); err != nil {
			t.Fatal(raw, err)
		}
	}
	for _, raw := range []string{"-1", "101", "all", "1.5"} {
		if _, err := townRobotCount(raw); err == nil {
			t.Fatal("unbounded population", raw)
		}
	}
}

func TestTownRobotDungeonReturnIntroducesActorsBeforeAreaList(t *testing.T) {
	w, f := townRobotWorld(t, 5)
	role, err := w.characters.TownDisplayCharacter(w.peer.actorID, "Player", 0, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	role.ID = 1
	role.AccountID = 1
	w.role = role
	w.ensureTownRobots()
	w.leaveScene()
	f.tick()
	if len(f.bots) != 0 {
		t.Fatal("empty origin did not clear")
	}
	w.activeDungeon = &dungeon.Session{}
	plan, err := w.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	if len(f.bots) != 5 {
		t.Fatal("pending dungeon pointer prevented return population")
	}
	introduced := 0
	for _, packet := range plan {
		if packet.Name == "town_return_peer_info" {
			introduced++
		}
		if packet.ID == 24 && introduced != 5 {
			t.Fatal("area list preceded robot identity", introduced)
		}
	}
	if introduced != 5 {
		t.Fatal("return did not introduce current town actors")
	}
}
