package protocol

import (
	"encoding/binary"
	"testing"
)

// TestHoneyTimeEventInfoZero115 钉住 NOTI1294 的**长度与活动号偏移**。
//
// 长度 **81** 是**结构性**的（handler `sub_1452D4B30` 的逐条读流 + 循环边界反汇编）：
// 档数取自 `sub_143CC6090(obj, 1)`，它直接 `return 4`（常量，不从流里读）⇒ 尾部恒 16 B。
// ★ 这里写死 81 是**故意的**：80 会少一个字节，客户端回 CMD217 OVERFLOW_INFO 并
// **丢弃当前 TCP 段剩下的帧**（2026-10-10 实测：紧跟其后的 NOTI398 被吞掉 ⇒ 左上角
// Liberation Trace 面板复活）。谁把它"简化"回 80，这条用例立刻红。
//
// ⚠️ 各字段的**语义**尚未确证（见 honey_time115.go 的注释）：这里只守形状，
// 不假装能证明"客户端认这一帧"—— 那只能靠实机面板。
func TestHoneyTimeEventInfoZero115(t *testing.T) {
	const eventID = 313
	if NotiHoneyTimeEventInfoBytes != 81 {
		t.Fatalf("NOTI1294 长度被改成了 %d —— 80 会少一字节触发 PACKET OVERFLOW（见文件头）", NotiHoneyTimeEventInfoBytes)
	}
	body := HoneyTimeEventInfoZero115(eventID)
	if len(body) != NotiHoneyTimeEventInfoBytes {
		t.Fatalf("载荷 %d 字节, want %d", len(body), NotiHoneyTimeEventInfoBytes)
	}
	if got := binary.LittleEndian.Uint32(body); got != eventID {
		t.Fatalf("活动号 = %d, want %d", got, eventID)
	}
	for i := 4; i < len(body); i++ {
		if body[i] != 0 {
			t.Fatalf("中性态的第 %d 字节非 0: %d", i, body[i])
		}
	}
}

// TestHoneyTimeEventInfoWriterWritesOnlyRequestedOffsets 钉住"按偏移写入"那条口子：
// 写哪一格就只有那一格变（语义未证期间，改一格看一格是唯一的翻法）。
func TestHoneyTimeEventInfoWriterWritesOnlyRequestedOffsets(t *testing.T) {
	body := HoneyTimeEventInfo115(313, func(offset int, value byte) byte {
		if offset == 4 {
			return 7
		}
		return value
	})
	if body[4] != 7 {
		t.Fatalf("偏移 4 没写进去: %d", body[4])
	}
	for i := 5; i < len(body); i++ {
		if body[i] != 0 {
			t.Fatalf("只该改偏移 4，第 %d 字节却是 %d", i, body[i])
		}
	}
}

// TestDecodeHoneyTimeClaimRealSamples 钉住 313 的 680 领奖请求口径：
//
//	8 字节   `39010000 00000000`                  ← 进城自动帧：只有活动号 + 动作类型，**读不出行号**
//	16 字节  `39010000 01000000 00000000 00000000` ← 点第 1 档
//	16 字节  `39010000 01000000 01000000 00000000` ← 点第 2 档
//
// ★ 为什么必须写死这两个样本：**行号在第 3 个 u32（偏移 8）**，第 2 个是动作类型。
// 一开始照 331 的"活动号 + 行号"口径读第 2 个 ⇒ 第 2 档会被读成"又点第 1 档"，
// 于是又一次去领已领过的那一档（无邮件、也不报错），玩家看到的是"点了没反应/像被拒"。
// 同族命令共享包头，但**载荷口径不共享** —— 谁把行号改回偏移 4，这里立刻红。
// 实机样本（2026-10-10，客户端轨迹 + 服务端明文双证）：
//
//	8 字节   `39010000 00000000`                   ← 进城自动帧：活动号 + 动作类型 0
//	16 字节  `39010000 01000000 00000000 00000000`  ← 点日档第 1 档
//	16 字节  `39010000 01000000 01000000 00000000`  ← 点日档第 2 档
//
// ★ 为什么必须写死这两个样本：**下标在第 3 个 u32（偏移 8）**，第 2 个是动作类型。
// 一开始照 331 的"活动号 + 行号"口径读第 2 个 ⇒ 第 2 档会被读成"又点第 1 档"，
// 于是又一次去领已领过的那一档（无邮件、也不报错），玩家看到的是"点了没反应/像被拒"。
// 同族命令共享包头，但**载荷口径不共享** —— 谁把下标改回偏移 4，这里立刻红。
func TestDecodeHoneyTimeClaimRealSamples(t *testing.T) {
	const (
		autoEntry  = "3901000000000000"
		firstTier  = "39010000010000000000000000000000"
		secondTier = "39010000010000000100000000000000"
	)
	// 进城自动帧：长度不够 ⇒ 必须报错（调用方据此放行、不回应答）。
	if _, _, err := DecodeHoneyTimeRequest115(mustHex(t, autoEntry)); err == nil {
		t.Fatal("8 字节的进城自动帧不该能读出档位下标")
	}
	// 点日档第 1 档：动作类型 1、下标 0。
	action, offset, err := DecodeHoneyTimeRequest115(mustHex(t, firstTier))
	if err != nil || action != 1 || offset != 0 {
		t.Fatalf("第 1 档：action=%d offset=%d err=%v，应为 1/0", action, offset, err)
	}
	// 点日档第 2 档：动作类型仍是 1、下标 1（★ 变的是第 3 个 u32）。
	action, offset, err = DecodeHoneyTimeRequest115(mustHex(t, secondTier))
	if err != nil || action != 1 || offset != 1 {
		t.Fatalf("第 2 档：action=%d offset=%d err=%v，应为 1/1", action, offset, err)
	}
}

// 动作类型 → 轨（上游从 UI 虚拟回调 14412c770 读出：四个 group 数组对应 action 1/2/3/5，
// **action 4 被跳过**）。本季脚本只有 daily/period 两块 ⇒ 1 = 日档、2 = 期间轨。
//
// ⚠️ action 2（期间轨）目前**没有实机样本**（本仓只抓到过 action 1 的点击帧）——
// 这条断言锁的是"映射表不许被改回去"，**不是**"实机已证 action 2"。取到 313/2 的真实帧后
// 应把样本补进来。
func TestHoneyTimeTrackForActionMapping(t *testing.T) {
	cases := []struct {
		action uint32
		track  uint32
		ok     bool
	}{
		{1, HoneyTimeTrackDaily, true},
		{2, HoneyTimeTrackPeriod, true},
		{3, HoneyTimeTrackSpecified, true},
		{5, HoneyTimeTrackExtendDaily, true},
		{0, 0, false}, // 同步帧
		{4, 0, false}, // 上游明确说被跳过
		{6, 0, false},
	}
	for _, c := range cases {
		track, ok := HoneyTimeTrackForAction(c.action)
		if ok != c.ok || track != c.track {
			t.Fatalf("action %d → track=%d ok=%v，应为 %d/%v", c.action, track, ok, c.track, c.ok)
		}
	}
}

