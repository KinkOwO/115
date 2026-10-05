package database

// sync.go 实现「双端同步」：备份、跨引擎覆盖式复制、还原（业主 2026-10-05 需求）。
//
// 边界（写在最前面，因为它决定了下面每一处的取舍）：
//
//   - **复制 ≠ 切档**。复制只改写目标端的数据（SQLite 文件 / PostgreSQL 库），
//     **不动** runtime/storage/local.json，也不动两个档位模板；这次跑哪一档仍然由
//     启动器「启动环境」决定。这样"搬家"和"选档"是两件互不牵连的事。
//   - 目标端在**被覆盖之前一定先自动备份**，备份失败就中止：不存在"没备份成功也照覆盖"。
//   - 备份目录：<storage>/backups/<时间戳>-<engine>/，里面有数据文件（SQLite 副本或
//     pg_dump -Fc 归档）与 manifest.json（引擎、时间、打码后的来源、文件 sha256、每张表行数）。
//     备份自带引擎身份，还原时靠它知道自己该回哪一端。
//   - 报告与日志里的 DSN 一律打码（MaskDSN）：manifest 也会落到磁盘上，同样不许出现口令。
//   - "服务端不在跑"由调用方（cmd/dfolauncher）守卫：本文件只做数据搬运，不判断端口。

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	// BackupDirName 是备份根目录名，位于 runtime/storage 之下。
	BackupDirName = "backups"
	// manifestName 是每份备份的清单文件名。它自己不进 Files：清单不能包含自己的哈希。
	manifestName = "manifest.json"
	// backupStampLayout 是备份目录名里的时间戳（可读、可排序：20261005-140312）。
	backupStampLayout = "20060102-150405"
)

// EngineTarget 描述同步要读写的那一端：用哪个引擎、配置是什么、报告里怎么称呼它。
type EngineTarget struct {
	Engine string
	Config Config
	// Label 是打码后的位置说明，会进 manifest、日志与界面。
	Label string
}

// SQLiteTarget 用一条库文件路径构造目标端。
func SQLiteTarget(path string) EngineTarget {
	path = strings.TrimSpace(path)
	return EngineTarget{
		Engine: DriverSQLite,
		Config: Config{Driver: DriverSQLite, SQLitePath: path},
		Label:  "SQLite（" + path + "）",
	}
}

// PostgresTarget 用一份配置构造目标端（标签里的 DSN 已打码）。
func PostgresTarget(cfg Config) EngineTarget {
	cfg.Driver = DriverPostgres
	return EngineTarget{
		Engine: DriverPostgres,
		Config: cfg,
		Label:  "PostgreSQL（" + DescribeDSN(cfg.PostgresDSN) + "）",
	}
}

// Tools 是 PostgreSQL 客户端工具的绝对路径。由调用方解析（便携 PG 的位置不是本包的事）。
type Tools struct {
	PgDump    string
	PgRestore string
}

