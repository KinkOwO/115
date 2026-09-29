package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD 205 = ENUM_CMDPACKET_INVEST_ITEM_AMPLIFY_OPTION —— 使用增幅书（红字书）给装备打次元属性。
//
// 实机抓包（2026-09-27 19:31~19:39，玩家三次尝试）解出的布局（共 45 字节）：
//
//	u32 @0  未知（三次都是 0）
//	u8  @4  未知（三次都是 2）
//	u16 @5  目标装备的槽位（三次分别 24 / 21 / 25）
//	u32 @7  目标装备的模板（108020744 / 100146510）
//	u16 @11 增幅书在背包里的槽位（82 / 77）
//	u32 @13 增幅书的模板（三次都是 10356325 = 玩家自己那本不过期的增幅书）
//	u8  @17 选定的次元属性类型（3 = 力量，取值集合 {1,2,3,4} = 体力/精神/力量/智力，
//	         与客户端 dstr 21306..21309 一一对应）
//	其余为 0
type AmplifyOptionRequest struct {
	Reserved          uint32
	Aux               byte
	EquipmentSlot     uint16
	EquipmentTemplate uint32
	BookSlot          uint16
	BookTemplate      uint32
	Type              byte
}

func DecodeAmplifyOption(p []byte) (AmplifyOptionRequest, error) {
	var r AmplifyOptionRequest
	if len(p) < 18 {
		return r, fmt.Errorf("增幅选项请求长度不足: %d", len(p))
	}
	r.Reserved = binary.LittleEndian.Uint32(p[0:])
	r.Aux = p[4]
	r.EquipmentSlot = binary.LittleEndian.Uint16(p[5:])
	r.EquipmentTemplate = binary.LittleEndian.Uint32(p[7:])
	r.BookSlot = binary.LittleEndian.Uint16(p[11:])
	r.BookTemplate = binary.LittleEndian.Uint32(p[13:])
	r.Type = p[17]
	return r, nil
}

// AmplifyOptionReply 是 CMD 205（增幅 / 打红字）的成功回包。
//
// 布局来自客户端真实 handler sub_145282510（由 sub_1459A2FB0(a1, 205,
// sub_145282510) 注册进 S2C 命令表）。它与 CMD80 的 handler sub_14529B2F0 是
// 两个不同的函数 —— 所以强化回包那套 35 字节布局不能套用到增幅上。
// 分派器已消费掉体首的 [0] 状态字节，handler 从 [1] 开始读：
//
//	u8  @1      结果码 v22：1=只刷新不播动画；非 1 会调用增幅动画 sub_145A61000；
//	           2 再额外读一个字节并播放特效 + 触发 UI 事件 12。成功要播动画，故取 2。
//	u16 @2..3   增幅书槽位（随后按 [4..7] 的剩余量刷新该格；剩余为 0 时客户端移除该格）
//	u32 @4..7   增幅书剩余数量
//	u32 @8..11  装备所在容器（sub_145A0E750 按此值选容器：0/28/35=背包，3=已穿戴）
//	u16 @12..13 装备槽位
//	u8  @14     次元属性类型 1..4 —— 为 0 时客户端整段跳过，红字不会生效
//	u16 @15..16 次元属性数值
//	u8  @17     特效文案用的数值（仅 [1]==2 时才会被读取）
//
// 旧实现（照抄强化回包的 35 字节布局）恰好解释实机的两个症状：它把「装备空间」
// 放到了 [14]，背包件空间为 0 会让客户端整段跳过 —— 红字永远停在 +0；同时
// [8..13] 是 0xffff/1/旧类型，装备对象查不到，动画自然不播。
func AmplifyOptionReply(equipSpace byte, equipSlot uint16, bookSlot uint16, bookRemaining uint32, ampType, value byte) []byte {
	p := []byte{1, 2}
	p = binary.LittleEndian.AppendUint16(p, bookSlot)
	p = binary.LittleEndian.AppendUint32(p, bookRemaining)
	p = binary.LittleEndian.AppendUint32(p, uint32(equipSpace))
	p = binary.LittleEndian.AppendUint16(p, equipSlot)
	p = append(p, ampType)
	p = binary.LittleEndian.AppendUint16(p, uint16(value))
	p = append(p, value)
	return p
}
