package protocol

import "encoding/binary"

// 「部位积分」协议：S2C **2634 / 2635**（`ENUM_NOTIPACKET_CHARACTER_PART_SET_POINT` /
// `ENUM_NOTIPACKET_ACHIEVEMENT_PART_SET_POINT`）。
//
// 为什么会有这两个包（2026-10-04 取证）：用户实机在「装备库 → 誓约」页签看到
// `0/750次` 与「已添加的 誓约积分: 0」，而 opcode 全表里**没有任何 C2S 查询包**与该积分相关
// ⇒ 积分只能由服务端按部位推送，而本服务端**从未发过这两个包**（Go 侧与文档 0 命中）。
//
// 载荷几何（IDA 工作副本；handler `sub_1452C9840`（2634）/ `sub_1452C5A10`（2635），
// 注册点 `0x1452fac1f` / `0x1452fac36`，注册函数 `sub_1452F9420`）：
//
//	v6 = 0; v7 = 0;                              // u16 + u64（紧挨着，packed）
//	result = sub_146EA0BE0(&v6, 10);             // 客户端**精确读 10 字节**
//	v1 = sub_145F0BFA0(qword_14E683C08, v6);     // 用 u16 查"部位"对象
//	return sub_145F06760(v1, (u32)v7, HIDWORD(v7));   // 两个 u32 应用到该部位
//
// ⇒ **每包一个部位，共 10 字节**（小端）：
//
//	[0:2)  u16 部位号（键进 qword_14E683C08 的注册表）
//	[2:6)  u32 值A
//	[6:10) u32 值B
//
// 字段语义与自身键按上游后续取证：A=誓约积分，B=套装积分，自身键=0xFFFF。
// 本机反汇编已核对 10 字节 reader、两个 u32 落点与自身键初始化。
// 源积分与登录时机见 docs/protocol/upstream/oath-set-points-20261004.md §7。
const (
	// PartSetPointOpcode 是 2634（当前值）。
	PartSetPointOpcode uint16 = 2634
	// PartSetPointAchievementOpcode 是 2635（达成值）。
	PartSetPointAchievementOpcode uint16 = 2635
	// PartSetPointSize 是这两个包的载荷长度（客户端 sub_146EA0BE0(&buf, 10) 精确读 10）。
	PartSetPointSize = 10
)

// PartSetPoint 组 2634/2635 的单部位载荷：`u16 部位号 + u32 值A + u32 值B`。
//
// 只做几何、不做语义：调用方负责给出部位号与两个数值。
func PartSetPoint(part uint16, a, b uint32) []byte {
	body := make([]byte, PartSetPointSize)
	binary.LittleEndian.PutUint16(body[0:2], part)
	binary.LittleEndian.PutUint32(body[2:6], a)
	binary.LittleEndian.PutUint32(body[6:10], b)
	return body
}
