package protocol

import (
	"encoding/binary"
	"fmt"
)

// NOTI1294 = `ENUM_NOTIPACKET_HONEY_TIME_EVENT_INFO`（活动 313 HoneyTime 的**状态帧**）。
//
// ## 形状（L0：handler `sub_1452D4B30` 的**逐条读流**）
//
// 用 `analysis/tools/ida_probe_read_stream.py` 列出每一次读调用，再**逐条核对循环边界的
// 反汇编**（`cmp ebx, 3 / jz`、`cmp ebx, 1 / jz`、`cmp ebx, 2 / jnz`、`cmp ebx, 4 / jl`），
// 得到恒定 **81 字节**（档数取自 `sub_143CC6090(obj, 1)`，它直接 `return 4` ⇒ 尾部恒 16 B）：
//
//	@0    u32   活动号（313）
//	--- v8=0（第 0 格）：#03 u32 / #04 u8 / #06 16B ---
//	@4    u32   进度/档位
//	@8    u8    序号（索引进对象行表，越界被夹到「行数-1」）
//	@9    16B   块
//	--- v8=1（第 1 格）：#02 u8 / #03 u32 / #04 u8 / #06 16B / #07 u8 ---
//	@25   u8    标志
//	@26   u32   进度/档位
//	@30   u8    序号
//	@31   16B   块
//	@47   u8    额外标志
//	--- v8=2（第 2 格）：#02 u8 / #03 u32 / #04 u8 / #05 u32 / #07 u8 ---
//	@48   u8    标志
//	@49   u32   进度/档位
//	@53   u8    序号
//	@54   u32   额外值
//	@58   u8    额外标志
//	--- 尾部 ---
//	@59   u8    字节 A  → sub_143CC79E0(obj, A)
//	@60   u32   第 3 格的进度/档位 → sub_143CC72A0(obj, 3, ·)
//	@64   4×u32 每档一个 u32（档数恒 4）
//	@80   u8    字节 B  → sub_143CC7910(obj, B)
//
// ## ★ 为什么是 81 而不是 80（这条别改回去）
//
// 2026-10-10 实机：按 80 字节发 ⇒ 客户端轨迹出现
// `NETWORK>> PACKET OVERFLOW : ENUM_NOTIPACKET_HONEY_TIME_EVENT_INFO`，
// 并回 **CMD217 `ENUM_CMDPACKET_OVERFLOW_INFO`**（我们收到的载荷 `000e050000000000`
// 里那个 u32 = `0x50E` = **正是 1294**）。
//
// 按本仓既有判据（`docs/protocol/noti2639-row-width-authoritative-20261007.md` §2）：
// **`PACKET OVERFLOW` 的成因是「读不满、游标越界」，不是「帧太长」**。
// 即 80 = **少了一个字节**；而客户端在溢出后会**丢弃当前 TCP 段里剩下的帧** ——
// 紧接着发的 **NOTI398（收掉左上角 Liberation Trace 面板的那一帧）就这样被吞掉**，
// 那块面板于是又冒了出来（与"改这一帧之前一直好好的"完全对得上）。
//
// 少的那一字节来自**循环边界读漏**：`#07`（写 `var_C4 + i` 的那个 u8）不只在 v8=2 读，
// **v8=1 也读一次**（`cmp ebx, 1 / jz` 在块分支之后紧接着它；v8>=2 那一支经 `jmp` 落到
// `cmp ebx, 2 / jnz`，同样会走到它）。伪代码里这两条支路被并成了 `if (v8 == 2)`，肉眼极易漏。
// ⇒ **教训：读流重建必须逐条核对循环边界的反汇编，不能只看 Hex-Rays 的 if/else 嵌套。**
//
// ## 其余已知
//
// 读完后 handler 把 4 格回填到对象（第 0/1 格走 `sub_143CC7740`、第 2/3 格走 `sub_143CC76E0`），
// 最后 `sub_14412C720(&v, 活动号)` 取**窗口对象**并调它的 `vtbl+552` —— **收帧即刷新窗口**。
//
// ⚠️ **逐格语义尚未确证**（哪个字段是"已领"、哪个是"进度"、16 字节块里是什么）。
// 本文件只提供**中性态**（除活动号外全 0）与按偏移写入的底层构造。
// ## 字段命名（取自上游那份独立反编译，同一 reader `1452d4b30`）
//
// 上游 `internal/game/protocol/honey_time115.go` 把同一份读流做成了具名结构，命名与我们的
// 偏移**逐字节对得上**。这里采用他们的名字（**不动偏移**）：
//
//	@4  Seconds   @8  Step   @9  Values[16]                       第 0 格（无 Enabled/Limit）
//	@25 Enabled   @26 Seconds   @30 Step   @31 Values[16]   @47 Limit   第 1 格
//	@48 Enabled   @49 Seconds   @53 Step   @54 Mask(u32)    @58 Limit   第 2 格
//	@59 SourceFlag   @60 PeriodSeconds   @64 LoopCounts[4]   @80 FinalFlag
//
// ★ `@60` 被命名为 **PeriodSeconds** ⇒ 期间轨（面板下面那行"累计在线时长"）的累计秒数
// 很可能就在**尾部这一格**（正好是我们 2026-10-10 探针里试过的"第 3 条轨"）。
// ★ `@64` 的 4 个 u32 是 **LoopCounts**（四条轨各自的 loop 计数），由
// `sub_143CC6090(obj, 1)` 固定为 4 个 —— 与我们"恒 4"的结论一致。
// ★ 第 0/1 格的 16 字节是**逐行的**计数（`Values`），第 2 格同一位置换成 **u32 掩码**（`Mask`）。
const NotiHoneyTimeEventInfoBytes = 81

