package quest

import (
	"context"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
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
	return s.Active(ctx, role)
}
