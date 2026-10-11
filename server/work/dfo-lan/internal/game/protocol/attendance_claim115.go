package protocol

import (
	"encoding/binary"
	"fmt"
)

// CMD680（通用活动领奖）在**签到**上的两种形态。这一族命令共享包头，
// 但 680 的载荷就是「活动号 + 行号」两个 u32，没有别的字段。
//
// 实机样本（2026-10-10 01:36 本机会话，业主点签到窗第 1 天的 Get）：
//
//	plain_hex = 4b010000 00000000      （8 字节）
//	            ^^^^^^^^ ^^^^^^^^
//	            331      行号 0
//
// ⇒ **就是两个 u32，没有补齐到 16 字节**（这一点与 op681 不同：681 线上补齐到 16）。
const AttendanceClaimRequestBytes = 8

// EventRequestEventID115 读**通用活动帧**（CMD680 领奖 / CMD681 查询）载荷里的活动号（首 u32）。
//
// 这一族是**多个活动共用**的：331 签到、313 HoneyTime、662/665 全是「u32 活动号 + …」。
// 谁都不能拿自己的校验去拒别人的请求 —— 判据只有这个首 u32。
func EventRequestEventID115(p []byte) (uint32, bool) {
	if len(p) < 4 {
		return 0, false
	}
	return binary.LittleEndian.Uint32(p), true
}

// DecodeAttendanceClaim115 读 CMD680 的领奖请求。返回活动号与**行号**（0 起）。
func DecodeAttendanceClaim115(p []byte) (event uint32, day uint32, err error) {
	if len(p) < AttendanceClaimRequestBytes {
		return 0, 0, fmt.Errorf("attendance claim request too short: %d", len(p))
	}
	return binary.LittleEndian.Uint32(p), binary.LittleEndian.Uint32(p[4:]), nil
}

// AttendanceClaimReply115 是 CMD680 的**成功**应答：
//
//	[u8 1][u32 活动号][10 × u32]      —— 共 45 字节
//
// 官服首个 u32 是**行号回显**，其余 9 个为 0（见《活动开启修复.md》§2 的 op680 行；
// 该文档记的是同一套 115 客户端）。行号回显必须写真实值：客户端的 331 对象虽然
// 只清在途标志、不读这些值（§3.2），但它是**唯一的**服务端回执，将来要排查
// "服务端认为领的是哪一天"时只有它可查。
func AttendanceClaimReply115(event, day uint32) []byte {
	out := make([]byte, 0, 1+4+40)
	out = append(out, 1)
	out = add32(out, event)
	for i := 0; i < 10; i++ {
		value := uint32(0)
		if i == 0 {
			value = day
		}
		out = add32(out, value)
	}
	return out
}
