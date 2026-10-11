package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"maps"
	"time"
)

// Select the requested source dungeon, keeping catalog definitions immutable.
// Native position words are presentation data; DGN start/maze and the COS
// slot table determine the owned room (raid_bakal_portal.go:94/132/145).
func (w *worldSession) bakalDungeonEntry(id uint32, difficulty byte, grid *[2]byte, now time.Time, final bool) ([]outboundPacket, error) {
	if w.bakal == nil || w.bakalRules == nil || w.bakalRules.DungeonCatalog == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("Bakal portal has no native map source")
	}
	if !w.bakal.HasDungeon(id) {
		return nil, fmt.Errorf("Bakal dungeon outside opening phase")
	}
	if w.bakal.IsCleared(id) {
		return nil, fmt.Errorf("Bakal dungeon already cleared")
	}
	native := *w.bakalRules.DungeonCatalog
	d, ok := native.Dungeons[id]
	if !ok {
		return nil, fmt.Errorf("native Bakal dungeon %d missing", id)
	}
	if grid != nil {
		d.Mazes = append([]catalog.DungeonMaze(nil), d.Mazes...)
		found := false
		for i, m := range d.Mazes {
			for _, room := range m.Rooms {
				if [2]byte{room.X, room.Y} == *grid {
					d.Mazes[i].Start = *grid
					found = true
					break
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("Bakal requested room not in source maze")
		}
		native.Dungeons = maps.Clone(native.Dungeons)
		native.Dungeons[id] = d
	}
	sel := protocol.DungeonSelection{ID: id, Difficulty: difficulty, Party: 65535}
	s, err := dungeon.Select(native, sel, w.level, nil)
	if err != nil {
		return nil, err
	}
	s.RaidManaged = true
	location, err := w.bakalLocationForRoom(s)
	if err != nil {
		return nil, err
	}
	channel := w.charactersChannel()
	plan, err := w.dungeonEntryPlanImpl(context.Background(), "dungeon_select_ack", 16, sel, s, &channel)
	if err != nil {
		return nil, err
	}
	frames, err := w.bakal.EnterDungeon(id, location, now)
	if err != nil {
		return nil, err
	}
	// Keep the native raid party: generic solo-party initialization is excluded.
	var filtered []outboundPacket
	for _, p := range plan {
		if p.Name != "solo_party_initialized" {
			filtered = append(filtered, p)
		}
	}
	head := dungeonSelectionHead()
	if final {
		head = bakalOutgoing([]legion.BakalFrame{{Name: "bakal_final_selection", ID: 27, Body: legion.BakalFinalSelectionFrame()}})
	}
	head = append(head, bakalOutgoing(frames[:1])...)
	// Party location precedes map start, as in the captured entry chain.
	for i, p := range filtered {
		if p.ID == 28 {
			nativeSymbols := bakalOutgoing(w.bakal.SymbolsSnapshot())
			prefix := append(nativeSymbols, bakalOutgoing(frames[1:])...)
			filtered = append(filtered[:i], append(prefix, filtered[i:]...)...)
			break
		}
	}
	w.activeDungeon = s
	w.bakalCurrent = id
	w.bakalLocation = location
	w.deathSent = map[uint16]bool{}
	w.drops = nil
	w.resetCards()
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.leaveScene()
	return append(head, filtered...), nil
}

func (w *worldSession) charactersChannel() [2]byte {
	if w.characters != nil {
		return w.characters.ChannelContext
	}
	return [2]byte{}
}

func (w *worldSession) bakalLocationForRoom(s *dungeon.Session) (uint32, error) {
	if slot, ok := w.bakalRules.SlotForMap(s.Definition.ID, s.Room.Map); ok {
		return slot.Index, nil
	}
	if s.Definition.ID == uint32(w.bakalRules.NormalPhase.FinalClearDungeon) {
		return uint32(w.bakalRules.NormalPhase.SettlementTimer.Sub), nil
	}
	return 0, fmt.Errorf("Bakal room map %d has no native raid location", s.Room.Map)
}

func (w *worldSession) bakalLoadedBoss() ([]outboundPacket, error) {
	s := w.activeDungeon
	if s == nil || !s.Loaded {
		return nil, nil
	}
	location, err := w.bakalLocationForRoom(s)
	if err != nil {
		return nil, err
	}
	w.bakalLocation = location
	var plan []outboundPacket
	// A source location may have multiple room maps. Per-room template
	// deduplication is insufficient after the leader died in another map.
	if w.bakal.IsLocationDefeated(location) {
		return nil, nil
	}
	for _, spawn := range w.bakal.MonsterPlacements() {
		if uint32(spawn.Location) != location {
			continue
		}
		m, ok := w.bakalRules.Monsters[spawn.Name]
		if !ok {
			return nil, fmt.Errorf("Bakal placement has missing monster %s", spawn.Name)
		}
		if m.SpecificMap >= 0 && uint32(m.SpecificMap) != s.Room.Map {
			continue
		}
		template := m.Template
		grid := s.Maze.Boss
		if m.HasGrid {
			grid = m.Grid
		}
		// Native InitialMonster uses LOCATION INFO / SPECIFIC GRID.
		for _, loc := range w.bakalRules.Locations {
			if uint32(loc.Index) == location {
				grid = [2]byte{byte(loc.X), byte(loc.Y)}
				if loc.SpecificX != nil && loc.SpecificY != nil {
					grid = [2]byte{byte(*loc.SpecificX), byte(*loc.SpecificY)}
				}
				break
			}
		}
		if spawn.Name == "bakal" && w.bakal.SecondPhase() {
			template = m.SecondTemplate
			grid = m.SecondGrid
		}
		// Handover raid_bakal_flow.go:280..285: source APPEAR GRID,
		// otherwise DGN maze Boss. A slot spans more than one room map.
		if [2]byte{s.Room.X, s.Room.Y} != grid {
			continue
		}
		row, added, err := s.AddRaidBoss(template, int32(m.Position[0]), int32(m.Position[1]))
		if err != nil {
			return nil, err
		}
		if added {
			body, err := protocol.UnassignedMonsterAdd115([]protocol.UnassignedMonster115{row})
			if err != nil {
				return nil, err
			}
			plan = append(plan, outboundPacket{"bakal_source_boss", 0, 2194, body})
		}
	}
	return plan, nil
}

// Battle reports cannot author deaths. The source-owned rank3 entity's CMD39
// advances its raid placement independently of ordinary actors in the room.
// Native create/delete presence must not wait for those actors' cleanup.
func (w *worldSession) bakalConfirmDefeats(now time.Time) ([]outboundPacket, error) {
	s := w.activeDungeon
	if w.bakal == nil || s == nil || !s.Loaded {
		return nil, nil
	}
	if s.Definition.ID == uint32(w.bakalRules.NormalPhase.FinalClearDungeon) {
		if bakalFinalActorDefeated(s) {
			return nil, w.bakal.ClearFinalDungeon(now)
		}
		return nil, nil
	}
	for _, spawn := range w.bakal.MonsterPlacements() {
		if uint32(spawn.Location) != w.bakalLocation {
			continue
		}
		m := w.bakalRules.Monsters[spawn.Name]
		template := m.Template
		if spawn.Name == "bakal" && w.bakal.SecondPhase() {
			template = m.SecondTemplate
		}
		dead := false
		for _, actor := range s.Monsters {
			if actor.Template == template && actor.Rank == 3 {
				if !s.Dead[actor.Entity] {
					return nil, nil
				}
				dead = true
			}
		}
		if !dead {
			return nil, nil
		}
		if spawn.Name == "bakal" && !w.bakal.SecondPhase() {
			w.bakal.MarkFirstPhaseDefeated()
			return nil, nil
		}
		if w.bakal.IsCleared(s.Definition.ID) {
			return nil, nil
		}
		frames, err := w.bakal.DefeatMonster(s.Definition.ID, w.bakalLocation, now)
		if err != nil {
			return nil, err
		}
		if w.bakal.Stage() == legion.BakalOpeningFinal {
			// Transition is driven by the script's SettlementDungeon, not a guessed
			// next boss. Build the real final DGN load instead of navigation alone.
			final, err := w.bakalDungeonEntry(uint32(w.bakalRules.NormalPhase.FinalClearDungeon), s.Difficulty, nil, now, true)
			if err != nil {
				return nil, err
			}
			return append(bakalOutgoing(frames), final...), nil
		}
		return bakalOutgoing(frames), nil
	}
	return nil, nil
}

func bakalFinalActorDefeated(s *dungeon.Session) bool {
	if s == nil || !s.Loaded || !s.RoomCleared() || s.Definition.SourceBoss == 0 {
		return false
	}
	for _, m := range s.Monsters {
		if m.Rank == 3 && m.Template == s.Definition.SourceBoss && s.Dead[m.Entity] {
			return true
		}
	}
	return false
}

// Native script returns share the town restore sequence but have no CMD42
// request to acknowledge (full successful log, 12:39:42.953..954).
func (w *worldSession) bakalCampPlan() ([]outboundPacket, error) {
	if w.service == nil {
		return nil, fmt.Errorf("Bakal native camp source unavailable")
	}
	plan, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	var out []outboundPacket
	for _, p := range plan {
		if p.Kind != 1 {
			out = append(out, p)
		}
	}
	return out, nil
}
