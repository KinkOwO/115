package loot

import (
	"dfolan/internal/boostup"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"encoding/json"
	"testing"
)

// TestBoostChallengeSnapshotAllocatesRowMap 钉住实机 2026-10-06 15:34:49 的掉线
// （会话 ..._20261006_153257_795354_next37）：第 11 关领奖帧 `960200000b000000` 之后
// 服务端 `connection_panic_recovered: assignment to entry in nil map` ⇒ 连接被拆，
// 客户端只看到「You have been disconnected」。
//
// 根因在这条投影里：`var out protocol.BoostChallengeState115` 的 Rows 是 nil map，
// 角色一旦真的登记了挑战（毕业时 UnlockChallenges 写入 Rows），第一行赋值就 panic。
// 旧端把 665 用开关关着，这条分支从未在「有行」的角色上跑过；665 按源常开以后
// 第一次毕业就踩中。断言故意只用状态与目录，不碰存储事务。
func TestBoostChallengeSnapshotAllocatesRowMap(t *testing.T) {
	c := &boostup.Catalog{GoalLevel: 115, Challenges: []boostup.ChallengeDefinition{
		{Index: 0, UnlockKind: "level", UnlockValue: 115, Kind: "clear endkeeper of order", Goal: 10, Repeat: 1, UnlockMail: true, ClearMail: true},
		{Index: 1, UnlockKind: "fame", UnlockValue: 55950, Kind: "clear higher or legion", Conditions: []uint32{26, 31, 35, 36}, Goal: 1, Repeat: 1, UnlockMail: true, ClearMail: true},
	}}
	st := boostup.State{
		Version: 1, Gifts: map[uint16]bool{}, Activated: true,
		Training: boostup.Training{Step: 12, Finished: true, Claimed: map[byte]bool{11: true}},
		Challenge: &boostup.ChallengeState{Version: 1, Enrolled: true, Rows: map[byte]boostup.ChallengeProgress{
			0: {Unlocked: true},
			1: {Unlocked: true, Progress: 1},
		}},
	}
	raw, e := boostup.WriteState(json.RawMessage(`{}`), st)
	if e != nil {
		t.Fatalf("角色状态写入失败: %v", e)
	}
	body, e := BoostChallengeSnapshot(c, Role{ID: 1, State: raw})
	if e != nil {
		t.Fatalf("毕业后的挑战快照不应失败: %v", e)
	}
	// 帧形：定长 323 B 记录块 = u16 等级标记 + u8 登记 + 32×{u8,u8,u32,u32}（客户端
	// sub_140BAF000 的 `manager+331+10*i`，与源里 [challenge info] 最多 32 行一致）。
	if len(body) != protocol.BoostChallengeBodyLen {
		t.Fatalf("包体 %d B, want 客户端读取的 %d B", len(body), protocol.BoostChallengeBodyLen)
	}
	if body[2] != 1 {
		t.Fatalf("登记标记 %d, want 1（sub_140BAF450 用它当 665 面板门禁）", body[2])
	}
	// 第 0 槽：已解锁、未领解锁奖励、进度 0。
	off := 3 + 10*0
	if body[off] != 1 || body[off+1] != 0 || binary.LittleEndian.Uint32(body[off+2:]) != 0 {
		t.Fatalf("第 0 槽 %x, want 解锁=1 已领=0 进度=0", body[off:off+10])
	}
	// 第 1 槽：已解锁且进度 1（原始计数，goal 由客户端自己按槽位取）。
	off = 3 + 10*1
	if body[off] != 1 || binary.LittleEndian.Uint32(body[off+2:]) != 1 {
		t.Fatalf("第 1 槽 %x, want 解锁=1 进度=1", body[off:off+10])
	}
	// 未使用的槽位必须保持全零：官服 16 帧里除前 6 字节外全零。
	for i, b := range body[3+2*10:] {
		if b != 0 {
			t.Fatalf("空槽位 %d 出现非零字节 %02x", 3+2*10+i, b)
		}
	}
}

// 未登记的角色（毕业前）仍要能出帧：登记位 0、全表零，但宽度照旧 323 B。
// 短于 323 不是「显示少一点」而是客户端崩溃：sub_146EA0BE0 在剩余长度不足时
// 执行 `MEMORY[0]=0`（写空地址）。
func TestBoostChallengeSnapshotBeforeEnrollment(t *testing.T) {
	st := boostup.State{Version: 1, Gifts: map[uint16]bool{}, Activated: true,
		Training: boostup.Training{Step: 3, Claimed: map[byte]bool{}}}
	raw, e := boostup.WriteState(json.RawMessage(`{}`), st)
	if e != nil {
		t.Fatal(e)
	}
	body, e := BoostChallengeSnapshot(&boostup.Catalog{Challenges: []boostup.ChallengeDefinition{{Index: 0, Goal: 1, Repeat: 1}}}, Role{ID: 1, State: raw})
	if e != nil {
		t.Fatal(e)
	}
	if len(body) != protocol.BoostChallengeBodyLen || body[2] != 0 {
		t.Fatalf("毕业前帧应为 %d B 且登记位 0, got %d B %x", protocol.BoostChallengeBodyLen, len(body), body[:8])
	}
	for i, b := range body {
		if b != 0 {
			t.Fatalf("毕业前帧第 %d 字节非零: %02x", i, b)
		}
	}
}
