package main

import "testing"

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
