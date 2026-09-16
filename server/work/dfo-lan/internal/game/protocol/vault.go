package protocol

import "fmt"

// NOTI13, native1452d5a80: inventory kind2, u16 slot capacity, u16 item
// count. Empty rows skip 181-byte item structures and have no further reads.
func EmptyPersonalVault(slots uint16) ([]byte, error) {
	if slots == 0 {
		return nil, fmt.Errorf("vault capacity must select a valid client grade")
	}
	return add16(add16([]byte{2}, slots), 0), nil
}
