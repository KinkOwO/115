package protocol

import "encoding/binary"

// ForestDungeonInfo is the 苏醒之森-specific NOTI28: the **official 48-byte
// forest shape**.
//
// 为什么要专属形状：通用 DungeonInfo 是 41B 短版，客户端在森林副本里因此把
// 消耗品全部本地禁用（私服 0312/0334 会话零 CMD44；官服同副本玩家正常用药 3 次）。
// 48B 形状解锁消耗品后，服务端再按军团口径限制每关 8 次（forest_flow 计数）。
//
// ⚠️ 2026-10-09 逐字节重校（业主实机第五轮，进图后 HUD 全丢的真因）：
// 本函数此前把 @19..26 写成 ff×8、并把官服常量 05 放在 @31，导致**整段右移一字节**。
// 三方对照（官服 Normal 2026-10-02 21:42:18.606 / 官服 Extreme 2026-10-08 22:05:45.580
// / 本仓实发）显示：**官服两份抓包这一帧除副本 id 与尾部动态值外逐字节相同**，
// 而本仓从 @26 起错位：
//
//	idx  本仓(旧) 官服      说明
//	 19..25 ff×7   ff×7     ✓ 一致
//	 26     ff      00      ← 旧实现多写了这个 ff，把后面全部推后一格
//	 27..29 01 00 00 01 00 00 ✓
//	 30     00      05      ← ★ 客户端 native 单读这一字节（0x1452ada9e / offset=30）
//	 31..36 05 01 00 01 00 01   官服 31..36 = 01 00 01 00 01 00
//	 37..40（旧为 38..41）ff ff ff ff
//	 41..45（旧为 42..45 + 46）随机种子（官服这 5 字节是会话变化值）
//
// 也就是说旧帧把「入口类型/许可」那一字节写到了 @31（客户端不读的位置），@30 却是 0。
// 官服 Normal 与 Extreme **两个难度**在这 48 字节上完全一致，所以本函数不区分难度。
//
// 字段对照（偏移相对正文，均已按上述两份抓包复核）：
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
//	@19 7B    ff×7（官服常量）
//	@26 u8    00（官服常量）
//	@27 u16   0100（官服常量）
//	@29 u8    00
//	@30 u8    05（官服常量；本仓客户端在该偏移单独读它）
//	@31 6B    01 00 01 00 01 00（官服常量——疑似三类消耗品许可）
//	@37 u32   ffffffff
//	@41 4B    随机种子（官服该位置起是会话变化值）
//	@45 u8    官服为动态字节（Normal 0x36 / Extreme 0x42），语义未回收 ⇒ 本仓写 0
//	@46 u16   0
func ForestDungeonInfo(id uint32, maze byte, boss [2]byte, seed uint32) []byte {
	p := make([]byte, 48)
	binary.LittleEndian.PutUint32(p[0:4], id)
	p[7] = maze
	p[8], p[9] = boss[0], boss[1]
	p[10], p[11] = 0xff, 0xff
	p[16], p[17] = 0x01, 0x00
	p[18] = 0x0b
	for i := 19; i < 26; i++ {
		p[i] = 0xff
	}
	p[26] = 0x00
	p[27], p[28] = 0x01, 0x00
	p[30] = 0x05
	copy(p[31:37], []byte{0x01, 0x00, 0x01, 0x00, 0x01, 0x00})
	p[37], p[38], p[39], p[40] = 0xff, 0xff, 0xff, 0xff
	binary.LittleEndian.PutUint32(p[41:45], seed)
	return p
}
