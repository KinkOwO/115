package main

// storagesync.go：storage-sync 子命令 —— 双端同步（备份 / 跨引擎覆盖式复制 / 还原）。
//
// 契约（GUI 与无 GUI 排查共用同一条入口）：
//
//   - 进度写 **stderr**，stdout 只留**一行结果 JSON**（init-storage 也是这个口径，
//     相邻启动器按行解析它）；
//   - 退出码：参数错 2、执行失败 1、成功 0；失败时 stdout 上那行 JSON 仍然给出
//     {"ok":false,"error":"…"}，人看的说明同时写 stderr；
//   - --dry-run 只打印计划：计划行进 stderr，stdout 仍是那行 JSON（带 dry_run 与 plan）；
//   - 结果里的 DSN 一律打码（database.MaskDSN）—— 它会进日志面板与 UI。
//
// 硬底线（顺序不可颠倒）：
//  1. 服务端在跑（7001 在监听 / SQLite 管理租约仍在）→ 直接拒绝，请先停止游戏；
//  2. 目标端在**被覆盖前**先自动备份，备份失败即中止；
//  3. 只读写数据本身，**绝不**改 runtime/storage/local.json 与两个档位模板。

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/launcher"
)

// storageSyncTimeout 是整条同步链的上限。库文件几百 KB、库表几十张，正常几秒；
// 留 30 分钟是为了让"以后数据变大"或 pg_dump 较慢时不必改代码。
const storageSyncTimeout = 30 * time.Minute

// storageSyncResult 是 stdout 上那行 JSON 的形状。
type storageSyncResult struct {
	OK           bool                    `json:"ok"`
	Action       string                  `json:"action"`
	Root         string                  `json:"root"`
	StorageDir   string                  `json:"storage_dir"`
	BackupsDir   string                  `json:"backups_dir"`
	ActiveEngine string                  `json:"active_engine"`
	Stores       []storageSyncStoreView  `json:"stores,omitempty"`
	Backups      []database.BackupEntry  `json:"backups,omitempty"`
	Backup       *database.BackupEntry   `json:"backup,omitempty"`
	Copy         *database.CopyReport    `json:"copy,omitempty"`
	Restore      *database.RestoreReport `json:"restore,omitempty"`
	Plan         []string                `json:"plan,omitempty"`
	DryRun       bool                    `json:"dry_run,omitempty"`
	Message      string                  `json:"message,omitempty"`
	Error        string                  `json:"error,omitempty"`
}

// storageSyncStoreView 是"这一端连的是哪儿"的可读说明（打码后的）。
type storageSyncStoreView struct {
	Engine string `json:"engine"`
	Label  string `json:"label"`
	Origin string `json:"origin"`
}

// storageSyncStore 是一个引擎的存储配置，以及它是从哪个文件读到的。
type storageSyncStore struct {
	engine  string
	config  database.Config
	profile map[string]any
	origin  string
	label   string
}

func (s storageSyncStore) target() database.EngineTarget {
	if s.engine == database.DriverPostgres {
		return database.PostgresTarget(s.config)
	}
	return database.SQLiteTarget(s.config.SQLitePath)
}

