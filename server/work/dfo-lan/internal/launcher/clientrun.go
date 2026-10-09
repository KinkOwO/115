package launcher

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// This file is Stage 3 of the Python-orchestration migration
// (docs/go-launch-migration-plan.md): the client half of channel_probe.py, L608-L721.
//
// 覆盖范围与口径：
//   - payload 选择（L608-L612）、probe.exe 的目录可见性与 argv（L613-L650）；
//   - run.json / probe.json（键序、json.dumps 默认分隔符、单行无换行）；
//   - probe 的等待语义（interactive / exception-trace 不设超时，其余 65s）与退出码告警
//     （L673-L694）；
//   - finally 里的网关 terminate（L697-L700）；
//   - 客户端 trace 的解码 / 脱敏 / 过滤 / 落盘（L701-L720）与收尾的 out 绝对路径（L721）。
//
// 唯一不实现的是 DFO_ENABLE_OBSERVER 观察者分支（L654-L672），理由见 LaunchClient 里那段注释。
//
// 与 Python 的进程结构差异：Python 是「launcher → helper → 网关 / probe」四个进程，
// 这里只有「启动器 → 网关 / probe」三个，helper 由本文件顶上。因此 helper 的每一行输出
// 去哪必须逐条对齐（Python 的 helper stdout 是 helper.out，helper stderr 是 helper.err），
// 而 channel_probe.py 里那个 `stdout` 变量其实指向 gateway.out（L222-L225）—— probe 的
// 命令行是写进 gateway.out 的，不是 helper.out。为了让验收能直接看，helper 侧的行同时也
// 回显到 console（Stage 2 对网关命令行、prune 告警就是这么做的）。

// helperMode 是 launch_local.py L298 传给 helper 的模式字符串。channel_probe.py 只认其中
// 四个（interactive / server-only / exception-trace / channel-check），client-only 与任何
// 别的名字都不匹配，于是四个开关全为假。这一条最容易看错：从启动器出发的 --client-only
// 仍然会起一个本机网关，交互关闭、65 秒超时、ui-mode 落到 trace-root-ui。这里照搬。
type helperMode string

const (
	modeInteractive    helperMode = "interactive"
	modeServerOnly     helperMode = "server-only"
	modeClientOnly     helperMode = "client-only"
	modeExceptionTrace helperMode = "exception-trace"
	modeChannelCheck   helperMode = "channel-check"
)

// launchMode 复刻 launch_local.py L298：能从命令行到达的只有三种模式
// （exception-trace / channel-check 是历史直调用法，启动器到不了，但 probe 的模式表要完整）。
func launchMode(opts LaunchOptions) helperMode {
	switch {
	case opts.ServerOnly:
		return modeServerOnly
	case opts.ClientOnly:
		return modeClientOnly
	default:
		return modeInteractive
	}
}

// channelCheck / exceptionTrace / interactive / serverOnly 复刻 channel_probe.py L16-L19。
func (m helperMode) channelCheck() bool   { return m == modeChannelCheck }
func (m helperMode) exceptionTrace() bool { return m == modeExceptionTrace }
func (m helperMode) interactive() bool    { return m == modeInteractive }
func (m helperMode) serverOnly() bool     { return m == modeServerOnly }

// probeWaitTimeout 复刻 channel_probe.py L673：只有 interactive / exception-trace 不设超时
// （客户端开着就得一直等），其余模式 65 秒。第二个返回值表示"有没有上限"。
func (m helperMode) probeWaitTimeout() (time.Duration, bool) {
	if m.interactive() || m.exceptionTrace() {
		return 0, false
	}
	return 65 * time.Second, true
}

// probeUIMode 复刻 channel_probe.py L623-L629（与 L639-L645 是同一段嵌套条件）。
func probeUIMode(mode helperMode) string {
	switch {
	case mode.channelCheck():
		return "normal-ui"
	case mode.exceptionTrace():
		return "trace-owned-ui"
	case mode.interactive():
		return "interactive-ui"
	default:
		return "trace-root-ui"
	}
}

