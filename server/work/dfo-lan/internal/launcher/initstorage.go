package launcher

// 首次初始化本地存储（数据库自举）。
//
// 这是 scripts/bootstrap_local.py（45 行）的 Go 复刻，也是"启动链去 Python"的最后一块
// 运行期依赖：业主 2026-10-05 决策"最小预装环境、不需要外部依赖、没有 python、不要回退路线"，
// 初始化不再由 Python 脚本承担，改由 `dfolauncher init-storage` 承担；相邻启动器
// （115us-dfolauncher/internal/storeboot）改调本子命令。
//
// 与原脚本的逐条对应（行号指 bootstrap_local.py）：
//
//	:8-9   runtime = <包根>/server/work/dfo-lan/runtime/storage，mkdir(parents=True)
//	:10-11 守卫：runtime/local.json 或 runtime/pgdata 已存在即拒绝（绝不覆盖、绝不删）
//	:12    端口占用自检：能 bind 127.0.0.1:<port> 才算空闲
//	:14-16 依赖文件：<postgres-bin>/{initdb,pg_ctl,createdb}.exe 必须存在
//	:17-19 密码 32 字节 URL-safe 随机 → runtime/initdb-password.tmp
//	:21-25 initdb -D pgdata -U dfo_owner --pwfile … ；stdout+stderr 全量写 initdb.log；
//	       非 0 → 报错并指向该日志；无论如何删掉 pwfile
//	:26-27 追加 postgresql.conf（listen_addresses/port/max_connections/shared_buffers）
//	:28-29 pg_ctl -D pgdata -l postgres.log -w start；输出追加到 pg-control.log
//	:30-31 PGPASSWORD=<随机> 后 createdb -h 127.0.0.1 -p <port> -U dfo_owner dfo_lan
//	:32-33 写 runtime/local.json（json.dumps(indent=2) 的口径）
//	:34    成功时向 stdout 打印 {"config_path":…,"postgres_port":…}
//
// 三处刻意的偏离（都在函数的注释里写明，且不影响可观察产物）：
//  1. 错误文案按本仓库习惯用中文（守卫那一句保留原英文，便于与 Python 版对照）；
//  2. initdb 失败时**不删**已生成的 pgdata——那是数据目录、宁可保留证据，但要明确提示
//     "pgdata 可能已部分生成，重试前请人工确认"（原脚本也不删）；
//  3. 没有内部超时（原脚本也没有）：超时由调用方（启动器的 180 秒上下文）负责。
import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// InitStorageDefaultPort 是 --postgres-port 的缺省值，与 bootstrap_local.py 的
// argparse 默认值（25438）以及 internal/launcher 的 PostgresPort 一致。
const InitStorageDefaultPort = 25438

// initdb 建出来的账号与库名，一字不差地取自原脚本。
const (
	initStorageUser     = "dfo_owner"
	initStorageDatabase = "dfo_lan"
)

// InitStorageOptions 是 init-storage 子命令的参数。
type InitStorageOptions struct {
	Root         string // 整合包根（含 server/ 那一层）
	PostgresBin  string // PostgreSQL 可执行文件目录；空 = 整合包内的便携运行时
	PostgresPort int    // 0 = InitStorageDefaultPort
	DryRun       bool   // 只打印将要执行的步骤，不落盘、不起进程
}

// InitStorageResult 是成功时打印到 stdout 的 JSON（与原脚本 print(json.dumps(...)) 同形）。
//
// 字段顺序必须与 Python 的 dict 字面量一致（config_path 在前、postgres_port 在后）：
// 启动器按行解析这一行，顺序不影响解析，但逐字节对照时它算证据。
type InitStorageResult struct {
	ConfigPath   string `json:"config_path"`
	PostgresPort int    `json:"postgres_port"`
}

