// Package protocol 里本文件是活动 1093「额外掉落奖励」的两个 NOTI 编码。
//
// ## 载荷长度来自 IDA（L0），字段**语义未证**
//
// 反编译 handler（`analysis/tasks/next200-…md §4`，探针 `ida_probe_1093_notis.py`）：
//
//	2880 ADDITIONAL_DROP_REWARD_EVENT_INFO     sub_1401E3F40
//	     sub_146EA0BE0(&v4, 8)   → 写 obj+0x58（一个 u64，即**两个 u32**）
//	2881 ADDITIONAL_DROP_REWARD_EVENT_GET_REWARD sub_1401E3EF0
//	     读 9 字节 → 写 obj+0x60(8 B) / obj+0x68(1 B)
//
// 所以这里**只钉长度与编码方式**（小端、不补齐），字段叫什么由调用方决定；
// 本仓既有惯例可对照：同族的 `2838 ENDKEEPER_OF_ORDER_REWARD` 也是
// "8 B = 两个 u32"（见 `oath_probe.go` 的记录）⇒ 大端小端与对齐方式沿用同一条。
//
// ⚠️ **不要**给这两个 u32 起"看起来像契约"的名字（例如 `Count`/`Max`）——
// 语义要靠实机看面板怎么读 `obj+0x58` 才能钉；调用方用 `DFO_EXTRADROP_INFO` 开关
// 送不同取值做 A/B（本仓对"未证字段"的既定做法）。
package protocol

import "encoding/binary"

const (
	// ExtraDropEventInfoBytes 是 NOTI2880 的载荷长度。
	ExtraDropEventInfoBytes = 8
	// ExtraDropGetRewardBytes 是 NOTI2881 的载荷长度。
	ExtraDropGetRewardBytes = 9
)

// ExtraDropEventInfo115 编 NOTI2880 的载荷：两个小端 u32。
//
// 长度与 handler 里的读流调用逐个对齐（`sub_146EA0BE0(&v, 8)`）。
func ExtraDropEventInfo115(first, second uint32) []byte {
	out := make([]byte, ExtraDropEventInfoBytes)
	binary.LittleEndian.PutUint32(out[0:4], first)
	binary.LittleEndian.PutUint32(out[4:8], second)
	return out
}

// ExtraDropGetReward115 编 NOTI2881 的载荷：一个小端 u64 + 一个 u8。
//
// handler 读 9 字节并整块落进对象（`obj+0x60` 8 B、`obj+0x68` 1 B）。
func ExtraDropGetReward115(value uint64, flag byte) []byte {
	out := make([]byte, ExtraDropGetRewardBytes)
	binary.LittleEndian.PutUint64(out[0:8], value)
	out[8] = flag
	return out
}
