package launcher

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"dfolan/internal/accountname"
)

// probePayload 的三条分支：默认用 ready.json 里的端口，channel-check 写死 7001，
// tag 以 _next30.._next34 结尾（_next35/36/37 降级之后也落在这里）同样写死 7001。
func TestProbePayloadSelection(t *testing.T) {
	const defaultShape = "13?127.0.0.1?56201?probe?00000000000000000000000000000000?0?0?30?0?0?0"
	const fixed7001 = "3?127.0.0.1?7001?probe?00000000000000000000000000000000?0?0?30?0?0?0"

	cases := []struct {
		name         string
		tag          string
		channelCheck bool
		want         string
	}{
		{"普通 tag 用端口", "channel_01", false, defaultShape},
		{"channel-check 用 7001", "channel_01", true, fixed7001},
		{"_next30", "roles_x_next30", false, fixed7001},
		{"_next31", "roles_x_next31", false, fixed7001},
		{"_next32", "roles_x_next32", false, fixed7001},
		{"_next33", "roles_x_next33", false, fixed7001},
		{"_next34", "roles_x_next34", false, fixed7001},
		{"_next37 降级后仍算 _next34", "roles_persist_next37", false, fixed7001},
		{"_next29 不算", "roles_x_next29", false, defaultShape},
	}
	for _, testCase := range cases {
		tag := testCase.tag
		if strings.HasSuffix(tag, "_next37") {
			// sessionTagName 生成的 tag 是 _next37，channel_probe.py L24-L32 会先降级。
			tag = newSessionTag(``, tag).Effective
		}
		if got := probePayload(56201, testCase.channelCheck, tag, ""); got != testCase.want {
			t.Errorf("%s：probePayload = %s，want %s", testCase.name, got, testCase.want)
		}
	}
}

// 启动器自己产出的 tag 一定以 _next37 结尾，降级后是 _next34，所以 interactive 拿到的是
// 7001 形态；这条断言把"默认启动"与 payload 之间的连接钉死。
func TestDefaultLaunchTagGetsTheFixedPortPayload(t *testing.T) {
	tag := newSessionTag(`C:\dfo-lan`, sessionTagName(time.Now()))
	payload := probePayload(49956, false, tag.Effective, "")
	if !strings.HasPrefix(payload, "3?127.0.0.1?7001?") {
		t.Errorf("默认 tag 的 payload = %s", payload)
	}
	if !strings.Contains(payload, "?0?0?30?0?0?0") || !strings.HasSuffix(payload, "00000000000000000000000000000000?0?0?30?0?0?0") {
		t.Errorf("payload 尾部与 Python 不一致：%s", payload)
	}
	if payload != probePayload7001 {
		t.Errorf("payload = %s，want %s", payload, probePayload7001)
	}
}

// 账号段（第 4 段）跟着会话走，其余分段一个字节都不动 —— 这是本批改动的验收口径：
// 「除账号那一段外逐字节一致」。服务端从不读这一段（身份来自 wireprobe 的 -account /
// DFO_ACCOUNT 与那条 UPSERT），但客户端显示的是它，所以换账号后必须与会话真实身份一致，
// 否则玩家看到的名字和存档归属对不上。
func TestProbePayloadAccountSegment(t *testing.T) {
	const withProbe = "13?127.0.0.1?56201?probe?00000000000000000000000000000000?0?0?30?0?0?0"
	const withTomeu = "13?127.0.0.1?56201?tomeu?00000000000000000000000000000000?0?0?30?0?0?0"

	if got := probePayload(56201, false, "channel_01", "tomeu"); got != withTomeu {
		t.Errorf("改名后的 payload = %s，want %s", got, withTomeu)
	}
	if got := probePayload(56201, false, "channel_01", ""); got != withProbe {
		t.Errorf("默认账号（空 = probe）的 payload = %s，want %s", got, withProbe)
	}
	probe, moved := strings.Split(withProbe, "?"), strings.Split(withTomeu, "?")
	if len(probe) != len(moved) {
		t.Fatalf("分段数变了：%d vs %d", len(probe), len(moved))
	}
	for i := range probe {
		if i == 3 {
			continue
		}
		if probe[i] != moved[i] {
			t.Errorf("第 %d 段被连带改动：%q -> %q", i, probe[i], moved[i])
		}
	}
	if got := probePayload(0, true, "", "tomeu"); got !=
		"3?127.0.0.1?7001?tomeu?00000000000000000000000000000000?0?0?30?0?0?0" {
		t.Errorf("7001 形态没跟着改名：%s", got)
	}
	// 默认账号时计划行必须与历史常量逐字节相同（Python 对拍的基线）。
	if got := planPayload(accountname.Default); got != probePayload7001 {
		t.Errorf("默认账号的计划行 = %s，want %s", got, probePayload7001)
	}
}

