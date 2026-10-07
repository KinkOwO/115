package main

import (
	"context"
	"encoding/binary"
	"net"
	"strings"
	"testing"
	"time"

	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/game/wire"
	"dfolan/internal/modpolicy"
)

func odysseyRulesSession(odyssey bool) *worldSession {
	return &worldSession{
		activeDungeon: &dungeon.Session{
			RunID:      "run",
			Loaded:     true,
			Definition: catalog.DungeonDefinition{Odyssey: odyssey},
			Room:       catalog.DungeonRoom{Map: 1},
		},
	}
}

// TestOdysseyConsumableGate：规则关 → 放行；规则开且是奥德赛副本 → 回 CMD44 拒绝；
// 规则开但不是奥德赛副本 → 不受影响；只开复活规则 → 消耗品仍可用。
func TestOdysseyConsumableGate(t *testing.T) {
	defer modpolicy.Reset()
	req := protocol.UseStackableRequest{Slot: 3, List: 0, Instance: 0, Template: 100}

	w := odysseyRulesSession(true)
	if got := w.odysseyConsumableGate(req); got != nil {
		t.Fatalf("规则未开时必须放行：%+v", got)
	}

	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanConsumables: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	got := w.odysseyConsumableGate(req)
	// 拦截 = 返回「非 nil 但空」的计划（已处理、不回任何包）。
	// **不能**回 UseStackableRefused：那个形状未被实机证实，2026-10-06 实机回它把客户端打崩了。
	if got == nil || len(got) != 0 {
		t.Fatalf("拦截时应回空计划（不回复任何包）：%+v", got)
	}

	w.activeDungeon.Definition.Odyssey = false
	if got := w.odysseyConsumableGate(req); got != nil {
		t.Fatalf("非奥德赛副本不该被拦：%+v", got)
	}

	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanReviveCoin: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if got := w.odysseyConsumableGate(req); got != nil {
		t.Fatalf("只开复活规则时消耗品不该被拦：%+v", got)
	}
}

// TestOdysseyReviveGate：门本身，以及它确实挂在 useCoinRevive 三级回退**之前**
// —— store 传 nil 也不该被碰到（碰了会 panic 或报存储错误，而不是规则拒绝）。
func TestOdysseyReviveGate(t *testing.T) {
	defer modpolicy.Reset()
	const wireID = 7
	w := odysseyRulesSession(true)
	w.pilotDeath = &odysseyDeath{Run: "run", Sequence: 1, Dead: true}
	p := make([]byte, 8)
	binary.LittleEndian.PutUint16(p, wireID)

	if err := w.odysseyReviveGate(); err != nil {
		t.Fatalf("规则未开时不该拦：%v", err)
	}
	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanReviveCoin: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := w.odysseyReviveGate(); err == nil {
		t.Fatal("奥德赛 + 规则开，门必须拦")
	}
	plan, err := w.useCoinRevive(context.Background(), nil, p, p, false)
	if err == nil || plan != nil {
		t.Fatalf("useCoinRevive 应在三级回退之前拒绝：plan=%v err=%v", plan, err)
	}
	if !strings.Contains(err.Error(), "policy: test") {
		t.Fatalf("拒绝原因应带规则来源，便于排障：%v", err)
	}

	w.activeDungeon.Definition.Odyssey = false
	if err := w.odysseyReviveGate(); err != nil {
		t.Fatalf("非奥德赛副本不该被拦：%v", err)
	}
}

// TestOdysseyRulesSurviveWithoutSession：会话字段缺失时门必须安静放行，
// 不能因为"没进本 / 没选角色"就 panic（这些请求本来就走不到业务里）。
func TestOdysseyRulesSurviveWithoutSession(t *testing.T) {
	defer modpolicy.Reset()
	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{
		BanConsumables: true, BanReviveCoin: true, Source: "test",
	}); err != nil {
		t.Fatal(err)
	}
	var w *worldSession
	if w.odysseyModeActive() {
		t.Fatal("nil 会话不该被判成奥德赛模式")
	}
	if err := w.odysseyReviveGate(); err != nil {
		t.Fatalf("nil 会话应放行：%v", err)
	}
	if got := w.odysseyConsumableGate(protocol.UseStackableRequest{}); got != nil {
		t.Fatalf("nil 会话应放行：%+v", got)
	}
	if w.odysseyImmediateDeathFail() {
		t.Fatal("nil 会话不该走立即判负")
	}
	empty := &worldSession{}
	if empty.odysseyModeActive() {
		t.Fatal("没有 activeDungeon 不该判成奥德赛模式")
	}
	if got := empty.odysseyConsumableGate(protocol.UseStackableRequest{}); got != nil {
		t.Fatalf("无副本应放行：%+v", got)
	}
	if empty.odysseyImmediateDeathFail() {
		t.Fatal("没有副本不该走立即判负")
	}
}

