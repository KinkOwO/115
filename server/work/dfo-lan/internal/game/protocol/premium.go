package protocol

import "encoding/binary"

// CeraSpecialItemNotification encodes ENUM_NOTIPACKET_CERA_SPECIALITEM (NOTI 66, 0x0042).
// Per 115 client native sub_1452C6E00:
// - u16 mode: 2 (activation)
// - u8 premium_type: 22=Conqueror, 27=Tactician, 73=Gabriel, 79=Growth, 92=Cube, 117=NeoBasic, 118=NeoPlus
// - i64 end_time: 8-byte unix timestamp
func CeraSpecialItemNotification(premiumType uint8, endTime int64) []byte {
	p := make([]byte, 11)
	binary.LittleEndian.PutUint16(p[0:2], 2)
	p[2] = premiumType
	binary.LittleEndian.PutUint64(p[3:11], uint64(endTime))
	return p
}