// argv 顺序是验收项：probe.exe、client_dir、client.log、"55"、ui-mode、breakpoints.txt、payload。
func TestProbeArgvOrder(t *testing.T) {
	got := probeArgv(`C:\tools\probe.exe`, `C:\client`, `C:\out`, "interactive-ui", "13?payload")
	want := []string{
		`C:\tools\probe.exe`,
		`C:\client`,
		`C:\out\client.log`,
		"55",
		"interactive-ui",
		`C:\out\breakpoints.txt`,
		"13?payload",
	}
	if len(got) != len(want) {
		t.Fatalf("argv 长度 = %d，want %d：%q", len(got), len(want), got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("argv[%d] = %q，want %q", index, got[index], want[index])
		}
	}
	if joined := strings.Join(got, " "); joined != strings.Join(want, " ") {
		t.Errorf("命令行 = %s", joined)
	}
}

// ui-mode 的四条分支（channel_probe.py L623-L629）。
func TestProbeUIModeTable(t *testing.T) {
	cases := map[helperMode]string{
		modeChannelCheck:   "normal-ui",
		modeExceptionTrace: "trace-owned-ui",
		modeInteractive:    "interactive-ui",
		modeClientOnly:     "trace-root-ui",
		modeServerOnly:     "trace-root-ui",
		helperMode("别的"):   "trace-root-ui",
	}
	for mode, want := range cases {
		if got := probeUIMode(mode); got != want {
			t.Errorf("probeUIMode(%q) = %q，want %q", mode, got, want)
		}
	}
}

// 等待语义：interactive / exception-trace 不设超时，其余 65 秒。
func TestProbeWaitTimeoutTable(t *testing.T) {
	unbounded := []helperMode{modeInteractive, modeExceptionTrace}
	for _, mode := range unbounded {
		if timeout, bounded := mode.probeWaitTimeout(); bounded {
			t.Errorf("%q 不该有超时，却给了 %s", mode, timeout)
		}
	}
	for _, mode := range []helperMode{modeClientOnly, modeServerOnly, modeChannelCheck} {
		timeout, bounded := mode.probeWaitTimeout()
		if !bounded || timeout != 65*time.Second {
			t.Errorf("%q 的超时 = %s/%v，want 65s", mode, timeout, bounded)
		}
	}
}

// 只有三种模式能从启动器到达；client-only 与任何开关名都不匹配（这是 Python 的行为）。
func TestLaunchModeFromOptions(t *testing.T) {
	if got := launchMode(LaunchOptions{}); got != modeInteractive {
		t.Errorf("默认模式 = %q", got)
	}
	if got := launchMode(LaunchOptions{ServerOnly: true}); got != modeServerOnly {
		t.Errorf("--server-only 模式 = %q", got)
	}
	if got := launchMode(LaunchOptions{ClientOnly: true}); got != modeClientOnly {
		t.Errorf("--client-only 模式 = %q", got)
	}
	if modeClientOnly.interactive() || modeClientOnly.channelCheck() || modeClientOnly.exceptionTrace() {
		t.Error("client-only 不该命中 channel_probe.py 的任何一个开关")
	}
	if !modeServerOnly.serverOnly() {
		t.Error("server-only 应当命中 server_only")
	}
}

