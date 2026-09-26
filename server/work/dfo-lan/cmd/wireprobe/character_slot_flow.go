package main

import (
	"context"
	"errors"
)

type rosterSlotService interface {
	ChangeSlot(context.Context, int64, []byte) error
}

func changeRosterSlot(ctx context.Context, service rosterSlotService, account, selectedCharacter int64, p []byte) error {
	if selectedCharacter != 0 {
		return errors.New("character slot change requires character selection screen")
	}
	return service.ChangeSlot(ctx, account, p)
}