// initStoragePaths 是一次初始化涉及的全部落点（都由 runtime 目录推导）。
type initStoragePaths struct {
	Module       string // <包根>/server/work/dfo-lan
	Runtime      string // …/runtime/storage
	Data         string // …/runtime/storage/pgdata
	Config       string // …/runtime/storage/local.json
	PasswordFile string // …/runtime/storage/initdb-password.tmp
	InitdbLog    string // …/runtime/storage/initdb.log
	PostgresConf string // …/runtime/storage/pgdata/postgresql.conf
	PostgresLog  string // …/runtime/storage/postgres.log
	ControlLog   string // …/runtime/storage/pg-control.log
}

// initStoragePlan 是"将要执行什么"的完整决定：纯计算，不落盘、不起进程。
// 真正的执行（InitStorage）与 --dry-run 都用它，两者因此不可能走偏。
type initStoragePlan struct {
	Paths       initStoragePaths
	PostgresBin string
	Port        int
	Initdb      string
	PgCtl       string
	CreateDB    string
	Steps       []string
}

// InitStorage 执行首次初始化，返回成功时该打印的结果。
//
// 守卫（已存在配置/端口被占/缺依赖）在执行任何写入之前完成，与 Python 的顺序一致；
// logf 为进度输出（调用方决定去 stdout 还是 stderr），可为 nil。
func InitStorage(ctx context.Context, opt InitStorageOptions, logf func(string, ...any)) (InitStorageResult, error) {
	if logf == nil {
		logf = func(string, ...any) {}
	}
	var res InitStorageResult
	plan, err := PlanInitStorage(opt)
	if err != nil {
		return res, err
	}
	if opt.DryRun {
		for _, step := range plan.Steps {
			logf("would %s", step)
		}
		return InitStorageResult{ConfigPath: plan.Paths.Config, PostgresPort: plan.Port}, nil
	}

	// :8-9 runtime.mkdir(parents=True, exist_ok=True)
	if err := os.MkdirAll(plan.Paths.Runtime, 0o755); err != nil {
		return res, fmt.Errorf("创建存储目录失败（%s）：%w", plan.Paths.Runtime, err)
	}

	// :17-19 密码：与 secrets.token_urlsafe(32) 同口径（32 字节随机、base64url、无填充）。
	password, err := tokenURLSafe(32)
	if err != nil {
		return res, err
	}
	if err := os.WriteFile(plan.Paths.PasswordFile, []byte(password), 0o600); err != nil {
		return res, fmt.Errorf("写入临时密码文件失败（%s）：%w", plan.Paths.PasswordFile, err)
	}
	// :25 finally: passwordfile.unlink(missing_ok=True) —— 无论 initdb 成败都要删。
	// （唯一偏离：unlink 本身失败时原脚本会抛 OSError 让整次初始化报错，Go 侧只提醒不失败：
	// 那种情况只在文件被别的进程占用时出现，且数据库此时已经建好了。）
	defer func() {
		if err := os.Remove(plan.Paths.PasswordFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			logf("警告：临时密码文件没能删掉（%s）：%v", plan.Paths.PasswordFile, err)
		}
	}()

	// :21-25 initdb
	logf("初始化数据库集群：%s -D %s -U %s …", plan.Initdb, plan.Paths.Data, initStorageUser)
	if err := runInitdb(ctx, plan); err != nil {
		return res, err
	}

	// :26-27 追加 postgresql.conf
	if err := appendPostgresConf(plan); err != nil {
		return res, err
	}

	// :28-29 起库
	logf("启动数据库：%s -D %s -l %s -w start", plan.PgCtl, plan.Paths.Data, plan.Paths.PostgresLog)
	if err := startInitdbPostgres(ctx, plan); err != nil {
		return res, err
	}

	// :30-31 建库
	logf("建库：%s -h 127.0.0.1 -p %d -U %s %s", plan.CreateDB, plan.Port, initStorageUser, initStorageDatabase)
	if err := createInitdbDatabase(ctx, plan, password); err != nil {
		return res, err
	}

	// :32-33 写配置
	res = InitStorageResult{ConfigPath: plan.Paths.Config, PostgresPort: plan.Port}
	if err := writeStorageConfig(plan, password); err != nil {
		return res, err
	}
	return res, nil
}