// run.json 三个键的键序与 json.dumps 的默认分隔符；probe.json 四个字段同理。
func TestSessionAndProbeJSONStyle(t *testing.T) {
	run := sessionRunJSON(2404, 15440, 54201)
	if run != `{"server_pid": 2404, "probe_pid": 15440, "port": 54201}` {
		t.Errorf("run.json = %s", run)
	}
	if strings.Contains(run, "\n") {
		t.Error("run.json 不该有换行")
	}
	if !(strings.Index(run, "server_pid") < strings.Index(run, "probe_pid") &&
		strings.Index(run, "probe_pid") < strings.Index(run, "port")) {
		t.Errorf("run.json 键序不对：%s", run)
	}

	// 与仓库里真实 Python 会话（runtime/..._20261004_221054_428343_next37/probe.json）逐字节同形。
	probe := probeReportJSON(15440, 3, `C:\Game\dof\115us\DFO`, probePayload7001)
	want := `{"probe_pid": 15440, "probe_returncode": 3, "client_dir": "C:\\Game\\dof\\115us\\DFO", ` +
		`"payload": "3?127.0.0.1?7001?probe?00000000000000000000000000000000?0?0?30?0?0?0"}`
	if probe != want {
		t.Errorf("probe.json =\n%s\nwant\n%s", probe, want)
	}
	if strings.Contains(probe, "\n") {
		t.Error("probe.json 不该有换行")
	}

	// 非 ASCII 路径走 json.dumps 的 ensure_ascii 口径：中文转 \uXXXX。
	escaped := probeReportJSON(1, 0, `C:\客户端\DFO`, "p")
	if !strings.Contains(escaped, `"client_dir": "C:\\\u5ba2\u6237\u7aef\\DFO"`) {
		t.Errorf("非 ASCII 路径转义不对：%s", escaped)
	}
}

// DFO_CLIENT_DIR 的回退：键存在（哪怕是空串）就用它，键缺失才退回 probe 工具目录下的
// dfo_probe_client —— 与 os.environ.get(key, default) 一致。
func TestProbeClientDirFallback(t *testing.T) {
	env := newChildEnv([]string{"PATH=C:\\Windows"})
	if got := probeClientDir(env, `C:\tools`); got != filepath.Join(`C:\tools`, "dfo_probe_client") {
		t.Errorf("缺失时的回退 = %q", got)
	}
	env.Set("DFO_CLIENT_DIR", `C:\client`)
	if got := probeClientDir(env, `C:\tools`); got != `C:\client` {
		t.Errorf("环境变量优先 = %q", got)
	}
	env.Set("DFO_CLIENT_DIR", "")
	if got := probeClientDir(env, `C:\tools`); got != "" {
		t.Errorf("空串不该回退（Python 的 get 只在键缺失时给默认值）= %q", got)
	}
}

