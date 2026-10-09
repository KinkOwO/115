package legion

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
)

// 苏醒之森（Forest of Awakening，频道 Type 96）家族常量与状态快照。
//
// 内容号与 NOTI 号的真源：
//   - 内容号 104：官服 2026-10-02 21:42:02.988 抓包 CMD2043 信封 @13 =
//     68 00 00 00；私服实机 2026-10-05 18:04 会话同位置一致。
//   - NOTI 2563 = ENUM_NOTIPACKET_FOREST_OF_AWAKENING_NORMAL_INFO、
//     NOTI 2564 = …_NORMAL_OPERATION、CMD 2226 = …_NORMAL_OPERATION_SELECT、
//     CMD 2227 = …_NORMAL_BUFF_SELECT：analysis/dumps/opcodes.tsv（IDA 枚举表）。
//   - 家族约定与末世录/伊斯/维纳斯同构：CMD2043/2044/2045/2046 共用信封
//     （13B 不透明前缀 + u32 内容号），按内容号分流；每个内容有专属 INFO
//     NOTI（末世录 2895 / 伊斯 2255 / 维纳斯 2655 / 苏醒之森 2563）。
//     官服文档与实机双重推翻「用末世录 N2895 开其它内容」：私服 18:04 会话
//     dispatchLegion 抢答了 ACK + N2895(内容107)，客户端对苏醒之森毫无反应。

const (
	// ForestContentID is the u32 carried at body offset 13 of the forest
	// CMD2043/2045/2046 requests (official capture 21:42:02.988, live 18:04
	// session), the analogue of IspinsContentID 101 / VenusContentID 106 /
	// apocalypse 107.
	ForestContentID uint32 = 104
	// ForestHardContentID is the Extreme (ForestOfAwakeningHard) content
	// number, **实机实锤**：2026-10-06 00:09 会话 forest_hard_start_observed
	// 捕获 CMD2043 信封 @13 = 69 00 00 00（观测点轮次落账）。
	ForestHardContentID uint32 = 105
	// NotiForestInfo is the forest-only state snapshot (136B) — the analogue
	// of NotiIspinsInfo 2255 / NotiVenusInfo 2655 / NotiLegionInfo 2895.
	NotiForestInfo uint16 = 2563
	// NotiForestHardInfo is the Extreme state snapshot（ENUM_NOTIPACKET_
	// FOREST_OF_AWAKENING_HARD_INFO）。**官服形状实锤**：正文 64B（帧 80B），
	// 见 D:\115US-001\DFO-115US-zhuabao\official_20261008-220048_live 的
	// session_s13（Extreme 三关全清）。此前的 136B 家族同构猜测（第 11/12 轮）
	// 已被推翻。
	NotiForestHardInfo uint16 = 2565
	// NotiForestHardPhaseTick is ENUM_NOTIPACKET_FOREST_OF_AWAKENING_HARD_
	// PHASE_CLEAR_TICK（2566）：官服在同一场 Extreme 终局关清关时发一次，
	// 正文 64B = 三关 N31 横幅 token（各 u32 + 4B 零）+ 尾部 nonce。
	NotiForestHardPhaseTick uint16 = 2566
	// CmdForestOperationSelect is the forest-specific operation window
	// command (2226). Its request decode is the next evidence round: official
	// samples show a 32B body with choice@13..16 and target@17..20 on
	// confirms, plus a verbatim "window ready" ping — do not guess a codec.
	CmdForestOperationSelect uint16 = 2226
	// CmdForestBuffSelect is the forest buff pick (2227), unimplemented.
	CmdForestBuffSelect uint16 = 2227
)

// DecodeForestStart reads the CMD2043 request. Live body (24B): 13B opaque
// envelope + u32 LE content 104 + 7 zero bytes, byte-shape identical to the
// ispins/venus starts.
func DecodeForestStart(p []byte) error {
	if len(p) < EnvelopeSize+4 {
		return fmt.Errorf("forest start payload %d bytes, want at least %d", len(p), EnvelopeSize+4)
	}
	if content := binary.LittleEndian.Uint32(p[EnvelopeSize:]); content != ForestContentID {
		return fmt.Errorf("forest start content %d, want %d", content, ForestContentID)
	}
	return nil
}

