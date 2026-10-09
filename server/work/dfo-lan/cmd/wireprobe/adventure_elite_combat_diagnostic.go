package main

import (
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
)

// Read-only snapshots around existing handlers. No DB queries, packets, content
// rules or ownership fallback. Processing success is not client acceptance.
func (w *worldSession) eliteCombatState(opcode uint16, body []byte) map[string]any {
	if w == nil || !w.adventureEliteEntryProbeUsed || w.adventureElitePrepared == nil {
		return nil
	}
	row := map[string]any{"level": w.level, "completion_sent": w.completionSent, "active": w.activeDungeon != nil, "entry_serial": w.adventureEliteEntrySerial,
		"odyssey": w.odyssey, "selecting_dungeon": w.selectingDungeon, "pending_town_arrival": w.pendingTownArrival != nil,
		"death_sent_count": len(w.deathSent), "drops_active": w.drops != nil, "result_sent": w.resultSent,
		"cards_active": w.cardPlan != nil || w.cardReceipt != nil}
	if opcode == 2062 {
		r, err := protocol.DecodeDungeonDirectMove(body)
		if err != nil {
			row["direct_move_decode_error"] = err.Error()
		} else {
			row["requested_dungeon"] = r.ID
			row["requested_difficulty"] = r.Difficulty
			row["requested_gate"] = r.Gate
			if w.dungeons != nil {
				if d, ok := w.dungeons.Dungeons[r.ID]; ok {
					row["requested_source_odyssey"] = d.Odyssey
					row["requested_source_script"] = d.Script.Path
					row["requested_source_sha256"] = d.Script.SHA256
				}
			}
		}
	}
	if s := w.activeDungeon; s != nil {
		row["run_id"] = s.RunID
		row["dungeon"] = s.Definition.ID
		row["map"] = s.Room.Map
		row["loaded"] = s.Loaded
		row["quest"] = s.Maze.Quest
		row["source_story_layers"] = len(s.Maze.Layers)
		row["source_odyssey"] = s.Definition.Odyssey
		row["source_designated_difficulty"] = s.Definition.DesignatedDifficulty
		row["source_hunt_boss"] = s.Definition.HuntBoss
		row["room_cleared"] = s.RoomCleared()
		row["completed"] = s.Completed()
		row["source_script"] = s.Definition.Script.Path
		row["source_sha256"] = s.Definition.Script.SHA256
		alive := 0
		for _, m := range s.Monsters {
			if !s.Dead[m.Entity] && !m.NonCombat {
				alive++
			}
		}
		row["combat_alive"] = alive
		if opcode == 39 {
			r, err := protocol.DecodeMonsterDeath(body)
			if err != nil {
				row["decode_error"] = err.Error()
			} else {
				row["entity"] = r.Entity
				row["killer"] = r.Killer
				row["dead"] = s.Dead[uint16(r.Entity)]
				row["unowned"] = s.Unowned[uint16(r.Entity)]
				row["death_sent"] = w.deathSent[uint16(r.Entity)]
			}
		}
	}
	return row
}
func (w *worldSession) noteEliteCombatRequest(opcode uint16, body []byte, before map[string]any, pending *dungeon.Session, plan []outboundPacket, err error, event func(map[string]any)) {
	if before == nil || event == nil {
		return
	}
	row := map[string]any{"kind": "adventure_elite_combat_request", "candidate_version": "0.3.18", "attempt": "odyssey 3/3", "direct_move_attempt": "1/3", "projection_reload_attempt": "2/3", "moon_attempt": "1/3", "candidate_stage": w.eliteCandidateStage(), "entry_serial": w.adventureEliteEntrySerial, "id": opcode,
		"character_id": w.role.ID, "owner_wire_id": w.role.WireID, "channel_type": w.channelType, "processed": err == nil, "client_acceptance": "pending",
		"before": before, "after": w.eliteCombatState(opcode, body)}
	if err != nil {
		row["reason"] = err.Error()
	}
	if pending != nil {
		row["pending_dungeon"] = pending.Definition.ID
		row["pending_map"] = pending.Room.Map
		row["pending_loaded"] = pending.Loaded
		row["pending_run_id"] = pending.RunID
		row["pending_quest"] = pending.Maze.Quest
	}
	packets := make([]map[string]any, 0, len(plan))
	for _, p := range plan {
		packets = append(packets, map[string]any{"id": p.ID, "kind": p.Kind, "name": p.Name, "bytes": len(p.Payload)})
	}
	row["packets"] = packets
	if p := w.adventureElitePrepared; p != nil {
		row["frozen_owner"] = p.Owner
		row["frozen_channel"] = p.Channel
		row["frozen_selected"] = p.Selected
		row["frozen_settings"] = fmt.Sprintf("%x", p.Settings)
	}
	event(row)
	if opcode == 2062 && pending != nil && err == nil {
		r, decodeErr := protocol.DecodeDungeonDirectMove(body)
		if decodeErr == nil {
			diagnostic := eliteEntryProbeDiagnostic(w, pending, nil, protocol.DungeonSelection{ID: r.ID, Difficulty: r.Difficulty, Party: 65535})
			diagnostic["entry_opcode"] = opcode
			diagnostic["origin_run_id"] = before["run_id"]
			event(diagnostic)
		}
	}
}

// Called only after all outbound packets and existing dispatch transitions succeed.
// Read-only: the original handlers retain death/card/drop reset responsibility.
func (w *worldSession) noteEliteCommittedState(opcode uint16, body []byte, before map[string]any, event func(map[string]any)) {
	after := w.eliteCombatState(opcode, body)
	if after == nil || event == nil || (before == nil && opcode != 16) {
		return
	}
	if before == nil {
		before = map[string]any{}
	}
	event(map[string]any{"kind": "adventure_elite_dispatch_committed", "id": opcode,
		"candidate_stage": w.eliteCandidateStage(), "entry_serial": w.adventureEliteEntrySerial,
		"origin_run_id": before["run_id"], "after": after})
}
