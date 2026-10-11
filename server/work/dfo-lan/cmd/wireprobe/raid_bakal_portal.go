package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"path"
	"strconv"
	"strings"
	"time"
)

// A dungeon-level connection is insufficient: its actual native warp object
// must be present in this room, otherwise a side door can cross another lane.
func bakalPortalInRoom(script catalog.ScriptRecord, objectPath string) bool {
	name := strings.SplitN(path.Base(objectPath), "_", 2)[0]
	id, err := strconv.ParseUint(name, 10, 32)
	if err != nil {
		return false
	}
	for i, cell := range script.Cells {
		if cell.Text != "[passive object]" {
			continue
		}
		for j := i + 1; j+3 < len(script.Cells) && script.Cells[j].Type == 0; j += 4 {
			if uint32(script.Cells[j].Value) == uint32(id) {
				return true
			}
		}
	}
	return false
}

// The native camp portal sends CMD2062 without an existing dungeon. Its first
// thirteen bytes contain opaque local data, not server identity or authority.
func (w *worldSession) bakalPortalSelection(p []byte, now time.Time) (protocol.DungeonSelection, [2]int32, error) {
	var sel protocol.DungeonSelection
	if w.channelType != 82 || !w.raidWaiting || w.bakalOpening == nil || w.bakalRules == nil || !w.soloPartyReady {
		return sel, [2]int32{}, fmt.Errorf("Bakal portal requires the active real subparty in camp")
	}
	members := w.bakalOpening.Members()
	if len(members) != 1 || members[0].Actor != w.role.WireID || members[0].Position != w.bakalParty {
		return sel, [2]int32{}, fmt.Errorf("Bakal portal member ownership changed")
	}
	inCamp := false
	for _, slot := range w.bakalRules.Slots {
		if slot.Kind == "camp" && slot.TownArea == w.state.Position.Area && w.bakalTown != 0 && w.state.Position.Town == w.bakalTown {
			inCamp = true
		}
	}
	if w.activeDungeon == nil && !inCamp {
		return sel, [2]int32{}, fmt.Errorf("Bakal portal is outside its native camp")
	}
	r, err := protocol.DecodeBakalPortal115(p)
	if err != nil {
		return sel, [2]int32{}, err
	}
	if r.Mode != 0 || r.Difficulty != 0 {
		return sel, [2]int32{}, fmt.Errorf("unsupported normal Bakal portal mode")
	}
	if err := w.bakalOpening.AuthorizeDungeon(r.Dungeon, now); err != nil {
		return sel, [2]int32{}, err
	}
	if w.activeDungeon != nil {
		if w.dungeons == nil {
			return sel, [2]int32{}, fmt.Errorf("Bakal portal has no native map source")
		}
		objectPath := w.bakalRules.PortalEdges[[2]uint32{w.activeDungeon.Definition.ID, r.Dungeon}]
		if !w.activeDungeon.RoomCleared() || objectPath == "" {
			return sel, [2]int32{}, fmt.Errorf("Bakal portal requires cleared room and native dungeon connection")
		}
		script, err := w.dungeons.MapScript(w.activeDungeon.Room.Map)
		if err != nil {
			return sel, [2]int32{}, err
		}
		present := false
		for _, sourceObject := range w.bakalRules.PortalObjects[[2]uint32{w.activeDungeon.Definition.ID, r.Dungeon}] {
			present = present || bakalPortalInRoom(script, sourceObject)
		}
		if !present {
			return sel, [2]int32{}, fmt.Errorf("Bakal warp object is absent from current source room")
		}
	}
	grid := r.TargetGrid
	// Native portals name the normal (post-combat) entry. A living occupant
	// owns the source specific arena instead. In particular Sparazzi's (0,0)
	// is a separate service room, not the entrance to its live 3x3 arena.
	var resolved bool
	for id, loc := range w.bakalRules.Locations {
		if loc.Dungeon != r.Dungeon || !loc.HasSpecificGrid || r.TargetGrid != [2]int32{int32(loc.Grid[0]), int32(loc.Grid[1])} {
			continue
		}
		m, alive := w.bakalOpening.InitialMonster(id)
		if !alive {
			continue
		}
		if resolved {
			return sel, [2]int32{}, fmt.Errorf("ambiguous occupied source portal entry")
		}
		grid = [2]int32{int32(m.Grid[0]), int32(m.Grid[1])}
		resolved = true
	}
	return protocol.DungeonSelection{ID: r.Dungeon, Party: 65535}, grid, nil
}

func (w *worldSession) enterBakalPortal(p []byte) (*dungeon.Session, []outboundPacket, error) {
	sel, grid, err := w.bakalPortalSelection(p, time.Now())
	if err != nil {
		return nil, nil, err
	}
	// Reuse source selection, map membership, fatigue and character entry.
	// The presentation rectangle is never trusted as a server spawn location.
	copy := *w
	copy.dungeons, err = w.bakalPortalCatalog(sel, grid)
	if err != nil {
		return nil, nil, err
	}
	copy.activeDungeon = nil
	s, plan, err := copy.prepareDungeonEntry(sel)
	if err != nil {
		return nil, nil, err
	}
	if grid != [2]int32{int32(s.Maze.Start[0]), int32(s.Maze.Start[1])} {
		return nil, nil, fmt.Errorf("Bakal portal grid does not match native dungeon entry")
	}
	head := dungeonSelectionHead()
	head = append(head, outboundPacket{"bakal_portal_ready", 0, 2281, protocol.LegionDirectMoveNotice115()})
	return s, append(head, plan...), nil
}

func (w *worldSession) bakalPortalCatalog(sel protocol.DungeonSelection, grid [2]int32) (*catalog.DungeonCatalog, error) {

	// A road portal can enter the far end of another source maze. Resolve
	// its requested grid against native rooms before selecting that entry.
	if w.dungeons == nil {
		return nil, fmt.Errorf("missing Bakal dungeon source")
	}
	definition, ok := w.dungeons.Dungeons[sel.ID]
	if !ok {
		return nil, fmt.Errorf("Bakal destination absent from source")
	}
	definition.Mazes = append([]catalog.DungeonMaze(nil), definition.Mazes...)
	found := false
	for i := range definition.Mazes {
		for _, room := range definition.Mazes[i].Rooms {
			if grid == [2]int32{int32(room.X), int32(room.Y)} {
				definition.Mazes[i].Start = [2]byte{room.X, room.Y}
				found = true
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("Bakal portal grid absent from source maze")
	}
	catalogCopy := *w.dungeons
	catalogCopy.Dungeons = map[uint32]catalog.DungeonDefinition{sel.ID: definition}
	return &catalogCopy, nil
}
