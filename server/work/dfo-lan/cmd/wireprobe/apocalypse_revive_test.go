package main

import (
	"context"
	"strings"
	"testing"

	"dfolan/internal/game/protocol"
	"dfolan/internal/legion"
)

// ★ 回归护栏（2026-10-08 实机）：末世录的复活币由**所选作战的 `[allow coin]`** 决定，
// 且按**关卡**计数。
//
// 真源：PVF `apocalypse.ctp` 的 `[operation data set]` → `[allow coin]`（导入见
// internal/catalog/apocalypse_import.go 的 colAllowCoin，二进 RunPlan.AllowCoin）。
// 实测只有作战①（难度1）带该列、值 `-1, 8`；作战②/③/④ 都没有该列。
// 业主 2026-10-08 定调语义：**最后一个数 = 每关上限；无该列 = 禁止复活**。
//
// 此前该字段**只被解析进 plan.AllowCoin 并写进日志事件，从未执行** ——
// 业主实测「难度1 能用掉全部 99 个币、难度2 也照样能用」就是这个成因。
func apocalypseCoinSession(t *testing.T, allow []int64, allowConfigured bool) *worldSession {
	t.Helper()
	w := apocalypseStageSession(t, 1, true) // 已进入第 1 关（副本表下标 1）
	w.apocalypse.Choice = 0
	w.legion.plan = &legion.RunPlan{AllowCoinConfigured: allowConfigured, AllowCoin: allow}
	return w
}

// 难度1：`[allow coin] = -1, 8` ⇒ 每关 8 次，第 9 次拒。
func TestApocalypseCoinBudgetCapsAtEightPerStage(t *testing.T) {
	w := apocalypseCoinSession(t, []int64{-1, 8}, true)

	for i := 1; i <= 8; i++ {
		applicable, err := w.apocalypseCheckCoinBudget()
		if !applicable {
			t.Fatalf("反弹 %d：本次作战应适用 [allow coin]", i)
		}
		if err != nil {
			t.Fatalf("第 %d 次复活就被拒了（上限应是 8）：%v", i, err)
		}
		w.apocalypseSpendCoinBudget()
	}
	if w.apocalypse.RevivesUsedThisStage != 8 {
		t.Fatalf("已用次数 = %d, want 8", w.apocalypse.RevivesUsedThisStage)
	}
	applicable, err := w.apocalypseCheckCoinBudget()
	if !applicable {
		t.Fatal("第 9 次：应仍适用（只是额度用尽）")
	}
	if err == nil {
		t.Fatal("第 9 次复活没有被拒 —— 上限 8 没有生效（实机症状：能用掉全部 99 个币）")
	}
	if !strings.Contains(err.Error(), "用尽") {
		t.Fatalf("第 9 次应报「用尽」，得到：%v", err)
	}
}

// 难度2/3：**没有** `[allow coin]` 列 ⇒ 一次都不许用。
func TestApocalypseCoinBudgetDeniesWithoutAllowCoin(t *testing.T) {
	w := apocalypseCoinSession(t, nil, false)

	applicable, err := w.apocalypseCheckCoinBudget()
	if !applicable {
		t.Fatal("本次作战应适用（在末世录关卡里）")
	}
	if err == nil {
		t.Fatal("没有 [allow coin] 的作战竟然允许复活 —— 业主口径：无该列 = 禁止")
	}
	if !strings.Contains(err.Error(), "禁止") {
		t.Fatalf("应报「禁止使用复活币」，得到：%v", err)
	}
	// 零值也要拒（-1 之类的负数按 0 处理）。
	zero := apocalypseCoinSession(t, []int64{-1, 0}, true)
	if _, err := zero.apocalypseCheckCoinBudget(); err == nil {
		t.Fatal("上限 0 的作战应拒绝复活")
	}
}