// PlanInitStorage 决定初始化要做什么，并完成全部**只读**前置检查：
// 守卫（已存在配置）、端口占用自检、依赖文件检查。--dry-run 与真正的执行共用它。
//
// 顺序与 Python 一致（守卫 → 端口 → 依赖），因此报错时给出的第一条原因也一致。
func PlanInitStorage(opt InitStorageOptions) (initStoragePlan, error) {
	var plan initStoragePlan

	root := opt.Root
	if root == "" {
		root = "."
	}
	absolute, err := filepath.Abs(root)
	if err != nil {
		return plan, fmt.Errorf("解析根目录失败（%s）：%w", root, err)
	}
	module := filepath.Join(absolute, "server", "work", "dfo-lan")
	runtime := filepath.Join(module, "runtime", "storage")
	data := filepath.Join(runtime, "pgdata")

	plan = initStoragePlan{
		Paths: initStoragePaths{
			Module:       module,
			Runtime:      runtime,
			Data:         data,
			Config:       filepath.Join(runtime, "local.json"),
			PasswordFile: filepath.Join(runtime, "initdb-password.tmp"),
			InitdbLog:    filepath.Join(runtime, "initdb.log"),
			PostgresConf: filepath.Join(data, "postgresql.conf"),
			PostgresLog:  filepath.Join(runtime, "postgres.log"),
			ControlLog:   filepath.Join(runtime, "pg-control.log"),
		},
	}

	// :14 pgbin = pathlib.Path(postgres_bin).resolve()
	bin := opt.PostgresBin
	if strings.TrimSpace(bin) == "" {
		bin = DefaultPostgresBin(absolute)
	}
	if !filepath.IsAbs(bin) {
		// 相对路径按调用者的工作目录解析，与原脚本 Path(...).resolve() 一致。
		if resolved, err := filepath.Abs(bin); err == nil {
			bin = resolved
		}
	}
	plan.PostgresBin = filepath.Clean(bin)

	plan.Port = initStoragePort(opt.PostgresPort)
	// 与 Python 的 socket.bind 对越界端口报错等价（bind 自己就会拒绝 0-65535 之外的值）。
	if plan.Port < 1 || plan.Port > 65535 {
		return plan, fmt.Errorf("端口超出范围：%d（应在 1-65535）", plan.Port)
	}

	// :10-11 守卫：**绝不**覆盖已有配置，也绝不碰既有 pgdata（那是玩家存档）。
	if existing := existingStorageConfiguration(plan.Paths); existing != "" {
		return plan, fmt.Errorf("Storage configuration already exists; "+
			"use it to inspect/restart rather than reinitialize（已存在：%s）", existing)
	}

	// :12 端口占用自检：成功 bind 才算空闲。这里用 bind 而不是"拨号探测"：
	// 拨号只能发现"有人在监听"，而原脚本的口径是"这个地址能不能被占用"。
	if err := checkPortBindable(plan.Port); err != nil {
		return plan, err
	}

	// :15-16 依赖文件
	plan.Initdb = filepath.Join(plan.PostgresBin, "initdb.exe")
	plan.PgCtl = filepath.Join(plan.PostgresBin, "pg_ctl.exe")
	plan.CreateDB = filepath.Join(plan.PostgresBin, "createdb.exe")
	for _, dependency := range []string{plan.Initdb, plan.PgCtl, plan.CreateDB} {
		if !regularFile(dependency) {
			return plan, fmt.Errorf("Missing dependency: %s（PostgreSQL 便携运行时可能不完整："+
				"可用 --postgres-bin 指定其它目录）", dependency)
		}
	}

	plan.Steps = initStorageSteps(plan)
	return plan, nil
}

