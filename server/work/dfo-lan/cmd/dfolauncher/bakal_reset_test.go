package main

import "testing"

func TestBakalResetGuardsEveryGatewayImage(t *testing.T) {
	for _, name := range []string{"DFO.exe", "wireprobe.exe", "wireprobe-pvf.exe", "wireprobe-handoff-source.exe", "wireprobe-bakal-weekly-quota-candidate.exe", "WIREPROBE-OTHER.EXE"} {
		busy, err := bakalSessionProcessList(`"` + name + `","42","Console","1","9,999 K"` + "\r\n")
		if err != nil || !busy {
			t.Fatalf("image %s bypassed guard: busy=%v err=%v", name, busy, err)
		}
	}
	for _, data := range []string{"", `"explorer.exe","42","wireprobe-pvf.exe","1","123 K"`, `"not-wireprobe.exe","42","Console","1","123 K"`} {
		busy, err := bakalSessionProcessList(data)
		if err != nil || busy {
			t.Fatalf("unrelated task falsely blocked: %q err=%v", data, err)
		}
	}
	if _, err := bakalSessionProcessList(`"unclosed`); err == nil {
		t.Fatal("failed process-list parsing allowed a write")
	}
}

func TestBakalResetHelpNeverOpensStorage(t *testing.T) {
	if code := runBakalReset([]string{"--help"}); code != 0 {
		t.Fatalf("help exit=%d", code)
	}
}
