//go:build windows

package launcher

import (
	"errors"
	"os"
	"syscall"
)

// Win32 错误码：Go 的 syscall 包没有导出 ERROR_INVALID_PARAMETER，所以按数值写。
// 两者都表示 OpenProcess 明确说「没有这个进程」；Access denied（5）不在其中，那种情况
// 归 PidUnknown（保守不动）。
const (
	errInvalidParameter = syscall.Errno(87) // ERROR_INVALID_PARAMETER（本机实测：不存在的 pid）
	errFileNotFound     = syscall.Errno(2)  // ERROR_FILE_NOT_FOUND（部分 Windows 版本的回答）
)

// realProcessStatus 在 Windows 上靠 OpenProcess（os.FindProcess 的底层）判断 pid 是否存在。
//
// 实测（本机 Go 1.26 / Windows 11）：
//
//	pid 不存在     -> OpenProcess: The parameter is incorrect.  = ERROR_INVALID_PARAMETER -> PidGone
//	pid 是本进程   -> 成功                                                                 -> PidAlive
//	pid 4（系统）  -> OpenProcess: Access is denied.             = ERROR_ACCESS_DENIED      -> PidUnknown
//
// 「权限不足」绝不能算死：那正是「活的被误判」的唯一入口，宁可多等 TTL。
func realProcessStatus(pid int) PidStatus {
	handle, err := os.FindProcess(pid)
	if err == nil {
		_ = handle.Release()
		return PidAlive
	}
	if errors.Is(err, errInvalidParameter) || errors.Is(err, errFileNotFound) {
		return PidGone
	}
	return PidUnknown
}