// initStorageSteps 是给 --dry-run 打印的步骤清单（也用来解释"这次到底会做什么"）。
func initStorageSteps(plan initStoragePlan) []string {
	return []string{
		"create " + plan.Paths.Runtime,
		fmt.Sprintf("run %s -D %s -U %s --pwfile %s --auth-host=scram-sha-256 "+
			"--auth-local=scram-sha-256 --encoding=UTF8 --locale=C（stdout+stderr 全量写入 %s，随即删除 --pwfile）",
			plan.Initdb, plan.Paths.Data, initStorageUser, plan.Paths.PasswordFile, plan.Paths.InitdbLog),
		fmt.Sprintf("append to %s: listen_addresses = '127.0.0.1' / port = %d / max_connections = 30 / shared_buffers = '64MB'",
			plan.Paths.PostgresConf, plan.Port),
		fmt.Sprintf("run %s -D %s -l %s -w start（pg_ctl 自身输出追加到 %s）",
			plan.PgCtl, plan.Paths.Data, plan.Paths.PostgresLog, plan.Paths.ControlLog),
		fmt.Sprintf("run %s -h 127.0.0.1 -p %d -U %s %s（PGPASSWORD=初始化密码）",
			plan.CreateDB, plan.Port, initStorageUser, initStorageDatabase),
		fmt.Sprintf("write %s（postgres_dsn / max_connections=12 / postgres_bin / postgres_data）", plan.Paths.Config),
	}
}

// initStoragePort 解析 --postgres-port 的缺省：0 表示"用默认端口"，
// 与原脚本 argparse 的 default=25438 同义（相邻启动器的 Options 也按这个口径传值）。
func initStoragePort(port int) int {
	if port == 0 {
		return InitStorageDefaultPort
	}
	return port
}

// existingStorageConfiguration 对应原脚本的 :10：local.json 或 pgdata **存在**即冲突。
// 目录存在性（而不是 PG_VERSION）是刻意的：上次初始化中途失败留下的空 pgdata 目录也算
// "已经动过"，与 storeboot.Detect 的 stale 判定同源。
func existingStorageConfiguration(paths initStoragePaths) string {
	if pathExists(paths.Config) {
		return paths.Config
	}
	if pathExists(paths.Data) {
		return paths.Data
	}
	return ""
}

// checkPortBindable 复刻原脚本的 `with socket.socket() as s: s.bind(('127.0.0.1', port))`：
// 绑得上才算空闲。Go 在 Windows 上不设 SO_REUSEADDR，所以"已被监听"会返回 WSAEADDRINUSE。
func checkPortBindable(port int) error {
	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if err != nil {
		return fmt.Errorf("端口 %d 已被占用（127.0.0.1），请先停止占用它的程序或用 --postgres-port 换一个端口：%w",
			port, err)
	}
	return listener.Close()
}

// tokenURLSafe 是 secrets.token_urlsafe(n) 的等价物：n 字节随机 → base64url（去掉 '=' 填充）。
// n=32 时得到 43 个字符（字母数字与 -_），与原脚本写进 DSN 的密码同口径。
func tokenURLSafe(n int) (string, error) {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("生成数据库密码失败：%w", err)
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// runInitdb 跑 initdb，并把 stdout+stderr **全量**写进 initdb.log（原脚本的 write_bytes）。
//
// 失败时：指向日志、带上日志尾部，并明确"pgdata 可能已部分生成"——原脚本同样不清理 pgdata，
// 因为那是数据目录，宁可留下证据也不能替玩家删。
func runInitdb(ctx context.Context, plan initStoragePlan) error {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, plan.Initdb,
		"-D", plan.Paths.Data,
		"-U", initStorageUser,
		"--pwfile", plan.Paths.PasswordFile,
		"--auth-host=scram-sha-256",
		"--auth-local=scram-sha-256",
		"--encoding=UTF8",
		"--locale=C")
	hideConsoleWindow(cmd)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	// Python 是 `write_bytes(result.stdout + result.stderr)`：两股输出按"先 stdout 后 stderr"
	// 拼接、覆盖写。Go 的两个 buffer 顺序相同，不追求与终端上的交错顺序一致。
	combined := append(append([]byte{}, stdout.Bytes()...), stderr.Bytes()...)
	if err := os.WriteFile(plan.Paths.InitdbLog, combined, 0o644); err != nil {
		return fmt.Errorf("写入初始化日志失败（%s）：%w", plan.Paths.InitdbLog, err)
	}
	if runErr != nil {
		return fmt.Errorf("initdb 失败：%v（日志：%s）%s\n"+
			"pgdata 可能已部分生成，重试前请人工确认（启动器不会替你删数据目录）：%s",
			runErr, plan.Paths.InitdbLog, fileTailHint(plan.Paths.InitdbLog), plan.Paths.Data)
	}
	return nil
}

