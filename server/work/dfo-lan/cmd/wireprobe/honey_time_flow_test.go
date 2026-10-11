package main

import (
	"dfolan/internal/honeytime"
	"testing"
)

// TestHoneyTimeStateBodyWritesOnlyChosenSlot 钉住「改一格看一格」的那条口子：
// 累计秒数只写进**选中的那条轨道**，其余字节全 0。
//
// 为什么必须这么严：逐格语义只确证了"那个 u32 是秒"（`sub_143CC72A0` 里 `v8 = 1000 * a3`），
// 其余字段还没证。所以实机翻语义时必须**一次只动一格**；一旦写入溢出到别的格，
// 面板的变化就说明不了任何事、还会污染结论。
func TestHoneyTimeStateBodyWritesOnlyChosenSlot(t *testing.T) {
	st := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1, Seconds: 1234}
	// 这个用例只关心"秒数只写选中那条轨"；把已领数开关也钉成缺省，免得外部环境干扰。
	// 期间轨**显式关掉**：本用例只谈日档，不该被期间轨的写入干扰（缺省现在是判别探针）。
	t.Setenv("DFO_HONEYTIME_CLAIMED", "")
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "off")
	for slot := 0; slot < 4; slot++ {
		t.Setenv("DFO_HONEYTIME_SLOT", string(rune('0'+slot)))
		body, got, err := honeyTimeStateBody(st)
		if err != nil {
			t.Fatalf("slot=%d 编码失败: %v", slot, err)
		}
		if got != slot {
			t.Fatalf("slot=%d 却写成 %d", slot, got)
		}
		if want := uint32(1234); honeyTimeSecondsAt(body, slot) != want {
			t.Fatalf("slot=%d 的秒数 = %d, want %d", slot, honeyTimeSecondsAt(body, slot), want)
		}
		for other := 0; other < 4; other++ {
			if other != slot && honeyTimeSecondsAt(body, other) != 0 {
				t.Fatalf("slot=%d 时第 %d 条轨道被意外写成了 %d", slot, other, honeyTimeSecondsAt(body, other))
			}
		}
		// 除活动号（@0..4）、那一格 u32、以及**该轨自己的「标志」字节**（有进度时必须置 1，
		// 否则整档会被 UI 跳过）外，其余字节必须全 0。
		guard := honeyTimeSlotOffsets[slot]
		flagOff := honeyTimeSlotFlagOffsets[slot]
		for i := 4; i < len(body); i++ {
			if i >= guard && i < guard+4 {
				continue
			}
			if flagOff != 0 && i == flagOff {
				if body[i] != 1 {
					t.Fatalf("slot=%d 的标志字节 = %d，应为 1（该轨有进度）", slot, body[i])
				}
				continue
			}
			if body[i] != 0 {
				t.Fatalf("slot=%d 的第 %d 字节非 0: %d", slot, i, body[i])
			}
		}
	}
}

// TestHoneyTimeSlotRejectsBadValue 钉住开关的取值口径：非法值必须报错而不是静默回落 ——
// 静默回落会让实机验证得到"改了开关但没变"的假结论。
func TestHoneyTimeSlotRejectsBadValue(t *testing.T) {
	for _, raw := range []string{"9", "-1", "x", "1.5"} {
		t.Setenv("DFO_HONEYTIME_SLOT", raw)
		if _, _, err := honeyTimeStateBody(honeytime.State{Version: 1, EventID: honeytime.EventID}); err == nil {
			t.Fatalf("%q 没有被拒", raw)
		}
	}
	// 未设 = 轨道 0
	t.Setenv("DFO_HONEYTIME_SLOT", "")
	if _, slot, err := honeyTimeStateBody(honeytime.State{Version: 1, EventID: honeytime.EventID}); err != nil || slot != 0 {
		t.Fatalf("缺省槽位应为 0: slot=%d err=%v", slot, err)
	}
}

