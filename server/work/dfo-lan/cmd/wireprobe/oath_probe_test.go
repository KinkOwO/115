package main

import (
	"fmt"
	"testing"
)

func TestParseOathInject(t *testing.T) {
	specs, err := parseOathInject("2837:64:0;8:71;12:45,2839:64:255")
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 {
		t.Fatalf("got %d specs", len(specs))
	}
	p := specs[0].payload()
	if len(p) != 64 || specs[0].ID != 2837 {
		t.Fatalf("spec0 = %+v payload=%d", specs[0], len(p))
	}
	if p[8] != 71 || p[12] != 45 || p[0] != 0 {
		t.Fatalf("sentinels not placed: %v %v %v", p[0], p[8], p[12])
	}
	if specs[1].payload()[0] != 255 || len(specs[1].payload()) != 64 {
		t.Fatal("spec1 wrong")
	}

	// 短载荷必须被拒：2026-09-27 一条 8 字节的 2839 就打崩了客户端（见 oath_probe.go 文件头）。
	for _, bad := range []string{
		"2837:64", "2837:64:0;99:1", "x:1:2", "2837:99999:0", "2837:64:0;8",
		"2839:8:71", "2839:14:0", "2838:7:0", "2842:103:0", "2841:5:0",
	} {
		if _, err := parseOathInject(bad); err == nil {
			t.Fatalf("%q must be rejected", bad)
		}
	}

	// Exactly the length that packet's parser reads must be accepted.
	for _, ok := range []string{
		"2838:8:0;0:45;4:45", "2839:15:0", "2837:20:0", "2836:69:0", "2842:104:0", "2841:6:0",
	} {
		if got, err := parseOathInject(ok); err != nil || len(got) != 1 {
			t.Fatalf("%q must be accepted: %v %v", ok, got, err)
		}
	}

	if off, err := parseOathInject(""); err != nil || off != nil {
		t.Fatal("an empty spec must disable the injector")
	}
}

// The floor is per packet: each parser reads one fixed-size block first, and a short
// payload trips the client's guard (a deliberate null write) and kills the process.
func TestOathInjectFloorIsPerPacket(t *testing.T) {
	cases := map[uint16]int{2836: 69, 2837: 20, 2838: 8, 2839: 15, 2841: 6, 2842: 104}
	for id, want := range cases {
		if got := oathInjectFloor(id); got != want {
			t.Errorf("oathInjectFloor(%d) = %d, want %d", id, got, want)
		}
		if _, err := parseOathInject(fmt.Sprintf("%d:%d:0", id, want)); err != nil {
			t.Errorf("id %d at its own parser length must be accepted: %v", id, err)
		}
		if _, err := parseOathInject(fmt.Sprintf("%d:%d:0", id, want-1)); err == nil {
			t.Errorf("id %d one byte short of its parser length must be rejected", id)
		}
	}
	if got := oathInjectFloor(9999); got != oathInjectUnknownFloor {
		t.Errorf("unknown id floor = %d, want %d", got, oathInjectUnknownFloor)
	}
}