// BackupFile 是备份目录里的一个数据文件及其哈希。
type BackupFile struct {
	Name   string `json:"name"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

// TableRows 是一张表的行数，备份清单与复制报告共用。
type TableRows struct {
	Name string `json:"name"`
	Rows int64  `json:"rows"`
}

// BackupEntry 是一份备份的清单。它同时是 manifest.json 的内容：落盘与回读走同一个结构，
// 所以"写进去的字段"与"渲染出来的字段"不会漂移。
type BackupEntry struct {
	Name      string       `json:"name"`
	Engine    string       `json:"engine"`
	Directory string       `json:"directory"`
	CreatedAt string       `json:"created_at"`
	Source    string       `json:"source"`
	Files     []BackupFile `json:"files"`
	Tables    []TableRows  `json:"tables,omitempty"`
	TotalRows int64        `json:"total_rows"`
	// SizeBytes 是整个备份目录的大小（含 manifest.json 自己），列表与 UI 用。
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Note      string `json:"note,omitempty"`
}

// CopyTable 是一张表的复制结果。Verified 表示行数对上、JSON 列内容也一致。
type CopyTable struct {
	Name        string   `json:"name"`
	Rows        int64    `json:"rows"`
	Verified    bool     `json:"verified"`
	JSONChecked int      `json:"json_checked,omitempty"`
	Skipped     []string `json:"skipped_columns,omitempty"`
}

// CopyReport 是一次跨引擎覆盖式复制的结论。
type CopyReport struct {
	From         string       `json:"from"`
	To           string       `json:"to"`
	Source       string       `json:"source"`
	Target       string       `json:"target"`
	TargetBackup *BackupEntry `json:"target_backup,omitempty"`
	Tables       []CopyTable  `json:"tables"`
	// Missing 是"目标端 schema 里没有"的源端表：正常部署下为空，不为空就说明两端
	// schema 版本不一致，必须让人看见，而不是悄悄少拷几张表。
	Missing   []string `json:"missing_tables,omitempty"`
	Dropped   []string `json:"dropped_tables,omitempty"`
	TotalRows int64    `json:"total_rows"`
	Note      string   `json:"note,omitempty"`
}

// RestoreReport 是一次还原的结论。
type RestoreReport struct {
	Backup       BackupEntry  `json:"backup"`
	Engine       string       `json:"engine"`
	Target       string       `json:"target"`
	TargetBackup *BackupEntry `json:"target_backup,omitempty"`
	Tables       []TableRows  `json:"tables,omitempty"`
	TotalRows    int64        `json:"total_rows"`
	Verified     bool         `json:"verified"`
	Note         string       `json:"note,omitempty"`
}

// ---------------------------------------------------------------------------
// 打码：报告、日志、manifest 只允许出现打码后的 DSN
// ---------------------------------------------------------------------------

// MaskDSN 把 DSN 里的口令换成 ***。
//
// 与相邻启动器 internal/config 的 maskDSN 同一口径（两个仓库没法共享代码，只能各自实现；
// 两边的判据都是"有口令就打码"，任何一侧改动都要同步另一侧）。url.String() 会把 * 转义成
// %2A，打码后应当仍然一眼能读，所以替换回去。
func MaskDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return ""
	}
	if u, err := url.Parse(dsn); err == nil && u.User != nil {
		if _, hasPassword := u.User.Password(); hasPassword {
			u.User = url.UserPassword(u.User.Username(), "***")
			return strings.ReplaceAll(u.String(), "%2A", "*")
		}
		return u.String()
	}
	// url.Parse 失败（例如口令里有未转义字符）时退回字符串替换：
	// scheme://user:password@ → scheme://user:***@
	scheme := strings.Index(dsn, "://")
	if scheme < 0 {
		return maskKeyValuePassword(dsn)
	}
	rest := dsn[scheme+3:]
	at := strings.LastIndex(rest, "@")
	if at < 0 {
		return dsn
	}
	creds, host := rest[:at], rest[at:]
	colon := strings.IndexByte(creds, ':')
	if colon < 0 {
		return dsn
	}
	return dsn[:scheme+3] + creds[:colon] + ":***" + host
}

// maskKeyValuePassword 处理 key=value 形式的连接串（password=xxx）。
func maskKeyValuePassword(dsn string) string {
	fields := strings.Fields(dsn)
	for i, field := range fields {
		if strings.HasPrefix(field, "password=") {
			fields[i] = "password=***"
		}
	}
	return strings.Join(fields, " ")
}

// DescribeDSN 把 DSN 收敛成"连到哪"的短说明：host:port/database，口令打码；缺 DSN 时如实说明。
func DescribeDSN(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if dsn == "" {
		return "未配置 DSN"
	}
	target := MaskDSN(dsn)
	if i := strings.Index(target, "://"); i >= 0 {
		target = target[i+3:]
	}
	if i := strings.LastIndex(target, "@"); i >= 0 {
		target = target[i+1:]
	}
	if i := strings.IndexByte(target, '?'); i >= 0 {
		target = target[:i]
	}
	return strings.TrimSuffix(target, "/")
}

// dsnDatabase 取 DSN 里的库名（备份文件名与报告都要用；取不到时给空串由调用方兜底）。
func dsnDatabase(dsn string) string {
	dsn = strings.TrimSpace(dsn)
	if u, err := url.Parse(dsn); err == nil && u.Path != "" {
		return strings.Trim(strings.TrimPrefix(u.Path, "/"), "/")
	}
	for _, field := range strings.Fields(dsn) {
		if strings.HasPrefix(field, "dbname=") {
			return strings.TrimPrefix(field, "dbname=")
		}
	}
	return ""
}

// pgEnv 把 DSN 拆成"给子进程的环境"与"去掉了口令的连接串"。
//
// 口令走 PGPASSWORD 而不是 argv：pg_dump/pg_restore 的命令行会出现在进程列表里，
// 而它属于本机所有用户都看得到的地方。去口令的连接串则照常传给客户端。
func pgEnv(dsn string) ([]string, string) {
	env := os.Environ()
	password := ""
	clean := dsn
	if u, err := url.Parse(dsn); err == nil && u.User != nil {
		if pw, has := u.User.Password(); has {
			password = pw
			u.User = url.User(u.User.Username())
			clean = u.String()
		}
	} else {
		fields := strings.Fields(dsn)
		kept := make([]string, 0, len(fields))
		for _, field := range fields {
			if strings.HasPrefix(field, "password=") {
				password = strings.TrimPrefix(field, "password=")
				continue
			}
			kept = append(kept, field)
		}
		clean = strings.Join(kept, " ")
	}
	if password != "" {
		env = append(env, "PGPASSWORD="+password)
	}
	return env, clean
}

// ---------------------------------------------------------------------------
// 备份
// ---------------------------------------------------------------------------

// Backup 把 target 的数据完整备份到 backupsDir 下的 <时间戳>-<engine> 目录。
//
// 一致性做法：
//   - SQLite 用 VACUUM INTO（SQLite 自己的在线备份语义）：一个读事务里把整库
//     （含 WAL 中已提交的部分）写成一份**一致**的新文件，不需要先 checkpoint，
//     也不会留下半个文件。副本写完还要只读打开、逐表计数，能读出来才算备份可用。
//   - PostgreSQL 用 pg_dump -Fc 导出到该目录（自定义格式：可 pg_restore、自带校验）。
func Backup(ctx context.Context, target EngineTarget, backupsDir string, tools Tools, log func(string, ...any)) (BackupEntry, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	if strings.TrimSpace(backupsDir) == "" {
		return BackupEntry{}, errors.New("未指定备份目录")
	}
	if err := os.MkdirAll(backupsDir, 0o755); err != nil {
		return BackupEntry{}, fmt.Errorf("创建备份目录失败：%w", err)
	}
	dir := uniquePath(filepath.Join(backupsDir, time.Now().Format(backupStampLayout)+"-"+target.Engine))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return BackupEntry{}, fmt.Errorf("创建备份目录失败：%w", err)
	}

	entry := BackupEntry{
		Name:      filepath.Base(dir),
		Engine:    target.Engine,
		Directory: dir,
		CreatedAt: time.Now().Format(time.RFC3339),
		Source:    target.Label,
	}
	var err error
	switch target.Engine {
	case DriverSQLite:
		err = backupSQLite(ctx, target, dir, &entry, log)
	case DriverPostgres:
		err = backupPostgres(ctx, target, dir, &entry, tools, log)
	default:
		err = fmt.Errorf("未知的存储引擎 %q", target.Engine)
	}
	if err != nil {
		// 失败不留半个目录：留着它会被 --list 当成一份"真备份"，比没有更糟。
		_ = os.RemoveAll(dir)
		return BackupEntry{}, err
	}

	body, marshalErr := json.MarshalIndent(entry, "", "  ")
	if marshalErr != nil {
		return BackupEntry{}, marshalErr
	}
	if writeErr := os.WriteFile(filepath.Join(dir, manifestName), append(body, '\n'), 0o644); writeErr != nil {
		return BackupEntry{}, fmt.Errorf("写入 manifest.json 失败：%w", writeErr)
	}
	entry.SizeBytes = dirSize(dir)
	log("已备份 %s → %s（%d 张表 / %d 行）", target.Label, dir, len(entry.Tables), entry.TotalRows)
	return entry, nil
}

// backupSQLite 用 VACUUM INTO 生成副本，并回读副本做行数统计。
func backupSQLite(ctx context.Context, target EngineTarget, dir string, entry *BackupEntry, log func(string, ...any)) error {
	path := strings.TrimSpace(target.Config.SQLitePath)
	if path == "" {
		return errors.New("SQLite 档缺少 sqlite_path，无法备份")
	}
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("SQLite 存档 %s 不可读：%w", path, err)
	}
	if st.IsDir() {
		return fmt.Errorf("SQLite 存档 %s 是目录，不是库文件", path)
	}
	dest := filepath.Join(dir, filepath.Base(path))
	db, err := openSQLite(ctx, path, 1, 10000)
	if err != nil {
		return fmt.Errorf("打开 SQLite 存档失败：%w", err)
	}
	defer db.Close()
	// VACUUM INTO 要求目标文件不存在 —— 这正是我们刚建的目录。
	// 路径转成斜杠并在单引号里转义，免得 Windows 反斜杠与引号把语句改味。
	if _, err := db.ExecContext(ctx, "VACUUM INTO "+quoteLiteral(filepath.ToSlash(dest))); err != nil {
		return fmt.Errorf("VACUUM INTO %s 失败：%w", dest, err)
	}
	file, err := describeFile(dest)
	if err != nil {
		return err
	}
	entry.Files = append(entry.Files, file)

	// 副本必须**能读出来**才算备份：只读 + immutable 打开，不写 -shm/-wal，也不动副本本身。
	tables, total, err := sqliteRowCounts(ctx, dest)
	if err != nil {
		return fmt.Errorf("备份副本 %s 读不出来（备份不可用）：%w", dest, err)
	}
	entry.Tables, entry.TotalRows = tables, total
	log("SQLite 在线备份完成：%s（%d 字节，%d 张表）", filepath.Base(dest), file.Size, len(tables))
	return nil
}

// backupPostgres 用 pg_dump -Fc 导出，并读源库统计行数。
func backupPostgres(ctx context.Context, target EngineTarget, dir string, entry *BackupEntry, tools Tools, log func(string, ...any)) error {
	dsn := strings.TrimSpace(target.Config.PostgresDSN)
	if dsn == "" {
		return errors.New("PostgreSQL 档缺少 postgres_dsn，无法备份")
	}
	if strings.TrimSpace(tools.PgDump) == "" {
		return errors.New("找不到 pg_dump.exe（便携 PostgreSQL 的 bin 目录）")
	}
	name := dsnDatabase(dsn)
	if name == "" {
		name = "postgres"
	}
	dest := filepath.Join(dir, name+".dump")
	env, clean := pgEnv(dsn)
	cmd := exec.CommandContext(ctx, tools.PgDump, "-Fc", "-f", dest, clean)
	cmd.Env = env
	// 进度走 stderr：它可能很长，只在失败时回显末尾几行。
	var sink bytes.Buffer
	cmd.Stdout, cmd.Stderr = &sink, &sink
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump 失败：%v\n%s", err, tailLines(sink.String(), 8))
	}
	file, err := describeFile(dest)
	if err != nil {
		return err
	}
	entry.Files = append(entry.Files, file)

	store, err := Open(ctx, target.Config)
	if err != nil {
		return fmt.Errorf("打开 PostgreSQL 源库失败：%w", err)
	}
	defer store.Close()
	pool, err := store.rawPool()
	if err != nil {
		return err
	}
	tables, total, err := postgresRowCounts(ctx, pool)
	if err != nil {
		return err
	}
	entry.Tables, entry.TotalRows = tables, total
	log("PostgreSQL 自定义格式导出完成：%s（%d 字节，%d 张表）", filepath.Base(dest), file.Size, len(tables))
	return nil
}

// ListBackups 列出备份目录下的所有备份，最新的排在最前面。
//
// manifest.json 读不出来的目录**照样列出来**（引擎留空、带一句说明）：它占着空间也需要
// 人去处理，静默隐藏只会让人以为磁盘里的东西不见了。
func ListBackups(backupsDir string) ([]BackupEntry, error) {
	entries, err := os.ReadDir(backupsDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []BackupEntry
	for _, item := range entries {
		if !item.IsDir() {
			continue
		}
		dir := filepath.Join(backupsDir, item.Name())
		entry := BackupEntry{Name: item.Name(), Directory: dir}
		body, readErr := os.ReadFile(filepath.Join(dir, manifestName))
		if readErr == nil {
			if jsonErr := json.Unmarshal(body, &entry); jsonErr != nil {
				entry = BackupEntry{Name: item.Name(), Directory: dir,
					Note: "manifest.json 解析失败：" + jsonErr.Error()}
			}
		} else {
			entry.Note = "缺少 manifest.json：" + readErr.Error()
		}
		entry.Name = item.Name()
		entry.Directory = dir
		entry.SizeBytes = dirSize(dir)
		out = append(out, entry)
	}
	// 目录名以时间戳开头，倒序即"最新在前"。
	sort.Slice(out, func(i, j int) bool { return out[i].Name > out[j].Name })
	return out, nil
}

// FindBackup 在备份目录里按名字取一份备份。名字只允许是单层目录名。
func FindBackup(backupsDir, name string) (BackupEntry, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return BackupEntry{}, errors.New("未指定备份名")
	}
	if name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return BackupEntry{}, fmt.Errorf("备份名 %q 只能是 backups 下的一个目录名", name)
	}
	dir := filepath.Join(backupsDir, name)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return BackupEntry{}, fmt.Errorf("备份 %s 不存在", name)
	}
	entry := BackupEntry{Name: name, Directory: dir}
	body, err := os.ReadFile(filepath.Join(dir, manifestName))
	if err != nil {
		return BackupEntry{}, fmt.Errorf("备份 %s 缺少 manifest.json，无法确认它属于哪一端", name)
	}
	if err := json.Unmarshal(body, &entry); err != nil {
		return BackupEntry{}, fmt.Errorf("备份 %s 的 manifest.json 解析失败：%w", name, err)
	}
	entry.Name = name
	entry.Directory = dir
	entry.SizeBytes = dirSize(dir)
	if entry.Engine != DriverSQLite && entry.Engine != DriverPostgres {
		return BackupEntry{}, fmt.Errorf("备份 %s 的 manifest.json 没写引擎（engine=%q），不知道该往哪一端还原", name, entry.Engine)
	}
	return entry, nil
}

// VerifyBackup 重算备份里每个文件的 sha256，与 manifest 对比。
//
// 还原之前必须先过这一关：从一份损坏的备份覆盖掉现有数据，是这套功能里唯一
// "两个方向都救不回来"的操作。
func VerifyBackup(entry BackupEntry) error {
	if len(entry.Files) == 0 {
		return fmt.Errorf("备份 %s 的 manifest.json 里没有任何数据文件", entry.Name)
	}
	for _, file := range entry.Files {
		path := filepath.Join(entry.Directory, file.Name)
		got, err := describeFile(path)
		if err != nil {
			return fmt.Errorf("备份文件 %s 读不出来：%w", file.Name, err)
		}
		if got.Size != file.Size || !strings.EqualFold(got.SHA256, file.SHA256) {
			return fmt.Errorf("备份文件 %s 与 manifest 不一致（大小 %d/%d，sha256 %s/%s）：备份已损坏，拒绝还原",
				file.Name, got.Size, file.Size, got.SHA256, file.SHA256)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 复制：覆盖式写入另一端
// ---------------------------------------------------------------------------

// CopyBetweenEngines 把 source 的数据覆盖式复制到 target（两端必须是不同引擎）。
//
// 顺序永远是：先自动备份 target（失败即中止）→ 再覆盖写入 → 再校验。
// **不修改**任何档位文件：复制不等于切档。
func CopyBetweenEngines(ctx context.Context, source, target EngineTarget, backupsDir string, tools Tools, log func(string, ...any)) (CopyReport, error) {
	if log == nil {
		log = func(string, ...any) {}
	}
	report := CopyReport{
		From:   source.Engine,
		To:     target.Engine,
		Source: source.Label,
		Target: target.Label,
	}
	if source.Engine == target.Engine {
		return report, fmt.Errorf("两端都是 %s：同引擎之间没有可搬运的差异（要换这次跑哪一档请用启动器的「启动环境」）", source.Engine)
	}
	backup, err := autoBackupTarget(ctx, target, backupsDir, tools, log)
	if err != nil {
		return report, err
	}
	report.TargetBackup = backup

	switch {
	case source.Engine == DriverPostgres && target.Engine == DriverSQLite:
		err = copyPostgresToSQLite(ctx, source, target, &report, log)
	case source.Engine == DriverSQLite && target.Engine == DriverPostgres:
		err = copySQLiteToPostgres(ctx, source, target, &report, log)
	default:
		err = fmt.Errorf("不支持的方向：%s → %s", source.Engine, target.Engine)
	}
	if err != nil {
		return report, err
	}
	return report, nil
}

// autoBackupTarget 在覆盖目标端之前自动备份它。目标端还没有数据时如实说明并跳过
// （没有可覆盖的东西，就不存在"备份失败还照覆盖"）。
func autoBackupTarget(ctx context.Context, target EngineTarget, backupsDir string, tools Tools, log func(string, ...any)) (*BackupEntry, error) {
	if log == nil {
		// 本函数既被 CopyBetweenEngines/RestoreBackup（已兜底）调用，也会被测试直接调用。
		log = func(string, ...any) {}
	}
	if target.Engine == DriverSQLite {
		path := strings.TrimSpace(target.Config.SQLitePath)
		if path == "" {
			return nil, errors.New("SQLite 目标端缺少 sqlite_path")
		}
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			log("目标端 %s 还没有数据（文件不存在），无需备份", target.Label)
			return nil, nil
		}
	}
	entry, err := Backup(ctx, target, backupsDir, tools, log)
	if err != nil {
		return nil, fmt.Errorf("覆盖前自动备份目标端失败，已中止：%w", err)
	}
	log("目标端已自动备份：%s", entry.Directory)
	return &entry, nil
}

// copyPostgresToSQLite 复用既有的单向量化器：它写到一个**临时新文件**并在写的过程中
// 校验行数、JSON 字节与外部键；只有全部通过才用它替换目标端，所以失败时目标端一个字节都没动。
func copyPostgresToSQLite(ctx context.Context, source, target EngineTarget, report *CopyReport, log func(string, ...any)) error {
	destPath := strings.TrimSpace(target.Config.SQLitePath)
	if destPath == "" {
		return errors.New("SQLite 目标端缺少 sqlite_path")
	}
	if !filepath.IsAbs(destPath) {
		return fmt.Errorf("sqlite_path 必须是绝对路径：%q 会相对服务端的工作目录解析", destPath)
	}
	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("创建目标端目录失败：%w", err)
	}
	store, err := Open(ctx, source.Config)
	if err != nil {
		return fmt.Errorf("打开 PostgreSQL 源库失败：%w", err)
	}
	defer store.Close()

	tmp := destPath + ".sync-tmp"
	if err := os.Remove(tmp); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("清理临时文件 %s 失败：%w", tmp, err)
	}
	convert, err := ConvertPostgresToSQLite(ctx, store, tmp)
	if err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("PostgreSQL → SQLite 复制失败（目标端未改动）：%w", err)
	}
	report.TotalRows = convert.TotalRows()
	report.Missing = convert.Missing
	for _, table := range convert.Tables {
		report.Tables = append(report.Tables, CopyTable{
			Name: table.Name, Rows: table.Rows, Verified: table.Verified, JSONChecked: table.JSONChecked,
		})
	}
	if err := replaceSQLiteFile(tmp, destPath); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	log("SQLite 目标端已覆盖：%s（%d 张表 / %d 行）", destPath, len(report.Tables), report.TotalRows)
	return nil
}

// copySQLiteToPostgres 把 SQLite 的数据覆盖式写进 PostgreSQL。
//
// 覆盖 = 在**一个事务**里 DROP 掉目标 schema 的全部表 → 按 0001_initial.sql 的分节重建
// （并写回同形的迁移账本）→ 按外部键依赖顺序插入全部行 → 逐表校验行数与 JSON 内容。
// DDL 在 PostgreSQL 里是可回滚的，所以任何一步失败都 rollback：目标端保持原样。
func copySQLiteToPostgres(ctx context.Context, source, target EngineTarget, report *CopyReport, log func(string, ...any)) error {
	srcPath := strings.TrimSpace(source.Config.SQLitePath)
	if srcPath == "" {
		return errors.New("SQLite 源端缺少 sqlite_path")
	}
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("SQLite 源库 %s 不可读：%w", srcPath, err)
	}
	src, err := openSQLite(ctx, srcPath, 2, 10000)
	if err != nil {
		return fmt.Errorf("打开 SQLite 源库失败：%w", err)
	}
	defer src.Close()

	store, err := Open(ctx, target.Config)
	if err != nil {
		return fmt.Errorf("打开 PostgreSQL 目标库失败：%w", err)
	}
	defer store.Close()
	// rawPool 的第三个使用者（前两个是测试夹具与诊断）：本路径要在一个事务里同时
	// 执行 DDL 与带参数的行插入，querySet/txHandle 这层抽象没有带参数的出入通道。
	pool, err := store.rawPool()
	if err != nil {
		return err
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	dropped, err := dropAllTables(ctx, tx)
	if err != nil {
		return err
	}
	report.Dropped = dropped
	if len(dropped) > 0 {
		log("目标端已有 %d 张表，将按源端重建：%s", len(dropped), strings.Join(dropped, "、"))
	}
	sequences, err := dropAllSequences(ctx, tx)
	if err != nil {
		return err
	}
	if len(sequences) > 0 {
		log("目标端另清掉 %d 个独立序列：%s", len(sequences), strings.Join(sequences, "、"))
	}
	if err := rebuildSchemaInTx(ctx, tx); err != nil {
		return err
	}

	srcTables, err := sqliteTableNames(ctx, src)
	if err != nil {
		return err
	}
	meta, err := postgresColumnMeta(ctx, tx)
	if err != nil {
		return err
	}
	order, cyclic := insertOrder(ctx, tx)
	// 依赖图来自 pg_class；information_schema 是列来源。两者理论上一致，但不一致时
	// 也**不能**悄悄少拷几张表：把漏掉的接在最后，并说明顺序是退化的。
	for name := range meta {
		if !listHas(order, name) {
			order = append(order, name)
			cyclic = append(cyclic, name)
		}
	}

	copied := 0
	for _, table := range order {
		if engineLocalTables[table] {
			continue
		}
		columns, ok := meta[table]
		if !ok || len(columns) == 0 {
			continue
		}
		if !listHas(srcTables, table) {
			// 目标端 schema 有、源端没有：重建时已经建成空表，这正是"覆盖"的结果。
			continue
		}
		tableReport, err := copySQLiteTable(ctx, src, tx, table, columns, log)
		if err != nil {
			return err
		}
		report.Tables = append(report.Tables, tableReport)
		report.TotalRows += tableReport.Rows
		copied++
	}
	for _, table := range srcTables {
		if engineLocalTables[table] {
			continue
		}
		if _, ok := meta[table]; !ok {
			report.Missing = append(report.Missing, table)
		}
	}
	if len(cyclic) > 0 {
		report.Note = "以下表未能从外部键依赖确定插入顺序，已按名字顺序插入：" + strings.Join(cyclic, "、")
		log("注意：%s", report.Note)
	}
	if err := syncSequences(ctx, tx, meta); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("提交目标端写入失败：%w", err)
	}
	log("PostgreSQL 目标端已覆盖：%s（%d 张表 / %d 行）", target.Label, copied, report.TotalRows)
	return nil
}

// copySQLiteTable 复制一张表：列取两端的交集（目标端顺序为准），逐行转换后插入，
// 最后校验行数与 JSON 列内容。
func copySQLiteTable(ctx context.Context, src *sql.DB, tx pgx.Tx, table string, columns []pgColumn, log func(string, ...any)) (CopyTable, error) {
	report := CopyTable{Name: table}
	srcColumns, err := sqliteColumnNames(ctx, src, table)
	if err != nil {
		return report, err
	}
	var shared []pgColumn
	for _, column := range columns {
		if listHas(srcColumns, column.Name) {
			shared = append(shared, column)
		}
	}
	for _, name := range srcColumns {
		if !hasColumn(columns, name) {
			report.Skipped = append(report.Skipped, name+"（目标端没有这一列）")
		}
	}
	for _, column := range columns {
		if !listHas(srcColumns, column.Name) && column.Nullable == "NO" && column.Default == nil {
			// 目标端 NOT NULL 且没有默认值的列必须由源端提供，否则这一行插不进去。
			return report, fmt.Errorf("表 %s 的 %s 列在目标端是 NOT NULL 且无默认值，但源端没有这一列", table, column.Name)
		}
	}
	if len(shared) == 0 {
		return report, fmt.Errorf("表 %s 在源端与目标端没有共同的列", table)
	}
	if len(report.Skipped) > 0 {
		log("表 %s：%s", table, strings.Join(report.Skipped, "；"))
	}

	quoted := make([]string, len(shared))
	selects := make([]string, len(shared))
	placeholders := make([]string, len(shared))
	anyIdentity := false
	for i, column := range shared {
		quoted[i] = quoteIdent(column.Name)
		selects[i] = quoteIdent(column.Name)
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		if column.Identity == "YES" {
			anyIdentity = true
		}
	}
	rows, err := src.QueryContext(ctx,
		"SELECT "+strings.Join(selects, ",")+" FROM "+quoteIdent(table))
	if err != nil {
		return report, fmt.Errorf("读源表 %s 失败：%w", table, err)
	}
	defer rows.Close()

	insert := "INSERT INTO " + quoteIdent(table) + "(" + strings.Join(quoted, ",") + ") "
	if anyIdentity {
		// 显式写入 identity 列必须声明 OVERRIDING SYSTEM VALUE，否则 PostgreSQL 直接拒绝。
		insert += "OVERRIDING SYSTEM VALUE "
	}
	insert += "VALUES(" + strings.Join(placeholders, ",") + ")"

	jsonIndexes := make([]int, 0, 2)
	for i, column := range shared {
		if column.DataType == "json" || column.DataType == "jsonb" {
			jsonIndexes = append(jsonIndexes, i)
		}
	}
	sourceJSON := make([][][]byte, len(shared))
	targets := make([]any, len(shared))
	for i := range targets {
		targets[i] = new(any)
	}

	for rows.Next() {
		if err := rows.Scan(targets...); err != nil {
			return report, fmt.Errorf("读源表 %s 的行失败：%w", table, err)
		}
		args := make([]any, len(shared))
		for i, column := range shared {
			converted, err := sqliteValueToPostgres(table, column, *targets[i].(*any))
			if err != nil {
				return report, err
			}
			args[i] = converted
			if containsInt(jsonIndexes, i) {
				canonical, err := canonicalJSON(converted)
				if err != nil {
					return report, fmt.Errorf("表 %s 的 %s 列不是合法 JSON：%w", table, column.Name, err)
				}
				sourceJSON[i] = append(sourceJSON[i], canonical)
			}
		}
		if _, err := tx.Exec(ctx, insert, args...); err != nil {
			return report, fmt.Errorf("写入表 %s 失败：%w", table, err)
		}
		report.Rows++
	}
	if err := rows.Err(); err != nil {
		return report, fmt.Errorf("读源表 %s 失败：%w", table, err)
	}

	var destRows int64
	if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+quoteIdent(table)).Scan(&destRows); err != nil {
		return report, err
	}
	if destRows != report.Rows {
		return report, fmt.Errorf("表 %s 行数不一致：源端 %d，目标端 %d", table, report.Rows, destRows)
	}
	for _, index := range jsonIndexes {
		destJSON, err := postgresJSONColumn(ctx, tx, table, shared[index].Name)
		if err != nil {
			return report, err
		}
		if jsonDigest(sourceJSON[index]) != jsonDigest(destJSON) {
			return report, fmt.Errorf("表 %s 的 %s 列 JSON 内容与源端不一致（%d 行）", table, shared[index].Name, len(destJSON))
		}
		report.JSONChecked++
	}
	report.Verified = true
	return report, nil
}

// postgresJSONColumn 读出目标端某一列的全部 JSON 值（规范化成可比形式）。
func postgresJSONColumn(ctx context.Context, tx pgx.Tx, table, column string) ([][]byte, error) {
	rows, err := tx.Query(ctx, "SELECT "+quoteIdent(column)+" FROM "+quoteIdent(table))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out [][]byte
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		canonical, err := canonicalJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("表 %s 的 %s 列在目标端不是合法 JSON：%w", table, column, err)
		}
		out = append(out, canonical)
	}
	return out, rows.Err()
}

// ---------------------------------------------------------------------------
// 还原：把一份备份写回它自己的引擎（覆盖）
// ---------------------------------------------------------------------------

// RestoreBackup 把 entry 还原到 target（target 必须是 entry 记录的引擎）。
//
// 与复制同一条底线：先校验备份自身的 sha256，再自动备份被覆盖端，最后才写。
func RestoreBackup(ctx context.Context, entry BackupEntry, target EngineTarget, backupsDir string, tools Tools, log func(string, ...any)) (RestoreReport, error) {
	report := RestoreReport{Backup: entry, Engine: entry.Engine, Target: target.Label}
	if log == nil {
		log = func(string, ...any) {}
	}
	if entry.Engine != target.Engine {
		return report, fmt.Errorf("备份 %s 属于 %s，不能还原到 %s", entry.Name, entry.Engine, target.Engine)
	}
	if err := VerifyBackup(entry); err != nil {
		return report, err
	}
	backup, err := autoBackupTarget(ctx, target, backupsDir, tools, log)
	if err != nil {
		return report, err
	}
	report.TargetBackup = backup

	switch entry.Engine {
	case DriverSQLite:
		err = restoreSQLite(ctx, entry, target, log)
	case DriverPostgres:
		err = restorePostgres(ctx, entry, target, tools, log)
	default:
		err = fmt.Errorf("未知的存储引擎 %q", entry.Engine)
	}
	if err != nil {
		return report, err
	}

	// 还原后按 manifest 里的行数复核。这一步不改数据，只如实报告：还原本身已经完成，
	// 而"少了几行"必须让人看见（覆盖前的备份路径就在上面，可以回退）。
	tables, total, verifyErr := currentRowCounts(ctx, target)
	if verifyErr != nil {
		report.Note = "还原后无法复核行数：" + verifyErr.Error()
		log("注意：%s", report.Note)
		return report, nil
	}
	report.Tables, report.TotalRows = tables, total
	if mismatch := rowCountMismatch(entry.Tables, tables); mismatch != "" {
		report.Note = "还原完成，但行数与备份清单不一致：" + mismatch
		log("警告：%s", report.Note)
		return report, nil
	}
	report.Verified = true
	log("已还原 %s → %s（%d 张表 / %d 行，与备份清单一致）", entry.Name, target.Label, len(tables), total)
	return report, nil
}

// restoreSQLite 用备份副本替换目标库文件。
func restoreSQLite(ctx context.Context, entry BackupEntry, target EngineTarget, log func(string, ...any)) error {
	dest := strings.TrimSpace(target.Config.SQLitePath)
	if dest == "" {
		return errors.New("SQLite 目标端缺少 sqlite_path")
	}
	if !filepath.IsAbs(dest) {
		return fmt.Errorf("sqlite_path 必须是绝对路径：%q", dest)
	}
	src := filepath.Join(entry.Directory, entry.Files[0].Name)
	if err := checkSQLiteFile(src); err != nil {
		return err
	}
	tmp := dest + ".sync-tmp"
	if err := copyFile(src, tmp); err != nil {
		return fmt.Errorf("准备还原副本失败：%w", err)
	}
	if err := replaceSQLiteFile(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	log("SQLite 已还原：%s（来自 %s）", dest, entry.Name)
	return nil
}

// restorePostgres 先确认归档可读，再清空目标 schema，最后 pg_restore 写入。
//
// 为什么先清空：pg_restore --clean 只删"归档里有"的对象，目标端多出来的表会留下，
// 结果是一份"既不是备份、也不是原来"的混合库。清空 + 还原才能得到"与备份一致"这一
// 唯一诚实的语义。清空之后 pg_restore 失败会留下一个空库 —— 能扛住这一点，靠的正是
// 覆盖前那份自动备份（它的路径会出现在报告与日志里），这也是本路径先备份再动手的原因。
func restorePostgres(ctx context.Context, entry BackupEntry, target EngineTarget, tools Tools, log func(string, ...any)) error {
	dsn := strings.TrimSpace(target.Config.PostgresDSN)
	if dsn == "" {
		return errors.New("PostgreSQL 目标端缺少 postgres_dsn")
	}
	if strings.TrimSpace(tools.PgRestore) == "" {
		return errors.New("找不到 pg_restore.exe（便携 PostgreSQL 的 bin 目录）")
	}
	archive := filepath.Join(entry.Directory, entry.Files[0].Name)
	env, clean := pgEnv(dsn)
	var sink bytes.Buffer
	list := exec.CommandContext(ctx, tools.PgRestore, "-l", archive)
	list.Env = env
	list.Stdout, list.Stderr = &sink, &sink
	if err := list.Run(); err != nil {
		return fmt.Errorf("归档 %s 读不出来（拒绝清空目标端）：%v\n%s", entry.Files[0].Name, err, tailLines(sink.String(), 6))
	}

	store, err := Open(ctx, target.Config)
	if err != nil {
		return fmt.Errorf("打开 PostgreSQL 目标库失败：%w", err)
	}
	defer store.Close()
	pool, err := store.rawPool()
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	dropped, err := dropAllTables(ctx, tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	sequences, err := dropAllSequences(ctx, tx)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	if len(dropped) > 0 {
		log("目标端原有 %d 张表已清空：%s", len(dropped), strings.Join(dropped, "、"))
	}
	if len(sequences) > 0 {
		log("目标端另清掉 %d 个独立序列：%s", len(sequences), strings.Join(sequences, "、"))
	}

	sink.Reset()
	restore := exec.CommandContext(ctx, tools.PgRestore,
		"--no-owner", "--no-privileges", "-d", clean, archive)
	restore.Env = env
	restore.Stdout, restore.Stderr = &sink, &sink
	if err := restore.Run(); err != nil {
		return fmt.Errorf("pg_restore 失败：%v\n%s", err, tailLines(sink.String(), 10))
	}
	log("PostgreSQL 已还原：%s（来自 %s）", target.Label, entry.Name)
	return nil
}

// ---------------------------------------------------------------------------
// 目标端 schema：重建、元数据、插入顺序、序列
// ---------------------------------------------------------------------------

// rebuildSchemaInTx 在调用方的事务里重建 PostgreSQL 的整个 schema。
//
// 分节顺序与账本口径和 execMigration 一致（同一个 migrationQuery、同一个 checksum 规范化），
// 但这里必须跑在**调用方的事务**里：清空 + 建表 + 灌数据要一起回滚。差异只有一处 ——
// 账本刚刚被 DROP 掉，所以不存在"已应用过"的分支，每一节都直接执行并记账。
func rebuildSchemaInTx(ctx context.Context, tx pgx.Tx) error {
	ledger, err := migrationQuery("0000_migration_ledger.sql")
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, string(ledger)); err != nil {
		return fmt.Errorf("创建迁移账本失败：%w", err)
	}
	for _, name := range sqliteMigrationSections {
		query, err := migrationQuery(name)
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, string(query)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
		// 与 execMigration 同一口径：Windows 检出策略不得改变迁移的身份。
		checksum := fmt.Sprintf("%x", sha256Sum(strings.ReplaceAll(string(query), "\r\n", "\n")))
		if _, err := tx.Exec(ctx,
			"INSERT INTO storage_migrations(name,checksum) VALUES($1,$2)", name, checksum); err != nil {
			return err
		}
	}
	return nil
}

// pgColumn 是目标端一列的元数据（information_schema.columns 的子集）。
type pgColumn struct {
	Name     string
	DataType string
	UDTName  string
	Identity string
	Nullable string
	Default  *string
}

// postgresColumnMeta 读出目标 schema 里每张基表的列（按 ordinal_position）。
func postgresColumnMeta(ctx context.Context, tx pgx.Tx) (map[string][]pgColumn, error) {
	rows, err := tx.Query(ctx, `
		SELECT c.table_name, c.column_name, c.data_type, c.udt_name,
		       COALESCE(c.is_identity,'NO'), c.is_nullable, c.column_default
		FROM information_schema.columns c
		JOIN information_schema.tables t
		  ON t.table_schema = c.table_schema AND t.table_name = c.table_name
		WHERE c.table_schema = current_schema() AND t.table_type = 'BASE TABLE'
		ORDER BY c.table_name, c.ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]pgColumn{}
	for rows.Next() {
		var table string
		var column pgColumn
		if err := rows.Scan(&table, &column.Name, &column.DataType, &column.UDTName,
			&column.Identity, &column.Nullable, &column.Default); err != nil {
			return nil, err
		}
		out[table] = append(out[table], column)
	}
	return out, rows.Err()
}

