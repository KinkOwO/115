package main

import (
	"context"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"fmt"
	"time"
)

func titleBookRestore(raw []byte) ([][]byte, error) {
	b, err := inventory.ReadBag(raw)
	if err != nil {
		return nil, err
	}
	out := [][]byte{}
	// Empty categories must also clear the previous character's session book.
	for category := byte(0); category < 5; category++ {
		p, e := b.TitleBookPayload(category)
		if e != nil {
			return nil, e
		}
		out = append(out, p)
	}
	return out, nil
}
func (w *worldSession) titleBookCommand(op uint16, p []byte, slots [2]uint16, event func(map[string]any)) ([]outboundPacket, error) {
	if w == nil || w.characters == nil {
		return nil, fmt.Errorf("title book before character selection")
	}
	b, err := inventory.ReadBag(w.role.State)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	key := fmt.Sprintf("title-book-v1:%d:%d:%d:%x", w.role.ID, b.TitleBookSeq, op, p)
	if op == 560 {
		template, e := protocol.DecodeTitleAppearance(p)
		if e != nil {
			return nil, e
		}
		saved, applied, e := w.characters.ApplyTitleAppearance(ctx, w.role, key, template)
		if e != nil {
			return nil, e
		}
		w.role = saved
		event(map[string]any{"kind": "title_appearance_saved", "character_id": saved.ID, "template": template, "applied": applied})
		w.titleRestoreCharacter = 0
		// Native 144249220 sets own-info clone title, then broadcasts UDP88.
		// It skips both actions when the current actor is absent. Do not rebuild
		// the actor after the successful cosmetic selection.
		return []outboundPacket{{"title_appearance_changed", 1, 560, protocol.TitleAppearanceSuccess(template)}}, nil
	}
	r, e := protocol.DecodeTitleBookTransfer(p)
	if e != nil {
		return nil, e
	}
	saved, applied, e := w.characters.TransferTitleBook(ctx, w.role, key, r, op == 404, slots)
	if e != nil {
		return nil, e
	}
	w.role = saved
	next, e := inventory.ReadBag(saved.State)
	if e != nil {
		return nil, e
	}
	book, e := next.TitleBookPayload(byte(r.Category))
	if e != nil {
		return nil, e
	}
	ordinary, e := protocol.InventoryRestore(next.Rows(), next.Expansion)
	if e != nil {
		return nil, e
	}
	// The reply moves the existing source object locally. Send it before replacing
	// either container, then feed both authoritative snapshots.
	plan := []outboundPacket{{"title_book_transfer_ack", 1, op, protocol.TitleBookTransferSuccess(r)}, {"title_book_restored", 0, 344, book}, {"title_book_inventory_refreshed", 0, 13, ordinary}}
	if r.Space == 3 {
		worn, e := inventory.WornPayload(saved.State)
		if e != nil {
			return nil, e
		}
		plan = append(plan, outboundPacket{"title_worn_refreshed", 0, 13, worn})
	}
	look, e := w.characters.AppearanceProbe(saved, [2]byte{})
	if e != nil {
		return nil, e
	}
	plan = append(plan, outboundPacket{"title_actor_refreshed", 0, 2, look}, outboundPacket{"title_appearance_restored", 1, 560, protocol.TitleAppearanceSuccess(next.TitleSkin)})
	event(map[string]any{"kind": "title_book_transfer_saved", "character_id": saved.ID, "opcode": op, "template": r.Template, "category": r.Category, "book_slot": r.BookSlot, "applied": applied})
	return plan, nil
}