// probePayload 复刻 channel_probe.py L608-L612。tag 是**降级后**的 tag：_next35/_next36/
// _next37 会先降到 _next34，所以它们命中的也是同一支（7001 形态）。
func probePayload(port int, channelCheck bool, tag string) string {
	if channelCheck || hasAnySuffix(tag, "_next30", "_next31", "_next32", "_next33", "_next34") {
		// Python 这里写死 7001，用的不是 ready.json 里的端口。
		return fmt.Sprintf("3?127.0.0.1?%d?probe?00000000000000000000000000000000?0?0?30?0?0?0", GatewayPort)
	}
	return fmt.Sprintf("13?127.0.0.1?%d?probe?00000000000000000000000000000000?0?0?30?0?0?0", port)
}

// probeClientDir 复刻 channel_probe.py L615：环境变量优先，**键缺失**时才退回 probe 工具
// 目录下的 dfo_probe_client（Python 用的是 os.environ.get 的默认值，"有键但为空"不走默认值）。
func probeClientDir(env *childEnv, probeDir string) string {
	if env != nil && env.Has("DFO_CLIENT_DIR") {
		return env.Get("DFO_CLIENT_DIR")
	}
	return filepath.Join(probeDir, "dfo_probe_client")
}

// probeArgv 复刻 channel_probe.py L633-L650 的参数序，与 L618-L632 打印出来的完全一致。
func probeArgv(probeExe, clientDir, out, uiMode, payload string) []string {
	return []string{
		probeExe,
		clientDir,
		filepath.Join(out, "client.log"),
		"55",
		uiMode,
		filepath.Join(out, "breakpoints.txt"),
		payload,
	}
}

// sessionRunJSON 复刻 json.dumps({"server_pid":…, "probe_pid":…, "port":…})：默认分隔符是
// `": "` / `", "`，键序固定，单行无换行。run.json 是启动器与别的进程读的接口，风格必须一致。
func sessionRunJSON(serverPID, probePID, port int) string {
	return fmt.Sprintf(`{"server_pid": %d, "probe_pid": %d, "port": %d}`, serverPID, probePID, port)
}

// probeReportJSON 复刻 json.dumps({"probe_pid":…, "probe_returncode":…, "client_dir":…, "payload":…})。
func probeReportJSON(pid, returnCode int, clientDir, payload string) string {
	return fmt.Sprintf(`{"probe_pid": %d, "probe_returncode": %d, "client_dir": %s, "payload": %s}`,
		pid, returnCode, pythonJSONString(clientDir), pythonJSONString(payload))
}

// probeExitLines 复刻 channel_probe.py L687-L694 的打印：先一行退出码，非 0 再补两行。
// 原文是三个相邻字符串字面量隐式拼接，空格个数照抄（"client.log; on real machines"）。
func probeExitLines(code int, clientDir string) []string {
	lines := []string{fmt.Sprintf("probe.exe exited with code %d", code)}
	if code != 0 {
		lines = append(lines,
			"WARNING: the game client was not launched correctly.",
			"  client dir passed to probe: "+clientDir+
				"  (return code 3 = probe could not see DFO.exe there and it exits before writing client.log;"+
				" on real machines this usually means security software blocked probe.exe)")
	}
	return lines
}

// probeFallbackEnvKey 是"强制走 probe.exe"的开关：验收要能确定地走回退路径
// （probe.exe 的 WFP 隔离需要管理员权限，Go 版能装上的机器上没法靠权限逼出回退）。
const probeFallbackEnvKey = "DFO_FORCE_PROBE_EXE"

// requireGoIsolationEnvKey 是"不许回退"的开关：置 1 时 Go 隔离装不上就直接失败，
// 而不是回退 probe.exe。涉及网络隔离时这是最严的口径（业主可以据此起一套只认 Go 隔离
// 的环境），默认不设，保持 probe.exe 作为回退。
const requireGoIsolationEnvKey = "DFO_REQUIRE_GO_ISOLATION"

