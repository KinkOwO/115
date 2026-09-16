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

// SellItemRequest is the 24-byte payload (20B content + 4B pad) for CMD22
// (ENUM_CMDPACKET_SELL_ITEM).
type SellItemRequest struct {
	NpcID    uint32 // NPC identifier (observed 1)
	ActorID  uint32 // Actor identifier (observed 2)
	Flag     byte   // Observed 1
	List     byte   // Inventory list type, observed 0 (main bag)
	Slot     uint16 // Inventory slot
	Template uint32 // Category flag (observed constant 1, not item identity)
	Price    uint32 // Actual item identity context
}

// DecodeSellItem decodes a 24-byte CMD22 request.
func DecodeSellItem(p []byte) (SellItemRequest, error) {
	var r SellItemRequest
	if len(p) != 24 {
		return r, fmt.Errorf("sell item requires 24 bytes")
	}
	for _, b := range p[20:] {
		if b != 0 {
			return r, fmt.Errorf("nonzero sell item padding")
		}
	}
	r.NpcID = binary.LittleEndian.Uint32(p[0:])
	r.ActorID = binary.LittleEndian.Uint32(p[4:])
	r.Flag = p[8]
	r.List = p[9]
	r.Slot = binary.LittleEndian.Uint16(p[10:])
	r.Template = binary.LittleEndian.Uint32(p[12:])
	r.Price = binary.LittleEndian.Uint32(p[16:])
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
	List     byte
	Slot     uint16
	Template uint32
}

// SellItemSuccess builds the 16-byte ACK for CMD22 for a single sold item:
// u8(1) + u32(goldGained) + u32(count=1) + count x (u8 list, u16 slot, u32 template).
// Multi-item path is unsupported on CMD22 and rejected with an error.
func SellItemSuccess(goldGained uint32, items []SoldItem) ([]byte, error) {
	if len(items) != 1 {
		return nil, fmt.Errorf("unsupported multi-item path: count=%d", len(items))
	}
	p := make([]byte, 0, 16)
	p = append(p, 1)
	p = add32(p, goldGained)
	p = add32(p, uint32(len(items)))
	for _, it := range items {
		p = append(p, it.List)
		p = add16(p, it.Slot)
		p = add32(p, it.Template)
	}
	return p, nil
}
