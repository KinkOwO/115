package protocol

import "encoding/binary"

// STACKABLE_DUNGEON_LIMIT（NOTI1584）：副本消耗品许可。
//
// 决定性证据链：官服森林进图 21:42:18.601 实发本包，明文首字段 u32 = **8**
// （= 每关消耗品使用上限，与军团口径一致）；伊斯进图 replay 也带它（伊斯副本
// 能用药）；苏醒之森此前没发 → 客户端本地禁用全部消耗品（0312/0334/0953
// 会话零 CMD44，N28 48B 修复无效——限制载体不是 N28）。
// 官服原文：08 000000 | 0e e3 e4 af | 3a 000000 | 00000000
// （@0=8 上限；@4 起为会话变化值，按随机种子填充）。
func StackableDungeonLimit(limit uint32, seed uint32) []byte {
	p := make([]byte, 16)
	binary.LittleEndian.PutUint32(p[0:4], limit)
	binary.LittleEndian.PutUint32(p[4:8], seed)
	return p
}
