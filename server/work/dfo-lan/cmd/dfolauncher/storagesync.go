package main

// storagesync.go：storage-sync 子命令 —— 单引擎（SQLite）的备份 / 还原。
//
// 契约（GUI 与无 GUI 排查共用同一条入口）：
//
//   - 进度写 **stderr**，stdout 只留**一行结果 JSON**（相邻启动器按行解析它）；
//   - 退出码：参数错 2、执行失败 1、成功 0；失败时 stdout 上那行 JSON 仍然给出
//     {"ok":false,"error":"…"}，人看的说明同时写 stderr；
//   - --dry-run 只打印计划：计划行进 stderr，stdout 仍是那行 JSON（带 dry_run 与 plan）；
//   - 「跨引擎覆盖式复制」随 PostgreSQL 一起移除（2026-10-05 业主口径，见根 AGENTS.md
//     §0.6）：SQLite 是唯一引擎，另一端不存在了，所以 --copy-to / --restore-to 不再存在。
//
// 硬底线（顺序不可颠倒）：
//  1. 服务端在跑（7001 在监听 / SQLite 管理租约仍在）→ 直接拒绝，请先停止游戏；
//  2. 目标端在**被覆盖前**先自动备份，备份失败即中止；
//  3. 只读写数据本身，**绝不**改 runtime/storage/local.json 与档位模板。

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"dfolan/internal/database"
	"dfolan/internal/launcher"
)

// storageSyncTimeout 是整条同步链的上限。库文件几百 KB、库表几十张，正常几秒；
// 留 30 分钟是为了让"以后数据变大"时不必改代码。
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

// storageSyncStore 是这一端的存储配置，以及它是从哪个文件读到的。
type storageSyncStore struct {
	engine  string
	config  database.Config
	profile map[string]any
	origin  string
	label   string
	usable  bool
}

func (s storageSyncStore) target() database.EngineTarget {
	return database.SQLiteTarget(s.config.SQLitePath)
}