// insertOrder 按外部键依赖给出插入顺序，返回顺序与"因成环而退回名字顺序"的表。
//
// PostgreSQL 的外部键是逐语句检查的，先插子表、后插父表必然失败。所以顺序不能靠猜：
// 依赖图从 pg_constraint 读，再做一次拓扑排序。本 schema 无环，成环分支只是兜底。
func insertOrder(ctx context.Context, tx pgx.Tx) ([]string, []string) {
	names := []string{}
	rows, err := tx.Query(ctx, `
		SELECT t.relname
		FROM pg_class t
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE n.nspname = current_schema() AND t.relkind = 'r'
		ORDER BY t.relname`)
	if err != nil {
		return names, nil
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return names, nil
		}
		names = append(names, name)
	}
	rows.Close()

	edges := map[string]map[string]bool{}
	for _, name := range names {
		edges[name] = map[string]bool{}
	}
	fk, err := tx.Query(ctx, `
		SELECT t.relname, p.relname
		FROM pg_constraint c
		JOIN pg_class t ON t.oid = c.conrelid
		JOIN pg_class p ON p.oid = c.confrelid
		JOIN pg_namespace n ON n.oid = t.relnamespace
		WHERE c.contype = 'f' AND n.nspname = current_schema()`)
	if err == nil {
		for fk.Next() {
			var child, parent string
			if err := fk.Scan(&child, &parent); err != nil {
				break
			}
			// 自引用不构成顺序约束。
			if child == parent {
				continue
			}
			if _, ok := edges[child]; ok {
				edges[child][parent] = true
			}
		}
		fk.Close()
	}

	done := map[string]bool{}
	var order []string
	for len(order) < len(names) {
		progressed := false
		for _, name := range names {
			if done[name] {
				continue
			}
			ready := true
			for parent := range edges[name] {
				if !done[parent] {
					ready = false
					break
				}
			}
			if ready {
				done[name] = true
				order = append(order, name)
				progressed = true
			}
		}
		if !progressed {
			// 成环：把剩下的按名字顺序接上，并如实说明（调用方会记进报告）。
			var rest []string
			for _, name := range names {
				if !done[name] {
					rest = append(rest, name)
					done[name] = true
				}
			}
			sort.Strings(rest)
			order = append(order, rest...)
			return order, rest
		}
	}
	return order, nil
}