// appendPostgresConf 追加监听地址/端口/连接数/共享内存四行。
//
// 注意 CRLF：原脚本用文本模式 open('a')，Windows 上 '\n' 会被翻成 '\r\n'（initdb 自己写的
// postgresql.conf 也是 CRLF）。Go 的 WriteString 不做翻译，所以这里显式写 CRLF，
// 保证与 Python 版逐字节一致。
func appendPostgresConf(plan initStoragePlan) error {
	conf := fmt.Sprintf("\nlisten_addresses = '127.0.0.1'\nport = %d\nmax_connections = 30\nshared_buffers = '64MB'\n",
		plan.Port)
	conf = strings.ReplaceAll(conf, "\n", "\r\n")
	file, err := os.OpenFile(plan.Paths.PostgresConf, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开 %s 失败：%w", plan.Paths.PostgresConf, err)
	}
	defer file.Close()
	if _, err := file.WriteString(conf); err != nil {
		return fmt.Errorf("写入 %s 失败：%w", plan.Paths.PostgresConf, err)
	}
	return nil
}

// startInitdbPostgres 跑 `pg_ctl -D pgdata -l postgres.log -w start`，pg_ctl 自身的输出
// 追加到 pg-control.log。
//
// **不能**用 CombinedOutput（2026-10-05 实机挂死）：pg_ctl start 会拉起 postgres，而 postgres
// 继承 pg_ctl 的 stdout/stderr 管道并长期持有，读端永远等不到 EOF。写成文件既躲开死锁，
// 也与原脚本（stdout=control_log, stderr=control_log）一致。
func startInitdbPostgres(ctx context.Context, plan initStoragePlan) error {
	control, err := os.OpenFile(plan.Paths.ControlLog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("打开 pg_ctl 日志失败（%s）：%w", plan.Paths.ControlLog, err)
	}
	defer control.Close()

	cmd := exec.CommandContext(ctx, plan.PgCtl,
		"-D", plan.Paths.Data,
		"-l", plan.Paths.PostgresLog,
		"-w", "start")
	hideConsoleWindow(cmd)
	cmd.Stdout = control
	cmd.Stderr = control
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_ctl start 失败：%v%s（见 %s）", err,
			fileTailHint(plan.Paths.ControlLog, plan.Paths.PostgresLog), plan.Paths.ControlLog)
	}
	return nil
}

// createInitdbDatabase 以初始化密码建库；PGPASSWORD 走环境变量（原脚本 copy os.environ 再赋值）。
func createInitdbDatabase(ctx context.Context, plan initStoragePlan, password string) error {
	cmd := exec.CommandContext(ctx, plan.CreateDB,
		"-h", "127.0.0.1",
		"-p", strconv.Itoa(plan.Port),
		"-U", initStorageUser,
		initStorageDatabase)
	hideConsoleWindow(cmd)
	// Go 的 exec 对环境里的重复键取最后一个，所以这里的 PGPASSWORD 覆盖继承来的同名变量。
	cmd.Env = append(os.Environ(), "PGPASSWORD="+password)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("createdb 失败：%v\n%s", err, tailText(string(out), 12))
	}
	return nil
}

// storageLocalConfig 是写进 runtime/storage/local.json 的内容。
//
// 字段顺序就是原脚本 dict 字面量的顺序（json.dumps 保留插入顺序），
// 这样 Go 与 Python 写出来的文件在结构上完全一致。
type storageLocalConfig struct {
	PostgresDSN    string `json:"postgres_dsn"`
	MaxConnections int    `json:"max_connections"`
	PostgresBin    string `json:"postgres_bin"`
	PostgresData   string `json:"postgres_data"`
}