// 不在末世录关卡里时闸门「不适用」：不能影响其它内容的复活。
func TestApocalypseCoinBudgetNotApplicableOutsideStage(t *testing.T) {
	w := apocalypseCoinSession(t, []int64{-1, 8}, true)
	w.activeDungeon = nil
	if applicable, err := w.apocalypseCheckCoinBudget(); applicable || err != nil {
		t.Fatalf("非末世录关卡应不适用：applicable=%v err=%v", applicable, err)
	}
}

// 进关（攻坚房 / 每一关）必须把「本关已用次数」归零 —— 业主口径是**每关** 8 个，
// 不是每局 8 个。三条进关路径都经过 apocalypseStageEntry，这里是唯一归零点。
func TestApocalypseStageEntryResetsCoinBudget(t *testing.T) {
	w := apocalypseStageSession(t, 0, true)
	w.level = 200
	w.dungeons = apocalypseTestDungeons(t)
	w.apocalypse.RevivesUsedThisStage = 7

	if _, _, err := w.apocalypseStageEntry(protocol.DungeonSelection{
		ID: legion.ApocalypseStageDungeons[1], Difficulty: 0, Party: 65535}); err != nil {
		t.Fatalf("stage entry: %v", err)
	}
	if w.apocalypse.RevivesUsedThisStage != 0 {
		t.Fatalf("进关后本关已用次数 = %d, want 0（否则第 2 关会带着第 1 关的消耗）",
			w.apocalypse.RevivesUsedThisStage)
	}
}

// apocalypseReviveSession 借用现有的 CMD41 夹具（ceraReviveFixture）再把它搬进
// 末世录第 1 关，这样走的是**真实的** useCoinRevive 路径（含存档消费与幂等键）。
func apocalypseReviveSession(t *testing.T, allow []int64, allowConfigured bool) (*worldSession, *ceraReviveFake, []byte) {
	t.Helper()
	w, store, p := ceraReviveFixture(t, 0)
	w.activeDungeon.Definition.ID = legion.ApocalypseStageDungeons[1]
	w.apocalypse = legion.NewApocalypseRunState()
	w.apocalypse.Choice = 0
	w.apocalypse.Entered = true
	w.legion = &legionSession{plan: &legion.RunPlan{AllowCoinConfigured: allowConfigured, AllowCoin: allow}}
	return w, store, p
}

// 端到端：CMD41 走 useCoinRevive 时闸门必须生效。
// 难度2（无 [allow coin]）**第一次就该被拒**，而且不能碰存档（拒绝要发生在消费之前）。
func TestApocalypseUseCoinReviveRefusedWithoutAllowCoin(t *testing.T) {
	w, store, p := apocalypseReviveSession(t, nil, false)
	before := store.balance

	if plan, err := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false); err == nil || plan != nil {
		t.Fatalf("难度2 竟然允许用币复活：plan=%+v err=%v", plan, err)
	}
	if len(store.grants) != 0 || store.balance != before {
		t.Fatalf("被拒的复活不该消耗存档：grants=%+v balance=%d→%d", store.grants, before, store.balance)
	}
	if !w.pilotDeath.Dead {
		t.Fatal("被拒的复活不该把角色标成已复活")
	}
}

// 端到端：难度1 的第一次复活应当成功，并把本关已用次数记上。
func TestApocalypseUseCoinReviveChargesAllowCoin(t *testing.T) {
	w, store, p := apocalypseReviveSession(t, []int64{-1, 8}, true)

	plan, err := w.useCoinRevive(context.Background(), store, p, []byte{41, 3}, false)
	if err != nil {
		t.Fatalf("难度1 第一次复活应被允许：%v", err)
	}
	if len(plan) == 0 {
		t.Fatal("复活成功却没有发包")
	}
	if w.apocalypse.RevivesUsedThisStage != 1 {
		t.Fatalf("本关已用次数 = %d, want 1", w.apocalypse.RevivesUsedThisStage)
	}
}
