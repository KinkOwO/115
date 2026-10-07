package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/raid"
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
	"time"
)

func (w *worldSession) bakalScriptPlan(effects []raid.BakalEffect) ([]outboundPacket, error) {
	if len(effects) == 0 || w.bakalOpening == nil {
		return nil, nil
	}
	var plan []outboundPacket
	symbols := map[uint32]int32{}
	locations := map[uint32]bool{}
	healthUpdates := map[uint32]uint32{}
	partyChanged, raidBuffChanged := false, false
	usedBuff := uint32(25)
	dungeons := map[uint32]bool{}
	var lastDungeon uint32
	kick := false
	ended := ""
	for _, e := range effects {
		switch e.Op {
		case "symbol":
			symbols[w.bakalRules.Symbols[e.Kind]] = e.Value
			for slot, m := range w.bakalOpening.Monsters() {
				if e.Kind == "["+strings.ToUpper(m.Kind)+" HP]" {
					locations[slot] = true
				}
			}
		case "monster":
			locations[e.Location] = true
		case "health":
			locations[e.Location] = true
			healthUpdates[e.Location] = uint32(e.Value)
		case "party-buff":
			partyChanged = true
		case "raid-buff", "raid-buff-used":
			raidBuffChanged = true
			if e.Op == "raid-buff-used" {
				usedBuff = e.ID
			}
		case "dungeon":
			dungeons[e.ID] = true
		case "last":
			lastDungeon = e.ID
		case "kick":
			kick = kick || w.activeDungeon != nil && w.activeDungeon.Definition.ID == e.ID
		case "clear", "fail":
			ended = e.Op
		case "timer", "buff": // Domain timers run on the serial connection tick; initial buff inventory is sent by openingPlan.
		default:
			return nil, fmt.Errorf("unhandled native Bakal effect %s", e.Op)
		}
	}
	values, err := bakalSymbolValuePlan(symbols)
	if err != nil {
		return nil, err
	}
	plan = append(plan, values...)
	var ids []int
	for id := range locations {
		if id > 0 && id <= 51 {
			ids = append(ids, int(id))
		}
	}
	sort.Ints(ids)
	monsters := w.bakalOpening.Monsters()
	var rows []protocol.BakalMonsterInfo
	for _, id := range ids {
		row := protocol.BakalMonsterInfo{Location: uint32(id), Action: 1, Parties: [3]int8{-1, -1, -1}}
		if m, exists := monsters[uint32(id)]; exists {
			row.Kind, err = protocol.BakalMonsterType(m.Kind)
			if err != nil {
				return nil, err
			}
			row.Health = uint32(w.bakalRules.InitialVariables["[MONSTER MAX HP]"])
			if hp := w.bakalRules.Symbols["["+strings.ToUpper(m.Kind)+" HP]"]; hp != 0 {
				row.Health = uint32(w.bakalOpening.SymbolValues()[hp])
				healthUpdates[uint32(id)] = row.Health
			}
			if hp, exists := healthUpdates[uint32(id)]; exists {
				row.Health = hp
			}
		}
		rows = append(rows, row)
	}
	if len(rows) > 0 {
		body, err := protocol.BakalMonsterInfoPayload(rows)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_script_monsters", 0, 2286, body})
	}
	var hpRows []protocol.BakalMonsterInfo
	for _, row := range rows {
		if _, exists := healthUpdates[row.Location]; exists && row.Kind != 0 {
			row.Action = 2
			hpRows = append(hpRows, row)
		}
	}
	if len(hpRows) > 0 {
		body, err := protocol.BakalMonsterInfoPayload(hpRows)
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_script_actor_health", 0, 2286, body})
	}
	if raidBuffChanged {
		body, err := protocol.BakalBuffInfoPayload(w.bakalOpening.RaidBuffCounts(), usedBuff, uint32(w.bakalParty-1))
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_script_buff_inventory", 0, 2288, body})
	}
	if partyChanged && w.activeDungeon != nil {
		body, err := w.bakalRoomPartyPlan(w.activeDungeon)
		if err != nil {
			return nil, err
		}
		plan = append(plan, body...)
	}
	ids = nil
	for id := range dungeons {
		ids = append(ids, int(id))
	}
	sort.Ints(ids)
	if len(ids) > 0 {
		body := []byte{0, byte(len(ids))}
		states := w.bakalOpening.DungeonStates()
		for _, id := range ids {
			body = binary.LittleEndian.AppendUint32(body, uint32(id))
			state := byte(0)
			if states[uint32(id)] != "open" {
				state = 1
			}
			body = append(body, state)
		}
		body = binary.LittleEndian.AppendUint32(body, 0)
		plan = append(plan, outboundPacket{"bakal_script_dungeon_states", 0, 572, body})
	}
	if lastDungeon != 0 {
		if lastDungeon != w.bakalOpening.FinalDungeon() {
			return nil, fmt.Errorf("final transition is outside native script")
		}
		def, exists := w.dungeons.Dungeons[lastDungeon]
		if !exists || len(def.Mazes) == 0 {
			return nil, fmt.Errorf("native final dungeon content missing")
		}
		catalog, err := w.bakalPortalCatalog(protocol.DungeonSelection{ID: lastDungeon}, [2]int32{int32(def.Mazes[0].Start[0]), int32(def.Mazes[0].Start[1])})
		if err != nil {
			return nil, err
		}
		copyWorld := *w
		copyWorld.dungeons = catalog
		s, entry, err := copyWorld.prepareDungeonEntry(protocol.DungeonSelection{ID: lastDungeon, Party: 65535})
		if err != nil {
			return nil, err
		}
		w.activeDungeon = s
		w.deathSent = nil
		w.completionSent = false
		w.drops = nil
		w.resetCards()
		plan = append(plan, outboundPacket{"bakal_final_selection", 0, 27, protocol.EnterDungeonSelection()}, outboundPacket{"bakal_final_move", 0, 2281, protocol.LegionDirectMoveNotice115()})
		for _, p := range entry {
			if p.Kind == 0 {
				plan = append(plan, p)
			}
		}
		return plan, nil
	}
	if kick || ended != "" {
		if w.activeDungeon != nil {
			route, err := w.leaveDungeon()
			if err != nil {
				return nil, err
			}
			plan = append(plan, route[1:]...)
			w.activeDungeon = nil
			w.deathSent = nil
			w.completionSent = false
			w.drops = nil
			w.resetCards()
		}
		location, err := w.bakalPartyAt(51 + w.bakalParty)
		if err != nil {
			return nil, err
		}
		body, err := protocol.BakalPartyInfoPayload([]protocol.BakalPartyInfo{location})
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"bakal_script_return_to_camp", 0, 2285, body})
		if ended == "clear" && w.bakalRewardService() != nil {
			// C39 completion sends this plan immediately, before the serial tick.
			// Publish the durable result before removing native raid phase ownership.
			// Failed awards remain pending for the tick's existing retry path.
			if result, err := w.bakalSettlementPlan(time.Now()); err == nil {
				plan = append(plan, result...)
			}
		}
		if ended != "" {
			plan = append(plan, outboundPacket{"bakal_script_phase_ended", 0, 574, []byte{0, 0}})
		}
	}
	return plan, nil
}