func runStorageSync(args []string) int {
	flags := flag.NewFlagSet("storage-sync", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	list := flags.Bool("list", false, "列出备份与当前档状态（只读，可在游戏运行时跑）")
	backup := flags.Bool("backup", false, "备份当前活动档")
	restore := flags.String("restore", "", "还原 backups 下的某个备份目录（覆盖当前档）")
	dryRun := flags.Bool("dry-run", false, "只打印计划，不落盘、不起进程")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	actions := 0
	for _, chosen := range []bool{*list, *backup, *restore != ""} {
		if chosen {
			actions++
		}
	}
	if actions > 1 {
		fmt.Fprintln(os.Stderr, "storage-sync: --list / --backup / --restore 只能选一个")
		return 2
	}
	if actions == 0 {
		fmt.Fprintln(os.Stderr, "storage-sync: 需要 --list、--backup 或 --restore 之一")
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
	default:
		result.Action = "restore"
	}

	// 进度一律 stderr：stdout 留给那行 JSON。
	logf := func(format string, args ...any) { fmt.Fprintf(os.Stderr, format+"\n", args...) }
	if err := storageSyncRun(absolute, &result, storageSyncOptions{
		Restore: *restore,
		DryRun:  *dryRun,
		Log:     logf,
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
	Restore string
	DryRun  bool
	Log     func(string, ...any)
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
	store, activeEngine, err := resolveStorageSyncStore(root)
	if err != nil {
		return err
	}
	result.ActiveEngine = activeEngine
	if store.usable {
		result.Stores = append(result.Stores, storageSyncStoreView{
			Engine: store.engine, Label: store.label, Origin: store.origin,
		})
	}
	backupsDir := result.BackupsDir

	switch result.Action {
	case "list":
		backups, err := database.ListBackups(backupsDir)
		if err != nil {
			return err
		}
		result.Backups = backups
		result.Message = storageSyncStatusMessage(activeEngine, store, backups)
		if blocked := storageSyncBlocked(store); blocked != "" {
			result.Message += "；当前不可执行备份/还原：" + blocked
		}
		fmt.Fprintln(os.Stderr, result.Message)
		return nil

	case "backup":
		if !store.usable {
			return errors.New("当前档没有可用的 SQLite 配置：请先在启动器「启动环境」里选一档，或检查 runtime/storage/local.json")
		}
		if blocked := storageSyncBlocked(store); blocked != "" {
			return errors.New(blocked)
		}
		if options.DryRun {
			result.Plan = []string{
				"备份 " + store.label + " → " + filepath.Join(backupsDir, "<时间戳>-"+store.engine),
			}
			for _, line := range result.Plan {
				logf("（dry-run）%s", line)
			}
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), storageSyncTimeout)
		defer cancel()
		entry, err := database.Backup(ctx, store.target(), backupsDir, logf)
		if err != nil {
			return err
		}
		result.Backup = &entry
		result.Message = fmt.Sprintf("已备份 %s 到 %s（%d 张表 / %d 行）",
			store.label, entry.Directory, len(entry.Tables), entry.TotalRows)
		fmt.Fprintln(os.Stderr, result.Message)
		return nil

	case "restore":
		entry, err := database.FindBackup(backupsDir, options.Restore)
		if err != nil {
			return err
		}
		if entry.Engine != database.DriverSQLite {
			return fmt.Errorf("备份 %s 是 %s 的（PostgreSQL 支持已移除，见根 AGENTS.md §0.6）："+
				"要读它请回退到移除前的构建并用 dfo-tool sqliteconvert 搬运", entry.Name, entry.Engine)
		}
		if !store.usable {
			return errors.New("当前档没有可用的 SQLite 配置：无法确定要还原到哪个库")
		}
		if blocked := storageSyncBlocked(store); blocked != "" {
			return errors.New(blocked)
		}
		plan := []string{"1) 校验备份 " + entry.Name + " 的文件哈希",
			"2) 自动备份被覆盖的 " + store.label,
			"3) 把 " + entry.Name + " 还原到 " + store.label}
		if options.DryRun {
			result.Plan = plan
			for _, line := range plan {
				logf("（dry-run）%s", line)
			}
			return nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), storageSyncTimeout)
		defer cancel()
		report, err := database.RestoreBackup(ctx, entry, store.target(), backupsDir, logf)
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
		fmt.Fprintln(os.Stderr, result.Message)
		return nil
	}
	return fmt.Errorf("未知的动作 %q", result.Action)
}

// storageSyncStatusMessage 是 --list 的人话摘要（同一句话进 stdout JSON 的 message）。
func storageSyncStatusMessage(active string, store storageSyncStore, backups []database.BackupEntry) string {
	var parts []string
	if !store.usable {
		if active == "" {
			parts = append(parts, "活动档没有可用的存储配置")
		} else {
			parts = append(parts, "活动档声明了不支持的引擎 "+active+"（PostgreSQL 支持已移除，只认 sqlite）")
		}
	} else {
		parts = append(parts, "当前档："+store.label)
	}
	parts = append(parts, fmt.Sprintf("备份 %d 份", len(backups)))
	return strings.Join(parts, "；")
}

// storageSyncBlocked 说清"现在为什么不能动库"。两条判据都只会拒绝，不会去改状态：
// 服务端在监听 7001，或者 SQLite 管理租约仍在（后者见 launcher.AdminLeaseHeld，
// 判据故意保守：只要说不清持有者是否还活着，就当有人持有）。
func storageSyncBlocked(store storageSyncStore) string {
	if launcher.PortListening(launcher.GatewayPort, 700*time.Millisecond) {
		return fmt.Sprintf("游戏服务端正在运行（端口 %d 在监听）：请先停止游戏再做备份/还原", launcher.GatewayPort)
	}
	if !store.usable {
		return ""
	}
	cfg := launcher.StorageConfig{Driver: store.engine, SQLitePath: store.config.SQLitePath}
	if held, note := launcher.AdminLeaseHeld(cfg); held {
		return fmt.Sprintf("该存储档的管理租约仍在（%s）：请先停止游戏（或等租约过期）再做备份/还原", note)
	}
	return ""
}

// resolveStorageSyncStore 给出当前档的存储配置与"活动档是哪一档"。
//
// 顺序（与「启动环境」的语义一致）：
//  1. 活动档 local.json 写的是 sqlite → 用它；
//  2. 否则用模板 local.sqlite.json（切档不会删模板）；
//  3. 都没写 → 这一端当前不可用，如实报错，不去猜路径、更不新建配置。
func resolveStorageSyncStore(root string) (storageSyncStore, string, error) {
	dir := storageSyncDir(root)
	activePath := filepath.Join(dir, "local.json")
	activeProfile, activeOK := readStorageProfile(activePath)
	activeEngine := ""
	if activeOK {
		activeEngine = profileEngine(activeProfile)
	}

	if activeOK && activeEngine == database.DriverSQLite {
		store, err := buildStorageSyncStore(activePath, activeProfile)
		if err != nil {
			return storageSyncStore{}, activeEngine, err
		}
		return store, activeEngine, nil
	}
	templatePath := filepath.Join(dir, "local."+database.DriverSQLite+".json")
	profile, ok := readStorageProfile(templatePath)
	if ok && profileEngine(profile) == database.DriverSQLite {
		store, err := buildStorageSyncStore(templatePath, profile)
		if err != nil {
			// 模板存在但内容有问题（例如 sqlite_path 是相对路径）：这是"这一端不可用"，
			// 不是整条命令的失败 —— --list 照样应该能列备份。
			fmt.Fprintf(os.Stderr, "storage-sync: 忽略 %s：%v\n", templatePath, err)
			return storageSyncStore{}, activeEngine, nil
		}
		return store, activeEngine, nil
	}
	return storageSyncStore{engine: activeEngine}, activeEngine, nil
}

// buildStorageSyncStore 把一份档位文件变成"这一端"的配置。
func buildStorageSyncStore(path string, profile map[string]any) (storageSyncStore, error) {
	config, err := database.LoadConfig(path)
	if err != nil {
		return storageSyncStore{}, err
	}
	store := storageSyncStore{
		engine: database.DriverSQLite, config: config, profile: profile,
		origin: path, usable: true,
	}
	dbPath := strings.TrimSpace(config.SQLitePath)
	if dbPath == "" {
		return storageSyncStore{}, fmt.Errorf("SQLite 档缺少 sqlite_path")
	}
	if !filepath.IsAbs(dbPath) {
		return storageSyncStore{}, fmt.Errorf("sqlite_path 必须是绝对路径（现在是 %q）：服务端会按自己的工作目录解析", dbPath)
	}
	store.label = database.SQLiteTarget(dbPath).Label
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
// 当成 SQLite 档去备份一个不存在的文件。写了 postgres 的档会原样返回 postgres，
// 由调用方明确报"该引擎已不再支持"，而不是静默当成 SQLite。
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

func storageSyncDir(root string) string {
	return filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
}

// trimStorageBOM 去掉 Windows 编辑器写下的 UTF-8 BOM（档位文件最可能带它）。
func trimStorageBOM(data []byte) []byte {
	return []byte(strings.TrimPrefix(string(data), "\ufeff"))
}
