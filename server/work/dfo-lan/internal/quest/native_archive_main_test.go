package quest

import (
	"os"
	"testing"

	"dfolan/internal/catalog"
)

// TestMain closes the process-wide native test archive opened through the
// catalog test helpers once the package's tests finish.
func TestMain(m *testing.M) {
	code := m.Run()
	_ = catalog.CloseNativeArchive()
	os.Exit(code)
}