func (w *worldSession) bakalConfirmDefeats() ([]outboundPacket, error) {
	if w.activeDungeon == nil || w.bakalOpening == nil {
		return nil, nil
	}
	s := w.activeDungeon
	if s.Definition.ID == w.bakalOpening.FinalDungeon() && w.bakalOpening.Ended() == "" && bakalFinalActorDefeated(s) {
		if err := w.bakalOpening.ClearFinalDungeon(s.Definition.ID, time.Now()); err != nil {
			return nil, err
		}
		return w.bakalScriptPlan(w.bakalOpening.DrainEffects())
	}
	for slot := range w.bakalOpening.Monsters() {
		m, exists := w.bakalOpening.InitialMonster(slot)
		if !exists {
			continue
		}
		loc := w.bakalRules.Locations[slot]
		if loc.Dungeon != s.Definition.ID || m.HasGrid && m.Grid != [2]byte{s.Room.X, s.Room.Y} {
			continue
		}
		for _, actor := range s.Monsters {
			if actor.Template == m.ID && actor.Rank == 3 && s.Dead[actor.Entity] {
				if err := w.bakalOpening.DefeatMonster(slot, actor.Template, time.Now()); err != nil {
					return nil, err
				}
			}
		}
	}
	return w.bakalScriptPlan(w.bakalOpening.DrainEffects())
}

// The ending's source dummy appears in both rooms. Native Last_2.act and the
// leader skip action kill that actor even in the first, non-boss maze room.
// Require an owned, confirmed dead hunt-boss; entering an empty scene cannot
// complete the raid.
func bakalFinalActorDefeated(s *dungeon.Session) bool {
	if s == nil || !s.Loaded || !s.RoomCleared() {
		return false
	}
	for _, actor := range s.Monsters {
		if actor.Rank == 3 && s.Dead[actor.Entity] {
			if s.Definition.SourceBoss != 0 && s.Definition.SourceBoss == actor.Template {
				return true
			}
		}
	}
	return false
}
