package legion

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// 官服 Extreme 状态包 N2565 是 **64B** 形状（2026-10-08 抓包 session_s13），
// 不是 N2563 的 136B。第 11/12 轮的「N2563 家族同构」正是被这条推翻的，
// 这里用尺寸/状态/三关副本号把它钉住。
func TestForestHardInfoShape(t *testing.T) {
	waiting := ForestHardWaitingInfo()
	if len(waiting) != ForestHardInfoSize {
		t.Fatalf("N2565 waiting body = %d bytes, want %d", len(waiting), ForestHardInfoSize)
	}
	if waiting[2] != ForestHardStateWaiting {
		t.Fatalf("waiting state = %#x, want %#x", waiting[2], ForestHardStateWaiting)
	}
	// 等待态三关字段全是 ff（官服 22:05:38.308）。
	for _, off := range []int{26, 38, 50} {
		if got := binary.LittleEndian.Uint32(waiting[off:]); got != 0xffffffff {
			t.Fatalf("waiting stage id @%d = %#x, want ffffffff", off, got)
		}
	}
	for stage := 0; stage < 3; stage++ {
		window, err := ForestHardWindowInfo(stage)
		if err != nil {
			t.Fatal(err)
		}
		if len(window) != ForestHardInfoSize {
			t.Fatalf("stage %d window = %d bytes", stage, len(window))
		}
		if window[2] != ForestHardStateActive {
			t.Fatalf("stage %d window state = %#x, want %#x", stage, window[2], ForestHardStateActive)
		}
		if int(window[10]) != stage {
			t.Fatalf("stage %d window @10 = %d", stage, window[10])
		}
		for i, want := range ForestHardStageDungeons {
			if got := binary.LittleEndian.Uint32(window[26+12*i:]); got != want {
				t.Fatalf("stage %d window dungeon slot %d = %d, want %d", stage, i, got, want)
			}
		}
		tick, err := ForestHardClearTickInfo(stage)
		if err != nil {
			t.Fatal(err)
		}
		if tick[6] != 3 {
			t.Fatalf("stage %d clear tick @6 = %#x, want 03", stage, tick[6])
		}
		if int(tick[10]) != stage {
			t.Fatalf("stage %d clear tick @10 = %d", stage, tick[10])
		}
	}
	if got := ForestHardFinalInfo()[2]; got != ForestHardStateFinale {
		t.Fatalf("finale state = %#x, want %#x", got, ForestHardStateFinale)
	}
	if got := ForestHardLeaveInfo()[2]; got != ForestHardStateLeave {
		t.Fatalf("leave state = %#x, want %#x", got, ForestHardStateLeave)
	}
	if got := ForestHardIdleInfo()[2]; got != ForestHardStateIdle {
		t.Fatalf("idle state = %#x, want %#x", got, ForestHardStateIdle)
	}
}

// N2566 过段 tick 携带三关 N31 横幅的 token —— 官服契约（两处必须一致）。
func TestForestHardPhaseClearTickMatchesBanners(t *testing.T) {
	tick := ForestHardPhaseClearTick()
	if len(tick) != ForestHardInfoSize {
		t.Fatalf("N2566 body = %d bytes, want %d", len(tick), ForestHardInfoSize)
	}
	for stage := 0; stage < 3; stage++ {
		banner, err := ForestHardStageClearEnabled(stage)
		if err != nil {
			t.Fatal(err)
		}
		want := uint32(banner[0]) | uint32(banner[1])<<8
		if got := binary.LittleEndian.Uint32(tick[8*stage:]); got != want {
			t.Fatalf("N2566 token %d = %#x, want %#x (banner %x)", stage, got, want, banner)
		}
	}
	// Extreme 与 Normal 的横幅 token 不同：用错模式客户端翻牌面板不显示。
	for stage := 0; stage < 3; stage++ {
		normal, err := ForestStageClearEnabledFor(stage, false)
		if err != nil {
			t.Fatal(err)
		}
		hard, err := ForestStageClearEnabledFor(stage, true)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Equal(normal[:2], hard[:2]) {
			t.Fatalf("stage %d Normal and Extreme banner tokens are identical: %x", stage, normal[:2])
		}
	}
}

