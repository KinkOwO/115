package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI13, native1452d5a80: inventory kind2, u16 slot capacity, u16 item
// count. Empty rows skip 181-byte item structures and have no further reads.
func EmptyPersonalVault(slots uint16) ([]byte, error) {
	return PersonalVault(slots, nil)
}

// Kind 2 reads capacity, count and exactly 181 bytes per row. Native
// 1452d61da and 1459a01e0/1459a0220 confirm there is no avatar/period tail.
// 145ad8d10 bounds-checks zero-based slots against capacity.
func PersonalVault(slots uint16, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	if slots == 0 {
		return nil, fmt.Errorf("vault capacity must select a valid client grade")
	}
	for _, row := range rows {
		if binary.LittleEndian.Uint16(row[:]) >= slots {
			return nil, fmt.Errorf("vault slot outside capacity")
		}
	}
	p, e := itemRows(rows)
	if e != nil {
		return nil, e
	}
	return append(add16([]byte{2}, slots), p...), nil
}

// PersonalVaultRestore retains the upstream API with slot validation.
func PersonalVaultRestore(slots uint16, rows [][CurrentItemRecordSize]byte) ([]byte, error) {
	return PersonalVault(slots, rows)
}
