package launcher

// 真机自举验收（默认跳过）：在**临时 root** 上跑一次完整的 initdb → pg_ctl start →
// createdb → local.json，然后回读产物、用 psql 连一次、最后把库停掉。
//
// 为什么要有它：这套逻辑的唯一价值就是"真的能把库建起来"，而纯单测只能证明分支。
// 真实环境（玩家的 runtime/storage）**绝不能被碰**，所以这里要求调用方显式给出临时 root：
//
//	DFO_INIT_STORAGE_TEST_ROOT=D:\tmp\initstore-test        ← 必须是干净目录（没有 local.json/pgdata）
//	DFO_INIT_STORAGE_PG_BIN=C:\Game\dof\115us\tools\pg\pgsql\bin
//	DFO_INIT_STORAGE_PORT=25539                             ← 可选，缺省 25539（避开真实环境的 25438）
//
// 测试自己会 `pg_ctl stop` 收尾（否则临时目录删不掉、端口也会一直占着）。

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestInitStorageLive(t *testing.T) {
	root := strings.TrimSpace(os.Getenv("DFO_INIT_STORAGE_TEST_ROOT"))
	bin := strings.TrimSpace(os.Getenv("DFO_INIT_STORAGE_PG_BIN"))
	if root == "" || bin == "" {
		t.Skip("DFO_INIT_STORAGE_TEST_ROOT / DFO_INIT_STORAGE_PG_BIN 未设置，跳过真机自举测试")
	}
	port := 25539
	if raw := strings.TrimSpace(os.Getenv("DFO_INIT_STORAGE_PORT")); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			t.Fatalf("DFO_INIT_STORAGE_PORT 不是整数：%q", raw)
		}
		port = value
	}

	// 安全护栏（这一条比测试本身重要）：临时 root 必须是干净的，且**不是**任何已有的存储目录。
	storage := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
	if pathExists(filepath.Join(storage, "local.json")) || pathExists(filepath.Join(storage, "pgdata")) {
		t.Fatalf("临时 root 不干净（已有 local.json 或 pgdata）：%s", storage)
	}

	result, err := InitStorage(t.Context(), InitStorageOptions{
		Root: root, PostgresBin: bin, PostgresPort: port,
	}, t.Logf)
	if err != nil {
		t.Fatalf("初始化失败：%v", err)
	}

	// 收尾：停掉本次初始化起的库。注册得比断言早，断言失败也照样收尾。
	t.Cleanup(func() {
		pgCtl := filepath.Join(bin, "pg_ctl.exe")
		data := filepath.Join(storage, "pgdata")
		cmd := exec.Command(pgCtl, "-D", data, "-m", "fast", "-w", "-t", "30", "stop")
		hideConsoleWindow(cmd)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("收尾时停库失败（请手工检查 %s）：%v\n%s", data, err, out)
		} else {
			t.Logf("已停止临时数据库：%s", data)
		}
	})

	// 1) 返回值与原脚本的 stdout 契约一致
	if result.ConfigPath != filepath.Join(storage, "local.json") {
		t.Errorf("config_path = %q，期望 %q", result.ConfigPath, filepath.Join(storage, "local.json"))
	}
	if result.PostgresPort != port {
		t.Errorf("postgres_port = %d，期望 %d", result.PostgresPort, port)
	}

	// 2) local.json 与 Python 版同构：四个字段、值对得上
	body, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("读 local.json：%v", err)
	}
	var cfg storageLocalConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		t.Fatalf("解析 local.json：%v", err)
	}
	parsed, err := url.Parse(cfg.PostgresDSN)
	if err != nil {
		t.Fatalf("postgres_dsn 不是合法 URL：%v", err)
	}
	if parsed.Scheme != "postgres" || parsed.Hostname() != "127.0.0.1" || parsed.Port() != strconv.Itoa(port) {
		t.Errorf("postgres_dsn 指向别处：%s", parsed.Redacted())
	}
	if parsed.Path != "/dfo_lan" {
		t.Errorf("postgres_dsn 的库名 = %q，期望 /dfo_lan", parsed.Path)
	}
	if parsed.User == nil || parsed.User.Username() != initStorageUser {
		t.Errorf("postgres_dsn 的账号不对：%v", parsed.User)
	}
	password, _ := parsed.User.Password()
	if len(password) != 43 {
		t.Errorf("随机密码长度 = %d，期望 43（token_urlsafe(32)）", len(password))
	}
	if cfg.MaxConnections != 12 {
		t.Errorf("max_connections = %d，期望 12", cfg.MaxConnections)
	}
	if cfg.PostgresBin != filepath.Clean(bin) {
		t.Errorf("postgres_bin = %q，期望 %q", cfg.PostgresBin, filepath.Clean(bin))
	}
	if cfg.PostgresData != filepath.Join(storage, "pgdata") {
		t.Errorf("postgres_data = %q，期望 %q", cfg.PostgresData, filepath.Join(storage, "pgdata"))
	}
	if _, err := os.Stat(filepath.Join(storage, "initdb-password.tmp")); !os.IsNotExist(err) {
		t.Errorf("临时密码文件没删掉：%v", err)
	}

	// 3) 日志落点必须都有内容（initdb.log / postgres.log 是排查现场的第一手材料）
	for _, name := range []string{"initdb.log", "postgres.log", "pg-control.log"} {
		info, err := os.Stat(filepath.Join(storage, name))
		if err != nil {
			t.Errorf("缺少日志 %s：%v", name, err)
			continue
		}
		if info.Size() == 0 {
			t.Errorf("日志 %s 是空的", name)
		}
	}

	// 4) 真的能连上刚建出来的库（createdb 建的是 dfo_lan，不是 postgres）
	psql := filepath.Join(bin, "psql.exe")
	if !regularFile(psql) {
		t.Skipf("没有 psql.exe，跳过连接复核：%s", psql)
	}
	checkCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(checkCtx, psql, cfg.PostgresDSN, "-tA", "-c", "select current_database()")
	hideConsoleWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("psql 连接临时库失败：%v\n%s", err, out)
	}
	if got := strings.TrimSpace(string(out)); got != initStorageDatabase {
		t.Errorf("current_database() = %q，期望 %q", got, initStorageDatabase)
	}
}