// dropAllTables 在事务里 DROP 掉当前 schema 的全部基表（含迁移账本），返回被删的表名。
func dropAllTables(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for start := 0; start < len(names); start += 40 {
		end := start + 40
		if end > len(names) {
			end = len(names)
		}
		statement := "DROP TABLE IF EXISTS " + quoteIdentList(names[start:end]) + " CASCADE"
		if _, err := tx.Exec(ctx, statement); err != nil {
			return names, fmt.Errorf("清空目标端数据表失败：%w", err)
		}
	}
	return names, nil
}

// dropAllSequences 把当前 schema 里**没被表带走**的序列删掉。
//
// 为什么必须单独做：DROP TABLE ... CASCADE 只带走从属于表的对象（identity 序列、被表引用的
// 索引），而本 schema 里有一个独立的 `CREATE SEQUENCE mailbox_id_seq`（被 character_mail.id
// 的 DEFAULT 引用，但没有 OWNED BY，所以它不依赖那张表）。留着它，pg_restore 的
// `CREATE SEQUENCE public.mailbox_id_seq` 就会撞名失败：
//
//	pg_restore: error: could not execute query: ERROR: relation "mailbox_id_seq" already exists
//	     （2026-10-05 实机验收实测，restore 以 exit status 1 结束）
//
// 所以在"清空目标端 schema"这一步里，表和序列都要清。
func dropAllSequences(ctx context.Context, tx pgx.Tx) ([]string, error) {
	rows, err := tx.Query(ctx, `
		SELECT sequence_name FROM information_schema.sequences
		WHERE sequence_schema = current_schema()
		ORDER BY sequence_name`)
	if err != nil {
		return nil, err
	}
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return nil, err
		}
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, name := range names {
		if _, err := tx.Exec(ctx, "DROP SEQUENCE IF EXISTS "+quoteIdent(name)+" CASCADE"); err != nil {
			return names, fmt.Errorf("清空目标端序列失败：%w", err)
		}
	}
	return names, nil
}

