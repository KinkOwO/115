package protocol

import "encoding/binary"

// N1584 reader1452b55e0 consumes one u32; setter145f06c60 resets used count
// and stores this maximum. Captured trailing bytes are not native operands.
func StackableDungeonLimit115(limit uint32) []byte {
	p := make([]byte, 4)
	binary.LittleEndian.PutUint32(p, limit)
	return p
}
