//go:build !windows

package launcher

import "os/exec"

// hideConsoleWindow 在非 Windows 上没有对应概念：启动链本身只跑 Windows
// （launch_local.py 第一句就拒绝非 nt），这个实现只是让包在别的平台也能编译与测试。
func hideConsoleWindow(cmd *exec.Cmd) {}
