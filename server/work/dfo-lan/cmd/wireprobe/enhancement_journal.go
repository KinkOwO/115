package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// NOTI14 updates only the supplied slots. WornSpaceUpdate omits removed
// equipment, so a destroyed worn slot needs the native -1 delete row.
func appendEnhancementRemoval(plan []outboundPacket, bag inventory.Bag, space byte, slot uint16) ([]outboundPacket, error) {
	if space != 3 {
		return plan, nil
	} // list0 already includes the removed slot.
	for _, gear := range bag.Worn {
		if gear.Slot == slot {
			return plan, nil
		} // replay must not delete a replacement.
	}
	body, err := protocol.InventorySpaceUpdate(space, [][protocol.CurrentItemRecordSize]byte{protocol.EmptyOrdinaryItem(slot)})
	if err != nil {
		return nil, err
	}
	return append(plan, outboundPacket{"enhancement_destroyed_worn", 0, 14, body}), nil
}

func (w *worldSession) appendEnhancementJournal(plan []outboundPacket, added []inventory.JournalRegistration, skipped []inventory.JournalSkip, event func(map[string]any)) []outboundPacket {
	if len(added) == 0 && len(skipped) == 0 {
		return plan
	}
	if event != nil {
		event(map[string]any{"kind": "enhancement_destroy_journal", "character_id": w.role.ID, "added": added, "skipped": skipped})
	}
	if len(added) == 0 {
		return plan
	}
	body, err := equipmentJournalEntryPayload(w.role, w.journalRules())
	if err != nil {
		// The mutation already committed. Keep the ACK and inventory updates;
		// entry restoration can resend the saved journal on the next login.
		if event != nil {
			event(map[string]any{"kind": "enhancement_journal_refresh_failed", "character_id": w.role.ID, "reason": err.Error()})
		}
		return plan
	}
	if len(body) > 0 {
		plan = append(plan, outboundPacket{"enhancement_journal_restored", 0, protocol.EquipmentJournalOpcode, body})
	}
	return plan
}
