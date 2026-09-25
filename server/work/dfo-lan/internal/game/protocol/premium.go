package protocol

import "encoding/binary"

// CeraSpecialItemNotification 保留上游编码接口；原生 NOTI66 接收剩余秒数，不能传到期时间戳。
func CeraSpecialItemNotification(premiumType uint8, remainingSecond int64) []byte {
	p := make([]byte, 11)
	binary.LittleEndian.PutUint16(p[0:2], 2)
	p[2] = premiumType
	binary.LittleEndian.PutUint64(p[3:11], uint64(remainingSecond))
	return p
}