// syncSequences 把 identity / nextval 序列推到位。
//
// 为什么必须做：覆盖式写入是**显式**写入主键的，序列本身不会被推进；不同步的话，
// 下一次服务端自己 INSERT 会拿到一个已经被占用的编号（重复键错误）。
func syncSequences(ctx context.Context, tx pgx.Tx, meta map[string][]pgColumn) error {
	for table, columns := range meta {
		for _, column := range columns {
			serial := column.Identity == "YES" ||
				(column.Default != nil && strings.HasPrefix(*column.Default, "nextval("))
			if !serial {
				continue
			}
			var sequence *string
			if err := tx.QueryRow(ctx, "SELECT pg_get_serial_sequence($1,$2)",
				quoteIdent(table), column.Name).Scan(&sequence); err != nil {
				return err
			}
			if sequence == nil || *sequence == "" {
				continue
			}
			if _, err := tx.Exec(ctx,
				"SELECT setval($1::regclass, COALESCE((SELECT max("+quoteIdent(column.Name)+") FROM "+quoteIdent(table)+"), 1))",
				*sequence); err != nil {
				return fmt.Errorf("同步 %s.%s 的序列失败：%w", table, column.Name, err)
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 值转换：SQLite → PostgreSQL
// ---------------------------------------------------------------------------

// sqliteValueToPostgres 把一个 SQLite 值转换成 PostgreSQL 驱动的入参。
//
// 反向规则与 sqliteconvert.go 的 convertValue 一一对应：时间在 SQLite 里是整数微秒
// （少数 DATE 列是 ISO 文本）、数组与 JSON 是文本、布尔是 0/1。没有对应关系的类型
// 原样传递，让驱动去决定（这正是两边表示相同的那些标量）。
func sqliteValueToPostgres(table string, column pgColumn, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch column.DataType {
	case "boolean":
		switch typed := value.(type) {
		case bool:
			return typed, nil
		case int64:
			return typed != 0, nil
		case float64:
			return typed != 0, nil
		case []byte:
			return parseBoolString(string(typed))
		case string:
			return parseBoolString(typed)
		}
	case "smallint", "integer", "bigint":
		number, err := toInt64(value)
		if err != nil {
			return nil, fmt.Errorf("表 %s 的 %s 列：%w", table, column.Name, err)
		}
		return number, nil
	case "text", "character varying", "character":
		return toString(value), nil
	case "timestamp with time zone", "timestamp without time zone":
		moment, err := toTime(value)
		if err != nil {
			return nil, fmt.Errorf("表 %s 的 %s 列：%w", table, column.Name, err)
		}
		return moment, nil
	case "date":
		if text, ok := value.(string); ok {
			if moment, err := time.Parse("2006-01-02", strings.TrimSpace(text)); err == nil {
				return moment, nil
			}
		}
		if raw, ok := value.([]byte); ok {
			if moment, err := time.Parse("2006-01-02", strings.TrimSpace(string(raw))); err == nil {
				return moment, nil
			}
		}
		moment, err := toTime(value)
		if err != nil {
			return nil, fmt.Errorf("表 %s 的 %s 列：%w", table, column.Name, err)
		}
		return moment, nil
	case "json", "jsonb":
		// JSON 文本原样交给驱动（pgx 对 string 走"直接写入"的快路径），由 PostgreSQL
		// 自己归一化；写完之后 canonicalJSON 会逐行比对内容（见 copySQLiteTable）。
		return toString(value), nil
	case "ARRAY":
		list, err := toIntSlice(value)
		if err != nil {
			return nil, fmt.Errorf("表 %s 的 %s 列（%s）：%w", table, column.Name, column.UDTName, err)
		}
		return list, nil
	case "bytea":
		switch typed := value.(type) {
		case []byte:
			return typed, nil
		case string:
			return []byte(typed), nil
		}
	}
	return value, nil
}

// toInt64 把 SQLite 可能给出的几种表示收敛成 int64。
func toInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case int:
		return int64(typed), nil
	case float64:
		return int64(typed), nil
	case bool:
		if typed {
			return 1, nil
		}
		return 0, nil
	case []byte:
		return parseIntString(string(typed))
	case string:
		return parseIntString(typed)
	case time.Time:
		return typed.UTC().UnixMicro(), nil
	}
	return 0, fmt.Errorf("无法把 %T 当成整数", value)
}

func parseIntString(text string) (int64, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, errors.New("空字符串不是整数")
	}
	var out int64
	if _, err := fmt.Sscanf(text, "%d", &out); err != nil {
		return 0, fmt.Errorf("%q 不是整数", text)
	}
	return out, nil
}

func parseBoolString(text string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(text)) {
	case "1", "t", "true", "yes", "on":
		return true, nil
	case "0", "f", "false", "no", "off", "":
		return false, nil
	}
	return false, fmt.Errorf("%q 不是布尔值", text)
}

