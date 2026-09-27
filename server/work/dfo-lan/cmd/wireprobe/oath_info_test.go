package main

import "testing"

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
	got, err := parseOathGrades("")
	if err != nil {
		t.Fatal(err)
	}
	if got != [2]uint16{oathGradeDefault, oathGradeDefault} {
		t.Fatalf("default = %v, want %d twice", got, oathGradeDefault)
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

// The default is the tier that reaches the hidden boss: `oath_max == 45` is the only
// branch that selects nox_is_orthaire (confirmed live: c:nox_index became 109019264).
func TestOathGradeDefaultReachesTheHiddenBoss(t *testing.T) {
	var primer, oath uint16 = oathGradeDefault, oathGradeDefault
	if _, ok := oathGradeTiers[primer]; !ok {
		t.Fatalf("primer default %d is not a legal tier", primer)
	}
	if oath != 45 {
		t.Fatalf("oath default = %d; 45 (primitive) is the only tier that summons the hidden boss", oath)
	}
	if idx := int32(primer) - 40; idx < 0 || idx > 7 {
		t.Fatalf("primer default %d indexes o:hp_limit out of bounds (%d)", primer, idx)
	}
}

// 发出去的包必须是「一个 id + 8 字节」，且 kind 名可被日志认出来。
func TestOathInfoPacketsShape(t *testing.T) {
	w := &worldSession{oathGrades: [2]uint16{45, 45}}
	plan := w.oathInfoPackets()
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
