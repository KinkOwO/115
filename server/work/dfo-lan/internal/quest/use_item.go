package quest

import (
	"context"
	"dfolan/internal/storage"
)

// UseItem advances only accepted objectives matching an item that the owned
// character has actually used through a committed item consume event.
func (s *Service) UseItem(ctx context.Context, role storage.Character, template uint32, eventKey string) ([]uint16, error) {
	if template == 0 || eventKey == "" || role.ConfigVersion != s.Catalog.Source.Checksum {
		return nil, nil
	}
	var advanced []uint16
	for _, id := range s.Index().ByUseItem[template] {
		applied, err := s.Store.CompleteQuestUseObjective(ctx, role.AccountID, role.ID, id,
			s.Catalog.Source.Checksum, SingleUseItem, eventKey, template)
		if err != nil {
			return advanced, err
		}
		if applied {
			advanced = append(advanced, id)
		}
	}
	return advanced, nil
}
