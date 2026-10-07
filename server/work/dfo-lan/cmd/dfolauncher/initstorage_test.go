package main

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// stubPostgresBin 造一份"看起来完整"的 PostgreSQL bin 目录（三个依赖文件都在）。
func stubPostgresBin(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	for _, name := range []string{"initdb.exe", "pg_ctl.exe", "createdb.exe"} {
		if err := os.WriteFile(filepath.Join(bin, name), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return bin
}

// freePort 找一个当前没人监听的端口（拿不到就跳过）。
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法绑定回环端口：%v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// 退出码约定：参数错 2、环境不干净（守卫拒绝）1、--dry-run 判定完成 0 且不落盘。
func TestRunInitStorageExitCodes(t *testing.T) {
	if code := runInitStorage([]string{"--nope"}); code != 2 {
		t.Errorf("未知开关 exit = %d, want 2", code)
	}

	// 已经有 local.json：必须拒绝（那是既有存档/配置，不是可以覆盖的产物）。
	root := t.TempDir()
	config := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	if err := os.MkdirAll(filepath.Dir(config), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(config, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := stubPostgresBin(t)
	if code := runInitStorage([]string{"--root", root, "--postgres-bin", bin, "--postgres-port", "25538"}); code != 1 {
		t.Errorf("已有配置 exit = %d, want 1", code)
	}

	// 干净环境：--dry-run 打印步骤并返回 0，且一个字节都不写。
	clean := t.TempDir()
	port := strconv.Itoa(freePort(t))
	if code := runInitStorage([]string{
		"--root", clean, "--postgres-bin", bin,
		"--postgres-port", port, "--dry-run"}); code != 0 {
		t.Errorf("dry-run exit = %d, want 0", code)
	}
	if _, err := os.Stat(filepath.Join(clean, "server")); !os.IsNotExist(err) {
		t.Errorf("--dry-run 建出了目录：%v", err)
	}
}

// 缺依赖（这里直接把 bin 指到空目录）必须在碰任何东西之前失败。
func TestRunInitStorageRefusesMissingDependencies(t *testing.T) {
	root := t.TempDir()
	empty := t.TempDir()
	if code := runInitStorage([]string{
		"--root", root, "--postgres-bin", empty,
		"--postgres-port", strconv.Itoa(freePort(t))}); code != 1 {
		t.Errorf("缺依赖 exit = %d, want 1", code)
	}
	if _, err := os.Stat(filepath.Join(root, "server")); !os.IsNotExist(err) {
		t.Errorf("校验失败却已经建了目录：%v", err)
	}
}
