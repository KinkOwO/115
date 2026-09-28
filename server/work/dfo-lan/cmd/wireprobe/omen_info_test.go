package main

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"

	"dfolan/internal/loot"
)

func TestOmenInfoPayloadGeometry(t *testing.T) {
	var seats [omenInfoSeats][4]uint32
	var states [omenInfoSeats]uint8
	seats[0] = [4]uint32{3, 0, 0, 0}
	states[0] = 1
	seats[2] = [4]uint32{4, 2, 1, 0}
	states[2] = 4

	got := omenInfoPayload(seats, states, 0)
	// 69 是客户端解析器写死的读取长度（sub_146EA0BE0(&buf, 69)）；常量不许漂。
	if omenInfoPayloadLen != 69 {
		t.Fatalf("omenInfoPayloadLen = %d, want 69", omenInfoPayloadLen)
	}
	if len(got) != omenInfoPayloadLen {
		t.Fatalf("payload = %d bytes, want %d", len(got), omenInfoPayloadLen)
	}
	// 座位 0：u32[0]=3，状态 1，落在记录头与 +16。
	if v := binary.LittleEndian.Uint32(got[0:4]); v != 3 {
		t.Fatalf("seat0 u32[0] = %d, want 3", v)
	}
	if got[16] != 1 {
		t.Fatalf("seat0 state = %d, want 1", got[16])
	}
	// 座位 1 全零。
	for i := 17; i < 34; i++ {
		if got[i] != 0 {
			t.Fatalf("seat1 byte %d = %d, want 0", i, got[i])
		}
	}
	// 座位 2 从 34 开始。
	if v := binary.LittleEndian.Uint32(got[34:38]); v != 4 {
		t.Fatalf("seat2 u32[0] = %d, want 4", v)
	}
	if v := binary.LittleEndian.Uint32(got[42:46]); v != 1 {
		t.Fatalf("seat2 u32[2] = %d, want 1", v)
	}
	if got[50] != 4 {
		t.Fatalf("seat2 state = %d, want 4", got[50])
	}
	// 尾标志字节。
	if got[68] != 0 {
		t.Fatalf("trailing flag = %d, want 0", got[68])
	}
	if got2 := omenInfoPayload(seats, states, 1); got2[68] != 1 {
		t.Fatalf("trailing flag = %d, want 1", got2[68])
	}
}

func TestParseOmenInfoReadable(t *testing.T) {
	got, err := parseOmenInfo("2,0,0,0,1;0;0;0;0")
	if err != nil {
		t.Fatal(err)
	}
	want := omenInfoPayload([omenInfoSeats][4]uint32{{2, 0, 0, 0}}, [omenInfoSeats]uint8{1}, 0)
	if hex.EncodeToString(got) != hex.EncodeToString(want) {
		t.Fatalf("payload mismatch:\n got %s\nwant %s", hex.EncodeToString(got), hex.EncodeToString(want))
	}
	// 尾标志段可省、可用 1。
	if g, err := parseOmenInfo("1,0,0,0,1;;;;"); err != nil {
		t.Fatal(err)
	} else if g[68] != 0 {
		t.Fatalf("omitted flag = %d, want 0", g[68])
	}
	if g, err := parseOmenInfo("1,0,0,0,1;;;;1"); err != nil {
		t.Fatal(err)
	} else if g[68] != 1 {
		t.Fatalf("flag = %d, want 1", g[68])
	}
	// 空串 = 不注入。
	if g, err := parseOmenInfo("   "); err != nil || g != nil {
		t.Fatalf("blank spec = %v, %v; want nil, nil", g, err)
	}
}

func TestParseOmenInfoRawHex(t *testing.T) {
	raw := make([]byte, omenInfoPayloadLen)
	for i := range raw {
		raw[i] = byte(i)
	}
	spec := strings.ToUpper(hex.EncodeToString(raw))
	got, err := parseOmenInfo(spec)
	if err != nil {
		t.Fatal(err)
	}
	if hex.EncodeToString(got) != hex.EncodeToString(raw) {
		t.Fatalf("raw round trip mismatch")
	}
	// 长度不对的十六进制串要报错，而不是被当成可读写法静默接受。
	if _, err := parseOmenInfo(strings.Repeat("ab", 68)); err == nil {
		t.Fatal("136 hex chars must be rejected (69 bytes expected)")
	}
}

