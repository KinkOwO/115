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
	if en := x.Entries[uint32(run.Maze.Quest)]; s.hogasBossClearMatch(en, role, run) {
		if _, err := s.Store.CompleteQuestObjective(ctx, role.AccountID, role.ID, run.Maze.Quest, source, SeekAndMeetNPC); err != nil {
			return nil, err
		}
	}
	return s.Active(ctx, role)
}

// hogasBossClearMatch is limited to the current source quest whose required
// NPC is physically present in its own boss room. The held cure alone is not
// enough: the run must have completed the quest's source boss room.
func (s *Service) hogasBossClearMatch(en *Entry, role storage.Character, run *dungeon.Session) bool {
	if en == nil || en.ID != 3634 || en.Model != SeekAndMeetNPC ||
		run == nil || run.Definition.ID != 71 || run.Maze.Quest != 3634 || run.Maze.Index != 2 ||
		run.Room.Map != 91798 || !run.Room.Boss || s.Dungeons == nil ||
		s.Dungeons.Source.Checksum != s.Catalog.Source.Checksum || en.NPC != 1298 {
		return false
	}
	definition, ok := s.Catalog.Quests[en.ID]
	if !ok || definition.Kind != "[seek n meet npc]" || len(en.Seek.Items) != 1 ||
		en.Seek.Items[0] != (ItemNeed{Template: 10164777, Amount: 1}) {
		return false
	}
	info := cells(definition.Script.Cells, "[dungeon info]")
	if len(info) != 2 || info[0].Type != 0 || info[0].Value != 71 || info[1].Type != 0 || info[1].Value != -1 {
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
	inside := false
	for _, cell := range script.Cells {
		if cell.Type == 3 {
			inside = cell.Text == "[NPC]"
			continue
		}
		if inside && cell.Type == 0 && cell.Value > 0 && uint32(cell.Value) == npc {
			return true
		}
	}
	return false
}