// 官服 2026-10-02 抓包的 N2563 原文（S2C，136B）。两份向量按帧回放：
//   - waiting：21:42:03.375，CMD2043 后首个状态（choice FF / state@3=1 /
//     stage@11=FFFFFFFF / 三条 12B 阶段记录全 FF —— 与军团表 last_phase 2
//     即 3 关吻合）。
//   - operation window：21:42:06.202（开始点击后 3.2s，其间无任何 c2s 请求，
//     是服务端主动推送），state@3=6，@19 记录数 3，@27/@39/@51 =
//     100003878 / 100003880 / 100003881 —— 这正是军团表缺失的
//     [dungeon info data] 三阶段副本号（channels.json 的 [198] 为占位错误）。
//
// 帧尾 5 字节逐帧变化（ab99c8453f / 92b4929234…，与 21:43:02/21:43:20 两条
// 同状态帧尾不同），是时钟/nonce 类字段而非状态；waiting 向量按帧原样回放，
// 不解释、不改写。窗口向量的 dc@116 等计数字段同样保持原值。
const (
	forestWaitingInfoHex = "0000ff0100000000000000ffffffff01" +
		"000000ff000000ffffffffffffffffff" +
		"000000ffffffffffffffffff000000ff" +
		"ffffffffffffff000000000000000000" +
		"00000000000000000000000000000000" +
		"00000000000000000000000000000000" +
		"00000000000000000000000000000000" +
		"00000000000000000000000000ab99c8" +
		"453f000000000000"
)

func forestInfoFromHex(v string) []byte {
	b, err := hex.DecodeString(v)
	if err != nil {
		panic("forest info vector: " + err.Error())
	}
	return b
}

// ForestWaitingInfo returns the post-CMD2043 waiting state (official frame
// 21:42:03.375, verbatim 136B).
func ForestWaitingInfo() []byte { return bytes.Clone(forestInfoFromHex(forestWaitingInfoHex)) }

// ForestOperationWindowInfo returns the operation-select window state the
// official server pushed ~3s after the start (frame 21:42:06.202, verbatim
// 136B, carrying the three phase dungeons).
func ForestOperationWindowInfo() []byte {
	return bytes.Clone(forestInfoFromHex(forestOperationWindowHex))
}

// —— 第三轮：作战选择（CMD2226）、进图（CMD2045）与关卡循环 ——

// 官服时序（frames.jsonl 21:42-21:45，三关完整样本）：
//
//	CMD2226 a1（就绪 ping，@13=1、@17=ffff）→ N2563 state2 + ACK(32B)
//	CMD2226 a2（确认音符，@13=2、@17=目标 5/6）→ N2563 state2+目标 + ACK(32B)
//	（客户端 ~3s 后自行发 CMD2045 进图，阶段号 @17）
//	boss 死亡（CMD39）→ N31 → N2 → N2563 cleared → CMD46 → N2563 下一关作战窗
//	终局（stage2）清关后：CMD46 → N2563 cleared；CMD2046 → ACK + N2563 state3
//	（终局演出）→ N2563 state5（面板关闭）。
//	全程无 CMD2062/无 CMD2046 非终局帧——苏醒之森每关回城重选，与维纳斯
//	的 2062 直进不同。
//
// N2563 记录区（@55 起每关一条 10B 音符记录 [目标 1B][3B 零][类型 1B]
// [00 03 00 00]）随关卡累积；下面按「阶段」逐字回放官服对应帧，不构造
// 记录区。目标值只出现在 ACK @6..9（u32）与 chosen 帧记录首字节——
// 官服仅见 5/6 两值（5→类型 0x14，6→类型 0x04），其它值的类型字节
// 沿用向量原值并在 chosen 帧注释标明。
const (
	// 21:42:06.202 —— 首关作战窗（开始点击后 3.2s 服务端主动推，其间无任何
	// c2s 请求），state6，@19 记录数 3，@27/@39/@51 = 100003878 / 100003880 /
	// 100003881 —— 这正是军团表缺失的 [dungeon info data] 三阶段副本号
	// （channels.json 的 [198] 为占位错误）。
	forestOperationWindowHex = "0000ff06000000000000000000000001" +
		"000000030000000000000026f0f50502" +
		"0000000200000028f0f5050000000002" +
		"00000029f0f505000000000000000000" +
		"00000000000000000000000000000000" +
		"00000000000100000082000100000004" +
		"00000037000200000005000000140003" +
		"00000000dc000000000000000092b492" +
		"9234000000000000"
	// 21:43:02.294 —— stage0 清关投影（state2，记录区含 (5,0x14) 一条）。
	forestClearedInfoHex = "0000ff02000000030000000000000001" +
		"000000030000000100000026f0f50502" +
		"0000000200000028f0f5050000000002" +
		"00000029f0f505050000001400030000" +
		"00000000000000000000000000000000" +
		"00000000000100000082000100000004" +
		"00000037000200000005000000140003" +
		"00000000000000000000000000db4fb2" +
		"3e3f000000000000"
	// 21:44:14.516 —— stage1 清关投影（state2，记录区含 (5,14)+(6,04) 两条）。
	// 第七轮补配：此前缺这一条导致第二关击杀后 completeForestStage 报
	// 「no official cleared vector for stage 1」、清关链整条不发（21:22 会话
	// dungeon_completion_error 实证）。
	forestCleared1Hex = "0000ff02000000030000000100000001" +
		"000000030000000100000026f0f50502" +
		"0000000100000028f0f5050000000002" +
		"00000029f0f505050000001400030000" +
		"00060000000400030000000000000000" +
		"00000000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"0000000000000000000000000011e411" +
		"773d000000000000"
	// 21:43:02.340 —— stage1 作战窗（CMD46 应答）。
	forestWindow1Hex = "0000ff06000000000000000100000001" +
		"000000030000000100000026f0f50502" +
		"0000000000000028f0f5050000000002" +
		"00000029f0f505050000001400030000" +
		"00000000000000000000000000000000" +
		"00000000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"00000000dd00000000000000003498ef" +
		"5d42000000000000"
	// 21:44:14.567 —— stage2 作战窗（CMD46 应答）。
	forestWindow2Hex = "0000ff06000000000000000200000001" +
		"000000030000000100000026f0f50502" +
		"0000000100000028f0f5050000000000" +
		"00000029f0f505050000001400030000" +
		"00060000000400030000000000000000" +
		"00000000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"00000000dd0000000000000000eef40d" +
		"833c000000000000"
	// 21:45:25.262 —— stage2 清关投影（state2，记录区三条齐全）。
	forestCleared2Hex = "0000ff02000000030000000200000001" +
		"000000030000000100000026f0f50502" +
		"0000000100000028f0f5050000000001" +
		"00000029f0f505050000001400030000" +
		"00060000000400030000000600000004" +
		"00030000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"00000000000000000000000000685119" +
		"863a000000000000"
	// 21:45:39.092 —— CMD2046 终局应答（state3，终局演出）。
	forestFinalInfoHex = "0000ff03000000010000000200000001" +
		"000000030000000100000026f0f50502" +
		"0000000100000028f0f5050000000001" +
		"00000029f0f505050000001400030000" +
		"00060000000400030000000600000004" +
		"00030000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"00000000dd00000000000000005c78be" +
		"bf34000000000000"
	// 21:45:42.559 —— 终局演出结束（state5，面板关闭）。
	forestLeaveInfoHex = "0000ff05000000010000000200000001" +
		"000000030000000100000026f0f50502" +
		"0000000100000028f0f5050000000001" +
		"00000029f0f505050000001400030000" +
		"00060000000400030000000600000004" +
		"00030000000100000082000100000003" +
		"00000037000200000006000000040003" +
		"00000000dd0000000000000000752a31" +
		"c435000000000000"
	// CMD2226 就绪 ACK（21:42:10.452）：@0 成功 01、@1 动作回显、@5..8 目标
	// u32（ffff=未选）、@12 计数（逐帧 07/54/90）、@13..15=b5bf6a 会话不变
	// 量、@17=a0（160s 时限）、尾部 5B nonce 逐帧变化按帧回放。
	forestReadyAckHex = "0101000000ffff000000000007b5bf6a00a000002951f7c73900000000000000"
	// CMD2226 确认 ACK（21:42:15.213，目标 5）：同布局，@5..8 = 目标 u32。
	forestConfirmAckHex = "0102000000050000000000000000000000a0000023a9b70b3f00000000000000"
	// N31 清关横幅（stage0/1/2：21:43:02.213 / 21:44:14.429 / 21:45:25.116）。
	forestClearStage0Hex = "9fa70000b32687723d00000000000000"
	forestClearStage1Hex = "3f9d000013202d5f3d00000000000000"
	forestClearStage2Hex = "26c2000005095b9d3300000000000000"
)

