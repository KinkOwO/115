package protocol

import "encoding/binary"

// ImageCommunicationAck matches CMD467's native response reader at
// 0x1452812C0. The generic CMD acknowledgement dispatcher consumes the first
// byte as the success flag (as in CMD31); the handler then consumes two u32s
// and passes the second to the NPC summon routine. The first u32 is unused.
func ImageCommunicationAck(npc uint32) []byte {
	p := make([]byte, 9)
	p[0] = 1
	binary.LittleEndian.PutUint32(p[5:], npc)
	return p
}
