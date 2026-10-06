package protocol

import "encoding/binary"

// ForestDungeonInfo is the苏醒之森-specific NOTI28: the **official 48-byte
// forest shape** (2026-10-02 capture 21:42:18.606, dungeon 100003878).
//
// 为什么要专属形状：通用 DungeonInfo 是 41B 短版，客户端在森林副本里因此
// 把消耗品全部本地禁用（私服 0312/0334 会话零 CMD44；官服同副本玩家正常
// 用药 3 次——官服 48B 版本携带的许可字段在短版里缺省为禁）。48B 形状解锁
// 消耗品后，服务端再按军团口径限制每关 8 次（forest_flow 计数）。
//
// 字段对照（偏移相对正文）：
//
//	@0  u32   dungeon id（按关替换）
//	@4  u8    difficulty（0=normal）
//	@5  u16   0
//	@7  u8    maze index（森林 0）
//	@8  u8    boss X（森林 0）
//	@9  u8    boss Y（森林 0）
//	@10 u16   hell position ffff（无地狱）
//	@12 u32   0
//	@16 u16   0100（官服常量）
//	@18 u8    0b（官服常量——短版缺失字段之一）
//	@19 8B    ff×8（官服常量）
//	@27 u16   0100（官服常量）
//	@29 u16   0
//	@31 u8    05（官服常量）
//	@32 6B    01 00 01 00 01 00（官服常量——疑似三类消耗品许可）
//	@38 u32   ffffffff
//	@42 u32   随机种子（官服该位置为会话变化值）
//	@46 u16   0
func ForestDungeonInfo(id uint32, maze byte, boss [2]byte, seed uint32) []byte {
	p := make([]byte, 48)
	binary.LittleEndian.PutUint32(p[0:4], id)
	p[7] = maze
	p[8], p[9] = boss[0], boss[1]
	p[10], p[11] = 0xff, 0xff
	p[16], p[17] = 0x01, 0x00
	p[18] = 0x0b
	for i := 19; i < 27; i++ {
		p[i] = 0xff
	}
	p[27], p[28] = 0x01, 0x00
	p[31] = 0x05
	copy(p[32:38], []byte{0x01, 0x00, 0x01, 0x00, 0x01, 0x00})
	p[38], p[39], p[40], p[41] = 0xff, 0xff, 0xff, 0xff
	binary.LittleEndian.PutUint32(p[42:46], seed)
	return p
}
