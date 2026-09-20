package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// disjointItem answers CMD26 (ENUM_CMDPACKET_DISJOINT_ITEM).
// It deletes requested equipment, awards clear cube fragments into the
// account material storage (they never stay in the bag: the 115 client pins
// the seventeen shared materials to account list 35), returns the ACK with
// deleted slots and rewards, then republishes the authoritative list35
// storage snapshot followed by the list0 bag snapshot so the client harvest
// moves the stacks into the soul-storage panel.
func (w *worldSession) disjointItem(p []byte) ([]outboundPacket, error) {
	if w == nil || w.role.ID == 0 || w.loot == nil {
		return nil, fmt.Errorf("disjoint service unavailable")
	}
	r, e := protocol.DecodeDisjointItem(p)
	if e != nil {
		return nil, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, _, e := w.loot.Disjoint(ctx, w.role, r)
	if e != nil {
		return nil, e
	}
	// The disjoint reward templates are account-shared materials; move them
	// out of the bag into the account storage before any snapshot is built.
	saved, materials, e := sweepAccountMaterials(ctx, w.loot.Store, saved)
	if e != nil {
		return nil, e
	}
	for i := range receipt.Rewards {
		receipt.Rewards[i].Slot = storageDestinationSlot(receipt.Rewards[i].Template, receipt.Rewards[i].Slot)
	}
	ack, e := protocol.DisjointItemSuccess(protocol.DisjointItemResult{
		DeletedSlots: receipt.DeletedSlots,
		List:         0,
		ToolSlot:     receipt.ToolSlot,
		Rewards:      receipt.Rewards,
	})
	if e != nil {
		return nil, e
	}
	refresh, e := accountMaterialRefreshPackets(materials, saved)
	if e != nil {
		return nil, e
	}
	w.role = saved
	plan := []outboundPacket{{"disjoint_item_ack", 1, 26, ack}}
	for _, p := range refresh {
		p.Name = "disjoint_item_" + p.Name
		plan = append(plan, p)
	}
	return plan, nil
}