// writeStorageConfig 写 local.json，口径与 json.dumps(config, indent=2) 一致：
// 两空格缩进、": " 分隔、无尾随换行；Windows 上原脚本的文本模式还会把 '\n' 翻成 '\r\n'，
// 这里同样显式写 CRLF。
//
// 唯一无法 1:1 的地方：Python 默认 ensure_ascii=True（非 ASCII 字符写成 \uXXXX），
// Go 直接写 UTF-8。两者被任何 JSON 解析器读出来都是同一个字符串，对 local.json 的语义没有影响
// （它只被 Go/Python 的 json 读）。
func writeStorageConfig(plan initStoragePlan, password string) error {
	cfg := storageLocalConfig{
		PostgresDSN: fmt.Sprintf("postgres://%s:%s@127.0.0.1:%d/%s?sslmode=disable",
			initStorageUser, password, plan.Port, initStorageDatabase),
		MaxConnections: 12,
		PostgresBin:    plan.PostgresBin,
		PostgresData:   plan.Paths.Data,
	}
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	// 关掉 HTML 转义：Python 不会把 & < > 写成 \u00xx，路径里出现这些字符时两边才一致。
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("序列化存储配置失败：%w", err)
	}
	body := bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
	body = []byte(strings.ReplaceAll(string(body), "\n", "\r\n"))
	if err := os.WriteFile(plan.Paths.Config, body, 0o644); err != nil {
		return fmt.Errorf("写入存储配置失败（%s）：%w", plan.Paths.Config, err)
	}
	return nil
}

// DefaultPostgresBin 给出 --postgres-bin 缺省时的取值：整合包内的便携 PostgreSQL。
//
// 这是相邻启动器 internal/toolpath.PgBin 的等价物（本仓库没有 toolpath 包），候选顺序照抄它：
//
//	$DFO_TOOLS/pg/pgsql/bin → <包根>/tools/pg/pgsql/bin → <包根>/../tools/pg/pgsql/bin
//
// 第二条以上的存在性判断按"这个目录在不在"（与 toolpath.In 相同）：整合包可能把整个 tools/
// 放在包根外一层（2026-10-04 实测布局：包根 C:\Game\dof\115us\115，运行时 C:\Game\dof\115us\tools）。
// 一条都不存在时返回标准位置，好让调用方的报错指向玩家该去看的地方。
func DefaultPostgresBin(root string) string {
	standard := ""
	if root != "" {
		standard = filepath.Join(root, "tools", "pg", "pgsql", "bin")
	}
	if tools := strings.TrimSpace(os.Getenv("DFO_TOOLS")); tools != "" {
		if candidate := filepath.Join(tools, "pg", "pgsql", "bin"); pathExists(candidate) {
			return candidate
		}
	}
	if standard != "" && pathExists(standard) {
		return standard
	}
	if root != "" {
		if candidate := filepath.Join(root, "..", "tools", "pg", "pgsql", "bin"); pathExists(candidate) {
			return candidate
		}
	}
	if standard != "" {
		return standard
	}
	return filepath.Join("pg", "pgsql", "bin")
}

// ---------- 内部工具 ----------

// fileTailHint 取若干日志文件的尾部若干行，拼成给玩家看的提示（都读不到就返回空串）。
// 与 launch/stop 的报错风格一致：失败要能直接从错误信息里看到线索，而不只是"失败了"。
func fileTailHint(paths ...string) string {
	var parts []string
	for _, path := range paths {
		body, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if tail := tailText(strings.TrimSpace(string(body)), 8); tail != "" {
			parts = append(parts, fmt.Sprintf("%s 尾部：\n%s", filepath.Base(path), tail))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return "\n" + strings.Join(parts, "\n")
}

// tailText 取文本最后 n 行（不足则全取）。
func tailText(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}