// ---------------------------------------------------------------------------
// Extreme（ForestOfAwakeningHard，内容 105）官方向量
//
// 真源：官服 2026-10-08 22:00 抓包
// `official_20261008-220048_live` 的 session_s13（TCP 50074 ↔ 52.23.96.246:10012），
// 一场三关全清的 Extreme 挑战。全部按帧原文回放，**不做家族同构推断**
// （第 11/12 轮的 N2563 同构猜测已被该抓包推翻：Extreme 的 N2565 正文是 64B，
// 不是 136B）。
//
// 官服时序（时间戳来自 pcapng 的 TCP 段）：
//
//	22:05:24.914 c2s CMD12 建队（队伍类型 0x19 = Extreme）
//	22:05:29.112 c2s CMD36 → town198 area3（Extreme 集结区，军团表
//	                   ForestOfAwakeningHard.waiting_area = 198/3）
//	22:05:37.983 c2s CMD2043（内容 105）
//	22:05:38.308 s2c N2565 等待态（state1）+ CMD2043 ACK
//	22:05:41.244 s2c N2565 作战窗（state2，三关副本号 @26/@38/@50）
//	22:05:41.255 c2s CMD2045（内容 105，阶段 0）
//	22:05:45.580 s2c 窗态副本 + N1584 + N28(100004079) + N29 + CMD2045 ACK
//	22:06:06.567 c2s CMD39 首关 boss 死亡 → s2c N31(34510000…)
//	22:06:06.839 c2s CMD46 → s2c N2565 清关 tick + 下一关窗态
//	22:06:15.191 c2s CMD2062 → 100004080（第二关直进）
//	22:06:44.102 c2s CMD39 → N31(2a6f0000…)；CMD46 → tick + 窗态
//	22:06:46.183 c2s CMD2062 → 100003877（第三关直进）
//	22:07:23.050 c2s CMD39 + CMD117 → s2c N31(f88c0000…) + N2566（三关 token）
//	                     + N2252 + N2253 + N2254；CMD46 → N2565 tick2
//	22:07:37.004 c2s CMD2046 → s2c N2565 终局态(state3) + CMD2046 ACK
//	22:07:37.382 c2s CMD191(0) → N170；22:08:32 c2s CMD191(1) → N170
//	22:08:33.372 s2c N2565 leave(state5) → c2s CMD72 回城
//	22:08:45.691 s2c N2565 idle(state0e)（离队后复位）
//
// N2565 正文（64B）字段对照（逐帧 diff 得出，@54..58 为 5B nonce，按帧原样保留）：
//
//	@0..1  00 00（idle 态为 ff ff）
//	@2     State：01 等待 / 02 进行 / 03 终局演出 / 05 演出结束 / 0e 复位
//	@6     清关 tick 标记（普通 00，清关帧 03，终局/leave 01，复位 04）
//	@10    当前关号（0/1/2；等待态 ff）
//	@14    01 常量
//	@22    首次清关之后的「已有过清关」标记（清关前 00，之后 01）
//	@26/@38/@50  三关副本号（100004079 / 100004080 / 100003877）
//	@30    01 常量
//	@34    第 1 关状态：02 未开始 / 00 进行中 / 01 已清
//	@42    02 常量
//	@46    第 2 关状态：02 未开始 / 00 进行中 / 01 已清
const (
	// 22:05:38.308 —— CMD2043 后首个状态（等待态，三关字段全 ff）。
	forestHardWaitingHex = "00000100000000000000ffffffff01000000ff000000ffffffffffffffffff00" +
		"0000ffffffffffffffffff000000ffffffffffffffff3a807fde360000000000"
	// 22:05:41.244 —— 首关作战窗（state2、@10=0、三关副本号齐）。
	forestHardWindow0Hex = "0000020000000000000000000000010000000000000000000000eff0f5050100" +
		"000002000000f0f0f505020000000200000025f0f5050516748e3b0000000000"
	// 22:06:06.894 —— 首关清关 tick（@6=3、@22=1）。
	forestHardClearTick0Hex = "0000020000000300000000000000010000000000000001000000eff0f5050100" +
		"000002000000f0f0f505020000000200000025f0f5058da316bf330000000000"
	// 22:06:06.894 —— 第二关作战窗（@10=1、第 1 关状态 0）。
	forestHardWindow1Hex = "0000020000000000000001000000010000000000000001000000eff0f5050100" +
		"000000000000f0f0f505020000000200000025f0f5055e91d2b8340000000000"
	// 22:06:44.387 —— 第二关清关 tick（第 1 关状态 1、第 2 关状态 2）。
	forestHardClearTick1Hex = "0000020000000300000001000000010000000000000001000000eff0f5050100" +
		"000001000000f0f0f505020000000200000025f0f505c6e33d5e3c0000000000"
	// 22:06:44.387 —— 第三关作战窗（@10=2、第 2 关状态 0）。
	forestHardWindow2Hex = "0000020000000000000002000000010000000000000001000000eff0f5050100" +
		"000001000000f0f0f505020000000000000025f0f50524ad6fc43c0000000000"
	// 22:07:23.375 —— 第三关清关 tick（CMD46 应答）。
	forestHardClearTick2Hex = "0000020000000300000002000000010000000000000001000000eff0f5050100" +
		"000001000000f0f0f505020000000100000025f0f50580b55c723c0000000000"
	// 22:07:37.264 —— CMD2046 终局态（state3，触发通关演出）。
	forestHardFinalInfoHex = "0000030000000100000002000000010000000000000001000000eff0f5050100" +
		"000001000000f0f0f505020000000100000025f0f50593402c89330000000000"
	// 22:08:33.372 —— 演出结束（state5，收起右上角面板）。
	forestHardLeaveInfoHex = "0000050000000100000002000000010000000000000001000000eff0f5050100" +
		"000001000000f0f0f505020000000100000025f0f5051da9dbc9390000000000"
	// 22:08:45.691 —— 离队后的复位态（state0e，三关字段全 ff）。
	forestHardIdleInfoHex = "ffff0e00000004000000fffffffffffffffffff0ed7fffffffffffffffffff00" +
		"a063ffffffffffffffffff000000ffffffffffffffff56c8bbc2380000000000"
	// 22:07:23.311 —— N2566 过段 tick：三关 N31 横幅 token（u32+4B 零）+
	// 尾部 nonce。三 token 与 22:06:06/22:06:44/22:07:23 的 N31 头 2B 逐一吻合。
	forestHardPhaseClearTickHex = "34510000000000002a6f000000000000f88c0000000000000000000000000000" +
		"00000000000000000000000000000000751e2594330000000000000000000000"
	// N31 清关横幅（Extreme 三关各有自己的 token：0x5134 / 0x6f2a / 0x8cf8；
	// 与 Normal 的 0xa79f / 0x9d3f / 0xc226 完全不同 —— N2252 @7760 必须回填
	// 本模式该关的 token）。
	forestHardClearStage0Hex = "34510000630492c13300000000000000"
	forestHardClearStage1Hex = "2a6f00005e6ec6763c00000000000000"
	forestHardClearStage2Hex = "f88c0000e6eb62ee3c00000000000000"
	// CMD2045 进图 ACK（内容 105 + 阶段号，24B）：@9..12 = 阶段 u32。
	forestHardEnterAck0Hex = "01000000006900000000000000016d3b1a17430000000000"
	forestHardEnterAck1Hex = "0100000000690000000100000000421cd6b6360000000000"
	forestHardEnterAck2Hex = "010000000069000000020000000062bc37eb3c0000000000"
	// CMD2046 终局 ACK（内容 105 + 阶段 2，32B）。
	forestHardRewardEndAckHex = "0100000000690000000200000002d05a2db73c00000000000000000000000000"
	// CMD2043 开战 ACK（官服 16B；Normal 官服同帧逐字节相同）。
	forestHardStartAckHex = "01000000003bfff77e43000000000000"
	// N2568 PREPARE_LEGION_ENTER_DUNGEON（22:05:41.514，CMD2045 之后立即送达）。
	forestHardPrepareEnterHex = "000000000100000004000000a8158106724db34742000000"
)

