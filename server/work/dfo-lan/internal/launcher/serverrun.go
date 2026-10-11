package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/accountname"
)

// This file is Stage 2 of the Python-orchestration migration: `dfolauncher launch
// --server-only` starts the game gateway itself.
//
// 它接管的是 channel_probe.py 从"起网关"到"写 run.json"这一段（L222-L607），以及
// launch_local.py 在它外面做的环境拼装与 7001 检查。Stage 3 的客户端拉起（probe.exe）
// 与 WFP 隔离复用同一个会话前半段：startSession 把 L222-L599 抽出来，LaunchServer 与
// LaunchClient（clientrun.go）各自接自己的后半段。

// readyPollInterval 是就绪轮询的间隔（channel_probe.py L597 的 time.sleep(0.05)）。
const readyPollInterval = 50 * time.Millisecond

// sessionTagName 是 launch_local.py L286-L290 拼出的 tag：固定前缀 + 微秒时间戳 + _next37。
// 微秒精度要单独拼：Go 布局里的 "000000" 是字面量，不是小数秒。
func sessionTagName(now time.Time) string {
	return launcherTagPrefix + now.Format("20060102_150405") + "_" + fmt.Sprintf("%06d", now.Nanosecond()/1000) + "_next37"
}

// sessionRun 是一次已经起好网关的会话。server-only / interactive / client-only 三种模式
// 从这里分叉：server-only 写两个键的 run.json 后等网关退出（channel_probe.py L600-L607），
// 其余写三个键的 run.json 再拉 probe（L608 之后，见 clientrun.go）。
type sessionRun struct {
	Session     sessionTag
	Env         *childEnv
	Child       *childProcess
	Address     string
	Port        int
	CommandLine string
	HelperOut   string
	HelperErr   string
}