// 尾标志段可以整段省略。这条是回归测试：默认值曾经写成只有 4 段的样子，
// 而解析器要求 5 段 —— 服务端在"进频道"那一刻 log.Fatalf，现象是进不去频道。
func TestParseOmenInfoAllowsOmittedTrailingFlag(t *testing.T) {
	// ④ 段 = 座位 0..3 全给值，尾标志省略 ⇒ 应当取 0，而不是报错。
	spec := "1,1,1,1,1;1,1,1,1,1;1,1,1,1,1;1,1,1,1,1"
	got, err := parseOmenInfo(spec)
	if err != nil {
		t.Fatalf("4 groups must be accepted (trailing flag defaults to 0): %v", err)
	}
	if len(got) != omenInfoPayloadLen {
		t.Fatalf("payload = %d bytes, want %d", len(got), omenInfoPayloadLen)
	}
	// 等价写法：显式补一个空段 / 显式写 0。
	for _, want := range []string{spec + ";", spec + ";0"} {
		g, err := parseOmenInfo(want)
		if err != nil {
			t.Fatal(err)
		}
		if hex.EncodeToString(g) != hex.EncodeToString(got) {
			t.Fatalf("%q must equal the omitted-flag form", want)
		}
	}
	// 四个座位的 u32 与状态字节都要真的落到记录里（每 17 字节一条）。
	for i := 0; i < omenInfoSeats; i++ {
		base := i * omenInfoRecordBytes
		for j := 0; j < 4; j++ {
			if v := binary.LittleEndian.Uint32(got[base+4*j:]); v != 1 {
				t.Fatalf("seat%d u32[%d] = %d, want 1", i, j, v)
			}
		}
		if got[base+16] != 1 {
			t.Fatalf("seat%d state = %d, want 1", i, got[base+16])
		}
	}
	if got[68] != 0 {
		t.Fatalf("trailing flag = %d, want 0", got[68])
	}
	// 段数仍受约束：3 段和 6 段都必须拒绝。
	for _, bad := range []string{"1;2;3", "1;1;1;1;1;1"} {
		if _, err := parseOmenInfo(bad); err == nil {
			t.Fatalf("spec %q must be rejected", bad)
		}
	}
}

func TestParseOmenInfoRejectsBadSpec(t *testing.T) {
	for _, spec := range []string{
		"1;2;3",               // 段数不足
		"1,2,3;0;0;0;0",       // 座位段数字不足
		"1,2,3,4;0;0;0;0",     // 座位段少一个（4 个数字）
		"1,2,3,4,5,6;0;0;0;0", // 座位段多一个
		"1,0,0,0,300",         // u8 越界
	} {
		if _, err := parseOmenInfo(spec); err == nil {
			t.Fatalf("spec %q must be rejected", spec)
		}
	}
}

func TestOmenInfoPacketsOffByDefault(t *testing.T) {
	w := &worldSession{}
	p, err := w.omenInfoPackets()
	if err != nil {
		t.Fatalf("unconfigured session must not fail: %v", err)
	}
	if p != nil {
		t.Fatalf("unconfigured session must send nothing, got %+v", p)
	}
	w.omenInfo = omenInfoPayload([omenInfoSeats][4]uint32{}, [omenInfoSeats]uint8{}, 0)
	p, err = w.omenInfoPackets()
	if err != nil {
		t.Fatalf("explicit payload: %v", err)
	}
	if len(p) != 1 || p[0].ID != omenInfoPacketID || len(p[0].Payload) != omenInfoPayloadLen {
		t.Fatalf("plan = %+v", p)
	}
}

