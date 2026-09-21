package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"strings"
	"time"
)

// unsealRandomOption answers CMD393 (UNSEAL_RANDOM_OPTION). The native client
// sends it when a magic-sealed equipment is right-clicked; until this handler
// existed the request met silence and the item stayed sealed. The durable roll
// commits first, then the acknowledgement and the authoritative single-row
// inventory update the client's sealed overlay clears on.
func (w *worldSession) unsealRandomOption(s *inventory.UnsealService, version string, p []byte) ([]outboundPacket, protocol.UnsealRequest, error) {
	var none protocol.UnsealRequest
	if w == nil || w.role.ID == 0 || s == nil {
		return nil, none, fmt.Errorf("unseal before character selection")
	}
	r, e := protocol.DecodeUnseal(p)
	if e != nil {
		return nil, none, e
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, receipt, _, e := s.Unseal(ctx, w.role, version, r)
	if e != nil {
		return nil, r, e
	}
	var row [protocol.CurrentItemRecordSize]byte
	copy(row[:], receipt.Record)
	update, e := protocol.InventoryUpdate([][protocol.CurrentItemRecordSize]byte{row})
	if e != nil {
		return nil, r, e
	}
	w.role = saved
	return []outboundPacket{
		{"unseal_ack", 1, 393, protocol.UnsealSuccess()},
		{"unseal_inventory_updated", 0, 14, update},
	}, r, nil
}

// unsealRefusalCode maps a failed unseal to the client's message code:
// 4 invalid target, 10 insufficient gold, 13 unsupported request.
func unsealRefusalCode(e error) uint16 {
	if e == nil {
		return protocol.UnsealRefusedInvalidTarget
	}
	msg := e.Error()
	switch {
	case strings.Contains(msg, "insufficient gold"):
		return protocol.UnsealRefusedInsufficientGold
	case strings.Contains(msg, "scroll slot"), strings.Contains(msg, "not proven"),
		strings.Contains(msg, "not source magic-seal"), strings.Contains(msg, "no option"),
		strings.Contains(msg, "no break seal cost"), strings.Contains(msg, "source mismatch"):
		return protocol.UnsealRefusedUnsupported
	default:
		return protocol.UnsealRefusedInvalidTarget
	}
}