// TestHoneyTimeStateBodyWritesPeriodFlag 钉住**期间轨的"标志"字节**（第 1 条轨 @25）。
//
// ★ 为什么不写它整档就废：UI 填表循环第一句是 `if (标志 == 0) continue;`，而客户端只让
// 第 1/2 条轨的标志来自流（第 0/3 条是常数 1）⇒ 我们全 0 发下去时，期间轨整档被跳过、
// 面板下面那行点不动（业主实机）。标志置 1 = 参与。
func TestHoneyTimeStateBodyWritesPeriodFlag(t *testing.T) {
	t.Setenv("DFO_HONEYTIME_SLOT", "0")
	t.Setenv("DFO_HONEYTIME_CLAIMED", "all")
	// 显式指定期间轨 = 第 1 条（缺省现在是判别探针，会写三个不同的值）。
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "1")

	// 期间轨有进度 ⇒ 第 1 条轨的标志必须 = 1。
	withPeriod := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1,
		Seconds: 700, Claimed: 0b1, PeriodSeconds: 216001}
	body, _, err := honeyTimeStateBody(withPeriod)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if got := body[honeyTimeSlotFlagOffsets[1]]; got != 1 {
		t.Fatalf("期间轨标志 = %d，应为 1（否则整档被 UI 跳过）", got)
	}
	if got := honeyTimeSecondsAt(body, 1); got != 216001 {
		t.Fatalf("期间轨秒数 = %d，应为 216001", got)
	}

	// 期间轨没进度 ⇒ 标志留 0（"这条轨不参与"，与 PC 房变种那条一致）。
	noPeriod := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1,
		Seconds: 700, Claimed: 0b1}
	body2, _, err := honeyTimeStateBody(noPeriod)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if got := body2[honeyTimeSlotFlagOffsets[1]]; got != 0 {
		t.Fatalf("期间轨无进度时标志 = %d，应为 0", got)
	}
	// 第 0 条轨的标志格**不在流里**（offset = 0）⇒ 我们一个字节都不碰（客户端常数 1）。
	if honeyTimeSlotFlagOffsets[0] != 0 {
		t.Fatal("第 0 条轨不该有标志格（客户端是常数 1）")
	}
}

