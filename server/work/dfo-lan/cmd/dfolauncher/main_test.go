package main

import (
	"os"
	"path/filepath"
	"testing"
)

// 交互式（要拉起客户端）属于 Stage 3：必须明确报未实现并以 1 退出，不能静默做半套。
// 这里用 --root 指到临时目录，目的是证明它在碰任何环境之前就返回了。
func TestRunLaunchRefusesInteractiveUntilStage3(t *testing.T) {
	if code := runLaunch([]string{"--root", t.TempDir()}); code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
}

// --json-mode 与 --repair-profile 互斥（launch_local.py 的 argparse 也是互斥组）。
func TestRunLaunchRefusesMutuallyExclusiveDataModes(t *testing.T) {
	args := []string{"--check", "--json-mode", "--repair-profile", "configs/pvf-default.json"}
	if code := runLaunch(args); code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

// prepare-inner-pvf（Stage 4）的退出码：参数错 2、没有可用的客户端配置 1、
// --dry-run 判定完成 0（且一个字节都不写）。
func TestRunPrepareInnerPVFExitCodes(t *testing.T) {
	if code := runPrepareInnerPVF([]string{"--nope"}); code != 2 {
		t.Errorf("未知开关 exit = %d, want 2", code)
	}

	// 空目录里既没有 launcher.local.json 也没有 client_dir。
	empty := t.TempDir()
	if code := runPrepareInnerPVF([]string{"--root", empty}); code != 1 {
		t.Errorf("无配置 exit = %d, want 1", code)
	}

	// 有配置、有客户端三件套，但产物还没有：--dry-run 只报告。
	root := t.TempDir()
	client := filepath.Join(root, "client")
	if err := os.MkdirAll(client, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"DFO.exe", "sk.dat", "Script.pvf"} {
		if err := os.WriteFile(filepath.Join(client, name), []byte("stub"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(root, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "server", "launcher.local.json"),
		[]byte(`{"client_dir":"../client"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if code := runPrepareInnerPVF([]string{"--root", root, "--dry-run"}); code != 0 {
		t.Errorf("dry-run exit = %d, want 0", code)
	}
	// --dry-run 不写盘：连产物目录都不该建。
	if _, err := os.Stat(filepath.Join(root, "server", "work", "client-build")); !os.IsNotExist(err) {
		t.Errorf("--dry-run 建了产物目录：%v", err)
	}

	// 同一个树，不给 --dry-run：stub 客户端不是 PE，必须失败并给出中文原因（exit 1），
	// 而且失败不能留下半截产物。
	if code := runPrepareInnerPVF([]string{"--root", root, "--client", client}); code != 1 {
		t.Errorf("坏客户端 exit = %d, want 1", code)
	}
	clientBuild := filepath.Join(root, "server", "work", "client-build")
	if entries, err := os.ReadDir(clientBuild); err == nil {
		for _, entry := range entries {
			t.Errorf("失败的生成留下了残留物：%s", entry.Name())
		}
	}
}
