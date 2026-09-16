package protocol

import "fmt"

// NOTI13, native1452d5a80: inventory kind2, u16 slot capacity, u16 item
// count followed by 181-byte item structures. Empty rows skip item structures.
func PersonalVaultRestore(slots uint16, items [][CurrentItemRecordSize]byte) ([]byte, error) {
	if slots == 0 {
		return nil, fmt.Errorf("vault capacity must select a valid client grade")
	}
	if len(items) > 65535 {
		return nil, fmt.Errorf("too many vault items")
	}
	p := add16(add16([]byte{2}, slots), uint16(len(items)))
	for _, r := range items {
		p = append(p, r[:]...)
	}
	return p, nil
}

func EmptyPersonalVault(slots uint16) ([]byte, error) {
	return PersonalVaultRestore(slots, nil)
}
