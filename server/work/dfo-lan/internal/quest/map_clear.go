package quest

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/storage"
	"fmt"
)

func (s *Service) MapClear(ctx context.Context, role storage.Character, run *dungeon.Session, source string) ([]protocol.ActiveQuest, error) {
	if run == nil || !run.Completed() || source != s.Catalog.Source.Checksum {
		return nil, fmt.Errorf("map clear requires confirmed owned source dungeon completion")
	}
	// A room transition requires the previous room to be cleared, so by the
	// time a run completes every room it entered is cleared. A source
	// objective naming any of those maps is satisfied; settling only the final
	// boss room silently stalls objectives set on an earlier room of the run.
	x := s.Index()
	for _, mapID := range run.ClearedMaps() {
		if _, err := s.Store.RecordQuestMapClear(ctx, role.AccountID, role.ID,
			run.RunID, mapID, source, SingleClearMap, x.ByClearMap[mapID]); err != nil {
			return nil, err
		}
	}
	if en := x.Entries[uint32(run.Maze.Quest)]; allRoomsUnderClearMatch(en, run) {
		if _, err := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, run.Maze.Quest, source, AllRoomsUnderClear); err != nil {
			return nil, err
		}
	}
	if en := x.Entries[uint32(run.Maze.Quest)]; s.seekMeetBossClearMatch(en, role, run) {
		if _, err := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, run.Maze.Quest, source, SeekAndMeetNPC); err != nil {
			return nil, err
		}
	}
	return s.Active(ctx, role)
}

// seekMeetBossClearMatch covers the source shape where a quest-specific maze
// ends in a boss map that declares the quest's meeting NPC. MapClear has
// already required confirmed completion; a client CMD33 is never proof here.
func (s *Service) seekMeetBossClearMatch(en *Entry, role storage.Character, run *dungeon.Session) bool {
	if en == nil || !en.Implemented || en.Model != SeekAndMeetNPC ||
		run == nil || run.Maze.Quest == 0 || en.ID != uint32(run.Maze.Quest) ||
		!run.Room.Boss || en.NPC == 0 || s.Dungeons == nil ||
		s.Dungeons.Source.Checksum != s.Catalog.Source.Checksum {
		return false
	}
	definition, ok := s.Catalog.Quests[en.ID]
	if !ok || definition.Kind != "[seek n meet npc]" || len(en.Seek.Items) == 0 || en.Seek.NPC != en.NPC {
		return false
	}
	info := cells(definition.Script.Cells, "[dungeon info]")
	if len(info) != 2 || info[0].Type != 0 || info[0].Value <= 0 ||
		info[1].Type != 0 || info[1].Value != -1 || uint32(info[0].Value) != run.Definition.ID {
		return false
	}
	sourceDungeon, ok := s.Dungeons.Dungeons[run.Definition.ID]
	if !ok || sourceDungeon.Script.SHA256 != run.Definition.Script.SHA256 {
		return false
	}
	sourceBoss := false
	for _, maze := range sourceDungeon.Mazes {
		if maze.Index != run.Maze.Index || maze.Quest != run.Maze.Quest || maze.Boss != run.Maze.Boss {
			continue
		}
		for _, room := range maze.Rooms {
			if room.Boss && room.Map == run.Room.Map && room.X == run.Room.X && room.Y == run.Room.Y {
				sourceBoss = true
				break
			}
		}
	}
	if !sourceBoss {
		return false
	}
	script, ok := s.Dungeons.Maps[run.Room.Map]
	if !ok || !mapContainsNPC(script, en.NPC) {
		return false
	}
	bag, err := inventory.ReadBag(role.State)
	return err == nil && holds(bag, en.Seek.Items)
}

func mapContainsNPC(script catalog.ScriptRecord, npc uint32) bool {
	c := script.Cells
	for i := 0; i < len(c); i++ {
		if c[i].Type != 3 || c[i].Text != "[NPC]" {
			continue
		}
		for i++; i+4 < len(c) && c[i].Type != 3; i += 5 {
			if c[i].Type != 0 || c[i+1].Type != 6 || c[i+2].Type != 0 || c[i+3].Type != 0 || c[i+4].Type != 0 {
				return false
			}
			if c[i].Value > 0 && uint32(c[i].Value) == npc {
				return true
			}
		}
	}
	return false
}
