package launcher

import (
	"encoding/base64"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeStubFile 造一个占位文件（内容无关紧要，只要"是文件"）。
func writeStubFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("stub"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// stubPostgresBin 造一份"看起来完整"的 PostgreSQL bin 目录（三个依赖文件都在）。
func stubPostgresBin(t *testing.T) string {
	t.Helper()
	bin := t.TempDir()
	for _, name := range []string{"initdb.exe", "pg_ctl.exe", "createdb.exe"} {
		writeStubFile(t, filepath.Join(bin, name))
	}
	return bin
}

// freePort 找一个当前没人监听的端口（拿不到就跳过：环境不允许绑定回环）。
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法绑定回环端口：%v", err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

// 守卫：local.json 或 pgdata 已存在时必须拒绝——**绝不**覆盖配置，也**绝不**删数据目录。
// 报错保留 Python 的原句，便于与 bootstrap_local.py 对照。
func TestPlanInitStorageRefusesExistingConfiguration(t *testing.T) {
	for _, existing := range []string{"local.json", "pgdata"} {
		root := t.TempDir()
		target := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", existing)
		if existing == "pgdata" {
			if err := os.MkdirAll(target, 0o755); err != nil {
				t.Fatal(err)
			}
			writeStubFile(t, filepath.Join(target, "PG_VERSION"))
		} else {
			writeStubFile(t, target)
		}
		_, err := PlanInitStorage(InitStorageOptions{Root: root, PostgresBin: stubPostgresBin(t)})
		if err == nil {
			t.Fatalf("%s 已存在时仍允许初始化", existing)
		}
		if !strings.Contains(err.Error(), "Storage configuration already exists") {
			t.Errorf("守卫报错丢了原句：%v", err)
		}
		if !strings.Contains(err.Error(), target) {
			t.Errorf("守卫报错没指出是哪个路径：%v", err)
		}
	}
}

// 依赖检查：三个可执行文件缺一不可，报错必须点出缺的是哪个（原脚本 "Missing dependency: …"）。
func TestPlanInitStorageRefusesMissingDependency(t *testing.T) {
	root := t.TempDir()
	bin := stubPostgresBin(t)
	if err := os.Remove(filepath.Join(bin, "createdb.exe")); err != nil {
		t.Fatal(err)
	}
	_, err := PlanInitStorage(InitStorageOptions{
		Root: root, PostgresBin: bin, PostgresPort: freePort(t)})
	if err == nil {
		t.Fatal("缺少 createdb.exe 时仍允许初始化")
	}
	if !strings.Contains(err.Error(), "Missing dependency") ||
		!strings.Contains(err.Error(), "createdb.exe") {
		t.Errorf("依赖报错不明确：%v", err)
	}
}

// 端口占用自检：能 bind 才算空闲（原脚本的口径），被占用时必须先失败、不许继续往下走。
func TestPlanInitStorageRefusesBusyPort(t *testing.T) {
	root := t.TempDir()
	bin := stubPostgresBin(t)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("无法绑定回环端口：%v", err)
	}
	busy := listener.Addr().(*net.TCPAddr).Port
	defer listener.Close()

	if _, err := PlanInitStorage(InitStorageOptions{Root: root, PostgresBin: bin, PostgresPort: busy}); err == nil {
		t.Fatal("端口被占用时仍允许初始化")
	} else if !strings.Contains(err.Error(), "已被占用") {
		t.Errorf("端口报错不明确：%v", err)
	}

	// 放掉之后同一个端口必须能通过（证明失败的原因确实是占用，而不是别的）。
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := PlanInitStorage(InitStorageOptions{Root: root, PostgresBin: bin, PostgresPort: busy}); err != nil {
		t.Fatalf("同样的输入在端口空闲后被拒绝：%v", err)
	}
}

// 默认端口与路径落点：与 bootstrap_local.py 的 argparse 默认值、runtime/storage 布局一致。
//
// 端口刻意用 freePort：本机真实环境就占着 25438，"默认端口"这条用纯函数
// （initStoragePort）验证，避免让测试依赖"默认端口恰好空闲"。
func TestPlanInitStoragePathsAndDefaultPort(t *testing.T) {
	if InitStorageDefaultPort != 25438 {
		t.Errorf("默认端口常量 = %d，期望 25438", InitStorageDefaultPort)
	}
	if got := initStoragePort(0); got != 25438 {
		t.Errorf("initStoragePort(0) = %d，期望 25438", got)
	}
	if got := initStoragePort(25538); got != 25538 {
		t.Errorf("initStoragePort(25538) = %d，期望原样返回", got)
	}

	root := t.TempDir()
	plan, err := PlanInitStorage(InitStorageOptions{
		Root: root, PostgresBin: stubPostgresBin(t), PostgresPort: freePort(t)})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	storage := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
	want := map[string]string{
		"Runtime":      storage,
		"Data":         filepath.Join(storage, "pgdata"),
		"Config":       filepath.Join(storage, "local.json"),
		"PasswordFile": filepath.Join(storage, "initdb-password.tmp"),
		"InitdbLog":    filepath.Join(storage, "initdb.log"),
		"PostgresLog":  filepath.Join(storage, "postgres.log"),
		"ControlLog":   filepath.Join(storage, "pg-control.log"),
	}
	got := map[string]string{
		"Runtime":      plan.Paths.Runtime,
		"Data":         plan.Paths.Data,
		"Config":       plan.Paths.Config,
		"PasswordFile": plan.Paths.PasswordFile,
		"InitdbLog":    plan.Paths.InitdbLog,
		"PostgresLog":  plan.Paths.PostgresLog,
		"ControlLog":   plan.Paths.ControlLog,
	}
	for name, wantPath := range want {
		if got[name] != wantPath {
			t.Errorf("%s = %q，期望 %q", name, got[name], wantPath)
		}
	}
	if len(plan.Steps) != 6 {
		t.Errorf("步骤清单有 %d 条，期望 6 条：%v", len(plan.Steps), plan.Steps)
	}
}

// --dry-run 只打印、不落盘：连 runtime 目录都不该被建出来。
func TestInitStorageDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	bin := stubPostgresBin(t)
	var logged []string
	result, err := InitStorage(t.Context(), InitStorageOptions{
		Root:         root,
		PostgresBin:  bin,
		PostgresPort: freePort(t),
		DryRun:       true,
	}, func(format string, args ...any) { logged = append(logged, format) })
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if len(logged) != 6 {
		t.Errorf("dry-run 打印了 %d 条步骤，期望 6 条", len(logged))
	}
	if result.PostgresPort == 0 || result.ConfigPath == "" {
		t.Errorf("dry-run 没给出将写入的配置与端口：%+v", result)
	}
	if pathExists(filepath.Join(root, "server")) {
		t.Error("--dry-run 建出了目录")
	}
}

