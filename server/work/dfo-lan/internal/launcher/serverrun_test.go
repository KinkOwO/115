package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// run.json 是要被别的进程按固定键名读走的接口文件，空格风格与键序都必须与
// json.dumps({"server_pid": pid, "port": port}) 一致，且不能有结尾换行。
func TestRunJSONMatchesJsonDumps(t *testing.T) {
	if got := runJSON(2404, 54201); got != historicalRunJSON {
		t.Errorf("run.json = %s\nwant %s", got, historicalRunJSON)
	}
	parsed := map[string]int{}
	if err := json.Unmarshal([]byte(runJSON(1, 2)), &parsed); err != nil {
		t.Fatalf("run.json 不是合法 JSON：%v", err)
	}
	if parsed["server_pid"] != 1 || parsed["port"] != 2 {
		t.Errorf("键名不对：%v", parsed)
	}
	if strings.Contains(runJSON(1, 2), "\n") {
		t.Error("run.json 不该有换行")
	}
}

// address 里的端口取最后一段（int(state["address"].split(":")[-1])）。
func TestPortFromAddress(t *testing.T) {
	cases := []struct {
		address string
		port    int
	}{
		{"127.0.0.2:54201", 54201},
		{"0.0.0.0:7001", 7001},
		{"[::1]:7001", 7001},
	}
	for _, testCase := range cases {
		got, err := portFromAddress(testCase.address)
		if err != nil {
			t.Fatalf("%s: %v", testCase.address, err)
		}
		if got != testCase.port {
			t.Errorf("portFromAddress(%q) = %d, want %d", testCase.address, got, testCase.port)
		}
	}
	for _, bad := range []string{"127.0.0.2", "127.0.0.2:port"} {
		if _, err := portFromAddress(bad); err == nil {
			t.Errorf("portFromAddress(%q) 应当报错", bad)
		}
	}
}

// 网关刚写完一半的 ready.json 不算就绪：继续轮询，而不是拿半截 JSON 去解析。
func TestReadReadyAddressIgnoresIncompleteFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ready.json")
	if _, ok := readReadyAddress(path); ok {
		t.Error("文件不存在时不该算就绪")
	}
	if err := os.WriteFile(path, []byte(`{"address": "127.0.0.2:54`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readReadyAddress(path); ok {
		t.Error("截断的 JSON 不该算就绪")
	}
	if err := os.WriteFile(path, []byte(`{"address": ""}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := readReadyAddress(path); ok {
		t.Error("空 address 不该算就绪")
	}
	if err := os.WriteFile(path, []byte(`{"address":"127.0.0.2:54201","pid":9}`), 0o644); err != nil {
		t.Fatal(err)
	}
	address, ok := readReadyAddress(path)
	if !ok || address != "127.0.0.2:54201" {
		t.Errorf("ready = %q %v", address, ok)
	}
}

// 就绪轮询的三条分支：先轮询后成功、进程先退出、轮询用尽超时。两种失败都必须把完整
// 命令行带进错误里 —— 现场只有它能分辨"参数错了"还是"起得慢"。
func TestAwaitReadyBranches(t *testing.T) {
	dir := t.TempDir()
	readyPath := filepath.Join(dir, "ready.json")
	command := []string{`C:\bin\wireprobe-pvf.exe`, "-fixture", `C:\out\channelinfo.bin`}
	noSleep := func(time.Duration) {}

	// 第 3 次轮询时 ready.json 出现。
	attempts := 0
	sleep := func(time.Duration) {
		attempts++
		if attempts == 3 {
			if err := os.WriteFile(readyPath, []byte(`{"address":"127.0.0.2:50001"}`), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	address, err := awaitReady(readyPath, 100, func() bool { return false }, sleep, command)
	if err != nil {
		t.Fatalf("awaitReady: %v", err)
	}
	if address != "127.0.0.2:50001" {
		t.Errorf("address = %q", address)
	}

	// 进程先退出：立刻失败，且错误里带完整命令行。
	if err := os.Remove(readyPath); err != nil {
		t.Fatal(err)
	}
	_, err = awaitReady(readyPath, 100, func() bool { return true }, noSleep, command)
	if err == nil || !strings.Contains(err.Error(), strings.Join(command, " ")) {
		t.Errorf("退出分支错误信息 = %v", err)
	}

	// 轮询用尽：超时，同样带命令行，且只睡了 checks 次。
	slept := 0
	_, err = awaitReady(readyPath, 4, func() bool { return false }, func(time.Duration) { slept++ }, command)
	if err == nil || !strings.Contains(err.Error(), "超时") ||
		!strings.Contains(err.Error(), strings.Join(command, " ")) {
		t.Errorf("超时分支错误信息 = %v", err)
	}
	if slept != 4 {
		t.Errorf("轮询次数 = %d, want 4", slept)
	}

	// Python 在循环用尽后仍会读一次文件：这里也要认。
	if err := os.WriteFile(readyPath, []byte(`{"address":"127.0.0.2:1"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if address, err = awaitReady(readyPath, 4, func() bool { return false }, noSleep, command); err != nil || address != "127.0.0.2:1" {
		t.Errorf("末尾补读失败：%q %v", address, err)
	}
}

// 直读 PVF 要先校验来源再开存储，轮询上限比 JSON 档大（3600 vs 1000 次，间隔 50ms）。
func TestStartupCheckBudget(t *testing.T) {
	if got := startupChecks(`world,quests`); got != 3600 {
		t.Errorf("PVF 档上限 = %d, want 3600", got)
	}
	if got := startupChecks(""); got != 1000 {
		t.Errorf("JSON 档上限 = %d, want 1000", got)
	}
	if readyPollInterval != 50*time.Millisecond {
		t.Errorf("轮询间隔 = %s, want 50ms", readyPollInterval)
	}
}

// LaunchServer 在预检阶段就该失败：这里的能力探测拿不到 pvf-catalogs（夹具里的"二进制"
// 根本不是可执行文件），于是按 launch_local.py 的口径拒绝启动，不进到起进程那一步。
func TestLaunchServerRefusesWhenTheProbeFails(t *testing.T) {
	root := buildLaunchTree(t)
	err := LaunchServer(t.Context(), root, LaunchOptions{ServerOnly: true}, nil)
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), "已在监听") {
		// 这台机器上真有会话在跑：预检失败的原因就被 7001 占用挡在前面，换个时间再验。
		t.Skipf("7001 已被占用，本次跳过：%v", err)
	}
	if !strings.Contains(err.Error(), "PVF") {
		t.Errorf("err = %v", err)
	}
}
