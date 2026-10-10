//go:build !windows

package launcher

import (
	"fmt"
	"os/exec"
)

func startMemoryBoundedChild(cmd *exec.Cmd, budget uint64, label string) (*childProcess, error) {
	return nil, fmt.Errorf("%s: protected game startup requires Windows", label)
}
