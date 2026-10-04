package protocol

import (
	"encoding/hex"
	"testing"
)

// [AZURE-DEATH-REVIVE] NOTI32 的体是 `u16 actor, u8 state, u8 flag, u16 hp, u16 mp`。
//
// 官服抓包（F16-s2c.txt seq563/seq564）里 hp = mp = 0x03e8 = 1000 —— 也就是
// 「满血满蓝」的那个值。私服此前只把 state 置 1、hp/mp 留 0，玩家回到城镇就是
// 0% 的虚弱态（实机 2026-10-04，客户端只能弹收费的 Stamina Recovery 服务）。
func TestPlayerReviveStateCarriesFullHPMP(t *testing.T) {
	p, err := PlayerReviveState(11)
	if err != nil {
		t.Fatal(err)
	}
	if len(p) != 8 {
		t.Fatalf("NOTI32 body = %d bytes, want 8", len(p))
	}
	want, _ := hex.DecodeString("0b000100e803e803")
	if hex.EncodeToString(p) != hex.EncodeToString(want) {
		t.Fatalf("revive state = %x, want %x (actor 11, state 1, flag 0, hp/mp 1000)", p, want)
	}
	if hp := int(p[4]) | int(p[5])<<8; hp != int(deathStateReviveHPMP) {
		t.Fatalf("revive hp = %d, want %d", hp, deathStateReviveHPMP)
	}
	if mp := int(p[6]) | int(p[7])<<8; mp != int(deathStateReviveHPMP) {
		t.Fatalf("revive mp = %d, want %d", mp, deathStateReviveHPMP)
	}
	// 死亡帧保持原样：state 0、hp/mp 0（进死亡 UI，不该带血蓝）。
	d, err := PlayerDeathState(11)
	if err != nil {
		t.Fatal(err)
	}
	if d[2] != 0 || d[4] != 0 || d[5] != 0 || d[6] != 0 || d[7] != 0 {
		t.Fatalf("death state = %x, want state 0 且 hp/mp 为 0", d)
	}
	// 非法 actor 一律拒绝。
	for _, bad := range []uint16{0, 65535} {
		if _, err := PlayerReviveState(bad); err == nil {
			t.Fatalf("actor %d 应当被拒绝", bad)
		}
	}
}
