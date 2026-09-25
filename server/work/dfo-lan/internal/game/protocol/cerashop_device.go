package protocol

import (
	"encoding/binary"
	"fmt"
)

// The current client's (1,2036) reader consumes a fixed 0x88-byte window state.
const CeraShopDeviceStateSize = 0x88

// IsCeraShopDeviceAction recognizes the two captured 24-byte shop device
// requests: opening its window and pressing its open button.
func IsCeraShopDeviceAction(p []byte) bool {
	if len(p) != 24 {
		return false
	}
	return binary.LittleEndian.Uint32(p) == 0x27 || binary.LittleEndian.Uint32(p[8:]) == 7
}

// IsCeraShopDeviceRefresh separates the device's 8-byte CMD495 requests from
// the unrelated notice reports sharing that opcode.
func IsCeraShopDeviceRefresh(p []byte) bool {
	if len(p) != 8 || binary.LittleEndian.Uint32(p[4:]) != 1 {
		return false
	}
	action := binary.LittleEndian.Uint32(p)
	return action == 0x10 || action == 0x39
}

// CeraShopDeviceState fills the two counters consumed by the window. Remaining
// fields have no established server meaning in the available client evidence.
func CeraShopDeviceState(bonus, section uint32) ([]byte, error) {
	if bonus >= 10 || section >= 100 {
		return nil, fmt.Errorf("device counters out of range: bonus=%d section=%d", bonus, section)
	}
	p := make([]byte, CeraShopDeviceStateSize)
	binary.LittleEndian.PutUint32(p, bonus)
	binary.LittleEndian.PutUint32(p[4:], section)
	return p, nil
}