// ForestStageDungeons 是苏醒之森三阶段副本号（官服 N2563 作战窗 @27/@39/@51
// 与 N28 交叉实证；军团表该内容缺 [dungeon info data]，channels.json 的
// [forestofawakening]: [198] 为占位错误）。
var ForestStageDungeons = [3]uint32{100003878, 100003880, 100003881}

// IsForestStageDungeon reports whether the dungeon is a forest phase.
func IsForestStageDungeon(id uint32) bool {
	for _, d := range ForestStageDungeons {
		if d == id {
			return true
		}
	}
	return false
}

// ForestStageOfDungeon resolves the phase index of a forest stage dungeon.
func ForestStageOfDungeon(id uint32) (int, error) {
	for i, d := range ForestStageDungeons {
		if d == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("dungeon %d is not a forest stage", id)
}

// IsForestStageDungeonInPacket reports whether a raw CMD2062 body targets a
// forest stage dungeon（ID u32 @13，DecodeDungeonDirectMove 布局）。用于
// directMoveDungeon 在 activeDungeon 门控之前分流苏醒之森直进。
func IsForestStageDungeonInPacket(p []byte) bool {
	if len(p) < 17 {
		return false
	}
	return IsForestStageDungeon(binary.LittleEndian.Uint32(p[13:17])) || IsForestHardStageDungeon(binary.LittleEndian.Uint32(p[13:17]))
}

// IsForestDirectMoveCandidate 识别连战模式的直进帧：0231 会话实证 2062 @13
// 可能为 0（客户端本地记录表没有该阶段副本号），但难度位 @17=04 形状可辨。
// 仅当 @13 不是任何已知副本号时才作此兜底判定，避免吞掉其它内容的直进。
func IsForestDirectMoveCandidate(p []byte) bool {
	if len(p) != 48 {
		return false
	}
	if id := binary.LittleEndian.Uint32(p[13:17]); id != 0 {
		return false // 有副本号但不属于苏醒之森 → 不是本分支的候选
	}
	return p[17] == 0x04
}

// —— Extreme（ForestOfAwakeningHard，内容 105，连战模式）——

// ForestHardStageDungeons 是 Extreme 三关副本号（PVF script path 取证：
// contents/2024/forestofawakening/dungeon/hermitageforest_ex/hermitage_extreme_1/2/3.dgn）。
var ForestHardStageDungeons = [3]uint32{100004079, 100004080, 100003877}

// IsForestHardStageDungeon reports whether the dungeon is an Extreme stage.
func IsForestHardStageDungeon(id uint32) bool {
	for _, d := range ForestHardStageDungeons {
		if d == id {
			return true
		}
	}
	return false
}

// ForestHardStageOfDungeon resolves the phase index of an Extreme stage.
func ForestHardStageOfDungeon(id uint32) (int, error) {
	for i, d := range ForestHardStageDungeons {
		if d == id {
			return i, nil
		}
	}
	return 0, fmt.Errorf("dungeon %d is not a forest hard stage", id)
}

// IsForestStageDungeonAny reports whether the dungeon belongs to either mode.
func IsForestStageDungeonAny(id uint32) bool {
	return IsForestStageDungeon(id) || IsForestHardStageDungeon(id)
}

// ForestStageOfDungeonAny resolves the stage across both modes
// （hard=true 表示命中 Extreme 副本集）.
func ForestStageOfDungeonAny(id uint32) (stage int, hard bool, err error) {
	if s, err := ForestStageOfDungeon(id); err == nil {
		return s, false, nil
	}
	s, err := ForestHardStageOfDungeon(id)
	return s, true, err
}

// ForestHardInfoSize 是 Extreme 状态包 N2565 的正文长度（官服 64B，
// 帧 80B；**不是** N2563 的 136B）。
const ForestHardInfoSize = 64

// Extreme 状态值（N2565 @2，官方 2026-10-08 抓包逐帧 diff）。
const (
	ForestHardStateWaiting byte = 0x01 // 等待（CMD2043 应答）
	ForestHardStateActive  byte = 0x02 // 进行中（作战窗 / 清关 tick）
	ForestHardStateFinale  byte = 0x03 // 终局演出（CMD2046 应答）
	ForestHardStateLeave   byte = 0x05 // 演出结束（收起面板）
	ForestHardStateIdle    byte = 0x0e // 复位（离队后）
)

// ForestHardWaitingInfo 是 CMD2043 之后的等待态 N2565 正文（官服 22:05:38.308）。
func ForestHardWaitingInfo() []byte { return forestInfo(forestHardWaitingHex) }

// ForestHardWindowInfo 是第 stage 关的作战窗状态（state2；stage 0/1/2 各有
// 一帧官服原文——@10 关号与各关状态位逐帧不同，不能互相套用）。
func ForestHardWindowInfo(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestHardWindow0Hex), nil
	case 1:
		return forestInfo(forestHardWindow1Hex), nil
	case 2:
		return forestInfo(forestHardWindow2Hex), nil
	}
	return nil, fmt.Errorf("no official forest hard window vector for stage %d", stage)
}