// startSession 复刻 launch_local.py L228-L312 加上 channel_probe.py L222-L599：预检
// （与 --check 同一套）→ 7001 占用检查 → 起存储（SQLite 档无事可做）→ 写协议夹具 →
// 拼环境 → 构造并下发网关命令行 → 等 ready.json → 解析监听端口。
//
// --storage-only 在起网关之前返回 (nil, nil)（Python L283-L285）。
//
// console 收启动器自己要说的话（inner-PVF 结论、命令行与监听地址），与 Python 版 helper 的
// helper.out 对应；网关自己的 stdout/stderr 落在 runtime/<tag>/gateway.out / gateway.err。
func startSession(ctx context.Context, root string, opts LaunchOptions, console io.Writer) (*sessionRun, error) {
	if console == nil {
		console = io.Discard
	}
	module := filepath.Join(root, "server", "work", "dfo-lan")
	serverRoot := filepath.Join(root, "server")

	// 1. 预检。任何一项不过都不起进程，失败原因与 --check 完全一致。
	report, err := LaunchPlan(root, opts)
	if err != nil {
		return nil, err
	}
	if message := report.InnerPVF.Message; message != "" {
		fmt.Fprintln(console, message)
	}

	// 2. 已经有会话在 7001 上，就不要叠一个（launch_local.py L278-L279）。--storage-only
	//    不起网关，--client-only 连的是别的机器，Python 那一句把两者都排除在外。
	if !opts.StorageOnly && !opts.ClientOnly && PortListening(GatewayPort, portProbeTimeout) {
		return nil, fmt.Errorf("端口 %d 已在监听；请先检查现有会话再重试。", GatewayPort)
	}

	// 3. 存储。SQLite 是唯一引擎：它是个文件，引擎自己打开，没有服务要起。
	//    --client-only 不碰本机存储（Python L281）。
	storage, err := LoadStorageConfig(root)
	if err != nil {
		return nil, err
	}
	_ = storage
	if opts.StorageOnly {
		fmt.Fprintln(console, "Existing storage ready.")
		return nil, nil
	}

	// 4. 会话目录与协议夹具。目录用原始 tag，行为看降级后的 tag。
	tag := opts.Tag
	if tag == "" {
		tag = sessionTagName(time.Now())
	}
	session := newSessionTag(module, tag)
	if err := os.MkdirAll(session.Out, 0o755); err != nil {
		return nil, fmt.Errorf("创建会话目录失败：%w", err)
	}
	readyPath := filepath.Join(session.Out, "ready.json")
	// Python 的 out.mkdir(parents=True) 遇到已存在的目录会失败，所以它从不需要清理旧状态。
	// Go 允许 --tag 复用目录（对照测试要这么做），那就必须自己把上一次的 ready.json 删掉，
	// 否则轮询会立刻拿到过期地址。
	if err := os.Remove(readyPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("清理旧 ready.json 失败：%w", err)
	}
	if _, err := WriteSessionFixtures(module, session.Out, session.Effective); err != nil {
		return nil, fmt.Errorf("写协议夹具失败：%w", err)
	}

	// 5. 环境：launch_environment 的合并结果 + launch_local.py 追加的四个变量。
	env := newChildEnv(BuildServerEnv(CurrentEnv(), report.ProfileEnv, opts.JSONMode))
	env.Set("DFO_CLIENT_DIR", report.ClientDir)
	env.Set("DFO_SERVER_BINARY", report.Binary)
	if report.ChannelIdentity {
		env.Set("DFO_CHANNEL_IDENTITY", "1")
	} else {
		env.Set("DFO_CHANNEL_IDENTITY", "0")
	}
	env.Set("DFO_ENABLE_OBSERVER", "0")

	// 5.1 账号：会话登录用的开发账号名（口径见 applySessionAccount）。名字不合法在这里就失败，
	//     不等网关起来再报一条看不懂的启动错误；真正换了账号时打一行，让启动器日志里能看到
	//     这一场到底归属谁 —— 排查「建完号进不去游戏」时这一行是第一线索。
	account, err := applySessionAccount(env, opts)
	if err != nil {
		return nil, err
	}
	if account != accountname.Default {
		fmt.Fprintf(console, "本次会话账号：%s\n", account)
	}

	// 6. 命令行。exe -h 的能力探测按 Python 的做法跑两次（prune 一次、装备目录一次）。
	probe := func(exe string) map[string]bool { return ExeFlags(ctx, exe) }
	gateway, err := buildGatewayCommand(gatewayCommandInput{
		Project: module,
		Session: session,
		Env:     env,
		Probe:   probe,
	})
	if err != nil {
		return nil, err
	}
	if len(gateway.Args) == 0 {
		return nil, fmt.Errorf("内部错误：网关命令行是空的")
	}
	commandLine := strings.Join(gateway.Args, " ")
	helperOut := filepath.Join(session.Out, "helper.out")
	helperErr := filepath.Join(session.Out, "helper.err")
	// Python 把 prune 的 WARNING 打到 helper stdout、把角色目录告警打到 helper stderr。
	writeLogFile(helperOut, strings.Join(gateway.Notices, pythonTextNewline))
	writeLogFile(helperErr, strings.Join(gateway.Warnings, pythonTextNewline))
	for _, notice := range gateway.Notices {
		fmt.Fprintln(os.Stderr, notice)
	}
	for _, warning := range gateway.Warnings {
		fmt.Fprintln(os.Stderr, warning)
	}
	// 验收要求：完整命令行要打印出来，并且与 Python 写进 gateway.out 的那一行可比。
	fmt.Fprintln(console, commandLine)

	// 7. 起网关。stdout/stderr 分别落到 gateway.out / gateway.err，命令行先落盘并刷出，
	//    这样它一定是 gateway.out 的第一行（Python 走文本缓冲，靠的是运气）。
	gatewayOut, err := os.Create(filepath.Join(session.Out, "gateway.out"))
	if err != nil {
		return nil, fmt.Errorf("打开 gateway.out 失败：%w", err)
	}
	defer gatewayOut.Close()
	gatewayErr, err := os.Create(filepath.Join(session.Out, "gateway.err"))
	if err != nil {
		return nil, fmt.Errorf("打开 gateway.err 失败：%w", err)
	}
	defer gatewayErr.Close()
	if _, err := gatewayOut.WriteString(commandLine + pythonTextNewline); err != nil {
		return nil, fmt.Errorf("写 gateway.out 失败：%w", err)
	}
	if err := gatewayOut.Sync(); err != nil {
		return nil, fmt.Errorf("刷 gateway.out 失败：%w", err)
	}

	cmd := exec.Command(gateway.Args[0], gateway.Args[1:]...)
	// 网关的 cwd 是 server\（launch_local.py 给 helper 的 cwd，helper 起网关时没换）。
	// 出厂相对默认值因此解析不到，这正是上面所有配置文件都下发绝对路径的原因。
	cmd.Dir = serverRoot
	cmd.Env = env.List()
	cmd.Stdout = gatewayOut
	cmd.Stderr = gatewayErr
	hideConsoleWindow(cmd)
	child, err := startChild(cmd)
	if err != nil {
		return nil, fmt.Errorf("拉起网关失败：%w\n命令行：%s", err, commandLine)
	}

	// 8. 等 ready.json。直读 PVF 要先校验来源再开存储，上限比 JSON 档大（L589-L591）。
	address, err := awaitReady(readyPath, startupChecks(env.Get("DFO_PVF_CATALOGS")), child.exited, time.Sleep, gateway.Args)
	if err != nil {
		child.terminate(5 * time.Second)
		return nil, err
	}
	port, err := portFromAddress(address)
	if err != nil {
		child.terminate(5 * time.Second)
		return nil, err
	}
	return &sessionRun{
		Session:     session,
		Env:         env,
		Child:       child,
		Address:     address,
		Port:        port,
		CommandLine: commandLine,
		HelperOut:   helperOut,
		HelperErr:   helperErr,
	}, nil
}

