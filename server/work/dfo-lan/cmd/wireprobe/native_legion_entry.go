package main

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
	"fmt"
	"time"
)

// prepareNativeLegionStage builds both the authoritative monster roster and
// N28/N29 from the same source session. It never borrows another boss's capture
// or publishes activeDungeon before the entire entry projection succeeds.
func (w *worldSession) prepareNativeLegionStage(run *legion.NativeRun) (*dungeon.Session, []outboundPacket, error) {
	if w == nil || w.dungeons == nil || run == nil || run.State != legion.NativeReady || run.Selected < 0 || w.activeDungeon != nil {
		return nil, nil, fmt.Errorf("native legion entry requires owned ready run")
	}
	stage := run.Stage()
	sel := protocol.DungeonSelection{ID: stage.Dungeon, Party: 65535}
	s, err := dungeon.Select(*w.dungeons, sel, w.level, nil)
	if err != nil {
		return nil, nil, err
	}
	noteMazeEntry(s)
	bossFound := false
	for _, m := range s.Monsters {
		if m.Template == stage.Boss {
			bossFound = true
		}
	}
	// Dusky's later stages start in an empty approach room. Preserve their
	// source maze and normal room-movement/clear gates; only the source boss
	// actually standing in the initial room may make it an arena.
	s.ArenaBoss = bossFound && s.LegionArenaRosterHasBoss()
	if !s.ArenaBoss && run.Source.Name == "Dimension Cloister" {
		return nil, nil, fmt.Errorf("native content boss differs from source start room")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var channel *[2]byte
	if w.characters != nil {
		channel = &w.characters.ChannelContext
	}
	frames, err := w.dungeonEntryPlanImpl(ctx, "native_legion_enter_ack", legion.CmdEnterDungeon, sel, s, channel)
	if err != nil {
		return nil, nil, err
	}
	if len(frames) == 0 || frames[0].ID != legion.CmdEnterDungeon {
		return nil, nil, fmt.Errorf("native dungeon entry ACK contract changed")
	}
	content := byte(102)
	notice := legion.NotiDimCloisterInfo
	if run.Source.Name == "DuskyIsland" {
		content = byte(legion.DuskyContentID)
		notice = legion.NotiDuskyInfo
	}
	frames[0].Payload = legion.DimCloisterStageAck(content, uint32(run.Phase))
	actor, err := protocol.UserState(w.role.WireID, protocol.UserStateDungeon)
	if err != nil {
		return nil, nil, err
	}
	area, err := w.forestDungeonAreaNotice()
	if err != nil {
		return nil, nil, err
	}
	info, err := run.Info(2, 0)
	if err != nil {
		return nil, nil, err
	}
	clock, err := protocol.LegionDungeonTimeout115(s.StartedAt, time.Duration(s.Definition.TimeoutSeconds)*time.Second)
	if err != nil {
		return nil, nil, err
	}
	plan := []outboundPacket{{"native_legion_area_entered", 0, 23, area}, {"native_legion_info_active", 0, notice, info}}
	inserted := false
	for _, frame := range frames {
		if frame.ID == 28 && !inserted {
			plan = append(plan, outboundPacket{"native_legion_actor_state", 0, 3, actor}, outboundPacket{"native_legion_select_context", 0, 27, protocol.EnterDungeonSelection()})
			inserted = true
		}
		plan = append(plan, frame)
	}
	if !inserted {
		return nil, nil, fmt.Errorf("source native entry omitted dungeon info")
	}
	plan = append(plan, outboundPacket{"native_legion_source_clock", 0, legion.NotiDungeonTimeoutTime, clock})
	return s, plan, nil
}
