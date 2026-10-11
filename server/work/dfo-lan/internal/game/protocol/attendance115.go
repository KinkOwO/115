package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI1379 —— 每日签到的**状态帧**（ch0 推送，没有对应的 ch1 请求）。
//
// ## 载荷几何（L0，本机 IDB 逐条读取指令确认）
//
// handler = `sub_143869190`（由活动 331 的类构造 `sub_143868FC0` 在 0x143869014 注册，
// 注册助手 `sub_14599D5D0`，registry `qword_14E66C090`）。它的开头就是三次读流：
//
//	sub_146EA09F0(&v12, 1)          → p[0]      flag       (u8)
//	sub_146EA0BA0(&v14)             → p[1:5]    attended   (u32，助手默认读 4 字节)
//	for (i = 0; i < 28; ++i) { 每次 sub_146EA09F0(&v13, 1) }  → p[5:33] day_state[28]
//
// ⇒ **载荷恒 33 字节 = 1 + 4 + 28**。文档《活动开启修复.md》§3.1 给的布局与之一致
// （包括字段顺序），没有偏差。
//
// 读完之后 handler 做的四件事（同一份 IDB 证据）：
//
//	*(_BYTE *)(a2 + 88) = 0;                 // +0x58 在途/待处理标志清零
//	*(_BYTE *)(a2 + 72) = (…|| flag == 1);   // +0x48：flag 非 0 时置 1（文档：强制红点并影响活动 332）
//	*(_DWORD *)(a2 + 76) = v14;              // +0x4C = attended
//	qmemcpy((void *)(a2 + 89), v17, 28);     // +0x59 = day_state[28]
//
// 并且结尾会 `sub_14667BB90(qword_14E683C78, 77, 0)` 取**窗口 77** 的对象调它的
// `vtbl+552` —— 也就是**收到这一帧本身就会去动签到窗口**。所以服务端推的时机很重要：
// 窗口对象不存在时这一调用是空转（取对象返回 0），push 太早就等于没推。
//
// ## 与活动 331 的绑定
//
// 该 handler 属于活动 331（`AttendanceDailyEvent`）的类，只有 **331 在 NOTI108 表里打开**
// 时客户端才会注册它（见 cmd/wireprobe/event_info_activity.go）。所以"推 1379"之前
// 必须先有"108 里有 331 行"。
const (
	// AttendanceDailyStateBytes 是 NOTI1379 的**固定**载荷长度（客户端只读这 33 字节）。
	AttendanceDailyStateBytes = 33
	// AttendanceDailyDays 是脚本里 [reward info] 的条数（day 0..6）。
	AttendanceDailyDays = 7
	// AttendanceDailySlots 是 day_state 的槽位数（客户端恒读 28 个）。
	AttendanceDailySlots = 28
)

// AttendanceDailyStatus115 是 NOTI1379 的**语义**输入。
type AttendanceDailyStatus115 struct {
	// Flag 是 p[0]。官服样本全 0；非 0 会把客户端的 +0x48 置 1
	// （文档 §3.1：强制置红点并影响活动 332）。没有更多样本前一律发 0。
	Flag byte
	// Attended 是 p[1:5]。官服样本把**当前所在的那一天**写在这里，从 1 起：
	// 新号 = 1，前 3 天已领/第 4 天可领 = 4，第 4 天领完后仍是 4。见 AttendanceDailyProgress115。
	Attended uint32
	// DayState 是 p[5:33]。每槽 0 = 未到 / 1 = 可领（按钮可点）/ 2 = 已领。
	DayState [AttendanceDailySlots]byte
}

// AttendanceDailyState115 编码 NOTI1379 的 33 字节载荷。
//
// 只在这里做形状校验（day_state 的取值域 0..2），不做业务判断：调用方决定"第几天可领"。
func AttendanceDailyState115(st AttendanceDailyStatus115) ([]byte, error) {
	for i, v := range st.DayState {
		if v > 2 {
			return nil, fmt.Errorf("attendance day state [%d]=%d out of range 0..2", i, v)
		}
	}
	out := make([]byte, AttendanceDailyStateBytes)
	out[0] = st.Flag
	binary.LittleEndian.PutUint32(out[1:], st.Attended)
	copy(out[5:], st.DayState[:])
	return out, nil
}

// AttendanceDailyProgress115 由「已领天数 + 今天还能不能领」推出官服口径的
// attended / day_state。
//
// 口径由**两个官服样本**钉死（`[event period]` 那一期，见 §3.1）：
//
//	新号（进城）              attended=1  day_state = 01 00×27
//	前 3 天已领、第 4 天可领    attended=4  day_state = 02 02 02 01 00×24
//	领完第 4 天之后（同日）      attended=4  day_state = 02 02 02 02 00×24
//
// 两条规则同时满足三个样本：
//   - day_state[i] = 2（i < claimed）/ 1（i == claimed 且今天可领）/ 0（其余）；
//   - attended = claimed + (今天可领 ? 1 : 0)。
//
// `[allow accumulate attendance] 1`（漏签不清零）⇒ claimed 单调递增，"今天能不能领"
// 由**换日**单独决定（客户端按 06:00 UTC 换日，见 cmd/wireprobe/attendance_flow.go）。
func AttendanceDailyProgress115(claimed int, available bool) AttendanceDailyStatus115 {
	if claimed < 0 {
		claimed = 0
	}
	if claimed > AttendanceDailySlots {
		claimed = AttendanceDailySlots
	}
	// 本期只有 AttendanceDailyDays 天：领满了就不再"今天可领"，否则会把
	// day_state[7] 点亮成一个脚本里根本没有的第 8 天。这条是**结构性**的，
	// 不靠调用方记得判 —— 与 [reward info] 的条数同源。
	if claimed >= AttendanceDailyDays {
		available = false
	}
	st := AttendanceDailyStatus115{}
	for i := 0; i < AttendanceDailySlots; i++ {
		switch {
		case i < claimed:
			st.DayState[i] = 2
		case i == claimed && available:
			st.DayState[i] = 1
		default:
			st.DayState[i] = 0
		}
	}
	st.Attended = uint32(claimed)
	if available {
		st.Attended++
	}
	return st
}
