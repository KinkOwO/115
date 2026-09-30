package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCLIRefusesInvalidDomainAndReportOverwriteBeforeOpeningArchive(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if code := run([]string{"-domains", "world,world"}, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
		t.Fatal(code)
	}
	path := filepath.Join(t.TempDir(), "report.json")
	if err := os.WriteFile(path, []byte("existing report"), 0600); err != nil {
		t.Fatal(err)
	}
	if code := run([]string{"-output", path}, &stdout, &stderr); code != 1 {
		t.Fatal(code)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "existing report" {
		t.Fatalf("report changed: %q %v", raw, err)
	}
}
