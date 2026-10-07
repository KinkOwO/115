package launcher

import (
	"os/exec"
	"syscall"
)

// createNoWindow 是 Win32 的 CREATE_NO_WINDOW。Go 的 syscall 包没有导出它，所以自己写：
// 启动器被 GUI 拉起（没有控制台）时，子进程不该凭空弹出一个黑框。
const createNoWindow = 0x08000000

// hideConsoleWindow 对应 Python 的 creationflags=subprocess.CREATE_NO_WINDOW。
func hideConsoleWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: createNoWindow}
}
