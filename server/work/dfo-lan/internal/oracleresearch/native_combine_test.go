package oracleresearch

// 真源解析：`[combine]` 三个 result 槽的 `[args]` = 解锁门槛（本机 0 / 10 / 15）。
//
// 这条路径**每次启动都会走**（目录装配时解析），所以一旦源的形状变了、或者我们的解析写错，
// 服务端起不来 —— 用这个测试钉住它。本机 PVF 不在时自动跳过（与仓里其它 native 测试同一约定）。

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

func TestNativeCombineUnlockThresholds(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skipf("本机 PVF 不在: %s", p)
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1 << 30}, "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer a.Close()
	cat, err := Load(a)
	if err != nil {
		t.Fatalf("load 685 目录: %v", err)
	}
	if len(cat.CombineUnlock) != 3 {
		t.Fatalf("result 槽解锁门槛有 %d 个，期望 3: %v", len(cat.CombineUnlock), cat.CombineUnlock)
	}
	want := []uint32{0, 10, 15}
	for i := range want {
		if cat.CombineUnlock[i] != want[i] {
			t.Errorf("第 %d 行解锁门槛 = %d, want %d（全量: %v）", i+1, cat.CombineUnlock[i], want[i], cat.CombineUnlock)
		}
	}
	// 顺带把 24 对槽位表也钉住（2413/2414 全靠它把槽位号换回模块号）。
	if len(cat.Slots) != 24 {
		t.Fatalf("槽位表有 %d 对，期望 24", len(cat.Slots))
	}
	if got, ok := cat.ModuleAtSlot(15); !ok || got != 590015769 {
		t.Errorf("槽位 15 → %d(ok=%v), want 590015769（[shoulder]）", got, ok)
	}
	if len(cat.Recipes) != 143 {
		t.Errorf("配方 %d 条，期望 143（13 套装 × 11 部位）", len(cat.Recipes))
	}
	// 分解：`[disjoint stackable] [result item]` = Doom Oracle Box (1) × 30
	//（官方帮助原文："get 30 Doom Oracle Box (1) per Module"）。
	if cat.DisjointItem != 590015045 || cat.DisjointCount != 30 {
		t.Errorf("分解产出 = %d × %d，期望 590015045 × 30", cat.DisjointItem, cat.DisjointCount)
	}
}