// forceProbeFallback 报告这次是否被显式要求走 probe.exe。
func forceProbeFallback(env *childEnv) bool {
	if env == nil || !env.Has(probeFallbackEnvKey) {
		return false
	}
	value := strings.TrimSpace(env.Get(probeFallbackEnvKey))
	return value != "" && value != "0"
}

// requireGoIsolation 报告这次是否禁止回退。
func requireGoIsolation(env *childEnv) bool {
	if env == nil || !env.Has(requireGoIsolationEnvKey) {
		return false
	}
	value := strings.TrimSpace(env.Get(requireGoIsolationEnvKey))
	return value != "" && value != "0"
}

// probeClientDirWarning 是 channel_probe.py L616-L617 那行：probe 在写 client.log 之前就
// 退出（返回码 3）时，这行是分辨"目录给错了"还是"被安全软件拦了"的唯一线索。
func probeClientDirWarning(clientDir string) string {
	if regularFile(filepath.Join(clientDir, "DFO.exe")) {
		return ""
	}
	return "WARNING: probe cannot see DFO.exe under client dir: " + clientDir
}

// pythonJSONString 用的是 Stage 2 fixture.go 里那一份（json.dumps ensure_ascii=True 的
// 口径）：client_dir 可能是中文路径，probe.json 必须与 Python 逐字节一致。