// LaunchServer 执行 `launch --server-only` 的真实启动：startSession 起好网关之后，
// 写两个键的 run.json、打印监听地址，然后等网关退出（channel_probe.py L600-L607）。
func LaunchServer(ctx context.Context, root string, opts LaunchOptions, console io.Writer) error {
	if console == nil {
		console = io.Discard
	}
	// 这个入口只做 server-only / storage-only：--check/--dry-run 走 LaunchPlan，客户端侧是
	// Stage 3 的 LaunchClient。storage-only 不设置 ServerOnly，好让预检的依赖清单与
	// launch_local.py 的 --storage-only 一致（那一条分支要 probe.exe 与客户端三件套）；
	// server-only 才裁到 [网关程序]。
	if !opts.StorageOnly {
		opts.ServerOnly = true
	}
	opts.ClientOnly = false
	opts.Check = false
	opts.DryRun = false

	run, err := startSession(ctx, root, opts, console)
	if err != nil {
		return err
	}
	if run == nil {
		// --storage-only：Python 同样在这里结束（L283-L285）。
		return nil
	}
	child := run.Child

	// 9. run.json 与监听行。键名、键序、空格风格与 json.dumps 默认风格一致。
	if err := os.WriteFile(filepath.Join(run.Session.Out, "run.json"),
		[]byte(runJSON(child.pid(), run.Port)), 0o644); err != nil {
		child.terminate(5 * time.Second)
		return fmt.Errorf("写 run.json 失败：%w", err)
	}
	listening := fmt.Sprintf("Server listening on 127.0.0.1:%d and %s", GatewayPort, run.Address)
	fmt.Fprintln(console, listening)
	appendLogFile(run.HelperOut, listening)

	// launch_local.py 在 --server-only 下还会补这两行（客户端模式那边由 LaunchClient 打印同格式）。
	// 尤其是 `Logs: <目录>`：启动器就是靠它定位本次会话目录的；缺了它，那边只能用"runtime 下最新的
	// roles_*"来推定（能跑，但日志措辞绕、也可能在并发会话时指错目录）。2026-10-05 跨仓库对齐。
	if pid := child.pid(); pid > 0 {
		started := fmt.Sprintf("Game server started successfully (PID: %d). Port %d is active.", pid, GatewayPort)
		fmt.Fprintln(console, started)
		appendLogFile(run.HelperOut, started)
	}
	logsLine := "Logs: " + run.Session.Out
	fmt.Fprintln(console, logsLine)
	appendLogFile(run.HelperOut, logsLine)

	// 10. 等网关退出。Ctrl+C 与 Python 的 KeyboardInterrupt 一样：不再等，按 0 收场
	//     （控制台的 CTRL_C_EVENT 本来就会发给同组的网关，不需要我们代它自杀）。
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)
	defer signal.Stop(signals)
	select {
	case <-signals:
		return nil
	case <-ctx.Done():
		child.terminate(5 * time.Second)
		return ctx.Err()
	case err := <-child.done:
		child.reapWith(err)
		return nil
	}
}