// ForestHardClearTickInfo 是第 stage 关清关的 tick 状态（官方在 CMD46 应答
// 时下发；@6=3、@10=stage）。
func ForestHardClearTickInfo(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestHardClearTick0Hex), nil
	case 1:
		return forestInfo(forestHardClearTick1Hex), nil
	case 2:
		return forestInfo(forestHardClearTick2Hex), nil
	}
	return nil, fmt.Errorf("no official forest hard clear tick vector for stage %d", stage)
}

// ForestHardFinalInfo 是 CMD2046 终局态（state3，触发通关演出）。
func ForestHardFinalInfo() []byte { return forestInfo(forestHardFinalInfoHex) }

// ForestHardLeaveInfo 是演出结束态（state5，收起右上角面板）。
func ForestHardLeaveInfo() []byte { return forestInfo(forestHardLeaveInfoHex) }

// ForestHardIdleInfo 是离队后的复位态（state0e）。
func ForestHardIdleInfo() []byte { return forestInfo(forestHardIdleInfoHex) }

// ForestHardPhaseClearTick 是 N2566 过段 tick（三关 N31 token + 尾部 nonce）。
func ForestHardPhaseClearTick() []byte { return forestInfo(forestHardPhaseClearTickHex) }

// ForestHardStageClearEnabled 是 Extreme 第 stage 关的 N31 清关横幅（原文）。
func ForestHardStageClearEnabled(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestHardClearStage0Hex), nil
	case 1:
		return forestInfo(forestHardClearStage1Hex), nil
	case 2:
		return forestInfo(forestHardClearStage2Hex), nil
	}
	return nil, fmt.Errorf("no official forest hard clear banner for stage %d", stage)
}