// TestHoneyTimeStateBodyWritesClaimedCount 钉住"已领"落到哪几格、以及各开关档位的差别。
//
// 为什么这么严：默认那一格（16 字节块）是有 L0 证据链的（`sub_143CC7E70` 算"应得"、
// `sub_143CC6F30` 拿块逐字节比），但整条链没有**官服明文对照**；`seq`/`tail` 更是纯推断，
// 且 2026-10-10 实机已证明**只写 seq 不够**。实机翻语义靠的就是
// `DFO_HONEYTIME_CLAIMED=block|all|seq|tail|off` 对照 —— 开关一旦静默失效，结论就是假的。
//
// 另外：块内下标是**行首值**（`Tier.Index`），所以 Claimed=0b101 必须落在块的第 0、2 字节，
// 而不是"前两个字节"。
func TestHoneyTimeStateBodyWritesClaimedCount(t *testing.T) {
	st := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1, Seconds: 700, Claimed: 0b101}

	readU32 := func(body []byte, at int) uint32 {
		return uint32(body[at]) | uint32(body[at+1])<<8 | uint32(body[at+2])<<16 | uint32(body[at+3])<<24
	}
	// 检查：秒数在该轨的格子里；已领只出现在开关打开的那几格；其余全 0。
	check := func(t *testing.T, body []byte, slot int, wantSeq, wantTail uint32, wantBlock []int) {
		t.Helper()
		if got := honeyTimeSecondsAt(body, slot); got != 700 {
			t.Fatalf("slot%d 秒数 = %d，应为 700", slot, got)
		}
		sec, seq, tail, block := honeyTimeSlotOffsets[slot], honeyTimeSlotSeqOffsets[slot],
			honeyTimeTailCountOffsets[slot], honeyTimeSlotBlockOffsets[slot]
		allowed := func(i int) bool {
			if i >= sec && i < sec+4 {
				return true
			}
			if seq != 0 && i == seq && wantSeq != 0 {
				return true
			}
			if i >= tail && i < tail+4 && wantTail != 0 {
				return true
			}
			if block != 0 && i >= block && i < block+16 {
				return true
			}
			return false
		}
		for i := 4; i < len(body); i++ {
			if !allowed(i) && body[i] != 0 {
				t.Fatalf("slot%d 第 %d 字节非 0: %d（不该写到别处）", slot, i, body[i])
			}
		}
		if seq != 0 && wantSeq != 0 && body[seq] != byte(wantSeq) {
			t.Fatalf("slot%d 序号格 = %d，应为 %d", slot, body[seq], wantSeq)
		}
		if wantTail != 0 && readU32(body, tail) != wantTail {
			t.Fatalf("slot%d 尾部已领数 = %d，应为 %d", slot, readU32(body, tail), wantTail)
		}
		if block != 0 {
			for k := 0; k < 16; k++ {
				want := byte(0)
				for _, w := range wantBlock {
					if w == k {
						want = 1
					}
				}
				if body[block+k] != want {
					t.Fatalf("slot%d 块第 %d 字节 = %d，应为 %d", slot, k, body[block+k], want)
				}
			}
		}
	}

	claimedRows := []int{0, 2} // Claimed=0b101
	t.Setenv("DFO_HONEYTIME_SLOT", "0")
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "off") // 本用例只谈日档
	for _, tc := range []struct {
		mode      string
		wantSeq   uint32
		wantTail  uint32
		wantBlock []int
	}{
		{"", 2, 2, claimedRows},          // 默认 = all（块管按钮；序号/尾部谁是打勾还没定案）
		{"block", 0, 0, claimedRows},     // 只管按钮
		{"block,seq", 2, 0, claimedRows},
		{"seq", 2, 0, nil},
		{"tail", 0, 2, nil},
		{"all", 2, 2, claimedRows},
		{"off", 0, 0, nil},
	} {
		t.Setenv("DFO_HONEYTIME_CLAIMED", tc.mode)
		body, _, err := honeyTimeStateBody(st)
		if err != nil {
			t.Fatalf("mode=%q 编码失败: %v", tc.mode, err)
		}
		check(t, body, 0, tc.wantSeq, tc.wantTail, tc.wantBlock)
	}

	// 非法取值必须报错（含组合里的非法项）。
	for _, bad := range []string{"yes", "block,nope", ","} {
		t.Setenv("DFO_HONEYTIME_CLAIMED", bad)
		if _, _, err := honeyTimeStateBody(st); err == nil {
			t.Fatalf("DFO_HONEYTIME_CLAIMED=%q 没有被拒", bad)
		}
	}
	if _, _, err := honeyTimeStateBody(st); err == nil {
		t.Fatal("DFO_HONEYTIME_CLAIMED=yes 没有被拒")
	}

	// 第 3 条轨在流里既没有块也没有序号格 ⇒ 只有尾部那一格可写。
	t.Setenv("DFO_HONEYTIME_CLAIMED", "all")
	t.Setenv("DFO_HONEYTIME_SLOT", "3")
	body3, _, err := honeyTimeStateBody(st)
	if err != nil {
		t.Fatalf("slot3 编码失败: %v", err)
	}
	check(t, body3, 3, 2, 2, nil)

	// 一行都没领时，默认档位必须**一个字节都不写**（而不是写一堆 0 之外的东西）。
	t.Setenv("DFO_HONEYTIME_CLAIMED", "")
	t.Setenv("DFO_HONEYTIME_SLOT", "0")
	fresh := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1, Seconds: 700}
	bodyFresh, _, err := honeyTimeStateBody(fresh)
	if err != nil {
		t.Fatalf("空已领编码失败: %v", err)
	}
	check(t, bodyFresh, 0, 0, 0, nil)
}

// TestHoneyTimePeriodProbeWritesDistinctValues 钉住**判别探针**的取值。
//
// 探针的意义：写第 1 条轨那行不亮、写全 3 条也没让"Complete 80h"动起来 ⇒ 不能只看"亮不亮"，
// 得看**数字**。所以三条候选轨写三个互不相同、且**都比日档剩余小**的值，
// 面板顶部那个"剩余最短的轨"的框就会切过去，显示出来的分钟数 + 框标题即可定性。
//
// 顺便钉住"第 1/2 条轨的标志必须为 1"（否则那两条轨整档被 UI 跳过，探针白做）。
func TestHoneyTimePeriodProbeWritesDistinctValues(t *testing.T) {
	t.Setenv("DFO_HONEYTIME_SLOT", "0")
	t.Setenv("DFO_HONEYTIME_CLAIMED", "off") // 探针只看秒数
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "probe")

	st := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1, Seconds: 4576}
	body, _, err := honeyTimeStateBody(st)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	want := map[int]uint32{1: 143900, 2: 215800, 3: 287700}
	for slot, seconds := range want {
		if got := honeyTimeSecondsAt(body, slot); got != seconds {
			t.Fatalf("探针第 %d 条轨的秒数 = %d，应为 %d", slot, got, seconds)
		}
	}
	if got := honeyTimeSecondsAt(body, 0); got != 4576 {
		t.Fatalf("日档秒数被探针改动了：%d", got)
	}
	for _, slot := range []int{1, 2} {
		if body[honeyTimeSlotFlagOffsets[slot]] != 1 {
			t.Fatalf("第 %d 条轨的标志应为 1（否则整档被跳过，探针无效）", slot)
		}
	}
	// off：期间轨一格都不写。
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "off")
	body, _, err = honeyTimeStateBody(st)
	if err != nil {
		t.Fatalf("off 编码失败: %v", err)
	}
	for _, slot := range []int{1, 2, 3} {
		if got := honeyTimeSecondsAt(body, slot); got != 0 {
			t.Fatalf("off 时第 %d 条轨不该被写：%d", slot, got)
		}
	}
}

