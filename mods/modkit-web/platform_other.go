//go:build !windows

package main

import "os/exec"

// snapshotProcesses 在非 Windows 上没有实现（本项目的客户端只有 Windows 版）。
func snapshotProcesses() []processEntry { return nil }

// clientProcesses 在非 Windows 上恒为空。
func clientProcesses() []int { return nil }

// hideWindow 在非 Windows 上是空操作。
func hideWindow(cmd *exec.Cmd) {}
