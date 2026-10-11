package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

func (w *worldSession) bakalNativeRoomWarp(p []byte) (*dungeon.Session, []outboundPacket, error) {
	s := w.activeDungeon
	if w.channelType != 82 || !w.raidWaiting || s == nil || !s.Loaded || w.bakalOpening == nil || w.bakalRules == nil {
		return nil, nil, fmt.Errorf("Bakal scripted warp outside loaded owned combat")
	}
	grid, record, err := protocol.DecodeBakalRoomWarp115(p)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now()
	if err = w.bakalOpening.AuthorizeDungeon(s.Definition.ID, now); err != nil {
		return nil, nil, err
	}
	from := [2]byte{s.Room.X, s.Room.Y}
	var bound uint32
	for id, loc := range w.bakalRules.Locations {
		_, alive := w.bakalOpening.InitialMonster(id)
		sourceReturn := !alive && s.RoomCleared() && (grid == loc.Grid || grid == loc.SpecificGrid)
		if loc.Dungeon == s.Definition.ID && loc.HasSpecificGrid && (from == loc.SpecificGrid && grid == loc.Grid || sourceReturn) {
			if bound != 0 {
				return nil, nil, fmt.Errorf("ambiguous source scripted warp")
			}
			bound = id
		}
	}
	if bound == 0 {
		return nil, nil, fmt.Errorf("warp is not the source arena-to-normal-map route")
	}
	r := protocol.DungeonRoomTransition{Dungeon: s.Definition.ID, Position: grid, Record: record}
	m, exists := w.bakalOpening.InitialMonster(bound)
	if exists && m.SecondID != 0 && m.HasSecondGrid && m.SecondGrid == grid {
		owned := false
		for _, actor := range s.Monsters {
			owned = owned || actor.Template == w.bakalRules.MonsterDefinitions[m.Kind].ID && actor.Rank == 3
		}
		if !owned {
			return nil, nil, fmt.Errorf("phase cinematic has no owned first-stage actor")
		}
		values := w.bakalOpening.SymbolValues()
		if values[w.bakalRules.Symbols["[BAKAL HP UNLOCK GRADE]"]] >= w.bakalRules.Stage.UnlockGradeBelow {
			return nil, nil, fmt.Errorf("phase cinematic before source HP unlock")
		}
		// The native phase-shift object's die.act sends C2070 instead of a
		// C2071 health report. Treat this source-bound request as the threshold
		// report; the domain still enforces its current source HP lock.
		if values[w.bakalRules.Symbols["[BAKAL PHASE]"]] == 0 {
			hp := int32(int64(values[w.bakalRules.Symbols["[MONSTER MAX HP]"]]) * int64(w.bakalRules.Stage.HealthPercent) / 100)
			if err = w.bakalOpening.ReportHealth(bound, hp, now); err != nil {
				return nil, nil, err
			}
		}
		if _, ok := w.bakalOpening.StageWarp(s.Definition.ID, from, grid); !ok {
			return nil, nil, fmt.Errorf("source phase warp not activated")
		}
	} else {
		if !s.RoomCleared() {
			return nil, nil, fmt.Errorf("native return still has live enemies")
		}
		r.RaidReturn = true
	}
	next, plan, err := w.moveDungeonRoomDecoded(r)
	if err != nil {
		return nil, nil, err
	}
	// C2070 has its own ACK; an unrelated C45 success can consume a pending
	// ordinary door response in the native client.
	for i, packet := range plan {
		if packet.Name == "dungeon_move_ack" {
			plan = append(plan[:i], plan[i+1:]...)
			break
		}
	}
	effects, err := w.bakalScriptPlan(w.bakalOpening.DrainEffects())
	if err != nil {
		return nil, nil, err
	}
	return next, append(effects, plan...), nil
}