// 缺省 = `period`：期间轨写**真实存档秒数**到第 1 格（`period` 组 @26）与尾部
// PeriodSeconds @60。
//
// 2026-10-11 改动（吸纳上游命名，见 `analysis/tasks/next196-313与上游比对-20261011.md`）：
// 上游把 1294 尾部 `@60` 命名为 **PeriodSeconds**，而我们 10-10 探针试过的"第 3 条轨"
// 正是这一格 —— 两边指向同一处；而且这里发的是**真实秒数**（不是探针假值），
// 所以缺省从 `off` 改成 `period`。
//
// ⚠️ 这条用例只锁"缺省写的是真值、两格一致、日档不受影响、off 仍是空"；
// **面板下面那行会不会动仍未证**（10-10 探针写 @60 时它没动）⇒ 要实机看。
func TestHoneyTimePeriodDefaultWritesRealSeconds(t *testing.T) {
	t.Setenv("DFO_HONEYTIME_SLOT", "0")
	t.Setenv("DFO_HONEYTIME_CLAIMED", "off")
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "") // 空串 = 未设，走缺省分支

	st := honeytime.State{Version: 1, EventID: honeytime.EventID, CycleStart: 1, Day: 1,
		Seconds: 4576, PeriodSeconds: 200000}
	body, _, err := honeyTimeStateBody(st)
	if err != nil {
		t.Fatalf("编码失败: %v", err)
	}
	if got := honeyTimeSecondsAt(body, 1); got != 200000 {
		t.Fatalf("缺省时第 1 格（period 组 @26）应为真实期间秒数 200000，实际 %d", got)
	}
	if got := honeyTimeSecondsAt(body, 3); got != 200000 {
		t.Fatalf("缺省时第 3 格（尾部 PeriodSeconds @60）应为真实期间秒数 200000，实际 %d", got)
	}
	if got := honeyTimeSecondsAt(body, 2); got != 0 {
		t.Fatalf("第 2 格（specified 组）缺省不该被写：%d", got)
	}
	if got := honeyTimeSecondsAt(body, 0); got != 4576 {
		t.Fatalf("日档仍要正常写：%d", got)
	}
	// ★ 轮数那一格（`@47`，落进客户端 `obj+80*档+140`）：客户端拿它当「档 1 迭代几轮」，
	// 不写就是 0 ⇒ 迭代 0 轮 ⇒ 逐档应得与剩余时间都算成 0 ⇒ 期间轨按钮不亮。
	// 源脚本 period 的 `[loop]` = 1。
	if body[47] != 1 {
		t.Fatalf("期间轨轮数 @47 = %d，应为 1（否则客户端迭代 0 轮、按钮不亮）", body[47])
	}
	if honeyTimePeriodUnwired() {
		t.Fatal("缺省已是 period，不该再判「未接通」")
	}
	// 显式 off 仍然一格都不写。
	t.Setenv("DFO_HONEYTIME_PERIOD_SLOT", "off")
	body, _, err = honeyTimeStateBody(st)
	if err != nil {
		t.Fatalf("off 编码失败: %v", err)
	}
	for _, slot := range []int{1, 3} {
		if got := honeyTimeSecondsAt(body, slot); got != 0 {
			t.Fatalf("off 时第 %d 条轨不该被写：%d", slot, got)
		}
	}
	if !honeyTimePeriodUnwired() {
		t.Fatal("显式 off 时必须判定为「未接通」")
	}
}