// ForestHardStageToken 取 Extreme 第 stage 关 N31 头 2B token（N2252 @7760
// 必须与它一致，否则客户端翻牌面板不显示）。
func ForestHardStageToken(stage int) ([2]byte, error) {
	banner, err := ForestHardStageClearEnabled(stage)
	if err != nil {
		return [2]byte{}, err
	}
	return [2]byte{banner[0], banner[1]}, nil
}

// ForestHardStartAck 是 CMD2043 的官服 ACK（16B；Normal 官服同帧逐字节相同，
// 但 Normal 路径已用短 5B 形态实机验证，故只给 Extreme 用官服原文）。
func ForestHardStartAck() []byte { return forestInfo(forestHardStartAckHex) }

// ForestHardEnterAck 是 CMD2045 的官服 ACK（24B：01 + 内容 105 + 阶段号 +
// 尾部 nonce）。stage 0/1/2 各有官服样本，其它阶段拒绝。
func ForestHardEnterAck(stage uint32) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestHardEnterAck0Hex), nil
	case 1:
		return forestInfo(forestHardEnterAck1Hex), nil
	case 2:
		return forestInfo(forestHardEnterAck2Hex), nil
	}
	return nil, fmt.Errorf("no official forest hard enter ack for stage %d", stage)
}

// ForestHardRewardEndAck 是 CMD2046 的官服 ACK（32B，内容 105 + 阶段 2）。
func ForestHardRewardEndAck() []byte { return forestInfo(forestHardRewardEndAckHex) }