// HoneyTimeEventInfo115 按**已确证的偏移**拼一帧 NOTI1294。
//
// set 收到 (偏移, 当前字节) 并返回要写入的字节；nil = 全 0（中性态）。
// 语义未证的字段刻意不做具名封装 —— 免得"猜出来的名字"被后来的人当成契约。
func HoneyTimeEventInfo115(eventID uint16, set func(offset int, value byte) byte) []byte {
	out := make([]byte, NotiHoneyTimeEventInfoBytes)
	binary.LittleEndian.PutUint32(out[0:], uint32(eventID))
	if set != nil {
		for i := 4; i < len(out); i++ {
			out[i] = set(i, out[i])
		}
	}
	return out
}

// HoneyTimeEventInfoZero115 返回**中性态**：除活动号外全 0。
//
// 用途：第一个探针 —— 此前从没发过这一帧，客户端那份对象一直是默认值（面板显示
// 「Completed: 80h Honey Time!」且 GET 全不可点）。发一帧全 0 的状态，若面板归位到
// 「0 进度、第 1 档可领」，就说明形状对了、且 0 是"什么都没领"。
func HoneyTimeEventInfoZero115(eventID uint16) []byte {
	return HoneyTimeEventInfo115(eventID, nil)
}

// ---- CMD680（活动 313 的领奖/同步请求）----

const (
	// 四条轨 = UI 的四个数组 = 脚本里 `[reward info]` 的四个 `[reward type]`。
	HoneyTimeTrackDaily       uint32 = 0 // `daily`
	HoneyTimeTrackPeriod      uint32 = 1 // `period`（面板下面那行"累计在线时长"）
	HoneyTimeTrackSpecified   uint32 = 2 // `specified`
	HoneyTimeTrackExtendDaily uint32 = 3 // `extend_daily`
)

// HoneyTimeActionSync 是进城自动帧的动作类型（= 同步/查询，没有档位下标）。
const HoneyTimeActionSync uint32 = 0

