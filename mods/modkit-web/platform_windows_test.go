//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSnapshotProcessesFindsSelf 证明这条探测**真的在枚举进程**，而不是恒返回空。
//
// 为什么值得单测：这道门是"游戏在跑就拒绝写盘"的唯一判据。如果 Toolhelp 调用写错
// （句柄类型、结构体大小、UTF-16 转换），症状会是"永远查不到 DFO.exe"——
// 安全门静默失效，而任何假进程表都测不出来。
func TestSnapshotProcessesFindsSelf(t *testing.T) {
	procs := snapshotProcesses()
	if len(procs) == 0 {
		t.Fatal("进程快照是空的：枚举实现有问题")
	}
	self, err := os.Executable()
	if err != nil {
		t.Skipf("取不到自身可执行文件路径：%v", err)
	}
	want := filepath.Base(self)
	named := 0
	for _, p := range procs {
		if strings.TrimSpace(p.Name) == "" {
			t.Fatalf("快照里有没名字的进程项：%+v", p)
		}
		if p.PID > 0 {
			named++
		}
		if strings.EqualFold(p.Name, want) {
			return
		}
	}
	if named == 0 {
		t.Fatalf("快照里没有任何 PID>0 的进程项：枚举实现有问题")
	}
	t.Fatalf("快照里没有测试进程自己（%s）：枚举实现有问题", want)
}

// TestClientProcessesOnlyMatchesDFO 钉住过滤口径：只认 DFO.exe，不把别的进程算进来。
func TestClientProcessesOnlyMatchesDFO(t *testing.T) {
	for _, pid := range clientProcesses() {
		found := false
		for _, p := range snapshotProcesses() {
			if p.PID == pid && strings.EqualFold(p.Name, "DFO.exe") {
				found = true
			}
		}
		if !found {
			t.Fatalf("clientProcesses 返回了一个不是 DFO.exe 的进程号：%d", pid)
		}
	}
}
