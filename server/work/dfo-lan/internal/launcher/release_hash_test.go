package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReleaseHashWarningsSelectedClientAndSameSizeUpdate(t *testing.T) {
	root := t.TempDir()
	client := t.TempDir()
	file := filepath.Join(client, "Script.pvf")
	if err := os.WriteFile(file, []byte("before"), 0600); err != nil {
		t.Fatal(err)
	}
	old, err := fileSHA256(file)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(map[string]any{"files": []map[string]any{{"path": "client/Script.pvf", "sha256": old, "size": 6}, {"path": "../outside", "sha256": old}}})
	if err := os.WriteFile(filepath.Join(root, "release-manifest.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if got := releaseHashWarnings(root, client); len(got) != 0 {
		t.Fatal(got)
	}
	if err := os.WriteFile(file, []byte("after!"), 0600); err != nil {
		t.Fatal(err)
	}
	got := releaseHashWarnings(root, client)
	if len(got) != 1 || !strings.Contains(got[0], "哈希值不一致，继续启动") || !strings.Contains(got[0], old) {
		t.Fatal(got)
	}
	report := LaunchReport{Warnings: got}
	if !strings.Contains(strings.Join(report.Lines(false), "\n"), got[0]) {
		t.Fatal("warning absent from CLI output")
	}
}
