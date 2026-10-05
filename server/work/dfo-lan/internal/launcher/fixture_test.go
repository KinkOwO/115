package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 这三个哈希来自 Python 版的真实运行产物
// （runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261005_003333_830830_next37），
// 不是从 Go 版自己算出来的：只有这样它才是"与 Python 逐字节一致"的证据，而不是自证。
const (
	historicalChannelInfoSHA256 = "3dd366183cd74402151501e5e8904256ba25b8b1ccc064d3ee5c7ce6cb4a87c1"
	historicalFixtureJSONSHA256 = "9bd16c5bc72f3526c8aee3a6b8f29dda8f6095147926d737985e3c8fc2844389"
	historicalBreakpointsSHA256 = "d91caeeb3fd6b43bedaa53d0edc8f5b618749b27f26d04311ead66b590c30942"
	// run.json 是接口（别的进程按这两个键读），风格必须与 json.dumps 默认风格一致。
	historicalRunJSON = `{"server_pid": 2404, "port": 54201}`
)

// sessionTagForTest 是启动器实际会产生的 tag 形态（前缀 + 微秒时间戳 + _next37）。
const sessionTagForTest = launcherTagPrefix + "20261005_120000_000000_next37"

func sha256Of(t *testing.T, data []byte) string {
	t.Helper()
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// channelinfo.bin 是整套夹具的核心：任何一位错了，客户端都不会走通。这里用的是 Python
// 版历史产物的哈希，长度也一并钉住（16 字节头 + 加密体）。
func TestChannelFixtureMatchesTheHistoricalPythonBytes(t *testing.T) {
	fixture := buildChannelFixture()
	if len(fixture.File) != 1060 {
		t.Errorf("channelinfo.bin 长度 = %d，want 1060", len(fixture.File))
	}
	if got := sha256Of(t, fixture.File); got != historicalChannelInfoSHA256 {
		t.Errorf("channelinfo.bin sha256 = %s\nwant %s", got, historicalChannelInfoSHA256)
	}
	if len(fixture.PB) != 1040 {
		t.Errorf("pb 长度 = %d，want 1040", len(fixture.PB))
	}
	if len(fixture.Plain) != 4+len(fixture.PB) {
		t.Errorf("plain 长度 = %d，want 4+pb", len(fixture.Plain))
	}
	// 头部是 struct.pack("<B H I I I B")，逐字段对一遍，比只看总哈希更能定位问题。
	header := fixture.File[:16]
	if header[0] != 0 || header[1] != 1 || header[2] != 0 {
		t.Errorf("头部版本字段 = %v, want 0,1,0", header[:3])
	}
	wantSize := uint32(16 + len(fixture.Cipher))
	if got := uint32(header[3]) | uint32(header[4])<<8 | uint32(header[5])<<16 | uint32(header[6])<<24; got != wantSize {
		t.Errorf("头部长度字段 = %d, want %d", got, wantSize)
	}
	// fold 是 <B H I I I B> 里第 5 个字段，落在字节 11..14（小端），末尾还有一个 <B>。
	if got := uint32(header[11]) | uint32(header[12])<<8 | uint32(header[13])<<16 | uint32(header[14])<<24; got != uint32(fixture.Fold) {
		t.Errorf("头部 fold = %d, want %d", got, fixture.Fold)
	}
	if header[7] != 0 || header[8] != 0 || header[9] != 0 || header[10] != 0 || header[15] != 0 {
		t.Errorf("头部保留字段非零：%v", header)
	}
}

// fixture.json 的哈希同样是 Python 产物，顺带把 CRLF 与"结尾无换行"两条风格钉住。
func TestFixtureJSONMatchesTheHistoricalPythonBytes(t *testing.T) {
	body := []byte(fixtureJSON(buildChannelFixture()))
	if got := sha256Of(t, body); got != historicalFixtureJSONSHA256 {
		t.Errorf("fixture.json sha256 = %s\nwant %s", got, historicalFixtureJSONSHA256)
	}
	if len(body) != 4337 {
		t.Errorf("fixture.json 长度 = %d，want 4337", len(body))
	}
	if !strings.Contains(string(body), "\r\n") {
		t.Error("fixture.json 缺少 CRLF：Python 的 write_text 在 Windows 上是 os.linesep")
	}
	if strings.HasSuffix(string(body), "\n") {
		t.Error("fixture.json 结尾多了换行：json.dumps 不写结尾换行")
	}
	if !strings.HasPrefix(string(body), "{\r\n  \"candidate\": ") {
		t.Errorf("fixture.json 开头 = %q", string(body[:32]))
	}
}

// breakpoints.txt 的整串哈希只对启动器这一族 tag 成立（它由前缀决定，不看时间戳）。
func TestBreakpointsMatchTheHistoricalPythonBytes(t *testing.T) {
	body := []byte(breakpointsText(sessionTagForTest))
	if got := sha256Of(t, body); got != historicalBreakpointsSHA256 {
		t.Errorf("breakpoints.txt sha256 = %s\nwant %s", got, historicalBreakpointsSHA256)
	}
	if len(body) != 570 {
		t.Errorf("breakpoints.txt 长度 = %d，want 570", len(body))
	}
	if !strings.HasSuffix(string(body), "52fd9e3 AREA_USERS_DONE\r\n") {
		t.Errorf("breakpoints.txt 结尾 = %q", string(body[len(body)-32:]))
	}
}

// 每一段追加都由 tag 前缀决定，行数就是"追加顺序没被重排"的最小证据。
func TestBreakpointsLineCountsPerTagFamily(t *testing.T) {
	cases := []struct {
		tag   string
		lines int
	}{
		{"channel_01", 9},
		{"login_normal", 11},
		{"roles_basic", 13},
		{"roles_row_wide", 16},
		{"roles_persist_select_a", 17},
		{"roles_persist_select_actor_a", 20},
		{"roles_persist_select_actor_town_a", 23},
		{sessionTagForTest, 23},
	}
	for _, testCase := range cases {
		body := breakpointsText(testCase.tag)
		got := len(strings.Split(strings.TrimSuffix(body, pythonTextNewline), pythonTextNewline))
		if got != testCase.lines {
			t.Errorf("tag %q 的断点行数 = %d，want %d", testCase.tag, got, testCase.lines)
		}
	}
}

// responses.json 用的合成包根：testdata 里的登录报文不存在，于是走 runtime/login_ok.bin
// 那条回退，期望值可以写成固定字面量。
func TestResponsesJSONMatchesThePythonRendering(t *testing.T) {
	const project = `C:\root`
	cases := []struct {
		tag  string
		want string
	}{
		{
			"channel_01",
			`{"1554": "C:\\root\\runtime\\precheck_ok.bin"}`,
		},
		{
			"login_normal",
			`{"1554": "C:\\root\\runtime\\precheck_ok.bin", "1": "C:\\root\\runtime\\login_ok.bin"}`,
		},
		{
			"roles_persist_select_actor_town_world_live_detail_dungeon_manual_x_next37",
			`{"1554": "C:\\root\\runtime\\precheck_ok.bin", "1": "C:\\root\\runtime\\login_ok.bin", ` +
				`"8": "C:\\root\\runtime\\characters_ok.bin", "684": "C:\\root\\runtime\\name_ok.bin"}`,
		},
		{
			"roles_row_wide",
			`{"1554": "C:\\root\\runtime\\precheck_ok.bin", "1": "C:\\root\\runtime\\login_ok.bin", ` +
				`"8": "C:\\root\\runtime\\characters_row_ok.bin", "684": "C:\\root\\runtime\\name_ok.bin"}`,
		},
	}
	for _, testCase := range cases {
		if got := responsesJSON(project, testCase.tag); got != testCase.want {
			t.Errorf("tag %q\n got %s\nwant %s", testCase.tag, got, testCase.want)
		}
	}
}

// 登录报文优先用 testdata 里的 next22 样本，缺失才回退 runtime/login_ok.bin。
func TestLoginResponseBinPrefersTheNext22Sample(t *testing.T) {
	project := t.TempDir()
	if got := LoginResponseBin(project); got != filepath.Join(project, "runtime", "login_ok.bin") {
		t.Errorf("缺失样本时 = %q，want runtime/login_ok.bin", got)
	}
	sample := filepath.Join(project, "cmd", "wireprobe", "testdata", "login-normal22.bin")
	if err := os.MkdirAll(filepath.Dir(sample), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sample, []byte("sample"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := LoginResponseBin(project); got != sample {
		t.Errorf("存在样本时 = %q，want %q", got, sample)
	}
}

// pythonJSONString 的期望值由 Python 的 json.dumps（默认 ensure_ascii=True）产出。
func TestPythonJSONStringMatchesJsonDumps(t *testing.T) {
	cases := []struct{ value, want string }{
		{`C:\Game\dof\115us\115\server\work\dfo-lan\runtime\precheck_ok.bin`,
			`"C:\\Game\\dof\\115us\\115\\server\\work\\dfo-lan\\runtime\\precheck_ok.bin"`},
		{"a\"b\\c", `"a\"b\\c"`},
		{"tab\tnl\nq", `"tab\tnl\nq"`},
		{"中文路径", `"\u4e2d\u6587\u8def\u5f84"`},
		{"\U0001F600x", `"\ud83d\ude00x"`},
		{"ctrl\x01end", `"ctrl\u0001end"`},
	}
	for _, testCase := range cases {
		if got := pythonJSONString(testCase.value); got != testCase.want {
			t.Errorf("pythonJSONString(%q) = %s, want %s", testCase.value, got, testCase.want)
		}
	}
}

// 环境变量覆盖登录应答时要改在"1"这一格上，且不能把其它键挪位。
func TestOverrideLoginResponseKeepsTheKeyOrder(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses.json")
	if err := os.WriteFile(responses, []byte(responsesJSON(`C:\root`, "roles_x")), 0o644); err != nil {
		t.Fatal(err)
	}
	override := filepath.Join(dir, "custom.bin")
	if err := os.WriteFile(override, []byte("bin"), 0o644); err != nil {
		t.Fatal(err)
	}
	OverrideLoginResponse(responses, `C:\root`, override)
	data, err := os.ReadFile(responses)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"1554": "C:\\root\\runtime\\precheck_ok.bin", "1": "` +
		strings.ReplaceAll(override, `\`, `\\`) +
		`", "8": "C:\\root\\runtime\\characters_ok.bin", "684": "C:\\root\\runtime\\name_ok.bin"}`
	if string(data) != want {
		t.Errorf("覆盖后 = %s\nwant %s", data, want)
	}
}

// 覆盖目标不存在时保持原样（Python 的 except Exception: pass）。
func TestOverrideLoginResponseIgnoresMissingTarget(t *testing.T) {
	dir := t.TempDir()
	responses := filepath.Join(dir, "responses.json")
	original := responsesJSON(`C:\root`, "roles_x")
	if err := os.WriteFile(responses, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	OverrideLoginResponse(responses, `C:\root`, filepath.Join(dir, "missing.bin"))
	data, err := os.ReadFile(responses)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != original {
		t.Errorf("文件被改动了：%s", data)
	}
}

// 四个文件必须一次写全，且换行风格与 Python 版一致。
func TestWriteSessionFixturesWritesAllFourFiles(t *testing.T) {
	project := t.TempDir()
	out := filepath.Join(project, "runtime", "roles_x")
	if _, err := WriteSessionFixtures(project, out, "roles_x"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"channelinfo.bin", "fixture.json", "breakpoints.txt", "responses.json"} {
		if _, err := os.Stat(filepath.Join(out, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	fixtureBytes, err := os.ReadFile(filepath.Join(out, "channelinfo.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if got := sha256Of(t, fixtureBytes); got != historicalChannelInfoSHA256 {
		t.Errorf("落盘的 channelinfo.bin sha256 = %s", got)
	}
	responses, err := os.ReadFile(filepath.Join(out, "responses.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(responses), "\r") || strings.Contains(string(responses), "\n") {
		t.Errorf("responses.json 不该有换行：%q", responses)
	}
}