// toString 把 SQLite 的 TEXT/BLOB 表示收敛成 string。
func toString(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []byte:
		return string(typed)
	case nil:
		return ""
	}
	return fmt.Sprint(value)
}

// toTime 把 SQLite 的时间表示收敛成 time.Time（库里存的是整数微秒）。
func toTime(value any) (time.Time, error) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), nil
	case int64:
		return time.UnixMicro(typed).UTC(), nil
	case int:
		return time.UnixMicro(int64(typed)).UTC(), nil
	case float64:
		return time.UnixMicro(int64(typed)).UTC(), nil
	case []byte:
		return parseTimeString(string(typed))
	case string:
		return parseTimeString(typed)
	}
	return time.Time{}, fmt.Errorf("无法把 %T 当成时间", value)
}

func parseTimeString(text string) (time.Time, error) {
	text = strings.TrimSpace(text)
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02 15:04:05.999999-07", "2006-01-02 15:04:05"} {
		if moment, err := time.Parse(layout, text); err == nil {
			return moment.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("%q 不是可识别的时间", text)
}

// toIntSlice 把 SQLite 里的 JSON 数组文本转成 PostgreSQL 数组入参（本 schema 只有 bigint[]）。
func toIntSlice(value any) ([]int64, error) {
	text := strings.TrimSpace(toString(value))
	if text == "" {
		return nil, nil
	}
	var numbers []int64
	if err := json.Unmarshal([]byte(text), &numbers); err != nil {
		return nil, fmt.Errorf("数组文本 %q 解析失败：%w", text, err)
	}
	return numbers, nil
}

// ---------------------------------------------------------------------------
// JSON 校验：跨引擎比较内容而不是字节
// ---------------------------------------------------------------------------

// canonicalJSON 把一段 JSON 归一成"语义相同则字符串相同"的形式：
// 对象键排序、数字按精确有理数归一（1.0 与 1 相等）。
//
// 为什么不比字节：SQLite 存的是文本，PostgreSQL 的 jsonb 会在存储时重新归一
// （键序、空白、数字写法都可能变），字节比较必然误报。也不比 float64：
// 大整数会在往返里丢精度，同样误报。json.Number + big.Rat 两个坑一起避开。
func canonicalJSON(raw any) ([]byte, error) {
	var decoded any
	switch typed := raw.(type) {
	case []byte:
		if len(bytes.TrimSpace(typed)) == 0 {
			return []byte("null"), nil
		}
		if err := decodeNumbers(typed, &decoded); err != nil {
			return nil, err
		}
	case string:
		if strings.TrimSpace(typed) == "" {
			return []byte("null"), nil
		}
		if err := decodeNumbers([]byte(typed), &decoded); err != nil {
			return nil, err
		}
	default:
		return json.Marshal(typed)
	}
	return json.Marshal(canonicalizeNumbers(decoded))
}

func decodeNumbers(raw []byte, out *any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	return decoder.Decode(out)
}

func canonicalizeNumbers(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		for key, sub := range typed {
			typed[key] = canonicalizeNumbers(sub)
		}
		return typed
	case []any:
		for i, sub := range typed {
			typed[i] = canonicalizeNumbers(sub)
		}
		return typed
	case json.Number:
		if rat, ok := new(big.Rat).SetString(typed.String()); ok {
			return rat.RatString()
		}
		return typed.String()
	}
	return value
}

