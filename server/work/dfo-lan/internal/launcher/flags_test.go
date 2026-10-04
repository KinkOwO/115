package launcher

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// The usage parsing is pinned against a real usage block, so a change in Go's flag output
// or in this parser is caught here rather than by a gateway that refuses to start.
func TestExeFlagsParsesUsage(t *testing.T) {
	names := map[string]bool{}
	usage := "" +
		"Usage of wireprobe:\n" +
		"  -pvf-catalogs string\n" +
		"        which domains to read\n" +
		"  -channel-identity\n" +
		"        enable identity checks\n" +
		"  -listen 127.0.0.2:0\n" +
		"  -quest-equipment-catalog string\n" +
		"not a flag line\n"
	for _, line := range splitLines(usage) {
		if match := flagLine.FindStringSubmatch(line); match != nil {
			names[match[1]] = true
		}
	}
	for _, want := range []string{"pvf-catalogs", "channel-identity", "listen", "quest-equipment-catalog"} {
		if !names[want] {
			t.Errorf("usage parsing missed %s", want)
		}
	}
	if names["a"] {
		t.Error("usage parsing invented a flag")
	}
}

// The gateway used for PVF-direct runs must advertise pvf-catalogs, and the real binary in
// this tree does. Checked without Python so the assertion survives the tools removal.
func TestExeFlagsAgainstTheRealBinary(t *testing.T) {
	binary := filepath.Join("..", "..", "bin", "wireprobe-pvf.exe")
	if _, err := os.Stat(binary); err != nil {
		t.Skipf("the PVF gateway is not present (%v)", err)
	}
	absolute, err := filepath.Abs(binary)
	if err != nil {
		t.Fatal(err)
	}
	flags := ExeFlags(context.Background(), absolute)
	if flags == nil {
		t.Fatalf("%s -h produced no flags; the probe or the parser is wrong", absolute)
	}
	if !flags["pvf-catalogs"] {
		t.Errorf("the PVF gateway does not advertise pvf-catalogs; PVF-direct startup would be refused. Flags seen: %d", len(flags))
	}
}

func TestPruneCommandDropsFlagsAndTheirValues(t *testing.T) {
	command := []string{"srv.exe", "-keep", "value", "-gone", "value2", "-keep2", "positional", "-gone2"}
	supported := map[string]bool{"keep": true, "keep2": true}

	kept, dropped, err := PruneCommand(command, supported, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	want := []string{"srv.exe", "-keep", "value", "-keep2", "positional"}
	if len(kept) != len(want) {
		t.Fatalf("kept = %v, want %v", kept, want)
	}
	for i := range want {
		if kept[i] != want[i] {
			t.Fatalf("kept = %v, want %v", kept, want)
		}
	}
	// A dropped flag takes its value with it, and a valueless one takes nothing.
	if len(dropped) != 3 || dropped[0] != "-gone" || dropped[1] != "value2" || dropped[2] != "-gone2" {
		t.Errorf("dropped = %v, want [-gone value2 -gone2]", dropped)
	}
}

// A failed probe must leave the command alone: this is the difference between working
// around an older binary and breaking a current one.
func TestPruneCommandKeepsEverythingWhenTheProbeFailed(t *testing.T) {
	command := []string{"srv.exe", "-anything", "value"}
	kept, dropped, err := PruneCommand(command, nil, false)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if len(kept) != len(command) || len(dropped) != 0 {
		t.Errorf("kept = %v, dropped = %v; want the command unchanged", kept, dropped)
	}
}

// A PVF-direct run against a gateway that does not advertise support must refuse rather
// than start with a different content source.
func TestPruneCommandRefusesWithoutPVFSupport(t *testing.T) {
	command := []string{"srv.exe"}
	if _, _, err := PruneCommand(command, map[string]bool{"listen": true}, true); !errors.Is(err, errNoPVFSupport) {
		t.Errorf("err = %v, want errNoPVFSupport", err)
	}
	if _, _, err := PruneCommand(command, nil, true); !errors.Is(err, errNoPVFSupport) {
		t.Errorf("an unknown flag set must also refuse: %v", err)
	}
	// With the flag advertised it proceeds.
	if _, _, err := PruneCommand(command, map[string]bool{"pvf-catalogs": true}, true); err != nil {
		t.Errorf("a supporting gateway was refused: %v", err)
	}
}

func TestSetOptionValueReplacesOrAppends(t *testing.T) {
	command := []string{"srv.exe", "-quest-equipment-catalog", "old", "-other"}
	updated := SetOptionValue(append([]string{}, command...), "-quest-equipment-catalog", "pvf")
	if updated[2] != "pvf" || len(updated) != len(command) {
		t.Errorf("replace produced %v", updated)
	}
	appended := SetOptionValue(append([]string{}, command...), "-new-flag", "v")
	if len(appended) != len(command)+2 || appended[len(appended)-2] != "-new-flag" || appended[len(appended)-1] != "v" {
		t.Errorf("append produced %v", appended)
	}
}

func splitLines(text string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			lines = append(lines, text[start:i])
			start = i + 1
		}
	}
	if start < len(text) {
		lines = append(lines, text[start:])
	}
	return lines
}