// commandLine0 是命令行里的可执行文件（空命令行时给出明确交代，而不是越界 panic）。
func commandLine0(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// childProcess 是一个已经拉起的子进程：Wait 由后台 goroutine 负责，因此轮询可以随时问
// "退出没有"而不阻塞，最终收尸也只会发生一次。
type childProcess struct {
	cmd    *exec.Cmd
	done   chan error
	waited bool
	err    error
}

// startChild 拉起进程并开始收尸。
func startChild(cmd *exec.Cmd) (*childProcess, error) {
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	child := &childProcess{cmd: cmd, done: make(chan error, 1)}
	go func() { child.done <- cmd.Wait() }()
	return child, nil
}

// pid 是 run.json 里的 server_pid。
func (c *childProcess) pid() int { return c.cmd.Process.Pid }

// exited 对应 Python 的 server.poll() is not None：非阻塞地问一次。
func (c *childProcess) exited() bool {
	if c.waited {
		return true
	}
	select {
	case err := <-c.done:
		c.err = err
		c.waited = true
		return true
	default:
		return false
	}
}

// reap 收走已结束进程的状态（幂等）。
func (c *childProcess) reap() {
	if c.waited {
		return
	}
	c.err = <-c.done
	c.waited = true
}

// waitTimeout 对应 Python 的 probe.wait(timeout=...)：超时返回 false，进程仍在跑（还没收尸）。
// Python 的 subprocess.TimeoutExpired 不会杀子进程（channel_probe.py 的 finally 只收网关），
// 所以这里也不杀 —— 语义与 Python 保持一致，由调用方决定怎么报。
func (c *childProcess) waitTimeout(timeout time.Duration) bool {
	if c.waited {
		return true
	}
	select {
	case err := <-c.done:
		c.err = err
		c.waited = true
		return true
	case <-time.After(timeout):
		return false
	}
}

// exitCode 是 Python 的 probe.returncode：正常退出就是退出码，拿不到状态（被信号等）按
// Python 的负值口径给 -1。
func (c *childProcess) exitCode() int {
	if c.err == nil {
		return 0
	}
	var exit *exec.ExitError
	if errors.As(c.err, &exit) {
		return exit.ExitCode()
	}
	return -1
}

// reapWith 记录后台 Wait 的结果，用于 done 已经被本 goroutine 取走的情形。
func (c *childProcess) reapWith(err error) {
	if c.waited {
		return
	}
	c.err = err
	c.waited = true
}

// terminate 对应 Python finally 里的 server.terminate(); server.wait(timeout=5)。
func (c *childProcess) terminate(timeout time.Duration) {
	if c.waited {
		return
	}
	_ = c.cmd.Process.Kill()
	finished := make(chan struct{})
	go func() {
		c.reap()
		close(finished)
	}()
	select {
	case <-finished:
	case <-time.After(timeout):
	}
}

// startupChecks 复刻 channel_probe.py L589-L591 的轮询上限：直读 PVF 要先校验来源再开
// 存储，导入阶段更长（3600 × 50ms = 180s），JSON 档保持 1000 × 50ms = 50s。
func startupChecks(catalogs string) int {
	if catalogs != "" {
		return 3600
	}
	return 1000
}

// awaitReady 复刻 channel_probe.py L588-L599 的就绪轮询：先看进程死没死，再看 ready.json
// 在不在，然后睡 50ms。轮询用尽后 Python 会直接 read_text()（不在就抛异常），这里同样再读
// 一次。两种情况都把完整命令行带进错误信息里 —— 这是现场唯一能分辨"参数错了"的证据。
func awaitReady(readyPath string, checks int, exited func() bool, sleep func(time.Duration), command []string) (string, error) {
	commandLine := strings.Join(command, " ")
	for attempt := 0; attempt < checks; attempt++ {
		if exited() {
			return "", fmt.Errorf(
				"网关进程在写出 ready.json 之前就退出了（退出即失败，请查 gateway.err）。\n命令行：%s", commandLine)
		}
		if address, ok := readReadyAddress(readyPath); ok {
			return address, nil
		}
		sleep(readyPollInterval)
	}
	if address, ok := readReadyAddress(readyPath); ok {
		return address, nil
	}
	return "", fmt.Errorf("等待网关就绪超时（%d 次 × %s）。\n命令行：%s", checks, readyPollInterval, commandLine)
}

// readReadyAddress 读网关写的 ready.json，只取 address。文件还没写全（或不是合法 JSON）
// 时按"尚未就绪"处理，继续轮询。
func readReadyAddress(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	var state struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(stripBOM(data), &state); err != nil {
		return "", false
	}
	if state.Address == "" {
		return "", false
	}
	return state.Address, true
}

// portFromAddress 复刻 int(state["address"].split(":")[-1])。
func portFromAddress(address string) (int, error) {
	at := strings.LastIndex(address, ":")
	if at < 0 {
		return 0, fmt.Errorf("ready.json 的 address 里没有端口：%q", address)
	}
	port, err := strconv.Atoi(address[at+1:])
	if err != nil {
		return 0, fmt.Errorf("ready.json 的 address 端口无法解析：%q", address)
	}
	return port, nil
}

// runJSON 复刻 json.dumps({"server_pid": pid, "port": port})：默认分隔符是 `": "` / `", "`，
// 键序固定，单行无换行。run.json 是要被别的进程读的接口，风格必须与 Python 版一致。
func runJSON(pid, port int) string {
	return fmt.Sprintf(`{"server_pid": %d, "port": %d}`, pid, port)
}

// writeLogFile 覆盖写一个日志文件，失败不拦启动（日志不是启动条件）。
func writeLogFile(path, body string) {
	_ = os.WriteFile(path, []byte(body), 0o644)
}

// appendLogFile 追加一行日志。
func appendLogFile(path, line string) {
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer file.Close()
	_, _ = file.WriteString(line + pythonTextNewline)
}
