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
	// 2838 之后还会跟一帧 2859「调律之边界奖励档位」（档位是 45 ⇒ 太初演出那格，
	// 见 attunement_reward.go）。这里只钉 2838 自己的形状，所以按 id 取。
	var p outboundPacket
	for _, cand := range plan {
		if cand.ID == 2838 {
			p = cand
		}
	}
	if p.ID != 2838 {
		t.Fatalf("id = %d, want 2838（plan=%+v）", p.ID, plan)
	}
	if p.Name != "oath_system_grades" {
		t.Fatalf("kind name = %q", p.Name)
	}
	if len(p.Payload) != 8 {
		t.Fatalf("payload = %d bytes, want 8", len(p.Payload))
	}
}

// ⚠️ 诊断注入**不是**「只改演出」的旋钮：同一个数字直接喂掉落选池（w.drops.Tiers），
// 而 fixed 池在保底档以下没有条目 —— 注入更低的值以前会让**进本应答整段发不出去**，
// 客户端卡在加载界面（实机 2026-10-12 21:49 与 22:08，`DFO_OATH_GRADES=43,43` 两次）。
// 现在 primer 抬到本场保底档并念出来，**oath 那条线不动**（additional 反查 40..45 每档都有条目）。
func TestOathOverrideBelowThePoolFloorIsRaisedNotFatal(t *testing.T) {
	w := attuneConfirmSession(t, "run-clamp", 100005068, 0)
	w.oathGrades = [2]uint16{43, 43}
	plan, err := w.oathInfoPackets()
	if err != nil {
		t.Fatalf("a below-floor diagnostic must not fail the entry: %v", err)
	}
	if len(plan) == 0 || plan[0].ID != 2838 {
		t.Fatalf("plan = %+v, want the noti 2838 frame first", plan)
	}
	if p, o := binary.LittleEndian.Uint32(plan[0].Payload[0:4]), binary.LittleEndian.Uint32(plan[0].Payload[4:8]); p != 44 || o != 43 {
		t.Fatalf("payload primer=%d oath=%d, want 44/43", p, o)
	}
	if w.attunementRunTiers.Primer != 44 || w.attunementRunTiers.Oath != 43 {
		t.Fatalf("run tiers = %+v, want the clamped primer with oath left alone", w.attunementRunTiers)
	}
}

// 下界本身（44）与更高的档必须原样通过 —— 抬升只兜「不可能的那一端」，
// 否则这个旋钮就废了（业主拿它扫的就是 40..45 每一档）。
func TestOathOverrideAtOrAboveThePoolFloorIsLeftAlone(t *testing.T) {
	for _, c := range [][2]uint16{{44, 40}, {45, 41}, {44, 45}} {
		w := attuneConfirmSession(t, "run-passthrough", 100005068, 0)
		w.oathGrades = c
		plan, err := w.oathInfoPackets()
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		if p := binary.LittleEndian.Uint32(plan[0].Payload[0:4]); p != uint32(c[0]) {
			t.Fatalf("%v: primer = %d, want it untouched", c, p)
		}
	}
}