// ForestHardPrepareEnterInfo 是 N2568 PREPARE_LEGION_ENTER_DUNGEON 的官服
// 原文（22:05:41.514，24B）—— 客户端据此播「极·苏醒之森净化开始」横幅与演出。
//
// ⚠️ 必须与进图帧列**分两段发**（2026-10-09 实机两次校准）：
//
//	第一版把它和 N28/N29 放在同一批 → 客户端在演出中途收到进图帧，演出永不
//	收尾，进第 1 关后**屏幕 UI 全部消失**（技能仍可用）；
//	第二版干脆不发 → 业主首测发现「倒计时结束直接进图」，缺了官服那张
//	「极·苏醒之森净化开始」横幅与演出。
//
// 官服两帧之间固定 4.066s（22:05:41.514 → 22:05:45.580）。所以正确用法是：
// **先只发这一帧，等演出结束（~4.1s）再发进图帧列**，见 cmd/wireprobe 的
// `forestEntryDue` / `forestPurifyBannerSeconds`。
func ForestHardPrepareEnterInfo() []byte { return forestInfo(forestHardPrepareEnterHex) }

// ForestOperationRequest is the decoded CMD2226 body: 13B opaque envelope +
// u32 action @13（1=就绪 ping，2=确认音符）+ u32 value @17（就绪=ffff，
// 确认=音符目标，官服样本 5/6）。
type ForestOperationRequest struct {
	Action uint32
	Value  uint32
}

// DecodeForestOperation reads the CMD2226 request (32B live shape).
func DecodeForestOperation(p []byte) (ForestOperationRequest, error) {
	if len(p) < EnvelopeSize+8 {
		return ForestOperationRequest{}, fmt.Errorf("forest operation payload %d bytes, want at least %d", len(p), EnvelopeSize+8)
	}
	return ForestOperationRequest{
		Action: binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Value:  binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
	}, nil
}

// DecodeForestEnter reads the CMD2045/2046 request (13B envelope + u32
// content 104/105 + u32 stage), the family shape shared with ispins/venus.
// The caller validates the content against the run's mode（run.hard）。
func DecodeForestEnter(p []byte) (EnterDungeonRequest, error) {
	if len(p) < EnvelopeSize+8 {
		return EnterDungeonRequest{}, fmt.Errorf("forest enter payload %d bytes, want at least %d", len(p), EnvelopeSize+8)
	}
	req := EnterDungeonRequest{
		Channel: binary.LittleEndian.Uint32(p[EnvelopeSize:]),
		Stage:   binary.LittleEndian.Uint32(p[EnvelopeSize+4:]),
	}
	if req.Channel != ForestContentID && req.Channel != ForestHardContentID {
		return EnterDungeonRequest{}, fmt.Errorf("forest enter content %d, want %d or %d", req.Channel, ForestContentID, ForestHardContentID)
	}
	return req, nil
}

func forestInfo(v string) []byte { return bytes.Clone(forestInfoFromHex(v)) }

// ForestReadyInfo answers the CMD2226 action-1 ready ping for the window of
// `stage`（第六轮修正：官服每关的就绪态都是「该关作战窗 + @3 状态 6→2 +
// @116 计数清零」——三关 diff 仅 @3/@116/nonce 三处，见 21:42:10/21:43:27/
// 21:44:27 vs 各关窗口帧）。
func ForestReadyInfo(stage int) ([]byte, error) {
	body, err := ForestWindowInfo(stage)
	if err != nil {
		return nil, err
	}
	body[3] = 0x02
	body[116] = 0x00
	return body, nil
}

// forestMelodyKind 是音符目标 → 记录类型字节的映射（官服仅两样本：
// 目标 5→0x14、目标 6→0x04；其它目标无官服样本，默认 0x04——实机复核点）。
func forestMelodyKind(target uint32) byte {
	if target == 5 {
		return 0x14
	}
	return 0x04
}

// ForestChosenInfo answers the CMD2226 action-2 confirm for the window of
// `stage`：官服三关 diff（窗 vs chosen）= @2 选择标志 ff→02、@3 状态 6→2、
// 在 @55+10*stage 追加本关音符记录 [目标 1B][3B 零][类型 1B][00 03 00 00]、
// @116 dd→dc（stage0 窗口本为 dc 不变），nonce 保留窗口帧原值。
func ForestChosenInfo(stage int, target uint32) ([]byte, error) {
	body, err := ForestWindowInfo(stage)
	if err != nil {
		return nil, err
	}
	body[2] = 0x02
	body[3] = 0x02
	at := 55 + 10*stage
	if at+9 >= len(body) {
		return nil, fmt.Errorf("forest chosen record %d out of range", stage)
	}
	body[at] = byte(target)
	body[at+4] = forestMelodyKind(target)
	body[at+6] = 0x03
	if body[116] == 0xdd {
		body[116] = 0xdc
	}
	return body, nil
}

