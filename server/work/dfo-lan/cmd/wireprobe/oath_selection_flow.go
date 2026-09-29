package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
)

// oathSelectionPackets handles the current client's two C2S2382 forms.
// Player clicks mutate the option ledger; scene bootstrap only reads it.
func (w *worldSession) oathSelectionPackets(ctx context.Context, body []byte) ([]outboundPacket, error) {
	if w == nil || w.characters == nil || w.characters.Store == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("oath selection requires a selected character")
	}
	r, err := protocol.DecodeOathSystemRequest(body)
	if err != nil {
		return nil, err
	}
	if r.ExplicitSelection && w.activeDungeon != nil {
		return nil, fmt.Errorf("oath option can only be changed in town")
	}
	var current struct {
		Level  int
		ItemID uint32
		Option int
	}
	if r.ExplicitSelection {
		selected, err := w.characters.Store.SelectEquippedOathOption(ctx, w.account, w.role.ID, r.ItemID, int(r.Option))
		if err != nil {
			return nil, err
		}
		current.Level, current.ItemID, current.Option = selected.Level, selected.ItemID, selected.Option
	} else {
		selected, err := w.characters.Store.EquippedOathSelection(ctx, w.account, w.role.ID)
		if err != nil {
			return nil, err
		}
		if selected.Level < 115 || selected.ItemID != r.ItemID {
			return nil, fmt.Errorf("bootstrap oath item is not equipped and unlocked")
		}
		current.Level, current.ItemID, current.Option = selected.Level, selected.ItemID, selected.Option
	}
	info, err := protocol.OathSystemInfo(current.Level, current.Option)
	if err != nil {
		return nil, err
	}
	ack := outboundPacket{"oath_option_selection_accepted", 1, 2382, []byte{1}}
	snapshot := outboundPacket{"oath_system_info_refreshed", 0, 2839, info}
	if r.ExplicitSelection {
		return []outboundPacket{ack, snapshot}, nil
	}
	return []outboundPacket{snapshot, ack}, nil
}

// dungeonOathSelectionPacket re-reads the authoritative selection after the
// dungeon actor and its worn items have been rebuilt. Scene bootstrap requests
// are not guaranteed to arrive, so the loading sequence must publish it too.
func (w *worldSession) dungeonOathSelectionPacket(ctx context.Context) (outboundPacket, error) {
	selected, err := w.characters.Store.EquippedOathSelection(ctx, w.account, w.role.ID)
	if err != nil {
		return outboundPacket{}, err
	}
	info, err := protocol.OathSystemInfo(selected.Level, selected.Option)
	if err != nil {
		return outboundPacket{}, err
	}
	return outboundPacket{"dungeon_oath_selection_restored", 0, 2839, info}, nil
}
