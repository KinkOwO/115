//go:build !windows

package launcher

import (
	"errors"
	"syscall"
)

// realProcessStatus 在 POSIX 上用 `kill(pid, 0)`：ESRCH = 不存在（PidGone），
// EPERM = 存在但不属于我们（PidUnknown，保守不动）。
func realProcessStatus(pid int) PidStatus {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return PidAlive
	}
	if errors.Is(err, syscall.ESRCH) {
		return PidGone
	}
	return PidUnknown
}
