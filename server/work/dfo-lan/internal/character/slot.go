package character

import (
	"context"
	"dfolan/internal/game/protocol"
)

func (s *Service) ChangeSlot(ctx context.Context, account int64, p []byte) error {
	r, err := protocol.DecodeCharacterSlot(p)
	if err != nil {
		return err
	}
	return s.Store.ChangeCharacterSlots(ctx, account, CharacterSlotChange{
		Swap: r.Swap, Before: r.Before, FromFixed: r.FromFixed, ToFixed: r.ToFixed, From: r.From, To: r.To,
	}, s.Rules.MaxCharacters)
}