func runStorageSync(args []string) int {
	flags := flag.NewFlagSet("storage-sync", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	list := flags.Bool("list", false, "列出备份与两端状态（只读，可在游戏运行时跑）")
	backup := flags.Bool("backup", false, "备份当前活动档")
	copyTo := flags.String("copy-to", "", "把另一端覆盖式复制到 sqlite|postgres")
	restore := flags.String("restore", "", "还原 backups 下的某个备份目录（覆盖它自己那一端）")
	restoreTo := flags.String("restore-to", "", "配合 --restore：还原后再复制到 sqlite|postgres")
	dryRun := flags.Bool("dry-run", false, "只打印计划，不落盘、不起进程")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	actions := 0
	for _, chosen := range []bool{*list, *backup, *copyTo != "", *restore != ""} {
		if chosen {
			actions++
		}
	}
	if actions > 1 {
		fmt.Fprintln(os.Stderr, "storage-sync: --list / --backup / --copy-to / --restore 只能选一个")
		return 2
	}
	if actions == 0 {
		fmt.Fprintln(os.Stderr, "storage-sync: 需要 --list、--backup、--copy-to 或 --restore 之一")
		return 2
	}
	if *restoreTo != "" && *restore == "" {
		fmt.Fprintln(os.Stderr, "storage-sync: --restore-to 只能配合 --restore 使用")
		return 2
	}

	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}

	result := storageSyncResult{
		Root:       absolute,
		StorageDir: storageSyncDir(absolute),
		BackupsDir: filepath.Join(storageSyncDir(absolute), database.BackupDirName),
		DryRun:     *dryRun,
	}
	switch {
	case *list:
		result.Action = "list"
	case *backup:
		result.Action = "backup"
	case *copyTo != "":
		result.Action = "copy"
	default:
		result.Action = "restore"
	}

	// 进度一律 stderr：stdout 留给那行 JSON。
	logf := func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }
	if err := storageSyncRun(absolute, &result, storageSyncOptions{
		CopyTo:    *copyTo,
		Restore:   *restore,
		RestoreTo: *restoreTo,
		DryRun:    *dryRun,
		Log:       logf,
	}); err != nil {
		result.OK = false
		result.Error = err.Error()
		fmt.Fprintf(os.Stderr, "storage-sync: %v\n", err)
		writeStorageSyncResult(result)
		return 1
	}
	result.OK = true
	writeStorageSyncResult(result)
	return 0
}

type storageSyncOptions struct {
	CopyTo    string
	Restore   string
	RestoreTo string
	DryRun    bool
	Log       func(string, ...any)
}

// writeStorageSyncResult 把结论写成 stdout 上唯一的一行 JSON。
func writeStorageSyncResult(result storageSyncResult) {
	body, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "storage-sync: 序列化结果失败：%v\n", err)
		return
	}
	fmt.Println(string(body))
}