// TestOdysseyImmediateDeathFail 钉住"死亡即回城"的判据：
// 只有「奥德赛副本 + 禁复活规则开 + 本局已确认死亡」三条同时成立才走立即判负；
// 关了规则、非奥德赛副本、还活着、死亡记录属于别的 run —— 都得退回 10 秒倒计时那条路。
func TestOdysseyImmediateDeathFail(t *testing.T) {
	defer modpolicy.Reset()
	newSession := func(odyssey bool) *worldSession {
		w := odysseyRulesSession(odyssey)
		w.pilotDeath = &odysseyDeath{Run: "run", Sequence: 1, Dead: true}
		return w
	}
	w := newSession(true)
	if w.odysseyImmediateDeathFail() {
		t.Fatal("规则没开时不该立即判负（要走 10 秒倒计时）")
	}
	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanConsumables: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if w.odysseyImmediateDeathFail() {
		t.Fatal("只开消耗品规则时不该立即判负")
	}
	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanReviveCoin: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}
	if !w.odysseyImmediateDeathFail() {
		t.Fatal("奥德赛 + 禁复活 + 已死亡，必须走立即判负")
	}
	w.pilotDeath.Dead = false
	if w.odysseyImmediateDeathFail() {
		t.Fatal("还活着不该判负")
	}
	w.pilotDeath.Dead = true
	w.pilotDeath.Run = "other-run"
	if w.odysseyImmediateDeathFail() {
		t.Fatal("别的 run 的死亡记录不该判本局负")
	}
	w.pilotDeath.Run = "run"
	w.activeDungeon.Definition.Odyssey = false
	if w.odysseyImmediateDeathFail() {
		t.Fatal("非奥德赛副本不该立即判负")
	}
}

// TestOdysseyDeathFailLeaveSendsFailAndReturnsToTown：立即判负那一步真的发
// NOTI33 FAIL_CLEAR_DUNGEON 并走回城链；跑完清掉副本会话，重复调用是空操作
// （所以"立即 + 10 秒定时器"不会重复发包）。
func TestOdysseyDeathFailLeaveSendsFailAndReturnsToTown(t *testing.T) {
	defer modpolicy.Reset()
	if _, err := modpolicy.Configure(modpolicy.OdysseyRules{BanReviveCoin: true, Source: "test"}); err != nil {
		t.Fatal(err)
	}

	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	peer.SetDeadline(time.Now().Add(5 * time.Second))
	go func() {
		buf := make([]byte, 4096)
		for {
			if _, err := peer.Read(buf); err != nil {
				return
			}
		}
	}()

	events := make(chan map[string]any, 16)
	logf := func(e map[string]any) { events <- e }
	keys := make([]byte, wire.SessionKeyBytes)

	w := odysseyRulesSession(true)
	w.role = database.Character{ID: 1, WireID: 3}
	w.pilotDeath = &odysseyDeath{Run: "run", Sequence: 1, Dead: true}
	c := &gameConnection{
		gatewayRuntime: &gatewayRuntime{},
		worldState:     w,
		bootstrapped:   true,
		event:          logf,
		output:         newConnectionOutput(server, keys, "test", logf),
	}

	c.deathFailLeave(0)

	deadline := time.After(5 * time.Second)
	var sawLeave bool
	for !sawLeave {
		select {
		case e := <-events:
			if e["kind"] == "death_fail_leave" {
				if e["reason"] != byte(0) {
					t.Fatalf("立即判负的 reason 应为 0（默认死亡）：%+v", e)
				}
				if steps, _ := e["steps"].(int); steps < 2 {
					t.Fatalf("回城链应至少含 FAIL_CLEAR 与 leave ack：%+v", e)
				}
				sawLeave = true
			}
			if e["kind"] == "death_fail_leave_error" {
				t.Fatalf("回城链失败：%+v", e)
			}
		case <-deadline:
			t.Fatal("没有等到 death_fail_leave 事件（立即回城没发生）")
		}
	}
	if w.activeDungeon != nil || w.bleedingMineStart != nil {
		t.Fatalf("判负回城后副本会话应当已清空：%+v", w.activeDungeon)
	}

	// 重复调用（例如 10 秒定时器后来才触发）必须是空操作，不能再发一次包。
	c.deathFailLeave(deathFailTimeoutReason)
	select {
	case e := <-events:
		t.Fatalf("重复调用不该再产生事件：%+v", e)
	case <-time.After(200 * time.Millisecond):
	}
}