// DefaultPostgresBin 是相邻启动器 toolpath.PgBin 的等价物：候选顺序必须一致
// （DFO_TOOLS → <包根>/tools → <包根>/../tools → 标准位置）。
func TestDefaultPostgresBinLayouts(t *testing.T) {
	t.Setenv("DFO_TOOLS", "")

	// 标准布局
	root := t.TempDir()
	standard := filepath.Join(root, "tools", "pg", "pgsql", "bin")
	if err := os.MkdirAll(standard, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DefaultPostgresBin(root); got != standard {
		t.Errorf("标准布局 = %q，期望 %q", got, standard)
	}

	// tools 被整体移到包根外一层（2026-10-04 实测布局）
	parent := t.TempDir()
	nested := filepath.Join(parent, "115")
	if err := os.MkdirAll(filepath.Join(nested, "server"), 0o755); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(parent, "tools", "pg", "pgsql", "bin")
	if err := os.MkdirAll(moved, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := DefaultPostgresBin(nested); got != moved {
		t.Errorf("上移一层的布局 = %q，期望 %q", got, moved)
	}

	// DFO_TOOLS 显式指定优先（存在才认）
	external := t.TempDir()
	if err := os.MkdirAll(filepath.Join(external, "pg", "pgsql", "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DFO_TOOLS", external)
	if got := DefaultPostgresBin(root); got != filepath.Join(external, "pg", "pgsql", "bin") {
		t.Errorf("DFO_TOOLS 未优先：%q", got)
	}
	// 指到不存在的目录时忽略它，回到整合包内的那份
	t.Setenv("DFO_TOOLS", filepath.Join(external, "nope"))
	if got := DefaultPostgresBin(root); got != standard {
		t.Errorf("DFO_TOOLS 不存在时应忽略，实际 %q", got)
	}

	// 一条都不存在时给出标准位置（错误信息指向玩家该去看的地方）
	empty := t.TempDir()
	if got := DefaultPostgresBin(empty); got != filepath.Join(empty, "tools", "pg", "pgsql", "bin") {
		t.Errorf("都不存在时 = %q，期望标准位置", got)
	}
}

// 密码口径必须与 secrets.token_urlsafe(32) 一致：32 字节随机 → base64url（无填充，43 字符）。
func TestTokenURLSafeMatchesSecretsTokenURLSafe(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 32; i++ {
		token, err := tokenURLSafe(32)
		if err != nil {
			t.Fatalf("tokenURLSafe: %v", err)
		}
		if len(token) != 43 {
			t.Fatalf("长度 = %d，期望 43（32 字节 base64url 去掉填充）：%q", len(token), token)
		}
		if strings.ContainsAny(token, "+/=") {
			t.Errorf("出现了 URL-safe 之外的字符：%q", token)
		}
		raw, err := base64.RawURLEncoding.DecodeString(token)
		if err != nil || len(raw) != 32 {
			t.Fatalf("不是 32 字节的 RawURLEncoding：%q（%v）", token, err)
		}
		if seen[token] {
			t.Fatalf("随机密码重复：%q", token)
		}
		seen[token] = true
	}
}

// local.json 的渲染口径 = json.dumps(config, indent=2) + Windows 文本模式（CRLF），
// 且与原脚本一样**没有**尾随换行。
func TestWriteStorageConfigMatchesJSONDumpsIndent2(t *testing.T) {
	dir := t.TempDir()
	plan := initStoragePlan{
		Paths: initStoragePaths{
			Config: filepath.Join(dir, "local.json"),
			Data:   `C:\root\server\work\dfo-lan\runtime\storage\pgdata`,
		},
		PostgresBin: `C:\root\tools\pg\pgsql\bin`,
		Port:        25538,
	}
	if err := writeStorageConfig(plan, "PW-abc_123"); err != nil {
		t.Fatalf("writeStorageConfig: %v", err)
	}
	want := strings.ReplaceAll(`{
  "postgres_dsn": "postgres://dfo_owner:PW-abc_123@127.0.0.1:25538/dfo_lan?sslmode=disable",
  "max_connections": 12,
  "postgres_bin": "C:\\root\\tools\\pg\\pgsql\\bin",
  "postgres_data": "C:\\root\\server\\work\\dfo-lan\\runtime\\storage\\pgdata"
}`, "\n", "\r\n")
	body, err := os.ReadFile(plan.Paths.Config)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Errorf("local.json 与 Python 口径不一致：\n得到 %q\n期望 %q", body, want)
	}
}

// postgresql.conf 的追加内容与顺序必须与 Python 版逐字节一致（含 CRLF 与首尾空行）。
func TestAppendPostgresConfMatchesPythonAppend(t *testing.T) {
	dir := t.TempDir()
	conf := filepath.Join(dir, "postgresql.conf")
	// initdb 自己先写了一份（这里只留一小段做前缀）。
	if err := os.WriteFile(conf, []byte("max_wal_size = 1GB\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan := initStoragePlan{Paths: initStoragePaths{PostgresConf: conf}, Port: 25538}
	if err := appendPostgresConf(plan); err != nil {
		t.Fatalf("appendPostgresConf: %v", err)
	}
	// 前缀保持 initdb 写下的原样（本机实测 initdb 也写 CRLF，这里故意用 LF 以区分两段），
	// 只有追加的那一段按 Python 的文本模式翻成 CRLF。
	want := "max_wal_size = 1GB\n" + strings.ReplaceAll(
		"\nlisten_addresses = '127.0.0.1'\nport = 25538\nmax_connections = 30\nshared_buffers = '64MB'\n",
		"\n", "\r\n")
	body, err := os.ReadFile(conf)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != want {
		t.Errorf("postgresql.conf 追加内容不一致：\n得到 %q\n期望 %q", body, want)
	}
}