// ForestClearedInfo projects the stage-cleared standby state (verbatim per
// stage; the record area accumulates one 10B melody entry per cleared stage).
func ForestClearedInfo(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestClearedInfoHex), nil
	case 1:
		return forestInfo(forestCleared1Hex), nil
	case 2:
		return forestInfo(forestCleared2Hex), nil
	}
	return nil, fmt.Errorf("no official cleared vector for stage %d", stage)
}

// ForestWindowInfo returns the operation window state pushed by CMD46 for the
// next stage (stage 0 = the post-start push, 1/2 = official CMD46 answers).
func ForestWindowInfo(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestOperationWindowHex), nil
	case 1:
		return forestInfo(forestWindow1Hex), nil
	case 2:
		return forestInfo(forestWindow2Hex), nil
	}
	return nil, fmt.Errorf("no official window vector for stage %d", stage)
}

// ForestFinalInfo answers the terminal CMD2046 (state3, verbatim).
func ForestFinalInfo() []byte { return forestInfo(forestFinalInfoHex) }

// ForestLeaveInfo closes the HUD after the finale movie (state5, verbatim).
func ForestLeaveInfo() []byte { return forestInfo(forestLeaveInfoHex) }

// ForestOperationReadyAck answers the action-1 ping (verbatim 32B).
func ForestOperationReadyAck() []byte { return forestInfo(forestReadyAckHex) }

// ForestOperationConfirmAck answers the action-2 confirm: the official vector
// with the target u32 patched at @5..8.
func ForestOperationConfirmAck(target uint32) []byte {
	body := forestInfo(forestConfirmAckHex)
	binary.LittleEndian.PutUint32(body[5:9], target)
	return body
}

// ForestStageClearEnabled builds the N31 clear banner (verbatim per stage).
func ForestStageClearEnabled(stage int) ([]byte, error) {
	switch stage {
	case 0:
		return forestInfo(forestClearStage0Hex), nil
	case 1:
		return forestInfo(forestClearStage1Hex), nil
	case 2:
		return forestInfo(forestClearStage2Hex), nil
	}
	return nil, fmt.Errorf("no official clear vector for stage %d", stage)
}

// ForestStageClearEnabledFor 按模式取 N31 清关横幅：Normal 与 Extreme 的
// token 完全不同（0xa79f/0x9d3f/0xc226 vs 0x5134/0x6f2a/0x8cf8），
// 用错模式的横幅会让翻牌面板（N2252 @7760 校验 token）对不上。
func ForestStageClearEnabledFor(stage int, hard bool) ([]byte, error) {
	if hard {
		return ForestHardStageClearEnabled(stage)
	}
	return ForestStageClearEnabled(stage)
}

// ---------------------------------------------------------------------------
// 第四轮：清关横幅链（N2252 翻牌 / N2253 第二排 / N1474 关卡倒计时）
// ---------------------------------------------------------------------------

// ForestStageLimits 是每关倒计时上限（秒）。官服 N1474（21:42:19.431，CMD37
// 加载完成后）首个 u32 = 0x0e10 = 3600 = 60 分钟，三关同值。
var ForestStageLimits = [3]uint32{3600, 3600, 3600}

// ForestStageToken 取该关清关横幅（N31）头 2B token——N2252 @7760 必须与其
// 呼应（军团家族契约，客户端只校验这两处一致）。
func ForestStageToken(stage int) ([2]byte, error) {
	return ForestStageTokenFor(stage, false)
}

// ForestStageTokenFor 按模式取 N31 token（Extreme 的三关 token 与 Normal 不同）。
func ForestStageTokenFor(stage int, hard bool) ([2]byte, error) {
	banner, err := ForestStageClearEnabledFor(stage, hard)
	if err != nil {
		return [2]byte{}, err
	}
	return [2]byte{banner[0], banner[1]}, nil
}

// 官服清关链第二排 N2253（21:43:02.278 / 21:45:25.205 两关逐字节一致）：
// 官服以 zlib 压缩发送（56B，解压 2405B），内容为 40B 记录步长的清关材料
// 列表（10367278×11、10367238×10 等三条记录）。**私服按解压后的裸 2405B
// 发送**——本机客户端被维纳斯实证的 N2253 传输形状是裸字节（维纳斯裸
// 2405B 翻牌正常）；官服压缩形状在 2026-10-05 20:13 实测击杀瞬间卡死、
// 3 秒后闪退（CrashDNF2.cra），故弃用压缩、保留官服内容。
const forestAdditionalRewardHex = "012E179E000B0000000300000000000000000000000000000000000000000000" +
	"00000000000000000006139E000A000000030000000000000000000000000000" +
	"0000000000000000000000000000000000200E9E000600000003000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000000000000000000000000000000000000000000000000000000000" +
	"0000000000"

// len=2405B

// ForestAdditionalClearReward returns the official N2253 second row verbatim
// (raw 2405B — the transport shape proven on this client by venus).
func ForestAdditionalClearReward() []byte { return forestInfo(forestAdditionalRewardHex) }