func storageSyncRun(root string, result *storageSyncResult, options storageSyncOptions) error {
	logf := options.Log
	if logf == nil {
		logf = func(string, ...any) {}
	}
	stores, activeEngine, err := resolveStorageSyncStores(root)
	if err != nil {
		return err
	}
	result.ActiveEngine = activeEngine
	for _, engine := range []string{database.DriverSQLite, database.DriverPostgres} {
		if store, ok := stores[engine]; ok {
			result.Stores = append(result.Stores, storageSyncStoreView{
				Engine: store.engine, Label: store.label, Origin: store.origin,
			})
		}
	}
	backupsDir := result.BackupsDir
	tools, binDir := storageSyncTools(root, stores)

	switch result.Action {
	case "list":
		backups, err := database.ListBackups(backupsDir)
		if err != nil {
			return err
		}
		result.Backups = backups
		result.Message = storageSyncStatusMessage(activeEngine, stores, backups, binDir)
		if blocked := storageSyncBlocked(stores); blocked != "" {
			result.Message += "；当前不可执行同步：" + blocked
		}
		fmt.Fprintln(os.Stderr, result.Message)
		return nil

	case "backup":
		if activeEngine == "" {
			return errors.New("活动档没有可用的存储配置：请先在启动器「启动环境」里选一档")
		}
		store := stores[activeEngine]
		if blocked := storageSyncBlocked(stores); blocked != "" {
			return errors.New(blocked)
		}
		if options.DryRun {
			result.Plan = []string{
				"备份 " + store.label + " → " + filepath.Join(backupsDir, "<时间戳>-"+activeEngine),
			}
			for _, line := range result.Plan {
				logf("（dry-run）%s", line)
			}
			return nil
		}
		if err := ensurePostgresRunning(store, logf); err != nil {
			return err
		}
		if err := ensureStorageSyncTools(store, tools, binDir); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), storageSyncTimeout)
		defer cancel()
		entry, err := database.Backup(ctx, store.target(), backupsDir, tools, logf)
		if err != nil {
			return err
		}
		result.Backup = &entry
		result.Message = fmt.Sprintf("已备份 %s 到 %s（%d 张表 / %d 行）",
			store.label, entry.Directory, len(entry.Tables), entry.TotalRows)
		fmt.Fprintln(os.Stderr, result.Message)
		return nil

	case "copy":
		targetEngine, err := normalizeSyncEngine(options.CopyTo)
		if err != nil {
			return err
		}
		sourceEngine := otherSyncEngine(targetEngine)
		sourceStore, ok := stores[sourceEngine]
		if !ok {
			return fmt.Errorf("没有可用的 %s 源配置（%s）：请确认该档的配置文件在 %s 下",
				sourceEngine, syncEngineHint(sourceEngine), result.StorageDir)
		}
		targetStore, ok := stores[targetEngine]
		if !ok {
			return fmt.Errorf("没有可用的 %s 目标配置（%s）：请确认该档的配置文件在 %s 下",
				targetEngine, syncEngineHint(targetEngine), result.StorageDir)
		}
		if blocked := storageSyncBlocked(stores); blocked != "" {
			return errors.New(blocked)
		}
		if options.DryRun {
			result.Plan = []string{
				"1) 自动备份目标端 " + targetStore.label,
				"2) 把 " + sourceStore.label + " 的数据覆盖式复制到 " + targetStore.label,
			}
			for _, line := range result.Plan {
				logf("（dry-run）%s", line)
			}
			return nil
		}
		if err := ensurePostgresRunning(sourceStore, logf); err != nil {
			return err
		}
		if err := ensurePostgresRunning(targetStore, logf); err != nil {
			return err
		}
		if err := ensureStorageSyncTools(targetStore, tools, binDir); err != nil {
			return err
		}
		if err := ensureStorageSyncTools(sourceStore, tools, binDir); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), storageSyncTimeout)
		defer cancel()
		report, err := database.CopyBetweenEngines(ctx, sourceStore.target(), targetStore.target(), backupsDir, tools, logf)
		result.Copy = &report
		if err != nil {
			if report.TargetBackup != nil {
				fmt.Fprintf(os.Stderr, "（目标端覆盖前的自动备份仍在 %s，可回退）\n", report.TargetBackup.Directory)
			}
			return err
		}
		result.Message = fmt.Sprintf("已把 %s 复制到 %s（%d 张表 / %d 行）",
			sourceStore.label, targetStore.label, len(report.Tables), report.TotalRows)
		if len(report.Missing) > 0 {
			result.Message += fmt.Sprintf("；有 %d 张源端表在目标端 schema 里不存在：%s",
				len(report.Missing), strings.Join(report.Missing, "、"))
		}
		fmt.Fprintln(os.Stderr, result.Message)
		return nil

	case "restore":
		entry, err := database.FindBackup(backupsDir, options.Restore)
		if err != nil {
			return err
		}
		store, ok := stores[entry.Engine]
		if !ok {
			return fmt.Errorf("备份 %s 属于 %s，但本机没有可用的这一端配置（%s）",
				entry.Name, entry.Engine, syncEngineHint(entry.Engine))
		}
		if blocked := storageSyncBlocked(stores); blocked != "" {
			return errors.New(blocked)
		}
		plan := []string{"1) 校验备份 " + entry.Name + " 的文件哈希",
			"2) 自动备份被覆盖的 " + store.label,
			"3) 把 " + entry.Name + " 还原到 " + store.label}
		if other := strings.TrimSpace(options.RestoreTo); other != "" {
			otherEngine, err := normalizeSyncEngine(other)
			if err != nil {
				return err
			}
			if otherEngine != entry.Engine {
				if _, ok := stores[otherEngine]; !ok {
					return fmt.Errorf("没有可用的 %s 配置（%s）", otherEngine, syncEngineHint(otherEngine))
				}
				plan = append(plan, "4) 还原后再把 "+store.label+" 复制到 "+stores[otherEngine].label)
			}
		}
		if options.DryRun {
			result.Plan = plan
			for _, line := range plan {
				logf("（dry-run）%s", line)
			}
			return nil
		}
		if err := ensurePostgresRunning(store, logf); err != nil {
			return err
		}
		if err := ensureStorageSyncTools(store, tools, binDir); err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), storageSyncTimeout)
		defer cancel()
		report, err := database.RestoreBackup(ctx, entry, store.target(), backupsDir, tools, logf)
		result.Restore = &report
		if err != nil {
			if report.TargetBackup != nil {
				fmt.Fprintf(os.Stderr, "（覆盖前的自动备份仍在 %s，可回退）\n", report.TargetBackup.Directory)
			}
			return err
		}
		result.Message = fmt.Sprintf("已还原 %s 到 %s（%d 张表 / %d 行）",
			entry.Name, store.label, len(report.Tables), report.TotalRows)
		if report.Note != "" {
			result.Message += "；" + report.Note
		}
		if other := strings.TrimSpace(options.RestoreTo); other != "" {
			otherEngine, err := normalizeSyncEngine(other)
			if err != nil {
				return err
			}
			if otherEngine != entry.Engine {
				otherStore := stores[otherEngine]
				if err := ensurePostgresRunning(otherStore, logf); err != nil {
					return err
				}
				if err := ensureStorageSyncTools(otherStore, tools, binDir); err != nil {
					return err
				}
				logf("还原已完成，接着把它复制到 %s（还原 + 转换 = 两步各自先备份目标端）", otherStore.label)
				copyReport, err := database.CopyBetweenEngines(ctx, store.target(), otherStore.target(), backupsDir, tools, logf)
				result.Copy = &copyReport
				if err != nil {
					return err
				}
				result.Message += fmt.Sprintf("；随后已复制到 %s（%d 张表 / %d 行）",
					otherStore.label, len(copyReport.Tables), copyReport.TotalRows)
			}
		}
		fmt.Fprintln(os.Stderr, result.Message)
		return nil
	}
	return fmt.Errorf("未知的动作 %q", result.Action)
}