// 目录可见性告警：DFO.exe 不是文件（缺失、或是目录）都要打，是文件就不打。
func TestProbeClientDirWarning(t *testing.T) {
	dir := t.TempDir()
	if got := probeClientDirWarning(dir); got != "WARNING: probe cannot see DFO.exe under client dir: "+dir {
		t.Errorf("缺 DFO.exe 的告警 = %q", got)
	}
	if err := os.MkdirAll(filepath.Join(dir, "DFO.exe"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := probeClientDirWarning(dir); got == "" {
		t.Error("DFO.exe 是目录时也应当告警（Python 的 is_file 为假）")
	}
	if err := os.Remove(filepath.Join(dir, "DFO.exe")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "DFO.exe"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := probeClientDirWarning(dir); got != "" {
		t.Errorf("DFO.exe 存在时不该告警：%q", got)
	}
}

// 退出码告警：0 只有一行，非 0 再多两行，文案与 channel_probe.py L687-L694 逐字节一致。
func TestProbeExitLines(t *testing.T) {
	ok := probeExitLines(0, `C:\client`)
	if len(ok) != 1 || ok[0] != "probe.exe exited with code 0" {
		t.Errorf("退出码 0 的输出 = %q", ok)
	}
	bad := probeExitLines(3, `C:\client`)
	if len(bad) != 3 {
		t.Fatalf("退出码 3 的输出 = %q", bad)
	}
	if bad[0] != "probe.exe exited with code 3" {
		t.Errorf("第一行 = %q", bad[0])
	}
	if bad[1] != "WARNING: the game client was not launched correctly." {
		t.Errorf("第二行 = %q", bad[1])
	}
	want := "  client dir passed to probe: C:\\client" +
		"  (return code 3 = probe could not see DFO.exe there and it exits before writing client.log;" +
		" on real machines this usually means security software blocked probe.exe)"
	if bad[2] != want {
		t.Errorf("第三行 = %q\nwant      %q", bad[2], want)
	}
	// -1（拿不到状态）同样走告警分支。
	if lines := probeExitLines(-1, "d"); len(lines) != 3 || lines[0] != "probe.exe exited with code -1" {
		t.Errorf("负退出码 = %q", lines)
	}
}

// encodeClientTrace 是 channel_probe.py L703 那个变换的逆：解出来是 y = ROL2(x) ^ 118，
// 所以编回去是 x = ROR2(y ^ 118)。用它把明文编成 .trc 字节，decodeClientTrace 必须还原。
func encodeClientTrace(plain []byte) []byte {
	out := make([]byte, len(plain))
	for index, value := range plain {
		mixed := value ^ 118
		out[index] = byte(((mixed >> 2) | (mixed << 6)) & 255)
	}
	return out
}

// 解码：位移异或 + utf-8。原文能原样回来；坏字节按 Python errors="replace" 的粒度替换。
func TestDecodeClientTrace(t *testing.T) {
	plain := "MAC Address : 00-11-22\r\n[ETC] CHANNELINFO ok\n中文行\n"
	if got := decodeClientTrace(encodeClientTrace([]byte(plain))); got != plain {
		t.Errorf("往返失败：%q", got)
	}

	// 期望值是用 Python 3.11 实测出来的：
	//   for c in cases: c.decode("utf-8", "replace") 的码位
	cases := []struct {
		raw  []byte
		want []rune
	}{
		{[]byte{0xff}, []rune{0xfffd}},
		{[]byte{0xe0, 0x80}, []rune{0xfffd, 0xfffd}},
		{[]byte{0xe0, 0x80, 0x80}, []rune{0xfffd, 0xfffd, 0xfffd}},
		{[]byte{0xc3}, []rune{0xfffd}},
		{[]byte{0xf0, 0x9f}, []rune{0xfffd}},
		{[]byte{0xf0, 0x9f, 0x98}, []rune{0xfffd}},
		{[]byte{0x80, 0x80}, []rune{0xfffd, 0xfffd}},
		{[]byte{0xc3, 0x28}, []rune{0xfffd, '('}},
		{[]byte{0xed, 0xa0, 0x80}, []rune{0xfffd, 0xfffd, 0xfffd}},
		{[]byte{0xf4, 0x90, 0x80, 0x80}, []rune{0xfffd, 0xfffd, 0xfffd, 0xfffd}},
		{[]byte{0xff, 0xff, 0xff}, []rune{0xfffd, 0xfffd, 0xfffd}},
		{[]byte{0xc3, 0xa9}, []rune{'é'}},
	}
	for _, testCase := range cases {
		got := decodeClientTrace(encodeClientTrace(testCase.raw))
		if got != string(testCase.want) {
			t.Errorf("% x 的解码 = %q（%U），want %q（%U）",
				testCase.raw, got, []rune(got), string(testCase.want), testCase.want)
		}
	}
}

// splitlines：Python 把 \r、\v、\f、\x1c-\x1e、\x85、U+2028、U+2029 都当行界，
// 且结尾换行不产生空行（trace 文本里 \r 是真实存在的）。
func TestPythonSplitLines(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a\n", []string{"a"}},
		{"a\n\n", []string{"a", ""}},
		{"a\r\nb", []string{"a", "b"}},
		{"a\rb", []string{"a", "b"}},
		{"a\vb\fc", []string{"a", "b", "c"}},
		{"a\x1cb\x1dc\x1ed", []string{"a", "b", "c", "d"}},
		{"a\u0085b\u2028c\u2029d", []string{"a", "b", "c", "d"}},
		{"a\r\r\nb", []string{"a", "", "b"}},
	}
	for _, testCase := range cases {
		got := pythonSplitLines(testCase.text)
		if len(got) != len(testCase.want) {
			t.Errorf("%q 切成 %q，want %q", testCase.text, got, testCase.want)
			continue
		}
		for index := range got {
			if got[index] != testCase.want[index] {
				t.Errorf("%q 切成 %q，want %q", testCase.text, got, testCase.want)
				break
			}
		}
	}
}

// 脱敏 + 换行归一：顺序不能反（先按 [^\r\n]* 吃掉整行，再把 \r\r\n 并成 \n）。
func TestRedactClientTrace(t *testing.T) {
	text := "MAC Address : 00-11-22-33-44-55\r\nnext\r\r\nMAC Address abc\nend"
	got := redactClientTrace(text)
	want := "MAC Address [redacted]\r\nnext\nMAC Address [redacted]\nend"
	if got != want {
		t.Errorf("redact = %q，want %q", got, want)
	}
}

// 过滤：大小写不敏感、按行取、用 \n 连接、截断到 5000 字符（按字符不是字节）。
func TestFilterClientTrace(t *testing.T) {
	text := strings.Join([]string{
		"[ETC] nothing here",
		"[NET][TCP][RECV] ENUM_NOTIPACKET_CHANNELINFO (Size : 1044)",
		"[BASEINFO] skip Login : False",
		"mac address redacted",
		"[ETC] decrypt ok",
	}, "\n")
	got := filterClientTrace(text)
	want := strings.Join([]string{
		"[NET][TCP][RECV] ENUM_NOTIPACKET_CHANNELINFO (Size : 1044)",
		"[BASEINFO] skip Login : False",
		"[ETC] decrypt ok",
	}, "\n")
	if got != want {
		t.Errorf("filter = %q，want %q", got, want)
	}

	// 一行都不命中：Python 会 print 一个空串（一个空行），这里返回空串。
	if got := filterClientTrace("nothing matches here"); got != "" {
		t.Errorf("无命中 = %q，want 空串", got)
	}

	// 截断：5000 + 20 个字符，多出来的 20 个（含中文）都要被切掉，且不能切坏字符。
	long := strings.Repeat("LOGIN 中文\n", 700)
	filtered := filterClientTrace(long)
	if utf8.RuneCountInString(filtered) != clientTraceLimit {
		t.Errorf("截断后长度 = %d，want %d", utf8.RuneCountInString(filtered), clientTraceLimit)
	}
	if !utf8.ValidString(filtered) {
		t.Error("截断不该切坏 UTF-8")
	}
}

// write_text 口径：\n 变 \r\n，孤立 \r 原样保留（Python 文本模式在 Windows 上的行为）。
func TestPythonWriteTextUsesWindowsNewlines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "client_trace.txt")
	if err := pythonWriteText(path, "a\nb\rc\r\nd"); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "a\r\nb\rc\r\r\nd" {
		t.Errorf("落盘字节 = %q", body)
	}
}

