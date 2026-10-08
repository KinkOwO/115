//go:build windows

package main

import (
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

// snapshotProcesses 取一份进程快照（Windows：Toolhelp）。
//
// 口径与启动器的 internal/proc.FindByName 一致（按可执行文件名比较、忽略大小写）。
// 这里只用标准库，因为这个页面工具刻意**不依赖启动器代码**
// （见 README §五：数据层可以整体换成引擎实现，页面本身要保持可独立编译）。
func snapshotProcesses() []processEntry {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil || snap == syscall.InvalidHandle {
		return nil
	}
	defer syscall.CloseHandle(snap)

	var entry syscall.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	var out []processEntry
	for err = syscall.Process32First(snap, &entry); err == nil; err = syscall.Process32Next(snap, &entry) {
		out = append(out, processEntry{PID: int(entry.ProcessID), Name: syscall.UTF16ToString(entry.ExeFile[:])})
	}
	return out
}

// clientProcesses 返回正在运行的 DFO.exe 进程号。
func clientProcesses() []int {
	var out []int
	for _, p := range snapshotProcesses() {
		if strings.EqualFold(p.Name, "DFO.exe") {
			out = append(out, p.PID)
		}
	}
	return out
}

// hideWindow 让派生的控制台子进程不弹黑窗（本工具的宿主可能没有控制台）。
func hideWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
}
