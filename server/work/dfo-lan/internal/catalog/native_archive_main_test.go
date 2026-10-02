package catalog

import (
	"os"
	"testing"
)

// TestMain closes the process-wide native test archive after the package run so
// its backing file descriptor and read caches are released deterministically.
func TestMain(m *testing.M) {
	code := m.Run()
	_ = CloseNativeArchive()
	os.Exit(code)
}