// LaunchClient 执行 interactive / --client-only 的真实启动（Stage 3）：先走与 --server-only
// 完全相同的服务端半段（预检 → 存储 → 夹具 → 网关 → ready.json），再把客户端段接上去。
//
// 输出流的分工与 Python 一致：
//   - helper 自己的 print → helper.out（probe.exe 的 stdout/stderr 也接到 helper.out /
//     helper.err 上，Python 里它们继承的就是这两个句柄）；
//   - channel_probe.py 的 `stdout` 变量是 gateway.out，所以 probe 的命令行走那里；
//   - "Client launch requested..." 与 "Logs:" 是 launch_local.py 在 run.json 出现后打的，
//     属于启动器自己的话，只进控制台，且必须在等 probe 之前出现（现有解析靠它判成功）。
//
// 额外的可见性：probe 命令行、客户端目录告警、退出码与告警、收尾的 out 路径这几行同时回显
// 到控制台（Stage 2 对网关命令行与 prune 告警就是这么做的），本机验收可以直接对比；成块的
// 客户端 trace 只落 helper.out 与 client_trace.txt，不灌进启动器日志。
func LaunchClient(ctx context.Context, root string, opts LaunchOptions, console io.Writer) error {
	if console == nil {
		console = io.Discard
	}
	if opts.ServerOnly || opts.StorageOnly {
		return fmt.Errorf("内部错误：LaunchClient 只处理 interactive / --client-only")
	}
	// Validate the opt-in injection before starting any service or client.
	featureEnv := newChildEnv(BuildServerEnv(CurrentEnv(), nil, false))
	eliteDLL, err := adventureEliteDLL(root, featureEnv)
	if err != nil {
		return err
	}
	opts.Check = false
	opts.DryRun = false

	run, err := startSession(ctx, root, opts, console)
	if err != nil {
		return err
	}
	if run == nil {
		// startSession 只在 --storage-only 时返回 nil，而上面已经把它挡掉了。
		return nil
	}

	probeDir := filepath.Join(root, "server", "work", "dfo_probe_tools")
	mode := launchMode(opts)
	out := run.Session.Out
	helperOut := run.HelperOut
	helperErr := run.HelperErr

	// Python 的 finally 在 try 的任何出口都会收掉网关，这里用同一个兜底：任何提前返回都
	// 先 terminate 再退出（terminate 幂等，成功路径上在 trace 之前显式调一次）。
	gatewayStopped := false
	stopGateway := func() {
		if gatewayStopped {
			return
		}
		gatewayStopped = true
		run.Child.terminate(5 * time.Second)
	}
	defer stopGateway()

	// 1. payload（channel_probe.py L608-L612）。
	payload := probePayload(run.Port, mode.channelCheck(), run.Session.Effective)

	// 2. 客户端目录与其可见性告警（L613-L617）。
	clientDir := probeClientDir(run.Env, probeDir)
	if warning := probeClientDirWarning(clientDir); warning != "" {
		writeHelperLine(console, helperOut, warning)
	}

	// 3. probe 命令行（L618-L632）→ gateway.out，与下面 Popen 用的是同一个列表。
	probeExe := filepath.Join(probeDir, "probe.exe")
	argv := probeArgv(probeExe, clientDir, out, probeUIMode(mode), payload)
	commandLine := strings.Join(argv, " ")
	appendLogFile(filepath.Join(out, "gateway.out"), commandLine)
	fmt.Fprintln(console, commandLine)

	// 4. 拉起客户端。**首选 Go 隔离 + Go 宿主**（Stage 3 的最后一块，见 clienthost.go）；
	//    隔离装不上 / 非 Windows / 被显式要求回退时走 probe.exe，那条路一行没改。
	probeOut, err := os.OpenFile(helperOut, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开 helper.out 失败：%w", err)
	}
	defer probeOut.Close()
	probeErr, err := os.OpenFile(helperErr, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("打开 helper.err 失败：%w", err)
	}
	defer probeErr.Close()

	// 客户端进程的 pid 与退出码在两条路上是同一个含义（run.json / probe.json 读它）。
	var clientPID, clientExit int
	// "谁拉客户端"的分流（含汉化包装带来的硬约束，见 clientwrapper.go）：有包装变量时**必须**
	// 走 Go 宿主 —— probe.exe 是原生程序，没有"把被拉起的程序换成汉化宿主"这种能力。
	route, err := planClientLaunch(run.Env)
	if err != nil {
		return err
	}
	eliteDLL, err = adventureEliteDLL(root, run.Env)
	if err != nil {
		return err
	}
	wrapper := route.Wrapper
	// tryGoHost：只有 DFO_FORCE_PROBE_EXE 会在这里直接挡掉 Go 路径（它和包装变量同时出现时
	// planClientLaunch 已经报错了）；能不能装隔离要等 startHostedClient 试过才知道
	// （下面按"有没有装上"分流）。
	tryGoHost := !route.UseProbe
	var hostAttempt *HostedClient
	var hostErr error
	var hostCode int
	if tryGoHost {
		hostAttempt, hostCode, hostErr = startHostedClient(ClientHostOptions{
			ClientDir:         clientDir,
			LogPath:           filepath.Join(out, "client.log"),
			Seconds:           55,
			UIMode:            probeUIMode(mode),
			BreakpointsFile:   filepath.Join(out, "breakpoints.txt"),
			Args:              []string{payload},
			Env:               run.Env.List(),
			AdventureEliteDLL: eliteDLL,
			Console:           probeOut,
			Wrapper:           wrapper.Host,
			WrapperDLL:        wrapper.DLL,
			ErrorLog:          probeErr,
		}, true)
		if hostErr == nil {
			// 客户端已经退出、Job 已收、隔离已拆。pid 与退出码照实上报。
			clientPID, clientExit = hostAttempt.Pid, hostAttempt.ExitCode
			writeHelperLine(console, helperOut, wfpIsolationLine(hostAttempt))
		} else if hostCode != exitClientHostMissingExe && hostAttempt != nil && hostAttempt.Isolated {
			// 隔离装上了却起不了客户端：**不**回退 probe.exe（那会叠第二份隔离），
			// 如实报错，让调用方看见。启用汉化时还要点明"这一档必须经汉化启动宿主"，
			// 否则玩家看到的只是一句隔离相关的失败，不知道汉化也在其中。
			if wrapper.Enabled() {
				return fmt.Errorf("Go 客户端宿主失败（已装的 WFP 隔离已拆除；这次启用汉化，客户端必须由汉化启动宿主拉起）：%w\n"+
					"汉化宿主：%s\n客户端目录：%s", hostErr, wrapper.Host, clientDir)
			}
			return fmt.Errorf("Go 客户端宿主失败（已装的 WFP 隔离已拆除）：%w\n客户端目录：%s",
				hostErr, clientDir)
		}
	}
	if clientPID == 0 && clientExit != exitClientHostMissingExe {
		if eliteDLL != "" {
			return fmt.Errorf("精锐资格已启用，Go 宿主失败，不能回退到不注入的 probe.exe：%w", hostErr)
		}
		if wrapper.Enabled() {
			// 启用汉化时只有"Go 宿主 + 汉化启动宿主"这一条路（probe.exe 注入不了 dll），
			// 所以这里**不**回退：回退只会静默给出一个没汉化的客户端，而那正是本次要消灭的现象。
			return wrapperHostFailure(wrapper, clientDir, hostErr)
		}
		// 走到这里有两种情形，处置相同：回退 probe.exe。
		//   1. 本机装不上 Go 隔离（非 Windows / 缺 API / 无管理员权限 / 装过滤器失败）；
		//   2. 被 DFO_FORCE_PROBE_EXE 显式要求回退（验收与排障用）。
		if requireGoIsolation(run.Env) && !forceProbeFallback(run.Env) {
			// 最严口径：不许回退。装不上隔离就是启动失败，绝不静默放行。
			return fmt.Errorf("DFO_REQUIRE_GO_ISOLATION=1 且 Go 隔离不可用，拒绝以无隔离方式启动客户端："+
				"%v\n客户端目录：%s", hostErr, clientDir)
		}
		if !forceProbeFallback(run.Env) {
			reason := "Go 隔离不可用"
			if hostErr != nil {
				reason = hostErr.Error()
			}
			writeHelperLine(console, helperOut, "WFP 隔离：回退 probe.exe（"+reason+"）")
		} else {
			writeHelperLine(console, helperOut, "WFP 隔离：回退 probe.exe（"+wfpProbeIsolationReason(run.Env)+"）")
		}
		pid, code, fallbackErr := runProbeHost(run, root, argv, probeOut, probeErr, mode)
		if fallbackErr != nil {
			return fallbackErr
		}
		clientPID, clientExit = pid, code
	} else if clientExit == exitClientHostMissingExe {
		// Go 宿主在写 client.log 之前就发现看不到 DFO.exe（与 probe.exe 的返回码 3 同一语义）。
		// 这一步不是"隔离装不上"，所以不叠第二份隔离，也不回退 probe.exe。
		writeHelperLine(console, helperOut,
			"WFP 隔离：Go 版；宿主看不到 DFO.exe，按 probe.exe 的返回码 3 收场")
		clientPID, clientExit = 0, exitClientHostMissingExe
	}

	// 5. run.json 这次多一个 probe_pid（L651-L653）。Go 宿主跑在启动器进程里，
	//    probe_pid 就是客户端进程自己（probe.exe 那一路是 probe.exe 的 pid）；经汉化启动宿主
	//    拉起时它同样是客户端自己 —— 那个 pid 由宿主报回来（见 clienthost.go 的 WRAPPER_READY）。
	if err := os.WriteFile(filepath.Join(out, "run.json"),
		[]byte(sessionRunJSON(run.Child.pid(), clientPID, run.Port)), 0o644); err != nil {
		return fmt.Errorf("写 run.json 失败：%w", err)
	}
	// launch_local.py 一看到 run.json 就返回成功（L315-L325），这两行必须紧跟着出现。
	fmt.Fprintln(console, "Client launch requested. UI/gameplay acceptance remains separate.")
	fmt.Fprintln(console, "Logs: "+out)

	// DFO_ENABLE_OBSERVER 观察者分支（channel_probe.py L654-L672）本阶段不实现：那一步是
	// `[sys.executable, watch_monster_stats3x.py, out]` —— 观察者本身是 Python 脚本，正是本
	// 迁移要去掉的东西；而且 launch_local.py L297 固定注入 DFO_ENABLE_OBSERVER=0，从启动器
	// 出发根本不可达。真需要观察者时应当是独立决定（Go 版或保留 Python 入口），不塞进这里。

	// 6. probe.json 与退出码告警（L677-L694）。probe.json 先落盘，再打告警。
	if err := os.WriteFile(filepath.Join(out, "probe.json"),
		[]byte(probeReportJSON(clientPID, clientExit, clientDir, payload)), 0o644); err != nil {
		return fmt.Errorf("写 probe.json 失败：%w", err)
	}
	for _, line := range probeExitLines(clientExit, clientDir) {
		writeHelperLine(console, helperOut, line)
	}

	// 7. Python 的 finally：网关还活着就 terminate 并等最多 5 秒。trace 在它之后。
	stopGateway()

	// 8. 客户端 trace 与最后一行 out 绝对路径（L701-L721）。
	printClientTrace(helperOut, out)
	writeHelperLine(console, helperOut, out)
	return nil
}