// trace 段的端到端：伪造 USERPROFILE，放一份合成 .trc，检查 client_trace.txt 与打印内容。
func TestPrintClientTraceWritesAndPrints(t *testing.T) {
	home := t.TempDir()
	traceDir := filepath.Join(home, "AppData", "LocalLow", "DNF")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	plain := "line one\n[ETC] CHANNELINFO ready\r\nMAC Address : aa:bb\r\nno match at all\r"
	if err := os.WriteFile(filepath.Join(traceDir, "DFO.trc"), encodeClientTrace([]byte(plain)), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", home)

	out := t.TempDir()
	helperOut := filepath.Join(out, "helper.out")
	printClientTrace(helperOut, out)

	// 落盘走 write_text：每个 \n 变 \r\n（原来的 \r\n 因此变成 \r\r\n，与 Python 一致）。
	wantText := "line one\n[ETC] CHANNELINFO ready\r\nMAC Address [redacted]\r\nno match at all\r"
	body, err := os.ReadFile(filepath.Join(out, "client_trace.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != strings.ReplaceAll(wantText, "\n", "\r\n") {
		t.Errorf("client_trace.txt = %q", got)
	}
	// helper.out 里是命中关键字的行（MAC 那行不在关键字表里）、\n 连接，且每个 \n 变 \r\n。
	wantPrinted := "[ETC] CHANNELINFO ready"
	helperBody, err := os.ReadFile(helperOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(helperBody) != strings.ReplaceAll(wantPrinted, "\n", "\r\n")+"\r\n" {
		t.Errorf("helper.out = %q", helperBody)
	}
}

// trace 不存在（或 USERPROFILE 下没有这个文件）时什么都不做：不建文件、不打字。
func TestPrintClientTraceWithoutTraceFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("USERPROFILE", home)
	out := t.TempDir()
	printClientTrace(filepath.Join(out, "helper.out"), out)
	if _, err := os.Stat(filepath.Join(out, "client_trace.txt")); !os.IsNotExist(err) {
		t.Error("不该写 client_trace.txt")
	}
	if _, err := os.Stat(filepath.Join(out, "helper.out")); !os.IsNotExist(err) {
		t.Error("不该写 helper.out")
	}
}

// 一行都不命中时 Python 也会 print 一个空行，这里保持一致（helper.out 里是一行空行）。
func TestPrintClientTracePrintsEmptyLineWhenNothingMatches(t *testing.T) {
	home := t.TempDir()
	traceDir := filepath.Join(home, "AppData", "LocalLow", "DNF")
	if err := os.MkdirAll(traceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(traceDir, "DFO.trc"), encodeClientTrace([]byte("nothing to see\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("USERPROFILE", home)
	out := t.TempDir()
	helperOut := filepath.Join(out, "helper.out")
	printClientTrace(helperOut, out)
	body, err := os.ReadFile(helperOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "\r\n" {
		t.Errorf("helper.out = %q", body)
	}
}

// helper 侧的行要写进 helper.out（文本模式换行）并回显到 console。
func TestWriteHelperLineMirrorsToConsole(t *testing.T) {
	out := t.TempDir()
	helperOut := filepath.Join(out, "helper.out")
	var console bytes.Buffer
	writeHelperLine(&console, helperOut, "first")
	writeHelperLine(&console, helperOut, "multi\nline")
	body, err := os.ReadFile(helperOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "first\r\nmulti\r\nline\r\n" {
		t.Errorf("helper.out = %q", body)
	}
	if console.String() != "first\nmulti\nline\n" {
		t.Errorf("console = %q", console.String())
	}
}

// childProcess 的等待/终止语义就是 probe 那一段：waitTimeout 超时不杀进程（只报没等到），
// terminate 之后必须真的收尸，exitCode 要能读出退出码。
func TestChildProcessWaitTimeoutAndExitCode(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("启动链只跑 Windows")
	}
	// 正常退出：退出码要如实读出来。
	quick := exec.Command("cmd.exe", "/c", "exit", "3")
	child, err := startChild(quick)
	if err != nil {
		t.Fatalf("startChild: %v", err)
	}
	if !child.waitTimeout(10 * time.Second) {
		t.Fatal("cmd 应当已经退出")
	}
	if got := child.exitCode(); got != 3 {
		t.Errorf("exitCode = %d，want 3", got)
	}

	// 超时：进程还活着，waitTimeout 返回 false（Python 的 TimeoutExpired 也不杀它）。
	slow := exec.Command("ping.exe", "-n", "6", "127.0.0.1")
	child, err = startChild(slow)
	if err != nil {
		t.Fatalf("startChild: %v", err)
	}
	if child.waitTimeout(200 * time.Millisecond) {
		t.Error("200ms 内 ping 不该退出")
	}
	child.terminate(5 * time.Second)
	if !child.waitTimeout(time.Second) {
		t.Error("terminate 之后应当已经收尸")
	}
}

// LaunchClient 只接 interactive / --client-only，别的模式走 LaunchServer，误用要立刻报错。
func TestLaunchClientRefusesServerOnly(t *testing.T) {
	err := LaunchClient(t.Context(), t.TempDir(), LaunchOptions{ServerOnly: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "内部错误") {
		t.Errorf("err = %v", err)
	}
	err = LaunchClient(t.Context(), t.TempDir(), LaunchOptions{StorageOnly: true}, nil)
	if err == nil || !strings.Contains(err.Error(), "内部错误") {
		t.Errorf("err = %v", err)
	}
}

// 客户端目录里没有 DFO.exe 时，interactive 会在预检就停住 —— 与 Python 一样
// （launch_local.py L252-L254 的 required 里有 client/DFO.exe），根本走不到 probe。
// 这条断言同时说明"用不含 DFO.exe 的假 client 目录跑端到端"到不了返回码 3 那条路。
func TestLaunchClientStopsAtTheMissingClientExe(t *testing.T) {
	root := buildLaunchTree(t)
	clientExe := filepath.Join(root, "server", "client", "DFO.exe")
	if err := os.Remove(clientExe); err != nil {
		t.Fatal(err)
	}
	err := LaunchClient(t.Context(), root, LaunchOptions{}, &bytes.Buffer{})
	if err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(err.Error(), "Missing dependency: "+clientExe) {
		t.Errorf("err = %v", err)
	}
}