// HoneyTimeTrackForAction 把 CMD680 第 2 个 u32（**动作类型**）映射成轨号。
//
// ★★ 来源：上游那份独立反编译（`origin/main` 的 internal/game/protocol/honey_time115.go，
// 同一 reader 1452d4b30）从 UI 虚拟回调 `14412c770` 读出：**四个 group 数组对应
// action 1, 2, 3, 5（action 4 被跳过）**，action 后面那个 u32 是"选中数组里的 0 起下标"。
//
// 与本季脚本对得上：`honeytime.evt` 里只有两个 `[reward info]`，顺序是 `daily`(第 0) →
// `period`(第 1) ⇒ **action 1 = 日档、action 2 = 期间轨**。
//
// ⚠️ action 3 / 5 对应的 `specified` / `extend_daily` 本季**没有块**（没有正面样本）
// ⇒ 调用方只应"认出它、记事件、不猜语义"，别硬当成某条轨去发奖。
func HoneyTimeTrackForAction(action uint32) (uint32, bool) {
	switch action {
	case 1:
		return HoneyTimeTrackDaily, true
	case 2:
		return HoneyTimeTrackPeriod, true
	case 3:
		return HoneyTimeTrackSpecified, true
	case 5:
		return HoneyTimeTrackExtendDaily, true
	}
	return 0, false
}

// HoneyTimeClaimOffsetField 是**档位下标**在 680 载荷里的字节偏移（第 3 个 u32）。
//
// ★ 实机两种形态（2026-10-10，客户端轨迹 + 服务端明文双证）：
//
//	8 字节   `39010000 00000000`                   ← 进城**自动**发：活动号 + 动作类型 0（同步）
//	16 字节  `39010000 01000000 00000000 00000000`  ← 点日档第 1 档（action 1、下标 0）
//	16 字节  `39010000 01000000 01000000 00000000`  ← 点日档第 2 档（action 1、下标 1）
//
// ⇒ **变的是第 3 个 u32 ⇒ 下标在偏移 8**（0 起 = 该轨行表里的行序）。
// 第 2 个 u32 是**动作类型**（点击 = 1/2/3/5，进城自动帧 = 0）。
//
// ⚠️ 一开始照 331 的"活动号 + 行号"口径读**第 2 个** u32 —— 实机证伪：那样第 2 档会被读成
// "又点了第 1 档"，于是又一次去领已领过的那一档（无新邮件、也无报错），玩家看到的是
// "点了没反应/像是被拒"。**同族命令共享包头，但载荷口径不共享**。
//
// ⚠️ 也曾把**偏移 12** 那一格当"轨号"读过：实机两次点击它都是 0，而上游那份的
// `DecodeHoneyTimeRequest115` 明确只读 3 个字（帧长 12）⇒ 那一格是**补齐**，不是轨号。
// **轨号由动作类型决定**（见 HoneyTimeTrackForAction）。
const HoneyTimeClaimOffsetField = 8

// HoneyTimeClaimMinBytes 是能读出**档位下标**的最小载荷长度（动作类型 + 下标都得到齐）。
const HoneyTimeClaimMinBytes = HoneyTimeClaimOffsetField + 4

// DecodeHoneyTimeRequest115 读活动 313 的 680 请求：动作类型 + 档位下标（0 起）。
//
// 载荷不够长（进城那条 8 字节自动帧）时返回 error —— 调用方应当**原样放行、不回应答**
// （那种帧没有下标，它不是一次领取）。
//
// 长度取 **≥12** 而不是上游那样写死 `== 12`：本仓有实机反例 —— 点击帧实际是 **16 字节**
// （第 4 个字恒 0）。写死 12 会把真实帧拒掉。
func DecodeHoneyTimeRequest115(p []byte) (action uint32, offset uint32, err error) {
	if len(p) < HoneyTimeClaimMinBytes {
		return 0, 0, fmt.Errorf("honeytime request too short: %d", len(p))
	}
	action = binary.LittleEndian.Uint32(p[4:])
	offset = binary.LittleEndian.Uint32(p[HoneyTimeClaimOffsetField:])
	return action, offset, nil
}
