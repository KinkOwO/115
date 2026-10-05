package main

// 662 活动入口修复 + 诊断开关（2026-10-06）。
//
// ## 根因（由参考实现证实）
//
// 客户端对 NOTI108 是**整表替换**，不是合并：后发的一条会抹掉先发那一条的行。
// 参考实现 `活动Boost与胶囊教学-20260927.zip` 的
// `internal/game/protocol/boostup_events115.go` 带一个 `base` 参数，首行注释就是结论：
//
//	// Merge with our owned full snapshot, never send a second N108 which erases
//	// the existing channel gates.
//
// 它的接线是**只发一条** 108 —— 把活动行并进那张承载频道门的基础表；
// 参考版的 `cmd/wireprobe/boostup_flow.go` 里**根本没有**独立的活动 108 发送函数。
//
// 本树（以及 `662-merge-full` 血统）丢了这个 `base`：选角时先发一条**只含活动行**的
// 108（`boostup_flow.go` 的 sendBoostChannelEvents），进城时再发
// event_info_generated.go 那张 19 行频道门表（entry_flow.go 的 announce）。
// 第二条排在后面 ⇒ 进城后 662/10017/10018 **被抹掉** ⇒ 城里没有活动礼物图标。
//
// 修复 = 把活动行合并进进城那条表体，让最后一条 108 同时带上频道门与活动行。
// 选角那条含活动行的 108 保留：它排在前面，被覆盖也无害（若哪天真确认选角屏
// 也需要它，留着更稳）；参考版不发的做法与本修复的最终表状态一致。
//
// ## 证据链
//
//   - 客户端轨迹：选角 18:40:03 `[event on]: 662/10017/10018` → 进城 18:40:42
//     的 1144 B 表里没有这三条（`client_trace.txt`）。
//   - 对照实验：注掉 newchar.lua 全部发放后新建角色，图标依旧不出现
//     （`character_create` 计数 0）⇒ 与起始发放无关。
//   - 参考实现：`BoostOpeningEvents115(base, …)` 的合并语义 + 「第二条 N108 会抹掉」
//     的原文注释；参考版无独立活动 108 发送点。
//
// ## 开关
//
// 默认 = **合并**（修复后行为）。`DFO_EVENT_INFO_VARIANT` 可复现反例做 A/B：
//
//	（未设 / boost）合并表 —— 修复后行为，默认。
//	plain            未合并的 19 行表 —— 复现「城里没有活动图标」。
//	legacy           旧架构那份 54 B / 1 条 —— 更早的现场配置，仅留档取证。
//
// 由 `configs/pvf-default.json` 的 `environment.DFO_EVENT_INFO_VARIANT` 控制
// （启动器读 profile 的 environment 注入子进程），**换值只需重启，不必重编**。
import (
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"os"
)

// eventInfoTableLegacyHex 是 2026-10-04 之前的单记录表体（54 B：count 1 +
// 0x0308「Ispins Legion Open」记录 + 0x00 尾），逐字取自旧分支
// `feat/boostup662-on-main` 的 event_info_generated.go，不做任何改写。
const eventInfoTableLegacyHex = "" +
	"0100080301000012000000497370696e73204c6567696f6e204f70656e" +
	"0000000000000000c041f4635f6b6178000000000000000000"

// eventInfoTableLegacy 解码上表（只解一次）。
var eventInfoTableLegacy = mustHexDecode(eventInfoTableLegacyHex)

// townEventInfoTable 是**进城 announce 实际发出的** 108 表体。默认即合并表：
// 频道门 19 行 + 活动 3 行，一条发完，客户端不会再把活动行抹掉。
var townEventInfoTable = func() []byte {
	switch os.Getenv("DFO_EVENT_INFO_VARIANT") {
	case "plain":
		return eventInfoTable
	case "legacy":
		return eventInfoTableLegacy
	}
	// 缺省与 "boost" 都走合并；合并失败退回原表，别把进镇流程搞挂。
	if merged, ok := buildTownEventInfoTable(false); ok {
		return merged
	}
	return eventInfoTable
}()

// buildTownEventInfoTable 把活动行追加到生成表之后，得到「频道门 + 活动行」的
// 单张表。两个生产者（本函数与选角/进城两条 108）用的行长完全一致
// （`u16 id, u8×3, str×3, u32 start, u32 end, str×2, u8 flag`），所以这里直接
// 复用选角那条 108 已经在用的同一个编码器，**不在这里重新敲一遍行字节**
// （§0.2 单一规则）。challenge = 是否附带毕业后的 665 行（默认关）。
func buildTownEventInfoTable(challenge bool) ([]byte, bool) {
	var args []bool
	if challenge {
		args = []bool{true}
	}
	rows, err := protocol.BoostOpeningEvents115(0, boostup.EventEnd, args...)
	if err != nil || len(rows) < 3 {
		return nil, false
	}
	base := eventInfoTable
	// 生成表 = count(2) + N 条 + 0x00 尾；形状不符就不动它。
	if len(base) < 3 || base[len(base)-1] != 0 {
		return nil, false
	}
	added := len(rows) - 3 // 去掉前导 count(2) 与尾部 0x00
	boostRows := rows[2 : len(rows)-1]
	official := base[2 : len(base)-1]

	total := binary.LittleEndian.Uint16(base[:2]) + 3
	out := make([]byte, 0, len(base)+added)
	out = append(out, byte(total), byte(total>>8))
	out = append(out, official...)
	out = append(out, boostRows...)
	return append(out, 0), true
}