// TestOmenInfoPayloadForHeldCountsActiveStages 钉住「档位 = 记录里非零 u32 的个数」
// 这一条实机结论（docs §36：1 档一颗石头/亮第一格，4 档四颗/亮第四格）。它是 UI 与
// 场内表现唯一的联系点，写错就表现为「格子数不对」。
func TestOmenInfoPayloadForHeldCountsActiveStages(t *testing.T) {
	ids := []uint32{10416150, 10417545, 10417552, 10417571}
	table := append([]uint32{0}, ids...)
	for held := 0; held <= 4; held++ {
		active := omenActiveIDs(table, held)
		if len(active) != held {
			t.Fatalf("held %d: %d active ids", held, len(active))
		}
		p := omenInfoPayloadForHeld(active)
		if len(p) != omenInfoPayloadLen {
			t.Fatalf("held %d: payload %d bytes", held, len(p))
		}
		for seat := 0; seat < omenInfoSeats; seat++ {
			base := seat * omenInfoRecordBytes
			nonZero := 0
			for j := 0; j < 4; j++ {
				if binary.LittleEndian.Uint32(p[base+4*j:]) != 0 {
					nonZero++
				}
			}
			if nonZero != held {
				t.Fatalf("held %d seat %d: %d non-zero u32, want %d", held, seat, nonZero, held)
			}
			wantState := uint8(0)
			if held > 0 {
				wantState = 1
			}
			if p[base+16] != wantState {
				t.Fatalf("held %d seat %d: state %d, want %d", held, seat, p[base+16], wantState)
			}
			// 预览模板要落在**该档**那一格上，串档就等于 UI 显示错的奖励。
			for j := 0; j < 4; j++ {
				got := binary.LittleEndian.Uint32(p[base+4*j:])
				want := uint32(0)
				if j < held {
					want = ids[j]
				}
				if got != want {
					t.Fatalf("held %d seat %d slot %d: id %d, want %d", held, seat, j, got, want)
				}
			}
		}
	}
}

// TestOmenActiveIDsSkipsRowZero 守的是下标关系：行 0 是「有概率激活第一个征兆」，
// 它本身没有任何奖励条目，所以不能出现在载荷里。
func TestOmenActiveIDsSkipsRowZero(t *testing.T) {
	table := []uint32{0, 101, 102, 103, 104}
	if got := omenActiveIDs(table, 0); got != nil {
		t.Fatalf("held 0 must be an empty payload, got %v", got)
	}
	if got := omenActiveIDs(table, 1); len(got) != 1 || got[0] != 101 {
		t.Fatalf("held 1 = %v, want [101]", got)
	}
	if got := omenActiveIDs(table, 4); len(got) != 4 || got[3] != 104 || got[0] != 101 {
		t.Fatalf("held 4 = %v", got)
	}
	if got := omenActiveIDs(table, 5); got != nil {
		t.Fatalf("held past the last row must not index out of the table, got %v", got)
	}
	if got := omenActiveIDs(nil, 1); got != nil {
		t.Fatalf("no table means no ids, got %v", got)
	}
}

// TestOmenSettlementIsFull 钉住隐藏 BOSS 的触发条件（业主 2026-09-27 定调 B1）：
// 只有「集齐四档并结算」那一次才算，普通结算与只涨不结都不算。
func TestOmenSettlementIsFull(t *testing.T) {
	const stages = 5 // 行 0..4
	for _, tc := range []struct {
		name    string
		outcome loot.OmenOutcome
		want    bool
	}{
		{"full settlement", loot.OmenOutcome{Paid: true, Held: 4}, true},
		{"partial settlement stays rare", loot.OmenOutcome{Paid: true, Held: 2}, false},
		{"gain is not a settlement", loot.OmenOutcome{Gained: true, Held: 3}, false},
		{"reset without payment", loot.OmenOutcome{Held: 4}, false},
		{"no omen table at all", loot.OmenOutcome{Paid: true, Held: 4}, false},
	} {
		out := tc.outcome
		s := stages
		if tc.name == "no omen table at all" {
			s = 0
		}
		if got := omenSettlementIsFull(out, s); got != tc.want {
			t.Fatalf("%s: full = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestOrthaierDuePrefersOmenState 证明档位判断的两条来源不会互相串：
// -omen-state 开着时只读会话里那份（与 noti 2836 同源），关着时才回落旧通关保底。
func TestOrthaierDuePrefersOmenState(t *testing.T) {
	off := &worldSession{}
	due, err := off.orthaireDue()
	if err != nil {
		t.Fatal(err)
	}
	if due {
		t.Fatal("a session with neither a store nor a pity threshold must not summon the hidden boss")
	}

	on := &worldSession{omenState: true}
	if due, err = on.orthaireDue(); err != nil || due {
		t.Fatalf("fresh omen session: due=%v err=%v", due, err)
	}
	on.omenOrthaierDue = true
	if due, err = on.orthaireDue(); err != nil || !due {
		t.Fatalf("pending omen session: due=%v err=%v", due, err)
	}
}
