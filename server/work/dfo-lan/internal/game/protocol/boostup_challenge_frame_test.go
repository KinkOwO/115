package protocol

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// TestBoostChallengeStatusMatchesOfficialCapture 把 2722 的编码钉在两类证据上：
// 客户端读取宽度（sub_140BAE420 → sub_146EA0BE0(&buf,323)）与官服抓包
// （cap43/full/…/frames.jsonl 的 16 帧，除前 6 字节外全零）观察到的字段用法。
// 短于 323 B 会在 reader 的 `MEMORY[0]=0`（写空地址）上直接崩客户端——
// 实机 2026-10-06 15:50:39 我方 24 B 自造布局后 1.7 s 落下 CrashDNF2.cra。
func TestBoostChallengeStatusMatchesOfficialCapture(t *testing.T) {
	// 官服第 2 帧：登记，其余全零。
	body, e := BoostChallengeStatus115(BoostChallengeState115{Enrolled: true, Rows: map[byte]BoostChallengeRow115{}})
	if e != nil {
		t.Fatal(e)
	}
	if len(body) != BoostChallengeBodyLen || body[2] != 1 || !bytes.Equal(body[3:], make([]byte, BoostChallengeBodyLen-3)) {
		t.Fatalf("登记帧 %x", body[:16])
	}
	// 官服第 3~12 帧：@0=115（源 GoalLevel）、@2=1、第 0 槽 +0=1 然后 +1=1，
	// 槽内 u32 计数递增，其余槽位保持零。
	body, e = BoostChallengeStatus115(BoostChallengeState115{
		Enrolled: true, LevelMarker: 115,
		Rows: map[byte]BoostChallengeRow115{0: {Unlocked: true, UnlockClaimed: true, Progress: 3}},
	})
	if e != nil {
		t.Fatal(e)
	}
	if binary.LittleEndian.Uint16(body) != 115 || body[2] != 1 {
		t.Fatalf("头部 %x", body[:3])
	}
	if body[3] != 1 || body[4] != 1 || binary.LittleEndian.Uint32(body[5:]) != 3 || binary.LittleEndian.Uint32(body[9:]) != 0 {
		t.Fatalf("第 0 槽 %x", body[3:13])
	}
	if !bytes.Equal(body[13:], make([]byte, BoostChallengeBodyLen-13)) {
		t.Fatal("未使用的槽位必须保持零")
	}
	// 槽位越界必须拒绝，而不是写出越界字节。
	if _, e = BoostChallengeStatus115(BoostChallengeState115{Rows: map[byte]BoostChallengeRow115{BoostChallengeRecordCount: {Unlocked: true}}}); e == nil {
		t.Fatal("challenge index 32 accepted")
	}
}