// wfpProbeIsolationReason 报告"为什么回退到 probe.exe"，写进日志用。
func wfpProbeIsolationReason(env *childEnv) string {
	if forceProbeFallback(env) {
		return "DFO_FORCE_PROBE_EXE=1：按要求强制走 probe.exe"
	}
	if requireGoIsolation(env) {
		return "DFO_REQUIRE_GO_ISOLATION=1：禁止回退，Go 隔离不可用即失败"
	}
	return "Go 隔离不可用（非 Windows / 缺 API / 无管理员权限 / 装过滤器失败）"
}

// runProbeHost 是回退路径：拉起 probe.exe 并等它退出（channel_probe.py L633-L694）。
// 一行没改的原有行为，只是从 LaunchClient 里抽出来，好让两条路在同一个函数里对照。
//
// 返回 (probe_pid, probe_returncode, error)。超时（非 interactive 模式）沿用 Python 的
// 语义：**不杀** probe，只把网关收掉并把命令行带进错误 —— 所以那一支返回错误，
// 调用方直接返回，probe.json 不会写（与 Python 的 TimeoutExpired 一样）。
func runProbeHost(
	run *sessionRun,
	root string,
	argv []string,
	probeOut, probeErr *os.File,
	mode helperMode,
) (int, int, error) {
	commandLine := strings.Join(argv, " ")
	cmd := exec.Command(argv[0], argv[1:]...)
	// cwd 与网关一致（launch_local.py 给 helper 的 cwd 是 server\，probe 继承它）。
	cmd.Dir = filepath.Join(root, "server")
	cmd.Env = run.Env.List()
	cmd.Stdout = probeOut
	cmd.Stderr = probeErr
	hideConsoleWindow(cmd)
	probe, err := startChild(cmd)
	if err != nil {
		// Python 在 except 里 raise RuntimeError(command)，command 是**网关**命令行
		// （启动器那侧真正要看的参数），这里沿用同一口径。
		return 0, 0, fmt.Errorf("拉起 probe.exe 失败：%w\n命令行：%s\n网关命令行：%s",
			err, commandLine, run.CommandLine)
	}
	if timeout, bounded := mode.probeWaitTimeout(); bounded {
		if !probe.waitTimeout(timeout) {
			// Python 的 subprocess.TimeoutExpired：probe 还活着，Python **不杀它**（finally
			// 只收网关），这里也不杀，只把网关收掉并把命令行带进错误，语义保持一致。
			return probe.pid(), -1, fmt.Errorf("probe.exe 在 %s 内没有退出。\n网关命令行：%s",
				timeout, run.CommandLine)
		}
	} else {
		probe.reap()
	}
	return probe.pid(), probe.exitCode(), nil
}