// storageSyncStatusMessage 是 --list 的人话摘要（同一句话进 stdout JSON 的 message）。
func storageSyncStatusMessage(active string, stores map[string]storageSyncStore, backups []database.BackupEntry, binDir string) string {
	var parts []string
	if active == "" {
		parts = append(parts, "活动档未命名引擎（服务端会按 SQLite 兜底）")
	} else {
		parts = append(parts, "活动档："+stores[active].label)
	}
	for _, engine := range []string{database.DriverSQLite, database.DriverPostgres} {
		if other, ok := stores[engine]; ok && engine != active {
			parts = append(parts, "另一端："+other.label)
		}
	}
	parts = append(parts, fmt.Sprintf("备份 %d 份", len(backups)), "pg 工具目录："+binDir)
	return strings.Join(parts, "；")
}

// storageSyncBlocked 返回"现在不能同步"的原因（可以同步时返回空串）。
//
// 两道闸门，任一命中就拒绝：7001 在监听（游戏在跑）；SQLite 管理租约仍在（GM 或崩溃残留）。
// 后者的判据故意保守：只要说不清持有者是否还活着，就当有人持有（见 launcher.AdminLeaseHeld）。
func storageSyncBlocked(stores map[string]storageSyncStore) string {
	if launcher.PortListening(launcher.GatewayPort, 700*time.Millisecond) {
		return fmt.Sprintf("游戏服务端正在运行（端口 %d 在监听）：请先停止游戏再做双端同步", launcher.GatewayPort)
	}
	for _, engine := range []string{database.DriverSQLite, database.DriverPostgres} {
		store, ok := stores[engine]
		if !ok {
			continue
		}
		cfg := launcher.StorageConfig{
			Driver:      engine,
			SQLitePath:  store.config.SQLitePath,
			PostgresDSN: store.config.PostgresDSN,
		}
		if held, note := launcher.AdminLeaseHeld(cfg); held {
			return fmt.Sprintf("该存储档的管理租约仍在（%s）：请先停止游戏（或等租约过期）再做双端同步", note)
		}
	}
	return ""
}

// ensurePostgresRunning 在真正打开 PostgreSQL 之前先看一眼端口，把
// "connection refused" 换成一句能照做的中文说明。
func ensurePostgresRunning(store storageSyncStore, logf func(string, ...any)) error {
	if store.engine != database.DriverPostgres {
		return nil
	}
	port := dsnPort(store.config.PostgresDSN)
	if launcher.PortListening(port, 700*time.Millisecond) {
		return nil
	}
	return fmt.Errorf("PostgreSQL 未在运行（127.0.0.1:%d 未监听）：请先「开始游戏」（它会拉起便携 PG）或用 init-storage 初始化存储", port)
}

