package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/raid"
	"encoding/binary"
	"os"
	"testing"
	"time"
)

func TestBakalNativeRoadLocationAndConnectedPortals(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF archive required")
	}
	a, err := catalog.OpenTestArchiveCached(path, "")
	if err != nil {
		t.Fatal(err)
	}
	dir, err := catalog.ImportChannelDirectory(a)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := catalog.ImportRaidEntrances(a, &dir)
	if err != nil {
		t.Fatal(err)
	}
	rules := entries[82]
	var ids []uint32
	for _, d := range rules.Bakal.Dungeons {
		ids = append(ids, d.ID)
	}
	c, err := catalog.ImportDungeons(a, ids)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	opening, err := raid.PrepareBakalOpening(rules, []raid.Member{{Actor: 2, Position: 1}}, now.Add(-5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = opening.Activate(now); err != nil {
		t.Fatal(err)
	}
	s, err := dungeon.Select(c, protocol.DungeonSelection{ID: 100003160, Party: 65535}, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	w := &worldSession{role: database.Character{ID: 2, WireID: 2, State: []byte(`{"level":115}`)}, level: 115, channelType: 82, raidWaiting: true, soloPartyReady: true, bakalParty: 1, bakalTown: 152, bakalOpening: opening, bakalRules: rules.Bakal, dungeons: &c, activeDungeon: s, state: database.WorldState{Position: database.WorldPosition{Town: 152, Area: 2}}}
	plan, err := w.bakalRoomPartyPlan(s)
	if err != nil || len(plan) != 1 || plan[0].ID != 2285 || binary.LittleEndian.Uint32(plan[0].Payload[5:]) == 52 {
		t.Fatal("road retained camp location", plan, err)
	}
	for _, m := range s.Monsters {
		if !m.NonCombat {
			if _, err = s.ConfirmDeath(uint32(m.Entity), 2, 2); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !s.RoomCleared() {
		t.Fatal("native road not cleared")
	}
	objectsAudited := 0
	for edge, objects := range rules.Bakal.PortalObjects {
		for _, objectPath := range objects {
			objectsAudited++
			id := edge[1]
			var sourceGrid [2]int32
			found := false
			for _, room := range c.Dungeons[edge[0]].Mazes[0].Rooms {
				script, err := c.MapScript(room.Map)
				if err != nil {
					t.Fatal(err)
				}
				if bakalPortalInRoom(script, objectPath) {
					sourceGrid = [2]int32{int32(room.X), int32(room.Y)}
					found = true
					break
				}
			}
			if !found {
				t.Fatal("source warp object has no maze room", edge, objectPath)
			}
			sourceCatalog, err := w.bakalPortalCatalog(protocol.DungeonSelection{ID: edge[0], Party: 65535}, sourceGrid)
			if err != nil {
				t.Fatal(err)
			}
			source, err := dungeon.Select(*sourceCatalog, protocol.DungeonSelection{ID: edge[0], Party: 65535}, 115, nil)
			if err != nil {
				t.Fatal(err)
			}
			source.Loaded = true
			for _, actor := range source.Monsters {
				if !actor.NonCombat {
					if _, err := source.ConfirmDeath(uint32(actor.Entity), 2, 2); err != nil {
						t.Fatal(err)
					}
				}
			}
			w.activeDungeon = source
			body := make([]byte, 48)
			binary.LittleEndian.PutUint32(body[13:], id)
			grid := c.Dungeons[id].Mazes[0].Start
			binary.LittleEndian.PutUint32(body[21:], uint32(grid[0]))
			binary.LittleEndian.PutUint32(body[25:], uint32(grid[1]))
			sel, target, err := w.bakalPortalSelection(body, now)
			if err != nil {
				t.Fatal(edge, err)
			}
			pc, err := w.bakalPortalCatalog(sel, target)
			if err != nil {
				t.Fatal(err)
			}
			next, err := dungeon.Select(*pc, sel, 115, nil)
			if err != nil {
				t.Fatal(err)
			}
			packets, err := w.dungeonEntryPlan(context.Background(), "dungeon_select_ack", 16, sel, next)
			if err != nil || next.Definition.ID != id {
				t.Fatalf("connected portal %d refused: %v", id, err)
			}
			location, mapPacket := -1, -1
			for i, p := range packets {
				if p.Name == "bakal_room_party_location" {
					location = i
				}
				if p.ID == 29 {
					mapPacket = i
				}
			}
			if location < 0 || mapPacket < location {
				t.Fatalf("destination %d loaded before party location", id)
			}
			// The same dungeon-level edge must not be accepted from a room
			// that does not contain its actual passive-object portal.
			for _, room := range source.Maze.Rooms {
				script, err := c.MapScript(room.Map)
				if err != nil {
					t.Fatal(err)
				}
				present := false
				for _, other := range objects {
					present = present || bakalPortalInRoom(script, other)
				}
				if !present {
					source.Room = room
					if _, _, err := w.bakalPortalSelection(body, now); err == nil {
						t.Fatal("cross-lane warp accepted from wrong source room", edge, room.Map)
					}
					break
				}
			}
		}
	}
	t.Logf("audited all %d native inter-dungeon warp edges / %d objects", len(rules.Bakal.PortalEdges), objectsAudited)
	// Visit every source opening boss arena through ordinary room loading.
	// CMD2073 occurs only on dungeon entry; CMD37 must own later boss spawns.
	for _, placement := range rules.Bakal.Monsters {
		slot := rules.Bakal.Slots[placement.Location]
		loc := rules.Bakal.Locations[placement.Location]
		grid := loc.Grid
		if loc.HasSpecificGrid {
			grid = loc.SpecificGrid
		}
		sel := protocol.DungeonSelection{ID: slot.Dungeon, Party: 65535}
		pc, err := w.bakalPortalCatalog(sel, [2]int32{int32(grid[0]), int32(grid[1])})
		if err != nil {
			t.Fatal(err)
		}
		arena, err := dungeon.Select(*pc, sel, 115, nil)
		if err != nil {
			t.Fatal(err)
		}
		w.activeDungeon = arena
		plan, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatalf("%s ordinary arena load: %v", placement.Kind, err)
		}
		spawn, loaded := -1, -1
		for i, p := range plan {
			if p.Name == "bakal_source_boss" {
				spawn = i
				if binary.LittleEndian.Uint32(p.Payload[7:11]) != rules.Bakal.MonsterDefinitions[placement.Kind].ID || p.Payload[12] != 3 {
					t.Fatalf("%s source boss identity/grow type lost: %x", placement.Kind, p.Payload)
				}
			}
			if p.ID == 30 {
				loaded = i
			}
		}
		if spawn < 0 || loaded < spawn || arena.RoomCleared() {
			t.Fatalf("%s boss missing before loaded/clear: spawn=%d loaded=%d cleared=%v", placement.Kind, spawn, loaded, arena.RoomCleared())
		}
		duplicate, err := w.bakalLoadedBoss(arena, now)
		if err != nil || len(duplicate) != 0 {
			t.Fatalf("%s repeated CMD2073 duplicated boss: %v", placement.Kind, err)
		}
	}
	// Basilisk is a local raid clear. The doorway back into its entrance
	// room (0,0) must remain usable to reach the native gate into Bakal.
	sel := protocol.DungeonSelection{ID: 100003157, Party: 65535}
	pc, err := w.bakalPortalCatalog(sel, [2]int32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	guardian, err := dungeon.Select(*pc, sel, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = guardian
	if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	guardian.Loaded = true
	if _, err = guardian.Move(c, [2]byte{0, 0}); err == nil {
		t.Fatal("live guardian allowed premature exit")
	}
	for _, m := range guardian.Monsters {
		if !m.NonCombat {
			if _, err = guardian.ConfirmDeath(uint32(m.Entity), 2, 2); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !guardian.Completed() {
		m, _ := opening.InitialMonster(23)
		t.Fatalf("guardian clear fixture did not reach local completion: actor=%+v location=%+v room=%+v arena=%v", m, rules.Bakal.Locations[23], guardian.Room, guardian.ArenaBoss)
	}
	if _, err = w.bakalConfirmDefeats(); err != nil {
		t.Fatal(err)
	}
	for _, grid := range [][2]byte{{0, 0}} {
		next, _, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: sel.ID, Position: grid})
		if err != nil {
			t.Fatalf("entrance blocked after guardian at %v: %v", grid, err)
		}
		w.activeDungeon = next
		if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
			t.Fatal(err)
		}
		next.Loaded = true
		for _, m := range next.Monsters {
			if !m.NonCombat {
				if _, err = next.ConfirmDeath(uint32(m.Entity), 2, 2); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	// Reuse the completed guardian with the native raid type removed to
	// prove ordinary boss completion still blocks walking out of its arena.
	ordinary := *guardian
	ordinary.Definition.Bakal = false
	if _, err = ordinary.Move(c, [2]byte{0, 0}); err == nil {
		t.Fatal("ordinary completed boss room allowed navigation")
	}
	// Captured native gate from the entrance targets the right side of
	// Bakal's first arena, then the adjacent room contains the source boss.
	body := make([]byte, 48)
	binary.LittleEndian.PutUint32(body[13:], 100003149)
	binary.LittleEndian.PutUint32(body[21:], 2)
	binary.LittleEndian.PutUint32(body[25:], 1)
	destination, grid, err := w.bakalPortalSelection(body, now)
	if err != nil {
		t.Fatal(err)
	}
	pc, err = w.bakalPortalCatalog(destination, grid)
	if err != nil {
		t.Fatal(err)
	}
	arena, err := dungeon.Select(*pc, destination, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = arena
	arena.Loaded = true
	if arena.Room.Map != 100007435 {
		t.Fatal("wrong native main gate landing", arena.Room)
	}
	next, _, err := w.moveDungeonRoomDecoded(protocol.DungeonRoomTransition{Dungeon: destination.ID, Position: [2]byte{1, 1}})
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = next
	packets, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	spawned := false
	for _, p := range packets {
		spawned = spawned || p.Name == "bakal_source_boss"
	}
	if !spawned || next.Room.Map != 100007434 {
		t.Fatal("main arena boss missing after entrance", next.Room)
	}
	next.Loaded = true
	if _, err = w.bakalEnterSymbolPlan(next, true); err != nil {
		t.Fatal(err)
	}
	lockedWarp := make([]byte, 46)
	binary.LittleEndian.PutUint32(lockedWarp[13:], 1)
	binary.LittleEndian.PutUint32(lockedWarp[17:], 5)
	lockedWarp[21] = 1
	beforeHP := opening.SymbolValues()[rules.Bakal.Symbols["[BAKAL HP]"]]
	if _, _, err = w.bakalNativeRoomWarp(lockedWarp); err == nil {
		t.Fatal("cinematic bypassed three-dragon lock")
	}
	if opening.SymbolValues()[rules.Bakal.Symbols["[BAKAL HP]"]] != beforeHP {
		t.Fatal("rejected warp changed HP")
	}
	// Replay native guardian portals requesting the ordinary entry; living
	// dragons must load their specific arena, then return to ordinary entry.
	opening, err = raid.PrepareBakalOpening(rules, []raid.Member{{Actor: 2, Position: 1}}, now.Add(-5*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err = opening.Activate(now); err != nil {
		t.Fatal(err)
	}
	w.bakalOpening = opening
	for _, slot := range []uint32{26, 12, 40} {
		loc := rules.Bakal.Locations[slot]
		sel := protocol.DungeonSelection{ID: loc.Dungeon, Party: 65535}
		// Find the actual guardian room containing this dragon's native gate.
		var source *dungeon.Session
		for edge, objects := range rules.Bakal.PortalObjects {
			if edge[1] != loc.Dungeon {
				continue
			}
			for _, room := range c.Dungeons[edge[0]].Mazes[0].Rooms {
				script, err := c.MapScript(room.Map)
				if err != nil {
					t.Fatal(err)
				}
				present := false
				for _, object := range objects {
					present = present || bakalPortalInRoom(script, object)
				}
				if !present {
					continue
				}
				pc, err := w.bakalPortalCatalog(protocol.DungeonSelection{ID: edge[0], Party: 65535}, [2]int32{int32(room.X), int32(room.Y)})
				if err != nil {
					t.Fatal(err)
				}
				source, err = dungeon.Select(*pc, protocol.DungeonSelection{ID: edge[0], Party: 65535}, 115, nil)
				if err != nil {
					t.Fatal(err)
				}
				source.Loaded = true
				for _, actor := range source.Monsters {
					if !actor.NonCombat {
						if _, err = source.ConfirmDeath(uint32(actor.Entity), 2, 2); err != nil {
							t.Fatal(err)
						}
					}
				}
				break
			}
			if source != nil {
				break
			}
		}
		if source == nil {
			t.Fatal("native guardian gate missing", slot)
		}
		w.activeDungeon = source
		body := make([]byte, 48)
		binary.LittleEndian.PutUint32(body[13:], sel.ID)
		binary.LittleEndian.PutUint32(body[21:], uint32(loc.Grid[0]))
		binary.LittleEndian.PutUint32(body[25:], uint32(loc.Grid[1]))
		s, _, err := w.enterBakalPortal(body)
		if err != nil {
			t.Fatalf("slot %d actual portal: %v", slot, err)
		}
		if [2]byte{s.Room.X, s.Room.Y} != loc.SpecificGrid {
			t.Fatal("living dragon sent to service room", slot, s.Room)
		}
		w.activeDungeon = s
		if _, err = w.bakalEnterSymbolPlan(s, true); err != nil {
			t.Fatal(err)
		}
		if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
			t.Fatal(err)
		}
		s.Loaded = true
		m, ok := opening.InitialMonster(slot)
		if !ok {
			t.Fatal("source dragon missing", slot)
		}
		liveReturn := make([]byte, 46)
		liveReturn[21] = 1
		if _, _, err = w.bakalNativeRoomWarp(liveReturn); err == nil {
			t.Fatal("native return bypassed live dragon", slot)
		}
		found := false
		for _, actor := range w.activeDungeon.Monsters {
			if actor.Template == m.ID {
				found = true
				if _, err = w.activeDungeon.ConfirmDeath(uint32(actor.Entity), 2, 2); err != nil {
					t.Fatal(err)
				}
			}
		}
		if !found {
			t.Fatal("dragon not spawned after real entry path", slot)
		}
		if _, err = w.completeDungeon(); err != nil {
			t.Fatal(err)
		}
		// Sparazzi Last.act repositions into the same arena before its normal-map
		// return. Accept this only after confirmed defeat, without resurrecting it.
		reposition := make([]byte, 46)
		binary.LittleEndian.PutUint32(reposition[13:], uint32(loc.SpecificGrid[0]))
		binary.LittleEndian.PutUint32(reposition[17:], uint32(loc.SpecificGrid[1]))
		reposition[21] = 1
		repositioned, _, err := w.bakalNativeRoomWarp(reposition)
		if err != nil || [2]byte{repositioned.Room.X, repositioned.Room.Y} != loc.SpecificGrid {
			t.Fatalf("slot %d post-death arena reposition: %v", slot, err)
		}
		w.activeDungeon = repositioned
		loaded, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatal(err)
		}
		for _, packet := range loaded {
			if packet.Name == "bakal_source_boss" {
				t.Fatal("post-death reposition resurrected dragon", slot)
			}
		}
		repositioned.Loaded = true
		returnBody := make([]byte, 46)
		returnBody[21] = 1
		returnBody[25] = 255
		returnBody[26] = 255
		returnBody[31] = 255
		returnBody[32] = 255
		n, _, err := w.bakalNativeRoomWarp(returnBody)
		if err != nil || [2]byte{n.Room.X, n.Room.Y} != loc.Grid {
			t.Fatalf("slot %d native return: %v", slot, err)
		}
		w.activeDungeon = n
		// Returning to the same gate after a real defeat keeps the service entry.
		w.activeDungeon = source
		returned, _, err := w.enterBakalPortal(body)
		if err != nil || [2]byte{returned.Room.X, returned.Room.Y} != loc.Grid {
			t.Fatalf("defeated dragon entry slot %d: %v", slot, err)
		}
		w.activeDungeon = returned
		packets, err := w.finishDungeonLoading(make([]byte, 16))
		if err != nil {
			t.Fatal(err)
		}
		for _, packet := range packets {
			if packet.Name == "bakal_source_boss" {
				t.Fatal("defeated dragon respawned", slot)
			}
		}
	}
	// Replay the captured post-cinematic request, with no ordinary health
	// packet or boss death: C2070 itself is the native phase threshold report.
	pc, err = w.bakalPortalCatalog(protocol.DungeonSelection{ID: 100003149, Party: 65535}, [2]int32{1, 1})
	if err != nil {
		t.Fatal(err)
	}
	s, err = dungeon.Select(*pc, protocol.DungeonSelection{ID: 100003149, Party: 65535}, 115, nil)
	if err != nil {
		t.Fatal(err)
	}
	w.activeDungeon = s
	if _, err = w.bakalEnterSymbolPlan(s, true); err != nil {
		t.Fatal(err)
	}
	if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	s.Loaded = true
	body = make([]byte, 46)
	binary.LittleEndian.PutUint32(body[13:], 1)
	binary.LittleEndian.PutUint32(body[17:], 5)
	body[21] = 1
	n, _, err := w.bakalNativeRoomWarp(body)
	if err != nil {
		t.Fatal("post-cinematic native request failed", err)
	}
	w.activeDungeon = n
	if n.Room.Map != 100007763 {
		t.Fatal("phase warp wrong map", n.Room)
	}
	if _, err = w.finishDungeonLoading(make([]byte, 16)); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, actor := range n.Monsters {
		found = found || actor.Template == rules.Bakal.MonsterDefinitions["bakal"].SecondID
	}
	if !found {
		t.Fatal("second-stage source boss missing")
	}
	if _, _, err = w.bakalNativeRoomWarp(body); err == nil {
		t.Fatal("repeat cinematic warp accepted from second arena")
	}
}
