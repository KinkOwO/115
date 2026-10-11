package main

import (
	"encoding/hex"
	"testing"

	"dfolan/internal/game/wire"
	"dfolan/internal/legion"
)

// TestEvildomDispatchReleasesOtherLegionEnter 钉住业主实机报的现象：
// 「末世录军团副本进入了次元回廊」。
//
// 军团家族（末世录/伊斯/维纳斯/苏醒之森/次元回廊）**共用 CMD2045（ENTER_DUNGEON）
// 这一个信封**，而 dispatchEvildom 排在 dispatchLegion 之前。本派发器对 CMD2043 有
// 「内容号必须是次元回廊 0x0d」的门，CMD2045 分支此前**没有** `isDimCloister()` 门，
// 于是末世录的进图确认帧被次元回廊接走、加载成"第 1 界"。
//
// 本测试用同一条信封 + 一份客户端 CMD2045 形态的载荷调用派发器：
// 次元回廊身份不成立时必须**放行**（dispatchNext），不得接管。
func TestEvildomDispatchReleasesOtherLegionEnter(t *testing.T) {
	// 客户端 CMD2045 形态：信封 + 队伍类型字节 + 关卡号（身份判据只看身份，不看这串）。
	payload, err := hex.DecodeString("6600010000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	frame := &clientRequest{
		frame:     wire.Frame{Type: 1, ID: legion.CmdEnterDungeon},
		plaintext: payload,
		verified:  true,
	}

	// ① 非次元回廊频道（119 = 末世录）⇒ 必须放行给 dispatchLegion。
	other := &gameConnection{
		bootstrapped: true,
		worldState:   &worldSession{channelType: 119},
		event:        func(map[string]any) {},
	}
	other.legionState.channelType = 119
	if got := other.dispatchEvildom(frame); got != dispatchNext {
		t.Fatalf("频道 119 的 CMD2045 被次元回廊接走（返回 %v），应放行", got)
	}

	// ② 次元回廊频道（84 = Hall of Dimensions）⇒ 应当接管（只要求"不是放行"，
	//    具体帧内容由 dimension_cloister_*_test.go 负责）。
	cloister := &gameConnection{
		bootstrapped: true,
		worldState:   &worldSession{channelType: 84},
		event:        func(map[string]any) {},
	}
	cloister.legionState.channelType = 84
	if got := cloister.dispatchEvildom(frame); got == dispatchNext {
		t.Fatal("频道 84 的 CMD2045 被放行了：次元回廊自己的进图确认帧没人接管")
	}
}
