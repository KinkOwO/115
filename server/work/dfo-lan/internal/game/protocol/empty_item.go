package protocol

import "encoding/binary"

// EmptyOrdinaryItem returns a 181-byte record representing an empty/deleted slot.
// In DFO NOTI 14 (sub_1452E9810), template ID at offset 2 being 0xFFFFFFFF (-1)
// triggers sub_145AD4750 to delete/clear the item at slot.
func EmptyOrdinaryItem(slot uint16) [CurrentItemRecordSize]byte {
	var p [CurrentItemRecordSize]byte
	binary.LittleEndian.PutUint16(p[:], slot)
	binary.LittleEndian.PutUint32(p[2:], 0xFFFFFFFF)
	return p
}