// jsonDigest 对一列 JSON 值给出与行序无关的摘要：逐行哈希、排序后再折一次。
//
// 与行序无关是必须的：SQLite 与 PostgreSQL 的物理行序本来就不同，而"内容一致"
// 才是要证明的事。重复行不会互相抵消（每一行都单独进摘要）。
func jsonDigest(values [][]byte) string {
	hashes := make([]string, 0, len(values))
	for _, value := range values {
		sum := sha256Sum(string(value))
		hashes = append(hashes, hex.EncodeToString(sum))
	}
	sort.Strings(hashes)
	digest := sha256.New()
	for _, hash := range hashes {
		io.WriteString(digest, hash)
		io.WriteString(digest, "\n")
	}
	return hex.EncodeToString(digest.Sum(nil))
}

func sha256Sum(text string) []byte {
	sum := sha256.Sum256([]byte(text))
	return sum[:]
}

// ---------------------------------------------------------------------------
// 通用小工具
// ---------------------------------------------------------------------------

// currentRowCounts 按引擎读出目标端当前的每表行数（还原后复核用）。
func currentRowCounts(ctx context.Context, target EngineTarget) ([]TableRows, int64, error) {
	switch target.Engine {
	case DriverSQLite:
		return sqliteRowCounts(ctx, target.Config.SQLitePath)
	case DriverPostgres:
		store, err := Open(ctx, target.Config)
		if err != nil {
			return nil, 0, err
		}
		defer store.Close()
		pool, err := store.rawPool()
		if err != nil {
			return nil, 0, err
		}
		return postgresRowCounts(ctx, pool)
	}
	return nil, 0, fmt.Errorf("未知的存储引擎 %q", target.Engine)
}

