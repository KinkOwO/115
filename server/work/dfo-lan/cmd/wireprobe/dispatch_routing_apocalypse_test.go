package main

import (
	"dfolan/internal/database"
	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
	"testing"
)

// 实机回归（2026-10-08 13:57 会话）：内容 107（末世录）的 CMD2043 曾经被
// 苏醒之森的派发层吃掉 —— 它落在 forestContent 的 default 分支上，记一条
// forest_start_unmatched 并回共享 ACK，于是服务端从未进入 channel
// （events 里没有 legion_entered_channel），后续 CMD2354 因「还没有 CMD2043」
// 被拒，整个开团流程卡死在「选完开始作战没反应」。
//
// 这条测试走**完整的 dispatch 链**（beforeClientTypeDispatch 的真实顺序），
// 断言内容 107 的 CMD2043 最终由末世录处理器应答，且苏醒之森不再代答。
func TestApocalypseStartIsNotClaimedByForest(t *testing.T) {
	client, conn, events := newDispatchTestClient()
	client.worldState = &worldSession{
		channelType: apocalypseChannelType,
		role:        database.Character{ID: 2, WireID: 2, Name: "ApocalypseCap"},
	}
	client.selectedCharacterID = 2

	// 客户端 CMD2043 的实机正文：13 字节信封 + u32 内容号 107。
	body := make([]byte, legion.EnvelopeSize+4)
	for i := 0; i < legion.EnvelopeSize; i++ {
		body[i] = 0xff
	}
	body[legion.EnvelopeSize+0] = byte(legion.ApocalypseContent)
	body[legion.EnvelopeSize+1] = byte(legion.ApocalypseContent >> 8)

	result := client.dispatch(&clientRequest{
		frame:     wire.Frame{Type: 1, ID: legion.CmdStart},
		plaintext: body,
		verified:  true,
	})
	if result != dispatchHandled {
		t.Fatalf("dispatch result = %v, want handled", result)
	}
	// 苏醒之森不能**代答**这次开战：观测事件 forest_start_foreign_content 是
	// 允许的（它记录「这不是我的内容号」并放行），但绝不能再出现上面那个
	// 回共享 ACK 的 forest_start_unmatched / _ack。
	foreign := false
	for _, e := range *events {
		switch e["kind"] {
		case "forest_start_unmatched", "forest_start_unmatched_ack":
			t.Fatalf("the forest layer claimed an apocalypse CMD2043: %v", e)
		case "forest_start_foreign_content":
			foreign = true
		}
	}
	if !foreign {
		t.Fatalf("expected the forest layer to log and defer the foreign content: %v", *events)
	}
	// 末世录必须进入 channel 并应答（ACK + NOTI2895 两帧）。
	if !hasEvent(*events, "legion_entered_channel") {
		t.Fatalf("apocalypse channel entry missing: %v", *events)
	}
	if conn.Len() == 0 {
		t.Fatal("CMD2043 produced no response at all (the client would hang)")
	}
	run := client.legionState.apocalypseRun()
	if run.Choice != 0xff {
		t.Fatalf("fresh run choice = %#x, want 0xff", run.Choice)
	}
	if run.ID == "" {
		t.Fatal("run id was not generated on channel entry")
	}
}

// 内容 104/105（苏醒之森）仍然必须由苏醒之森处理 —— 修 107 不能把森林弄坏。
// 这里只断言「不落到末世录」，森林自身的完整链路有它自己的测试。
func TestForestStartStillNotClaimedByApocalypse(t *testing.T) {
	client, _, events := newDispatchTestClient()
	client.worldState = &worldSession{channelType: 96}
	client.selectedCharacterID = 2

	body := make([]byte, legion.EnvelopeSize+4)
	body[legion.EnvelopeSize+0] = byte(legion.ForestContentID)

	result := client.dispatch(&clientRequest{
		frame:     wire.Frame{Type: 1, ID: legion.CmdStart},
		plaintext: body,
		verified:  true,
	})
	if result != dispatchHandled {
		t.Fatalf("dispatch result = %v, want handled by the forest layer", result)
	}
	for _, e := range *events {
		if e["kind"] == "legion_entered_channel" {
			t.Fatalf("the apocalypse layer answered a forest CMD2043: %v", e)
		}
	}
}
