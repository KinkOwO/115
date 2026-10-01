package main

import (
	"encoding/binary"
	"testing"

	"dfolan/internal/inventory"
)

// noti 2838 的载荷是 8 字节、两个 little-endian u32；4/4 实机差分已把偏移钉死。
func TestOathInfoPayloadOrder(t *testing.T) {
	p := oathInfoPayload(45, 71)
	if len(p) != 8 {
		t.Fatalf("payload length = %d, want 8 (the parser reads exactly 8)", len(p))
	}
	if p[0] != 0x2d || p[1] != 0 || p[2] != 0 || p[3] != 0 {
		t.Fatalf("primer u32 wrong: % x", p[0:4])
	}
	if p[4] != 0x47 || p[5] != 0 || p[6] != 0 || p[7] != 0 {
		t.Fatalf("oath u32 wrong: % x", p[4:8])
	}

	// The two fields must be independent - a swap here silently reverses the
	// primer/oath semantics the client's two getters expose.
	a := oathInfoPayload(45, 71)
	b := oathInfoPayload(71, 45)
	if string(a) == string(b) {
		t.Fatal("primer and oath must not encode identically")
	}
	if a[0] != 0x2d || a[4] != 0x47 || b[0] != 0x47 || b[4] != 0x2d {
		t.Fatalf("order mixed up: a=% x b=% x", a, b)
	}
}

func TestParseOathGrades(t *testing.T) {
	// 空串 = 不覆盖（零值），由通关保底决定档位。
	got, err := parseOathGrades("")
	if err != nil {
		t.Fatal(err)
	}
	if got != [2]uint16{} {
		t.Fatalf("empty spec = %v, want the zero pair (no override)", got)
	}

	got, err = parseOathGrades(" 45 , 71 ")
	if err != nil {
		t.Fatal(err)
	}
	if got != [2]uint16{45, 71} {
		t.Fatalf("got %v", got)
	}

	// Every tier the script can index must be accepted.
	for _, v := range []string{"40", "41", "42", "43", "44", "45", "70", "71"} {
		if _, err := parseOathGrades(v + "," + v); err != nil {
			t.Errorf("tier %s must be accepted: %v", v, err)
		}
	}

	// Anything outside the eight tiers reproduces the "cannot be killed" bug,
	// so it must be refused at startup rather than sent.
	for _, bad := range []string{
		"72,72", "0,0", "46,46", "69,69", "45", "45,71,45", "x,45", "-1,45",
	} {
		if _, err := parseOathGrades(bad); err == nil {
			t.Errorf("%q must be rejected", bad)
		}
	}
}

// 没配保底（或没进计入保底的副本）时档位必须落回 normal —— 那才是「隐藏 BOSS 稀有」
// 的落点。恒发 45 会让 state machine 在 oath_now == 44 时必然 summon_orthaire
// （实机确认过 c:nox_index 变成 109019264），也就是场场登场。
//
// 2026-10-01 起 oath 不再固定 normal：中间四档按稀有度递减随机（业主拍板），
// 所以这里守的是「合法 + primer 恒 normal + 不随机出 primeval」，不再是固定 40/40。
func TestOathInfoPacketsFallsBackToNormalWithoutPity(t *testing.T) {
	w := &worldSession{} // 零值覆盖 + 保底场次 0（关闭）
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 || plan[0].ID != 2838 {
		t.Fatalf("plan = %+v", plan)
	}
	p := plan[0].Payload
	primer := binary.LittleEndian.Uint32(p[0:4])
	oath := binary.LittleEndian.Uint32(p[4:8])
	if primer != uint32(inventory.OathGradeNormal) {
		t.Fatalf("primer = %d, want normal(%d) always", primer, inventory.OathGradeNormal)
	}
	if _, ok := oathGradeTiers[uint16(oath)]; !ok {
		t.Fatalf("oath %d is outside the eight tiers the script accepts", oath)
	}
	// 国服实测爆率里 primeval 本身就有 0.35%，所以 45 是随机的合法结果；
	// 这里只要求它落在六档的取值域内（40..45）。
	if oath < 40 || oath > 45 {
		t.Fatalf("oath = %d, want one of the six rolled tiers (40..45)", oath)
	}
}

// 发出去的包必须是「一个 id + 8 字节」，且 kind 名可被日志认出来。
func TestOathInfoPacketsShape(t *testing.T) {
	w := &worldSession{oathGrades: [2]uint16{45, 45}}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) != 1 {
		t.Fatalf("got %d packets", len(plan))
	}
	p := plan[0]
	if p.ID != 2838 {
		t.Fatalf("id = %d, want 2838", p.ID)
	}
	if p.Name != "oath_system_grades" {
		t.Fatalf("kind name = %q", p.Name)
	}
	if len(p.Payload) != 8 {
		t.Fatalf("payload = %d bytes, want 8", len(p.Payload))
	}
}
