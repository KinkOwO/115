package protocol

import (
	"encoding/binary"
	"fmt"
)

// BuyItemRequest is the 24-byte (6x u32) plaintext payload for CMD21
// (ENUM_CMDPACKET_BUY_ITEM) verified by live capture & disassembly of 0x1467e7b30.
type BuyItemRequest struct {
	Template uint32 // Item template ID (+00, esi)
	Count    uint32 // Purchase count (+04, ebp)
	NpcID    uint32 // NPC identifier (+08)
	ActorID  uint32 // Actor identifier (+12)
	Category uint32 // Category / shop context (+16)
	Reserved uint32 // Reserved (+20)
}

// DecodeBuyItem decodes a 24-byte CMD21 request.
func DecodeBuyItem(p []byte) (BuyItemRequest, error) {
	var r BuyItemRequest
	if len(p) != 24 {
		return r, fmt.Errorf("buy item requires 24 bytes")
	}
	r.Template = binary.LittleEndian.Uint32(p[0:])
	r.Count = binary.LittleEndian.Uint32(p[4:])
	r.NpcID = binary.LittleEndian.Uint32(p[8:])
	r.ActorID = binary.LittleEndian.Uint32(p[12:])
	r.Category = binary.LittleEndian.Uint32(p[16:])
	r.Reserved = binary.LittleEndian.Uint32(p[20:])

	if r.Template == 0 {
		return r, fmt.Errorf("buy item requires non-zero template")
	}
	if r.Count == 0 {
		return r, fmt.Errorf("buy item requires non-zero count")
	}
	return r, nil
}

// SellItemRow is one sale entry inside a CMD22 request.
type SellItemRow struct {
	List  byte   // Inventory list type, observed 0 (main bag)
	Slot  uint16 // Inventory slot
	Count uint32 // Quantity to remove, not a template ID
	Check uint32 // 2*(list+slot+count) + confirmation bit; never a price
}

// SellItemRequest is the CMD22 (ENUM_CMDPACKET_SELL_ITEM) payload for one or
// more sale rows: u32 npcId, u32 actorId, u8 rows, rows x (u8 list, u16 slot,
// u32 count, u32 check), then zero padding up to the next 8-byte boundary
// (the cipher works on 8-byte blocks: 9 + 11*rows bytes of content, rounded up,
// so 1 row is 24 bytes and 7 rows are 88).
//
// The plain Sell button frames the rows=1 case. The "Sell All" panel
// (MultiSellItemWindow) registers several stacks and confirms with a single
// packet carrying every registered row, so the decoder must accept rows >= 1
// instead of rejecting anything but a single row — that rejection was why
// "Sell All" silently did nothing: the server refused every attempt, so the
// client retried and then gave up.
type SellItemRequest struct {
	NpcID   uint32
	ActorID uint32
	Rows    []SellItemRow
}

const sellItemRowSize = 11

// DecodeSellItem decodes a CMD22 request with one or more sale rows.
func DecodeSellItem(p []byte) (SellItemRequest, error) {
	var r SellItemRequest
	if len(p) < 13 {
		return r, fmt.Errorf("sell item requires at least 13 bytes")
	}
	rows := int(p[8])
	if rows == 0 {
		return r, fmt.Errorf("sell item requires at least one row")
	}
	content := 9 + sellItemRowSize*rows
	if want := (content + 7) &^ 7; len(p) != want {
		return r, fmt.Errorf("sell item requires %d bytes for %d rows", want, rows)
	}
	for _, b := range p[content:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero sell item padding")
		}
	}
	r.NpcID = binary.LittleEndian.Uint32(p[0:])
	r.ActorID = binary.LittleEndian.Uint32(p[4:])
	r.Rows = make([]SellItemRow, 0, rows)
	// Native sender 1467fb0b0 (2026-09-26): a row count, then list, slot,
	// quantity and a check word per row. The low check bit records confirmation.
	for i := 0; i < rows; i++ {
		off := 9 + i*sellItemRowSize
		row := SellItemRow{
			List:  p[off],
			Slot:  binary.LittleEndian.Uint16(p[off+1:]),
			Count: binary.LittleEndian.Uint32(p[off+3:]),
			Check: binary.LittleEndian.Uint32(p[off+7:]),
		}
		if row.Count == 0 || row.Count > 0x7fffffff {
			return r, fmt.Errorf("sell item requires a positive quantity")
		}
		if row.Check&^1 != (uint32(row.List)+uint32(row.Slot)+row.Count)*2 {
			return r, fmt.Errorf("sell item check mismatch")
		}
		r.Rows = append(r.Rows, row)
	}
	return r, nil
}

// BuyItemSuccess builds the 209-byte successful ACK for CMD21 when bonusCount is 0:
// u8(1) + u32(npcId) + u32(actorId) + u32(template) + u32(count) + u32(category) +
// 181B item record + u8(bonusCount=0) + u32(newGold) + u16(slot).
func BuyItemSuccess(req BuyItemRequest, record [CurrentItemRecordSize]byte, newGold uint32, slot uint16) ([]byte, error) {
	if req.Template == 0 || req.Count == 0 {
		return nil, fmt.Errorf("invalid buy item request")
	}
	p := make([]byte, 0, 209)
	p = append(p, 1)
	p = add32(p, req.NpcID)
	p = add32(p, req.ActorID)
	p = add32(p, req.Template)
	p = add32(p, req.Count)
	p = add32(p, req.Category)
	p = append(p, record[:]...)
	p = append(p, 0) // bonusCount = 0
	p = add32(p, newGold)
	p = add16(p, slot)
	return p, nil
}

// SoldItem represents one item reported in a sell ACK.
type SoldItem struct {
	List  byte
	Slot  uint16
	Count uint32
}

// SellItemSuccess builds the ACK for CMD22 for one or more sold items:
// u8(1) + u32(newGold) + u32(rows) + rows x (u8 list, u16 slot, u32 quantity).
// Native reader 145294c70 sets the gold balance (vtable+d0) and removes
// quantity (vtable+f0), or clears the slot when its full stack was sold.
// The rows=1 case stays byte-identical to before (16 bytes); the "Sell All"
// panel confirms with a multi-row request and expects every sold row echoed.
func SellItemSuccess(newGold uint32, items []SoldItem) ([]byte, error) {
	if len(items) == 0 {
		return nil, fmt.Errorf("sell acknowledgement requires at least one item")
	}
	p := make([]byte, 0, 9+7*len(items))
	p = append(p, 1)
	p = add32(p, newGold)
	p = add32(p, uint32(len(items)))
	for _, it := range items {
		if it.Count == 0 {
			return nil, fmt.Errorf("sell acknowledgement requires a positive quantity")
		}
		p = append(p, it.List)
		p = add16(p, it.Slot)
		p = add32(p, it.Count)
	}
	return p, nil
}
