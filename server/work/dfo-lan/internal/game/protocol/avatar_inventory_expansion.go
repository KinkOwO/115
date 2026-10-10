package protocol

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Native 1472097C0 maps [avatar inventory expansion] to action 73.
// 1458D9170 limits the upgrade count to 65; 1458A7210 adds eight slots
// per upgrade to the original 120-slot avatar container.
const (
	AvatarInventoryExpansionAction uint32 = 73
	MaxAvatarInventoryExpansion    byte   = 65
)

func AvatarInventorySlots(tier byte) uint16 {
	return 120 + uint16(tier)*8
}

// NOTI66 type 15 and NOTI13 list 1 both carry the absolute extra-slot count.
func AvatarInventoryExpansionNotice(tier byte) ([]byte, error) {
	if tier > MaxAvatarInventoryExpansion {
		return nil, fmt.Errorf("invalid avatar inventory expansion")
	}
	return add16(add16(nil, 15), uint16(tier)*8), nil
}

func DecodeAvatarInventoryExpansion(p []byte) (uint16, error) {
	slot, action, err := DecodeStackableAction(p)
	if err != nil {
		return 0, err
	}
	if p[2] != 0 || action != AvatarInventoryExpansionAction {
		return 0, fmt.Errorf("invalid avatar inventory expansion request")
	}
	return slot, nil
}

// CMD507 success is consumed before the absolute ordinary-bag refresh:
// the native 1459350C0 handler spends one ticket and displays dstr 33160.
func AvatarInventoryExpansionSuccess(slot uint16) []byte {
	p := make([]byte, 8)
	p[0] = 1
	binary.LittleEndian.PutUint16(p[1:], slot)
	binary.LittleEndian.PutUint32(p[4:], AvatarInventoryExpansionAction)
	return p
}

// The common command wrapper consumes success/error; the native failure
// handler then reads the same u16 slot, u8 list and u32 action as success.
func AvatarInventoryExpansionRefused(slot uint16, full bool) []byte {
	code := byte(19)
	if full {
		code = 23 // 1459350C0: action 73 + error 23 displays dstr 33161.
	}
	p := make([]byte, 9)
	p[1] = code
	binary.LittleEndian.PutUint16(p[2:], slot)
	binary.LittleEndian.PutUint32(p[5:], AvatarInventoryExpansionAction)
	return p
}

// NOTI488 mode 0 (1452E5830) reads a u32 byte count and string via
// 146D78070, then opens UI2875, the same dialog used for action 73's
// dstr33160. That reader converts the bytes with the host ANSI code page
// (CP936 on a Simplified-Chinese machine) — the exact path mailbox.go
// documents for its body text — so the string MUST be GBK, not UTF-8.
// Send this after CMD64, whose handler closes UI2875 on purchase success.
func AvatarInventoryExpansionPurchaseMessage(tier byte) ([]byte, error) {
	if tier == 0 || tier > MaxAvatarInventoryExpansion {
		return nil, fmt.Errorf("invalid avatar inventory expansion success")
	}
	text := fmt.Sprintf("装扮物品栏已扩展，当前容量：%d 格。", AvatarInventorySlots(tier))
	// 与 adventure.go / mailbox.go 一致：发包边界转 GBK（存档无关）。
	// 此前误发原始 UTF-8 ⇒ 客户端按 CP936 解 ⇒ 全是乱码。
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(text))
	if err != nil {
		return nil, fmt.Errorf("avatar inventory expansion message encode: %w", err)
	}
	return append(add32([]byte{0}, uint32(len(encoded))), encoded...), nil
}