// CMD2045/CMD2046 应答是官服原文：内容号 105 + 阶段号。
func TestForestHardAcksCarryContent105(t *testing.T) {
	for stage := uint32(0); stage < 3; stage++ {
		ack, err := ForestHardEnterAck(stage)
		if err != nil {
			t.Fatal(err)
		}
		if len(ack) != 24 || ack[0] != 1 {
			t.Fatalf("stage %d enter ack shape %x", stage, ack)
		}
		if got := binary.LittleEndian.Uint32(ack[5:]); got != ForestHardContentID {
			t.Fatalf("stage %d enter ack content = %d, want %d", stage, got, ForestHardContentID)
		}
		if got := binary.LittleEndian.Uint32(ack[9:]); got != stage {
			t.Fatalf("stage %d enter ack stage field = %d", stage, got)
		}
	}
	if _, err := ForestHardEnterAck(3); err == nil {
		t.Fatal("stage 3 has no official enter ack and must be refused")
	}
	end := ForestHardRewardEndAck()
	if len(end) != 32 || end[0] != 1 {
		t.Fatalf("reward end ack shape %x", end)
	}
	if got := binary.LittleEndian.Uint32(end[5:]); got != ForestHardContentID {
		t.Fatalf("reward end content = %d", got)
	}
	if got := binary.LittleEndian.Uint32(end[9:]); got != 2 {
		t.Fatalf("reward end stage = %d, want 2", got)
	}
	start := ForestHardStartAck()
	if len(start) != 16 || start[0] != 1 {
		t.Fatalf("start ack = %x, want the official 16B success frame", start)
	}
	if len(ForestHardPrepareEnterInfo()) != 24 {
		t.Fatalf("N2568 body = %d bytes, want 24", len(ForestHardPrepareEnterInfo()))
	}
}

// Extreme 终局翻牌是官服 2026-10-08 抓包解压后的真表：七行全部落在 44B 记录区
// （@1600+44k），40B 步长的头两行留空，@7760 = 第三关 token。
func TestForestHardRewardLayout(t *testing.T) {
	items := ForestHardFinalBasic()
	if len(items) != 7 {
		t.Fatalf("official Extreme N2252 has 7 rows, table has %d", len(items))
	}
	flip, err := ForestBasicClearReward(2, items, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(flip) != 7772 {
		t.Fatalf("N2252 body = %d bytes, want 7772", len(flip))
	}
	for off := 0; off < 80; off++ {
		if flip[off] != 0 {
			t.Fatalf("Extreme N2252 first two rows must stay empty, byte %d = %#x", off, flip[off])
		}
	}
	for i, item := range items {
		off := 1600 + 44*i
		if got := binary.LittleEndian.Uint32(flip[off:]); got != item.Template {
			t.Fatalf("row %d template = %d, want %d", i, got, item.Template)
		}
		if got := binary.LittleEndian.Uint32(flip[off+4:]); got != item.Amount {
			t.Fatalf("row %d amount = %d, want %d", i, got, item.Amount)
		}
	}
	// 官方 @1600/@1644/@1688/@1732/@1776/@1820/@1864（解压后逐字节对照）。
	want := []ForestRewardItem{
		{10360622, 33}, {10359558, 30}, {10358304, 20}, {10361705, 3},
		{10362184, 1}, {10403248, 1}, {10360622, 16},
	}
	for i, w := range want {
		if items[i] != w {
			t.Fatalf("row %d = %+v, want %+v", i, items[i], w)
		}
	}
	// 10403248（可选箱）行尾 +40 = 1，其余为 0。
	if got := binary.LittleEndian.Uint32(flip[1600+44*5+40:]); got != 1 {
		t.Fatalf("selectable box row flag = %d, want 1", got)
	}
	if got := binary.LittleEndian.Uint32(flip[1600+44*0+40:]); got != 0 {
		t.Fatalf("ordinary row flag = %d, want 0", got)
	}
	token, err := ForestHardStageToken(2)
	if err != nil {
		t.Fatal(err)
	}
	if flip[7760] != token[0] || flip[7761] != token[1] {
		t.Fatalf("N2252 token = %x %x, want Extreme stage2 token %x", flip[7760], flip[7761], token)
	}
}

// Extreme N2253 只有一行，且尾字节是 00（Normal 同类行是 03）。
func TestForestHardAdditionalRewardRow(t *testing.T) {
	add, err := ForestAdditionalReward(ForestHardFinalAdditional(), true)
	if err != nil {
		t.Fatal(err)
	}
	if len(add) != 2405 {
		t.Fatalf("N2253 body = %d bytes, want 2405", len(add))
	}
	if add[0] != 1 {
		t.Fatalf("row flag = %#x, want 01", add[0])
	}
	if got := binary.LittleEndian.Uint32(add[1:]); got != ForestHardRewardPromise {
		t.Fatalf("row template = %d, want %d", got, ForestHardRewardPromise)
	}
	if add[5] != 1 {
		t.Fatalf("row count = %d, want 1", add[5])
	}
	if add[9] != 0 {
		t.Fatalf("Extreme row tail byte = %#x, want 00", add[9])
	}
	normal, err := ForestAdditionalReward(ForestNormalStageRewards(), false)
	if err != nil {
		t.Fatal(err)
	}
	if normal[9] != 3 {
		t.Fatalf("Normal row tail byte = %#x, want 03 (regression guard)", normal[9])
	}
}
