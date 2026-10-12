package main

// CMD2414（分解）请求正文的解析。
//
// 样本是 2026-10-12 实机捕获的原帧（会话日志 `oracle_action_received`，`command=disjoint`
// 的 `request_hex`，120 字节）：只填了第 1 格 = {槽位 7, 件数 1}，其余 29 格是客户端自己铺的
// 空格子 (0xFFFF, 0)。

import (
	"encoding/hex"
	"strings"
	"testing"
)

func TestSimResearchDisjointRequest115RealSample(t *testing.T) {
	const raw = "07000100" + "ffff0000" // 下面按 30 格补齐
	hexs := raw + strings.Repeat("ffff0000", 28)
	plain, err := hex.DecodeString(hexs)
	if err != nil {
		t.Fatalf("坏样本: %v", err)
	}
	if len(plain) != 120 {
		t.Fatalf("样本长度 %d，期望 120", len(plain))
	}
	got, err := simResearchDisjointRequest115(plain)
	if err != nil {
		t.Fatalf("意外失败: %v", err)
	}
	if len(got) != 1 || got[0].Slot != 7 || got[0].Count != 1 {
		t.Fatalf("解析结果 = %+v，期望 [{Slot:7 Count:1}]", got)
	}
}

func TestSimResearchDisjointRequest115MultipleEntries(t *testing.T) {
	var b []byte
	// 3 格有效：{槽 0, 3}、{槽 23, 2}、{槽 15, 1}；其余空格。
	for i := 0; i < 30; i++ {
		slot, count := uint16(0xFFFF), uint16(0)
		switch i {
		case 0:
			slot, count = 0, 3
		case 1:
			slot, count = 23, 2
		case 2:
			slot, count = 15, 1
		}
		b = append(b, byte(slot), byte(slot>>8), byte(count), byte(count>>8))
	}
	got, err := simResearchDisjointRequest115(b)
	if err != nil {
		t.Fatalf("意外失败: %v", err)
	}
	want := []simResearchDisjointEntry{{Slot: 0, Count: 3}, {Slot: 23, Count: 2}, {Slot: 15, Count: 1}}
	if len(got) != len(want) {
		t.Fatalf("条目数 %d，期望 %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("第 %d 条 = %+v，期望 %+v", i, got[i], want[i])
		}
	}
}

func TestSimResearchDisjointRequest115Rejects(t *testing.T) {
	// 太短
	if _, err := simResearchDisjointRequest115(make([]byte, 8)); err == nil {
		t.Error("8 字节该被拒")
	}
	// 30 格全空
	all := make([]byte, 120)
	for i := 0; i < 30; i++ {
		all[i*4] = 0xFF
		all[i*4+1] = 0xFF
	}
	if _, err := simResearchDisjointRequest115(all); err == nil {
		t.Error("全空该被拒（客户端收到应答后会清空选择，全空说明本来就没选）")
	}
	// 件数为 0 的格子要按"空"处理：只有 (槽 5, 0) ⇒ 也算全空
	zero := make([]byte, 120)
	for i := 0; i < 30; i++ {
		zero[i*4] = 0xFF
		zero[i*4+1] = 0xFF
	}
	zero[0], zero[1] = 5, 0
	if _, err := simResearchDisjointRequest115(zero); err == nil {
		t.Error("件数 0 该按空格处理 ⇒ 全空该被拒")
	}
}