// writeHelperLine 把 helper 侧的一行写进 helper.out 并回显到 console（命令行的两行、客户端
// 目录可见性告警、退出码与告警、收尾的 out 路径）。helper.out 在 Python 里是文本流，写进去的
// 每个 "\n" 都会变成 "\r\n"，这里照做。
func writeHelperLine(console io.Writer, helperOut, text string) {
	writeHelperBody(helperOut, text)
	fmt.Fprintln(console, text)
}

// writeHelperBody 只落 helper.out（Python 的 print 也只到那里）。
func writeHelperBody(helperOut, text string) {
	body := strings.ReplaceAll(text, "\n", pythonTextNewline)
	if file, err := os.OpenFile(helperOut, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644); err == nil {
		_, _ = file.WriteString(body + pythonTextNewline)
		_ = file.Close()
	}
}

// clientTracePath 是 channel_probe.py L701 的位置：<home>\AppData\LocalLow\DNF\DFO.trc。
func clientTracePath(home string) string {
	return filepath.Join(home, "AppData", "LocalLow", "DNF", "DFO.trc")
}

// pythonHomeDir 复刻 pathlib.Path.home() 在 Windows 上的取法：先看 USERPROFILE（哪怕为空串
// 也算命中），否则用 HOMEDRIVE+HOMEPATH；两者都没有时 Python 的 expanduser 原样返回 "~"，
// 那个路径同样读不到 trace，于是这里返回空串让调用方直接跳过。
func pythonHomeDir() string {
	if home, present := os.LookupEnv("USERPROFILE"); present {
		return home
	}
	if homePath, present := os.LookupEnv("HOMEPATH"); present {
		return filepath.Join(os.Getenv("HOMEDRIVE"), homePath)
	}
	return ""
}

