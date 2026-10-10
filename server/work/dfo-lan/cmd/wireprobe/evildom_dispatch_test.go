package main

import (
	"encoding/hex"
	"reflect"
	"testing"
)

// TestIsEvildomPartyCreateUsesPartyTypeByte 用**业主 2026-10-10 实机会话的真实请求字节**
// 钉住建队判据。
//
// 背景：那次实测里玩家进的次元回廊频道是 **Type 84（Hall of Dimensions）**，
// 而实现最初按 Type 50（Evildom）单点判据 ⇒ CMD12 到了服务端却没有任何应答，
// 客户端表现为「输入队名点确认没反应」。所以判据必须只看**请求自带的队伍类型
// 字节 0x0d**，与频道类型无关。
//
// 两帧都是那次会话 events.jsonl 的 client_frame.plain_hex 原文（队名分别是
// 奇数长度的 "55555" 与偶数长度的 "2221"，用于覆盖偶数/奇数两种长度）。
func TestIsEvildomPartyCreateUsesPartyTypeByte(t *testing.T) {
	real := []string{
		// 16:10:17 队名 "55555"（名长=5，UTF-16LE 5 字节 + 1 补零）
		"00000500000035353535350400000000000000000d01000101020407070707ffffffff00000000000000000000000000",
		// 16:10:27 队名 "2221"（名长=4，UTF-16LE 恰好 4 字节）
		"000004000000323232310400000000000000000d01000101020407070707ffffffff0000000000000000000000000000",
	}
	for i, h := range real {
		p, err := hex.DecodeString(h)
		if err != nil {
			t.Fatal(err)
		}
		if !isEvildomPartyCreate(p) {
			t.Fatalf("第 %d 帧真实请求没有被识别为次元回廊建队（len=%d）", i, len(p))
		}
	}

	// 别的内容的队伍类型必须放行：末世录 0x26 / 伊斯 0x0b / 维纳斯 0x22。
	// 队伍类型字节在正文 @19（名长 @2 + 队名 @6 + 容量 @10 + 5 零）。
	base, _ := hex.DecodeString(real[1])
	for _, other := range []byte{0x26, 0x0b, 0x22, 0x00} {
		p := append([]byte(nil), base...)
		p[19] = other
		if isEvildomPartyCreate(p) {
			t.Fatalf("队伍类型 %#x 被误判为次元回廊", other)
		}
	}
	// 畸形载荷一律放行，不猜。
	if isEvildomPartyCreate(nil) || isEvildomPartyCreate(base[:10]) {
		t.Fatal("畸形载荷被当成次元回廊建队")
	}
}

// TestEvildomDispatchRunsBeforeLegion 钉住派发顺序：次元回廊那一层必须排在军团
// 家族派发层之前。
//
// 为什么这条顺序本身是契约：军团家族共用 CMD2043/2045/2046 信封，而
// legion.Requests 不含 CMD12；一旦 dispatchEvildom 排到 dispatchLegion 之后，
// CMD12 就会掉进家族派发层并被当成「不认识的命令」放行到通用队伍链 ——
// 队伍类型 0x0d 无人认领，客户端表现正是「输入队名点确认没反应」。
func TestEvildomDispatchRunsBeforeLegion(t *testing.T) {
	evildom := reflect.ValueOf((*gameConnection).dispatchEvildom).Pointer()
	legion := reflect.ValueOf((*gameConnection).dispatchLegion).Pointer()
	evildomAt, legionAt := -1, -1
	for i, stage := range beforeClientTypeDispatch {
		at := reflect.ValueOf(stage).Pointer()
		if at == evildom {
			evildomAt = i
		}
		if at == legion {
			legionAt = i
		}
	}
	if evildomAt < 0 {
		t.Fatal("dispatchEvildom 不在 beforeClientTypeDispatch 里（建队请求没人接管）")
	}
	if legionAt < 0 {
		t.Fatal("dispatchLegion 不在 beforeClientTypeDispatch 里")
	}
	if evildomAt > legionAt {
		t.Fatalf("dispatchEvildom 排在第 %d 位、dispatchLegion 在第 %d 位：顺序反了", evildomAt, legionAt)
	}
}

