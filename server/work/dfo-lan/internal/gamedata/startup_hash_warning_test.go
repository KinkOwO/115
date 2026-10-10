package gamedata

import (
	"bytes"
	"dfolan/internal/savecontract"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStartupHashWarningDoesNotAcceptCorruptArchive(t *testing.T) {
	path := filepath.Join(t.TempDir(), "corrupt.pvf")
	if err := os.WriteFile(path, []byte("not a readable archive"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: strings.Repeat("0", 64), StartupHashWarnings: true})
	if err == nil || strings.Contains(err.Error(), "source mismatch") {
		t.Fatalf("expected parsing error, got %v", err)
	}
}

func TestStartupHashWarningNativeArchiveAndCharacterPolicy(t *testing.T) {
	path := os.Getenv("DFO_STARTUP_WARNING_TEST_ARCHIVE")
	if path == "" {
		t.Skip("actual merged native archive required")
	}
	var logs bytes.Buffer
	old := log.Writer()
	log.SetOutput(&logs)
	defer log.SetOutput(old)
	wrong := strings.Repeat("0", 64)
	if src, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: wrong}); err == nil {
		src.Close()
		t.Fatal("audit unexpectedly accepted mismatched hash")
	}
	src, err := Open(Options{Mode: PVF, ArchivePath: path, ExpectedChecksum: wrong, StartupHashWarnings: true, DerivedCacheDir: testPVFCacheDir()})
	if err != nil {
		t.Fatal(err)
	}
	actual := src.Snapshot().Checksum
	src.Close()
	if actual == wrong || !strings.Contains(logs.String(), "PVF 哈希值不一致，继续启动") || !strings.Contains(logs.String(), actual) {
		t.Fatal("actual source or warning lost", logs.String())
	}
	// Use a stale policy fingerprint, retaining the full actual shortcut policy.
	policy, err := readPVFCharacterPolicy("../../configs/pvf-character-policy.json")
	if err != nil {
		t.Fatal(err)
	}
	policy.SourceChecksum = wrong
	policyPath := filepath.Join(t.TempDir(), "character-policy.json")
	data, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(policyPath, data, 0600); err != nil {
		t.Fatal(err)
	}
	c, err := PrepareCatalogs(CatalogInputs{Selection: "characters", ArchivePath: path, ArchiveChecksum: wrong, CharacterPolicyPath: policyPath, StartupHashWarnings: true, DerivedCacheDir: testPVFCacheDir()}, CatalogAdapters{})
	if err != nil {
		t.Fatal(err)
	}
	if c.SourceChecksum != actual || c.Characters == nil || c.Characters.Source.SaveIdentity() != savecontract.Identity() {
		t.Fatal("actual source/save identity lost")
	}
	if !strings.Contains(logs.String(), "角色配置 PVF 哈希值不一致，继续启动") {
		t.Fatal(logs.String())
	}
}
