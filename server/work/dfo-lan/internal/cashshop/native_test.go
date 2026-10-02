package cashshop

import (
	"sync"
	"testing"

	"dfolan/internal/catalog"
)

// nativePilot imports the current inner PVF named by DFO_PVF_CORE_TEST_ARCHIVE.
// Tests skip when the environment does not provide an archive, so a bare
// checkout stays runnable; with the archive set they assert against the one
// content source rather than a historical JSON export. The expensive
// ImportPilot runs once per test binary; NewPilot (which deep-copies) is then
// cheap per call.
var (
	nativePilotOnce   sync.Once
	nativePilotConfig PilotConfig
	nativePilotErr    error
)

func nativePilot(t *testing.T, release bool) *Pilot {
	t.Helper()
	a := catalog.OpenNativeArchive(t)
	nativePilotOnce.Do(func() {
		nativePilotConfig, nativePilotErr = ImportPilot(a)
	})
	if nativePilotErr != nil {
		t.Fatalf("import native cashshop: %v", nativePilotErr)
	}
	p, err := NewPilot(nativePilotConfig, nativePilotConfig.Source.Checksum, release)
	if err != nil {
		t.Fatalf("construct native cashshop: %v", err)
	}
	return p
}