// decodeClientTrace 复刻 channel_probe.py L703-L705：逐字节做 ((x>>6)|(x<<2))&255 ^ 118，
// 再按 utf-8 errors="replace" 解码。Python 的 replace 是按"最大子部分"替换的（一段坏字节
// 只出一个 U+FFFD，未结束的合法前缀整体出一个），Go 的 strings.ToValidUTF8 会把一整段
// 连续坏字节并成一个，所以这里自己按字节走。
func decodeClientTrace(data []byte) string {
	decoded := make([]byte, len(data))
	for index, value := range data {
		decoded[index] = byte((((value >> 6) | (value << 2)) & 255) ^ 118)
	}
	var out strings.Builder
	out.Grow(len(decoded))
	for index := 0; index < len(decoded); {
		r, size := utf8.DecodeRune(decoded[index:])
		if r != utf8.RuneError || size > 1 {
			out.WriteRune(r)
			index += size
			continue
		}
		out.WriteRune(utf8.RuneError)
		index += utf8MaximalSubpart(decoded[index:])
	}
	return out.String()
}

// utf8MaximalSubpart 返回 b[0] 起那个非法 UTF-8 序列"最大子部分"的长度（>=1），用来对齐
// Python errors="replace" 的替换粒度：首字节非法或续字节越界时只吃掉已合法的前缀，序列在
// 结尾被截断时整个前缀算一段。
func utf8MaximalSubpart(b []byte) int {
	first := b[0]
	need := 0
	low, high := byte(0x80), byte(0xbf)
	switch {
	case first >= 0xc2 && first <= 0xdf:
		need = 1
	case first == 0xe0:
		need, low = 2, 0xa0
	case first >= 0xe1 && first <= 0xec:
		need = 2
	case first == 0xed:
		need, high = 2, 0x9f
	case first >= 0xee && first <= 0xef:
		need = 2
	case first == 0xf0:
		need, low = 3, 0x90
	case first >= 0xf1 && first <= 0xf3:
		need = 3
	case first == 0xf4:
		need, high = 3, 0x8f
	default:
		// 0x80-0xc1（落单的续字节 / 过长编码）与 0xf5-0xff。
		return 1
	}
	length := 1
	for index := 1; index <= need; index++ {
		if index >= len(b) {
			return length
		}
		value := b[index]
		if index == 1 {
			if value < low || value > high {
				return length
			}
		} else if value < 0x80 || value > 0xbf {
			return length
		}
		length++
	}
	return length
}

// clientTraceMAC 是 L706 的脱敏规则；clientTraceNewlines 是紧随其后的 "\r\r\n" → "\n"。
// 顺序不能反：先脱敏（它按 [^\r\n]* 吃一行），再并换行。
var clientTraceMAC = regexp.MustCompile("MAC Address[^\r\n]*")

// clientTraceFilter 是 L714-L718 的关键字表（re.search(..., re.I)）。表里全是 ASCII，
// RE2 的 (?i) 与 Python 的忽略大小写结果一致。
var clientTraceFilter = regexp.MustCompile(
	`(?i)CHANNELINFO|LOGIN|GET_USERINFO|CREATE_CHARACTER|SELECT_CHARACTER|CHECK_CHARACTER_NAME|Checksum|decrypt`)

