package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"dfolan/internal/workflow"
	"fmt"
	"time"
)

type sortSession struct {
	nonce       [16]byte
	initialized bool
}

// handleSortItem adopts the bag arrangement the client already applied locally
// (CMD20 SORT_ITEM) and answers with the authoritative bag restore.
//
// The command used to be dropped entirely, which left the client's own
// "inventory in use" latch set until the next reconnect: every later inventory
// action, equipment disassembly included, was then refused locally with DSTR
// 1659 ("The Inventory is currently in use."). Answering it is what clears the
// latch, so a replayed frame is answered too.
func (s *sortSession) handle(service *inventory.WearService, w *worldSession, p, raw []byte) ([]outboundPacket, error) {
	if service == nil || w == nil || w.role.ID == 0 {
		return nil, fmt.Errorf("item sort requires the owned character")
	}
	r, e := protocol.DecodeSortItem(p)
	if e != nil {
		return nil, e
	}
	if !s.initialized {
		if _, e = rand.Read(s.nonce[:]); e != nil {
			return nil, e
		}
		s.initialized = true
	}
	hash := sha256.Sum256(raw)
	key := fmt.Sprintf("itemsort:%x:%x", s.nonce, hash)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	saved, applied, e := workflow.SortBag(ctx, w.store, w.role, service.BagRules, key, r)
	if e != nil {
		return nil, e
	}
	w.role = saved
	b, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	bagBody, e := protocol.InventoryRestore(b.Rows(), b.Expansion)
	if e != nil {
		return nil, e
	}
	// A replay still gets the answer: the client's latch clears on the answer,
	// not on the state change.
	name := "item_sort_committed"
	if !applied {
		name = "item_sort_replayed"
	}
	return []outboundPacket{
		{name, 1, 20, protocol.SortItemSuccess()},
		{"item_sort_inventory_restored", 0, 13, bagBody},
	}, nil
}