// ensureStorageSyncTools 只在确实要碰 PostgreSQL 时才要求 pg_dump/pg_restore 在盘上。
func ensureStorageSyncTools(store storageSyncStore, tools database.Tools, binDir string) error {
	if store.engine != database.DriverPostgres {
		return nil
	}
	if strings.TrimSpace(tools.PgDump) == "" || strings.TrimSpace(tools.PgRestore) == "" {
		return fmt.Errorf("在 %s 里找不到 pg_dump.exe / pg_restore.exe：PostgreSQL 那一端需要整合包内的便携 PG", binDir)
	}
	return nil
}

// storageSyncTools 解析 pg_dump / pg_restore 的位置：先看 PG 档里写的 postgres_bin，
// 再退回整合包内的便携运行时（$DFO_TOOLS → <包根>/tools → <包根>/../tools）。
// 文件不存在时留空，由 ensureStorageSyncTools 给出可照做的报错。
func storageSyncTools(root string, stores map[string]storageSyncStore) (database.Tools, string) {
	bin := ""
	if store, ok := stores[database.DriverPostgres]; ok {
		if value, _ := store.profile["postgres_bin"].(string); strings.TrimSpace(value) != "" {
			bin = strings.TrimSpace(value)
		}
	}
	if bin == "" {
		bin = launcher.DefaultPostgresBin(root)
	}
	tools := database.Tools{
		PgDump:    filepath.Join(bin, "pg_dump.exe"),
		PgRestore: filepath.Join(bin, "pg_restore.exe"),
	}
	if !storageSyncFileExists(tools.PgDump) {
		tools.PgDump = ""
	}
	if !storageSyncFileExists(tools.PgRestore) {
		tools.PgRestore = ""
	}
	return tools, bin
}

// resolveStorageSyncStores 给出两个引擎各自的存储配置与"活动档是哪一档"。
//
// 顺序（与「启动环境」的语义一致）：
//  1. 活动档 local.json 写的就是这个引擎 → 用它，这就是"从当前活动档复制"；
//  2. 否则用该档的模板 local.<engine>.json（切档不会删模板：还原另一边、复制到另一边靠它）；
//  3. 两边都没写这个引擎 → 这一端当前不可用，如实报错，不去猜路径、更不新建配置。
func resolveStorageSyncStores(root string) (map[string]storageSyncStore, string, error) {
	dir := storageSyncDir(root)
	stores := map[string]storageSyncStore{}
	activePath := filepath.Join(dir, "local.json")
	activeProfile, activeOK := readStorageProfile(activePath)
	activeEngine := ""
	if activeOK {
		activeEngine = profileEngine(activeProfile)
	}

	for _, engine := range []string{database.DriverSQLite, database.DriverPostgres} {
		if activeOK && activeEngine == engine {
			store, err := buildStorageSyncStore(engine, activePath, activeProfile)
			if err != nil {
				return nil, activeEngine, err
			}
			stores[engine] = store
			continue
		}
		templatePath := filepath.Join(dir, "local."+engine+".json")
		profile, ok := readStorageProfile(templatePath)
		if !ok || profileEngine(profile) != engine {
			continue
		}
		store, err := buildStorageSyncStore(engine, templatePath, profile)
		if err != nil {
			// 模板存在但内容有问题（例如 sqlite_path 是相对路径）：这是"这一端不可用"，
			// 不是整条命令的失败 —— 另一端的备份/还原照样应该能做。
			fmt.Fprintf(os.Stderr, "storage-sync: 忽略 %s：%v\n", templatePath, err)
			continue
		}
		stores[engine] = store
	}
	return stores, activeEngine, nil
}

