package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// 第四十二轮：维纳斯死亡被请离副本后，再次进入同关必须重置关卡倒计时——
// deathLeave 清掉该阶段的冻结开始时刻（重进后 venusStageTimer 从满额时限
// 重新冻结）；进度/难度锁/遗物保留不变；未进过图的 run 不触发。
func TestVenusDeathLeaveResetsStageClock(t *testing.T) {
	server, peer := net.Pipe()
	defer server.Close()
	defer peer.Close()
	events := make(chan map[string]any, 16)
	c := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys: make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 99,
			role:  database.Character{ID: 7, WireID: 7, Name: "001", State: []byte(`{"level":115,"advancement":5,"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100}}`)},
			state: database.WorldState{Position: database.WorldPosition{Town: 204, Area: 0, X: 700, Y: 300}},
			venus: &venusRun{choice: 1, stage: 1, relicMask: 1 << 3, entered: true,
				cleared:    [4]bool{true, false, false, false},
				stageClock: [4]time.Time{{}, time.Unix(1791000000, 0), {}, {}}},
			activeDungeon: &dungeon.Session{
				Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[1]},
				Loaded:     true,
			},
			pilotDeath: &odysseyDeath{Dead: true, Run: "venus-run-1"}},
		event: func(e map[string]any) { events <- e }}
	c.output = newConnectionOutput(server, c.keys, "test", c.event)
	go func() { c.deathFailLeave(100) }()
	peer.SetReadDeadline(time.Now().Add(4 * time.Second))
	// N33 FAIL_CLEAR 打头，随后是回城链——全部读走直到读超时。
	h := make([]byte, 16)
	for {
		if _, e := io.ReadFull(peer, h); e != nil {
			break
		}
		size := int(binary.LittleEndian.Uint32(h[3:7]))
		if size < 16 || size > wire.MaxPacketSize {
			t.Fatal(size)
		}
		b := make([]byte, size-16)
		if _, e := io.ReadFull(peer, b); e != nil {
			t.Fatal(e)
		}
	}
	run := c.worldState.venus
	if run == nil {
		t.Fatal("death eject must keep the run (progress preserved)")
	}
	if !run.stageClock[1].IsZero() {
		t.Fatalf("re-entry timer must reset: stageClock[1] = %v", run.stageClock[1])
	}
	if run.choice != 1 || !run.entered || run.clearedCount() != 1 || run.relicMask != 1<<3 {
		t.Fatalf("death eject must keep progress and lock: %+v", run)
	}
	select {
	case e := <-events:
		if e["kind"] != "venus_death_stage_timer_reset" {
			t.Fatal("unexpected event", e)
		}
	default:
		t.Fatal("timer reset not logged")
	}
	// 未进过图的 run（entered=false）不触发重置。
	c2 := &gameConnection{gatewayRuntime: &gatewayRuntime{}, bootstrapped: true, selectedCharacterID: 7,
		keys: make([]byte, wire.SessionKeyBytes),
		worldState: &worldSession{channelType: 99,
			venus: &venusRun{choice: 0xff},
			activeDungeon: &dungeon.Session{
				Definition: catalog.DungeonDefinition{ID: legion.VenusStageDungeons[0]},
				Loaded:     true,
			},
			pilotDeath: &odysseyDeath{Dead: true, Run: "r2"}},
		event: func(e map[string]any) { events <- e }}
	c2.output = newConnectionOutput(server, c2.keys, "test", c2.event)
	c2.deathFailLeave(100)
	select {
	case e := <-events:
		if e["kind"] == "venus_death_stage_timer_reset" {
			t.Fatal("unentered run must not log the reset")
		}
	default:
	}
}
