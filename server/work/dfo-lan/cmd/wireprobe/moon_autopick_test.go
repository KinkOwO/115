package main

import (
	"testing"
	"time"

	"dfolan/internal/dungeon"
)

// TestMoonAutoPickGate 守「沉月湖翻牌倒计时自动选第一张」的**门禁条件**。
//
// 实机 BUG（2026-10-08）：倒计时结束后服务端什么都不发，玩家一张牌都翻不到。
// 根因是沉月湖的翻牌走 dispatchSpecialContent -> moonHandle，绕过了
// dispatchDungeon 里那段「card_layout_ack 时置 cardAutoPickAt」的后处理。
//
// 这里覆盖判据本身（何时**不该**触发、何时该清理计时器）；真正到点选牌要走
// moonClaim，需要真实的 store/loot，不在本用例范围。
func TestMoonAutoPickGate(t *testing.T) {
	now := time.Unix(1800000000, 0)
	owner := &dungeon.MoonSoloOwner{}

	// ① 没到点：什么都不做，且**不清**计时器（等下一次 tick）。
	w := &worldSession{moon: moonSoloState{owner: owner, autoPickAt: now.Add(time.Second)}}
	if p, err := w.autoPickMoonCard(now); len(p) != 0 || err != nil {
		t.Fatalf("未到点不该发牌：%v %v", p, err)
	}
	if w.moon.autoPickAt.IsZero() {
		t.Fatal("未到点不该清掉计时器")
	}

	// ② 到点但不在副本里（owner == nil）：不发，也不 panic。
	w = &worldSession{moon: moonSoloState{autoPickAt: now.Add(-time.Second)}}
	if p, err := w.autoPickMoonCard(now); len(p) != 0 || err != nil {
		t.Fatalf("没有本局时不该发牌：%v %v", p, err)
	}

	// ③ 到点但奖单还没冻结（plan == nil）：不发，并清掉计时器。
	w = &worldSession{resultSent: true, moon: moonSoloState{owner: owner, autoPickAt: now.Add(-time.Second)}}
	w.cardLayoutSent = true
	if p, err := w.autoPickMoonCard(now); len(p) != 0 || err != nil {
		t.Fatalf("奖单未冻结时不该发牌：%v %v", p, err)
	}
	if !w.moon.autoPickAt.IsZero() {
		t.Fatal("前置不满足时应清掉计时器，免得每个 tick 空转")
	}

	// ④ 已经领过（claimed）：不发。—— 玩家自己选了之后不能再替他选一张。
	w = &worldSession{resultSent: true, moon: moonSoloState{owner: owner, autoPickAt: now.Add(-time.Second), claimed: true}}
	w.cardLayoutSent = true
	if p, err := w.autoPickMoonCard(now); len(p) != 0 || err != nil {
		t.Fatalf("已领过不该再发牌：%v %v", p, err)
	}

	// ⑤ 布局还没发出去：不发 —— 否则会在玩家还没看到牌的时候就定死第一张。
	w = &worldSession{resultSent: true, moon: moonSoloState{owner: owner, autoPickAt: now.Add(-time.Second)}}
	if p, err := w.autoPickMoonCard(now); len(p) != 0 || err != nil {
		t.Fatalf("布局未发出时不该发牌：%v %v", p, err)
	}
}