// buildStorageSyncStore 把一份档位文件变成"这一端"的配置。
func buildStorageSyncStore(engine, path string, profile map[string]any) (storageSyncStore, error) {
	config, err := database.LoadConfig(path)
	if err != nil {
		return storageSyncStore{}, err
	}
	store := storageSyncStore{engine: engine, config: config, profile: profile, origin: path}
	if engine == database.DriverSQLite {
		path := strings.TrimSpace(config.SQLitePath)
		if path == "" {
			return storageSyncStore{}, fmt.Errorf("SQLite 档缺少 sqlite_path")
		}
		if !filepath.IsAbs(path) {
			return storageSyncStore{}, fmt.Errorf("sqlite_path 必须是绝对路径（现在是 %q）：服务端会按自己的工作目录解析", path)
		}
		store.label = database.SQLiteTarget(path).Label
		return store, nil
	}
	if strings.TrimSpace(config.PostgresDSN) == "" {
		return storageSyncStore{}, fmt.Errorf("PostgreSQL 档缺少 postgres_dsn")
	}
	store.label = database.PostgresTarget(config).Label
	return store, nil
}

// readStorageProfile 读一份档位文件为对象（不存在 / 不是对象时 ok=false）。
func readStorageProfile(path string) (map[string]any, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var profile map[string]any
	if err := json.Unmarshal(trimStorageBOM(data), &profile); err != nil {
		return nil, false
	}
	return profile, true
}

// profileEngine 判断一份档位文件写的是哪个引擎；什么都没写时返回空串。
//
// 与 database.EngineForConfig 同一口径，差别只有一处：那个函数在"什么都没写"时兜底成
// SQLite（服务端的真实行为），而这里必须把"没写"与"写了 sqlite"区分开，否则一份空档会被
// 当成 SQLite 档去备份一个不存在的文件。
func profileEngine(profile map[string]any) string {
	if value, _ := profile["driver"].(string); strings.TrimSpace(value) != "" {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case database.DriverSQLite:
			return database.DriverSQLite
		case database.DriverPostgres:
			return database.DriverPostgres
		}
		return ""
	}
	if value, _ := profile["postgres_dsn"].(string); strings.TrimSpace(value) != "" {
		return database.DriverPostgres
	}
	if value, _ := profile["sqlite_path"].(string); strings.TrimSpace(value) != "" {
		return database.DriverSQLite
	}
	return ""
}

// normalizeSyncEngine 收敛 --copy-to / --restore-to 的取值。
func normalizeSyncEngine(value string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case database.DriverSQLite:
		return database.DriverSQLite, nil
	case database.DriverPostgres:
		return database.DriverPostgres, nil
	}
	return "", fmt.Errorf("未知的引擎 %q：只认 sqlite / postgres", value)
}

func otherSyncEngine(engine string) string {
	if engine == database.DriverPostgres {
		return database.DriverSQLite
	}
	return database.DriverPostgres
}

func syncEngineHint(engine string) string {
	if engine == database.DriverPostgres {
		return "local.postgres.json"
	}
	return "local.sqlite.json"
}

// storageSyncDir 是活动档与备份的落点：<包根>/server/work/dfo-lan/runtime/storage。
func storageSyncDir(root string) string {
	return filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
}

// dsnPort 从 DSN 里取端口（取不到时用项目默认的 25438）。
func dsnPort(dsn string) int {
	dsn = strings.TrimSpace(dsn)
	rest := dsn
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.LastIndex(rest, "@"); i >= 0 {
		rest = rest[i+1:]
	}
	if i := strings.IndexAny(rest, "/?"); i >= 0 {
		rest = rest[:i]
	}
	if i := strings.LastIndex(rest, ":"); i >= 0 {
		if port, err := strconv.Atoi(rest[i+1:]); err == nil && port > 0 {
			return port
		}
	}
	for _, field := range strings.Fields(dsn) {
		if strings.HasPrefix(field, "port=") {
			if port, err := strconv.Atoi(strings.TrimPrefix(field, "port=")); err == nil && port > 0 {
				return port
			}
		}
	}
	return launcher.PostgresPort
}

func storageSyncFileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// trimStorageBOM 去掉 Windows 编辑器写下的 UTF-8 BOM（档位文件最可能带它）。
func trimStorageBOM(data []byte) []byte {
	return []byte(strings.TrimPrefix(string(data), "\ufeff"))
}
