package main

import (
	"encoding/binary"
	"testing"
	"time"

	"dfolan/internal/legion"
)

// TestMoonTimerSync 守沉月湖的整局时限帧（N1474 ENUM_NOTIPACKET_DUNGEON_TIMEOUT_TIME）。
//
// 形态**逐字段照官服**（2026-10-09 从 13 帧解密抓包解出，全部 16 字节、S2C）：
//
//	u32[0] = 时限秒（征讨类统一 = 3600 = 1 小时，业主 2026-10-09 口径）
//	u32[1] = 发帧那一刻的 Unix 秒（官服样本与该帧抓包时间逐秒吻合）
//	u32[2] / u32[3] = 每帧都不同，语义未解 —— 当前占位 0
//
// 本仓原来的 `LegionDungeonTimeout115` 只写 8 字节，与官服不符；这条用例把 16 字节钉住。
// 另外它**不复用**那个共用函数，免得顺带改动军团本三族（ispins/venus/forest）的行为。
func TestMoonTimerSync(t *testing.T) {
	now := time.Unix(1800000000, 0)
	p := moonTimerSync(now)
	if len(p) != 1 {
		t.Fatalf("期望恰好一帧，得到 %d", len(p))
	}
	if p[0].ID != legion.NotiDungeonTimeoutTime {
		t.Fatalf("opcode = %d，期望 %d", p[0].ID, legion.NotiDungeonTimeoutTime)
	}
	if len(p[0].Payload) != 16 {
		t.Fatalf("帧长 = %d，期望 16（官服 13 帧全部 16 字节）", len(p[0].Payload))
	}
	if got := binary.LittleEndian.Uint32(p[0].Payload[0:]); got != moonTimerSeconds {
		t.Fatalf("u32[0] 时限秒 = %d，期望 %d", got, moonTimerSeconds)
	}
	if moonTimerSeconds != 3600 {
		t.Fatalf("征讨类时限应为 1 小时 = 3600 秒（减负后统一口径），当前 %d", moonTimerSeconds)
	}
	if got := binary.LittleEndian.Uint32(p[0].Payload[4:]); got != uint32(now.Unix()) {
		t.Fatalf("u32[1] 起始秒 = %d，期望 %d（发帧那一刻）", got, now.Unix())
	}
}
// TestMoonTimerLimitOverride 守取证开关 DFO_MOON_TIMER_LIMIT：
// 默认必须是 3600（口径不能漂），只有显式给正整数时才覆盖（否则忽略）。
func TestMoonTimerLimitOverride(t *testing.T) {
	if got := moonTimerLimit(); got != moonTimerSeconds {
		t.Fatalf("未设开关时 = %d，期望 %d", got, moonTimerSeconds)
	}
	t.Setenv("DFO_MOON_TIMER_LIMIT", "43200")
	if got := moonTimerLimit(); got != 43200 {
		t.Fatalf("设 43200 时 = %d", got)
	}
	t.Setenv("DFO_MOON_TIMER_LIMIT", "0")
	if got := moonTimerLimit(); got != moonTimerSeconds {
		t.Fatalf("设 0 应被忽略，得到 %d", got)
	}
	t.Setenv("DFO_MOON_TIMER_LIMIT", "abc")
	if got := moonTimerLimit(); got != moonTimerSeconds {
		t.Fatalf("非法值应被忽略，得到 %d", got)
	}
}