// rowCountMismatch 比较两份行数清单，一致时返回空串。
func rowCountMismatch(want, got []TableRows) string {
	wanted := map[string]int64{}
	for _, table := range want {
		wanted[table.Name] = table.Rows
	}
	var problems []string
	for _, table := range got {
		if expected, ok := wanted[table.Name]; !ok {
			problems = append(problems, fmt.Sprintf("%s 多出 %d 行", table.Name, table.Rows))
		} else if expected != table.Rows {
			problems = append(problems, fmt.Sprintf("%s 期望 %d 行、实际 %d 行", table.Name, expected, table.Rows))
		}
		delete(wanted, table.Name)
	}
	for name, rows := range wanted {
		problems = append(problems, fmt.Sprintf("%s 缺 %d 行", name, rows))
	}
	sort.Strings(problems)
	return strings.Join(problems, "；")
}

// sqliteRowCounts 数一份 SQLite 库每张表的行数（只读 + immutable，不动文件）。
func sqliteRowCounts(ctx context.Context, path string) ([]TableRows, int64, error) {
	db, err := openSQLiteImmutable(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer db.Close()
	names, err := sqliteTableNames(ctx, db)
	if err != nil {
		return nil, 0, err
	}
	var out []TableRows
	var total int64
	for _, name := range names {
		if engineLocalTables[name] {
			continue
		}
		var rows int64
		if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+quoteIdent(name)).Scan(&rows); err != nil {
			return nil, 0, fmt.Errorf("统计表 %s 行数失败：%w", name, err)
		}
		out = append(out, TableRows{Name: name, Rows: rows})
		total += rows
	}
	return out, total, nil
}

// postgresRowCounts 数目标库每张基表的行数（跳过引擎自己的账本表）。
func postgresRowCounts(ctx context.Context, pool *pgxpool.Pool) ([]TableRows, int64, error) {
	names, err := postgresTableNames(ctx, pool)
	if err != nil {
		return nil, 0, err
	}
	var out []TableRows
	var total int64
	for _, name := range names {
		if engineLocalTables[name] {
			continue
		}
		var rows int64
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+quoteIdent(name)).Scan(&rows); err != nil {
			return nil, 0, fmt.Errorf("统计表 %s 行数失败：%w", name, err)
		}
		out = append(out, TableRows{Name: name, Rows: rows})
		total += rows
	}
	return out, total, nil
}

func postgresTableNames(ctx context.Context, pool *pgxpool.Pool) ([]string, error) {
	rows, err := pool.Query(ctx, `
		SELECT table_name FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_type = 'BASE TABLE'
		ORDER BY table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// sqliteColumnNames 读一张 SQLite 表的列名（按声明顺序）。
func sqliteColumnNames(ctx context.Context, db *sql.DB, table string) ([]string, error) {
	rows, err := db.QueryContext(ctx, "PRAGMA table_info("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var cid int64
		var name, columnType string
		var notNull, primaryKey int64
		var defaultValue any
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

// openSQLiteImmutable 以只读 + 不可变方式打开一份**副本**（备份产物）。
//
// 与引擎的连接串刻意不同：不给副本建立 -shm/-wal，也不加锁，所以"读一遍备份"不会
// 改动备份目录里的任何字节（否则清单里的 sha256 会在校验之后失效）。
func openSQLiteImmutable(ctx context.Context, path string) (*sql.DB, error) {
	slashed := filepath.ToSlash(path)
	if !strings.HasPrefix(slashed, "/") {
		slashed = "/" + slashed // C:/x -> /C:/x，与 sqliteDSN 同一处理
	}
	db, err := sql.Open("sqlite", "file://"+slashed+"?mode=ro&immutable=1&_timezone=UTC&_dqs=0")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// replaceSQLiteFile 用 src（已校验完的新库）替换 dest。
//
// 替换之前必须先删掉 dest 的 -wal / -shm / -journal：它们是**旧库**的附属文件，
// 留着会让 SQLite 下次打开时按旧 WAL 恢复，表现为"覆盖成功但数据没变"，甚至损坏。
func replaceSQLiteFile(src, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("创建目标端目录失败：%w", err)
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if err := os.Remove(dest + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("删除旧库附属文件 %s 失败：%w", dest+suffix, err)
		}
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	// Windows 上目标文件被占用/已存在时 Rename 可能失败：删掉再试一次，仍失败就如实报错。
	if err := os.Remove(dest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("替换 %s 失败：%w", dest, err)
	}
	if err := os.Rename(src, dest); err != nil {
		return fmt.Errorf("替换 %s 失败：%w", dest, err)
	}
	return nil
}

// checkSQLiteFile 确认一个文件确实是 SQLite 库（文件头魔数）。
//
// 光比对 sha256 只能证明"文件没被改过"，证明不了"它是一份 SQLite 存档"：
// manifest 是人可读的 JSON，指错文件时用它覆盖目标端会直接毁掉目标库。
func checkSQLiteFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	header := make([]byte, 16)
	if _, err := io.ReadFull(file, header); err != nil {
		return fmt.Errorf("%s 读不出 SQLite 文件头：%w", path, err)
	}
	if string(header) != "SQLite format 3\x00" {
		return fmt.Errorf("%s 不是 SQLite 存档（文件头不符），拒绝用它覆盖目标端", path)
	}
	return nil
}

// copyFile 复制文件（先写 .partial 再改名，避免留下半个文件被当成真备份）。
func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".partial"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// describeFile 给出一个文件的路径、大小与 sha256。
func describeFile(path string) (BackupFile, error) {
	file, err := os.Open(path)
	if err != nil {
		return BackupFile{}, err
	}
	defer file.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, file)
	if err != nil {
		return BackupFile{}, err
	}
	return BackupFile{Name: filepath.Base(path), Size: size, SHA256: hex.EncodeToString(digest.Sum(nil))}, nil
}

// dirSize 递归统计目录大小（失败时返回已知部分，不因此让整个列表失败）。
func dirSize(dir string) int64 {
	var total int64
	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}

// uniquePath 在路径已存在时追加 -2、-3……（同一秒内对同一引擎备份两次不会互相覆盖）。
func uniquePath(path string) string {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		return path
	}
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", path, i)
		if _, err := os.Stat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return fmt.Sprintf("%s-%d", path, time.Now().UnixNano())
}

// quoteLiteral 把一个字符串放进 SQL 单引号里（SQLite 的 VACUUM INTO 不接受绑定参数）。
func quoteLiteral(text string) string {
	return "'" + strings.ReplaceAll(text, "'", "''") + "'"
}

// tailLines 取文本末尾若干行，用于把子进程（pg_dump/pg_restore）的报错缩到可读。
func tailLines(text string, n int) string {
	lines := strings.Split(strings.TrimRight(text, "\r\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// listHas 判断字符串切片里有没有某个值（本包最小的集合判断，别引入新依赖）。
func listHas(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func containsInt(list []int, want int) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func hasColumn(columns []pgColumn, name string) bool {
	for _, column := range columns {
		if column.Name == name {
			return true
		}
	}
	return false
}