// clientTraceLimit 是 L719 的 [:5000]：按**字符**（码位）截断，不是字节。
const clientTraceLimit = 5000

// redactClientTrace 复刻 L706-L708。
func redactClientTrace(text string) string {
	text = clientTraceMAC.ReplaceAllString(text, "MAC Address [redacted]")
	return strings.ReplaceAll(text, "\r\r\n", "\n")
}

// filterClientTrace 复刻 L710-L719：逐行取命中关键字的行，用 "\n" 连接，再截断到 5000 字符。
// 一行都没命中时 Python 也会 print 一个空串（一个空行），所以这里返回空串而不是"跳过"。
func filterClientTrace(text string) string {
	var matched []string
	for _, line := range pythonSplitLines(text) {
		if clientTraceFilter.MatchString(line) {
			matched = append(matched, line)
		}
	}
	joined := strings.Join(matched, "\n")
	if runes := []rune(joined); len(runes) > clientTraceLimit {
		return string(runes[:clientTraceLimit])
	}
	return joined
}

// pythonSplitLines 复刻 str.splitlines()：除了 \n 与 \r\n，Python 还把 \r、\v、\f、
// \x1c-\x1e、\x85、U+2028、U+2029 当行界，且结尾的换行不产生空行。trace 文本里 \r 是
// 真实存在的（只并掉 "\r\r\n"），用 Go 的 strings.Split(text, "\n") 会把这些行连在一起。
func pythonSplitLines(text string) []string {
	var lines []string
	var current strings.Builder
	for index := 0; index < len(text); {
		r, size := utf8.DecodeRuneInString(text[index:])
		switch r {
		case '\n', '\v', '\f', '\x1c', '\x1d', '\x1e', '\u0085', '\u2028', '\u2029':
			lines = append(lines, current.String())
			current.Reset()
			index += size
		case '\r':
			lines = append(lines, current.String())
			current.Reset()
			index += size
			if index < len(text) && text[index] == '\n' {
				index++
			}
		default:
			current.WriteString(text[index : index+size])
			index += size
		}
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}

// pythonWriteText 复刻 Path.write_text(text, encoding="utf-8")：Windows 文本模式把写出的每个
// "\n" 翻成 os.linesep（孤立的 "\r" 原样保留）。启动链只跑 Windows，所以写死 \r\n。
func pythonWriteText(path, text string) error {
	return os.WriteFile(path, []byte(strings.ReplaceAll(text, "\n", pythonTextNewline)), 0o644)
}

// printClientTrace 是 channel_probe.py L701-L720 的收尾：trace 存在就解码、脱敏、落盘
// client_trace.txt，并把命中关键字的行（截断 5000 字符）写进 helper.out；不存在就什么都不做。
//
// 这一段只进 helper.out，不回显控制台：它是成块的诊断内容（最多 5000 字符），Python 的 print
// 也只到 helper 的 stdout，而启动器控制台在 Python 版里从来没有过这些行。全文另存
// client_trace.txt（同一份文本）。
//
// 落盘失败在 Python 里会让 helper 带着 traceback 退出，而启动器早就按 run.json 返回 0 了；
// 单进程化后不拿一个诊断文件改写启动结果，只如实告警（行与 out 仍然照打）。
func printClientTrace(helperOut, out string) {
	home := pythonHomeDir()
	if home == "" {
		return
	}
	data, err := os.ReadFile(clientTracePath(home))
	if err != nil {
		// Python 的 if trace.exists()：不存在与不可读都走这条路。
		return
	}
	text := redactClientTrace(decodeClientTrace(data))
	if writeErr := pythonWriteText(filepath.Join(out, "client_trace.txt"), text); writeErr != nil {
		fmt.Fprintf(os.Stderr, "WARNING: 写 client_trace.txt 失败：%v\n", writeErr)
	}
	writeHelperBody(helperOut, filterClientTrace(text))
}
