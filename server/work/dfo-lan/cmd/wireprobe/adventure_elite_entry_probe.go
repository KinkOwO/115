package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
)

// Ordinary solo candidate, including source-validated quest mazes.
// The existing Select path retains quest ownership, level and source rules.
// Existing death/pickup/room/settlement handlers retain their own validation.
func (w *worldSession) validateEliteEntryProbe(r protocol.DungeonSelection) error {
	if w == nil || w.adventureElitePrepared == nil {
		return nil
	}
	fail := func(reason string) error {
		return fmt.Errorf("精锐助战尚未完成战斗接入；首房取证拒绝：%s", reason)
	}
	p := w.adventureElitePrepared
	if !w.ordinaryEliteSelectionVisible() || p.Owner != w.role.ID || p.Channel != w.channelType || p.Settings != w.adventureEliteSnapshot || p.Selected == ([3]int64{}) {
		return fail("准备身份或源频道不一致")
	}
	if w.activeDungeon != nil || !w.selectingDungeon || w.pendingTownArrival != nil {
		return fail("尚未完成回城或没有普通选图许可")
	}
	if r.Mode != 0 || r.Party != 65535 || w.dungeons == nil {
		return fail("此候选只支持普通单人战斗入口")
	}
	d, ok := w.dungeons.Dungeons[r.ID]
	if !ok || !w.eliteProbeDefinition(d) || dungeon.IsTrainingRoom(*w.dungeons, r.ID) {
		return fail("副本源不属于本阶段")
	}
	return nil
}
func (w *worldSession) eliteProbeDefinition(d catalog.DungeonDefinition) bool {
	return !d.Tutorial && d.Tower == nil && d.HellParty == nil &&
		(!d.Odyssey || (w.odyssey && character.OdysseyRole(w.role)))
}

func (w *worldSession) eliteCandidateStage() string {
	if w != nil && w.odyssey {
		return "ordinary-odyssey"
	}
	return "ordinary-story"
}
func (w *worldSession) eliteEntryProbeRequest(opcode uint16) error {
	if w == nil || !w.adventureEliteEntryProbeUsed || w.activeDungeon == nil {
		return nil
	}
	switch opcode {
	case 2015:
		return fmt.Errorf("精锐普通战斗候选尚未接入专用传送/直进副本")
	case 39, 43, 45, 46, 69, 70, 71, 72, 117, 2062:
		p := w.adventureElitePrepared
		if p == nil || !w.ordinaryEliteSelectionVisible() || p.Owner != w.role.ID || p.Channel != w.channelType ||
			p.Settings != w.adventureEliteSnapshot || p.Selected == ([3]int64{}) || w.role.WireID == 0 || w.role.WireID == 65535 ||
			!w.eliteProbeDefinition(w.activeDungeon.Definition) || !w.activeDungeon.Loaded {
			return fmt.Errorf("精锐普通战斗候选冻结身份/源范围/加载状态不符")
		}
	}
	if opcode == 2062 && (!w.odyssey || !w.activeDungeon.Definition.Odyssey ||
		!w.activeDungeon.Completed() || !w.activeDungeon.RoomCleared() || !w.completionSent || !w.resultSent ||
		w.selectingDungeon || w.pendingTownArrival != nil) {
		return fmt.Errorf("精锐直进只允许已通关并发送结算的奥德赛单人副本")
	}
	return nil
}

// Validate before any special-stage handler can mutate its own team state.
// Source minimum level/difficulty/maze/fatigue are still enforced by Select.
func (w *worldSession) validateEliteDirectMove(r protocol.DungeonDirectMove) error {
	if w.adventureElitePrepared == nil {
		return nil
	}
	if !w.adventureEliteEntryProbeUsed || w.activeDungeon == nil {
		return fmt.Errorf("精锐直进缺当前已接入副本")
	}
	if err := w.eliteEntryProbeRequest(2062); err != nil {
		return err
	}
	if w.dungeons == nil {
		return fmt.Errorf("精锐直进缺副本源")
	}
	d, ok := w.dungeons.Dungeons[r.ID]
	if !ok || !d.Odyssey || !w.eliteProbeDefinition(d) || dungeon.IsTrainingRoom(*w.dungeons, r.ID) {
		return fmt.Errorf("精锐直进目标不属于奥德赛普通单人源范围")
	}
	return nil
}
func eliteEntryProbeDiagnostic(w *worldSession, s *dungeon.Session, err error, request ...protocol.DungeonSelection) map[string]any {
	row := map[string]any{"kind": "adventure_elite_entry_probe", "attempt": "1/3", "accepted": err == nil, "battle_enabled": false, "owned_combat_candidate": true, "candidate_stage": w.eliteCandidateStage(), "odyssey_attempt": "3/3", "direct_move_attempt": "1/3", "entry_opcode": uint16(16), "story_attempt": "1/3", "reentry_attempt": "1/3"}
	if err != nil {
		row["reason"] = err.Error()
	}
	if w != nil {
		row["character_id"] = w.role.ID
		row["odyssey"] = w.odyssey
		row["created_as_odyssey"] = character.CreatedAsOdyssey(w.role)
		row["odyssey_role"] = character.OdysseyRole(w.role)
		row["published_channel"] = w.eliteChannelID
		row["owner_wire_id"] = w.role.WireID
		row["channel_type"] = w.channelType
		row["frozen_preparation"] = w.adventureElitePrepared
		row["entry_probe_used"] = w.adventureEliteEntryProbeUsed
		row["entry_serial"] = w.adventureEliteEntrySerial
	}
	if len(request) > 0 {
		row["requested_dungeon"] = request[0].ID
		row["requested_quest"] = request[0].Quest
		row["requested_mode"] = request[0].Mode
		row["requested_party"] = request[0].Party
		row["requested_difficulty"] = request[0].Difficulty
		if w != nil && w.dungeons != nil {
			if d, ok := w.dungeons.Dungeons[request[0].ID]; ok {
				row["requested_source_odyssey"] = d.Odyssey
				row["requested_source_script"] = d.Script.Path
				row["requested_source_sha256"] = d.Script.SHA256
				row["source_designated_difficulty"] = d.DesignatedDifficulty
			}
		}
	}
	if s != nil {
		row["run_id"] = s.RunID
		row["dungeon"] = s.Definition.ID
		row["map"] = s.Room.Map
		row["source_odyssey"] = s.Definition.Odyssey
		row["source_hunt_boss"] = s.Definition.HuntBoss
		row["source_boss_entrance_conditions"] = s.Definition.BossEntranceConditionIDs
		row["source_script"] = s.Definition.Script.Path
		row["source_sha256"] = s.Definition.Script.SHA256
		row["maze"] = s.Maze.Index
		row["quest"] = s.Maze.Quest
		row["source_boss_position"] = s.Maze.Boss
		row["source_story_layers"] = len(s.Maze.Layers)
		nonCombat := 0
		for _, m := range s.Monsters {
			if m.NonCombat {
				nonCombat++
			}
		}
		row["source_noncombat_actors"] = nonCombat
		row["monsters"] = len(s.Monsters)
	}
	return row
}
