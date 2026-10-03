package pvfaudit

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
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
	if code := run([]string{"-selection-scope", "-output", path}, &stdout, &stderr); code != 1 {
		t.Fatal(code)
	}
	raw, err := os.ReadFile(path)
	if err != nil || string(raw) != "existing report" {
		t.Fatalf("report changed: %q %v", raw, err)
	}
}

func TestSelectionScopeRefusesComparisonFlagsBeforeOpeningArchive(t *testing.T) {
	for _, args := range [][]string{
		{"-selection-scope", "-domains", "quests"},
		{"-selection-scope", "-character-catalog", "missing.json"},
		{"-selection-scope", "-difference-limit", "-1"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
			t.Fatal(code, stdout.String(), stderr.String())
		}
		if strings.Contains(stderr.String(), "open inner PVF") {
			t.Fatal("invalid request reached archive loading", stderr.String())
		}
	}
}

func TestRetiredDomainAuditRequiresExplicitHistoricalBaseline(t *testing.T) {
	for _, domain := range []string{"world", "quests"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"-domains", domain}, &stdout, &stderr); code != 1 || stdout.Len() != 0 {
			t.Fatal(code, stdout.String())
		}
		if !strings.Contains(stderr.String(), "explicit historical baseline") || strings.Contains(stderr.String(), "open inner PVF") {
			t.Fatal("retired default baseline reached archive loading", stderr.String())
		}
	}
}
