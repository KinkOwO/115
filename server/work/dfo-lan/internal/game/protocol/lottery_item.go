package protocol

import (
	"encoding/binary"
	"fmt"
)

// DecodeLotteryItemUse follows the CMD27 sender: u16 source slot, u32 zero.
// Captured plaintext is 8 bytes because the encrypted block is padded.
func DecodeLotteryItemUse(p []byte) (uint16, error) {
	if len(p) != 6 && len(p) != 8 {
		return 0, fmt.Errorf("lottery use body has %d bytes", len(p))
	}
	slot := binary.LittleEndian.Uint16(p)
	if slot == 0 || slot == 1 {
		return 0, fmt.Errorf("lottery use has invalid source slot")
	}
	for _, b := range p[2:] {
		if b != 0 {
			return 0, fmt.Errorf("lottery use has nonzero reserved data")
		}
	}
	return slot, nil
}

// LotteryItemSuccess follows the 115 client reader sub_14529EDE0:
// success flag, consumed slot, then one 181-byte item record. The dispatcher
// sub_1459A1BB0 reads the u16 error code only when the success flag is zero;
// putting an error code in a success reply shifts every field in this reader.
// Gold and stackable rewards do not take the equipment-only trailing u32.
func LotteryItemSuccess(sourceSlot uint16, reward [CurrentItemRecordSize]byte) []byte {
	p := add16([]byte{1}, sourceSlot)
	return append(p, reward[:]...)
}
