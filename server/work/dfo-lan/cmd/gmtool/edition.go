package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/localgm"
	"dfolan/internal/toolcmd/setlevel"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Set by the two release builds; empty preserves the historical GM executable.
var defaultEdition string

func onlineReady(db string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	return localgm.Ready(ctx, db)
}

type editionManager struct {
	kind        string
	config      database.Config
	backups     string
	progression catalog.Progression
	quests      catalog.QuestCatalog
	skills      *character.Service
	writeMu     sync.Mutex
}

func moduleDir(root string) string {
	for _, rel := range []string{filepath.Join("02_服务端", "server", "work", "dfo-lan"), filepath.Join("server", "work", "dfo-lan")} {
		p := filepath.Join(root, rel)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p
		}
	}
	return filepath.Join(root, "server", "work", "dfo-lan")
}

func editionRoot() string {
	if defaultEdition == "" {
		return defaultRootDir()
	}
	exe, _ := os.Executable()
	dir := filepath.Dir(exe)
	for i := 0; i < 7; i++ {
		p := editionPaths(dir, defaultEdition)
		if _, err := os.Stat(p.storage); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	if defaultEdition == "cloud" {
		return `C:\DFO115-cloud-test`
	}
	return `D:\115us`
}

func editionPaths(root, kind string) paths {
	p := resolvePaths(root)
	base := moduleDir(root)
	if kind == "cloud" {
		base = root
	}
	p.storage = filepath.Join(base, "runtime", "storage", "local.json")
	p.configs = filepath.Join(base, "configs")
	p.bagRules = filepath.Join(p.configs, "inventory.current37.json")
	if kind == "cloud" {
		p.clientPVF = filepath.Join(root, "resources", "Script.inner.pvf")
	} else {
		p.clientPVF = filepath.Join(filepath.Dir(base), "client-build", "Script.inner.pvf")
	}
	p.namesZH = filepath.Join(base, "runtime", "l10n", "names.zh.json")
	p.namesClient = filepath.Join(root, "gm-tool", "configs", "names.client.json")
	p.itemIndex = "" // Runtime content comes from the selected native PVF.
	p.equipmentSlots = filepath.Join(root, "gm-tool", "configs", "equipment.slots.json")
	p.equipmentFull = filepath.Join(p.configs, "equipment-full.json")
	return p
}

func validateLoopback(address string) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("GM只允许本机回环地址，不能监听公网：%s", address)
	}
	return nil
}

func newEditionManager(p paths, cfg database.Config, progression catalog.Progression, expectedDatabase ...string) (*editionManager, error) {
	if defaultEdition != "local" && defaultEdition != "cloud" {
		return nil, fmt.Errorf("未知GM版本")
	}
	driver, err := database.EngineForConfig(cfg)
	if err != nil || driver != database.DriverSQLite {
		return nil, fmt.Errorf("GM需要现有SQLite存档：%v", err)
	}
	base := moduleDir(p.root)
	name := "dfolan.sqlite3"
	if defaultEdition == "cloud" {
		base = p.root
		name = "cloud-test.sqlite3"
	}
	expected, err := filepath.Abs(filepath.Join(base, "runtime", "storage", name))
	if err != nil {
		return nil, err
	}
	actual := cfg.SQLitePath
	if !filepath.IsAbs(actual) {
		actual = filepath.Join(base, actual)
	}
	actual, err = filepath.Abs(actual)
	if err != nil {
		return nil, err
	}
	// A previous folder reorganisation left the old root/server path in the local
	// profile. Map only that exact historical path, in memory, never edit the file.
	historical := filepath.Join(p.root, "server", "work", "dfo-lan", "runtime", "storage", name)
	if defaultEdition == "local" && strings.EqualFold(actual, historical) {
		legacyInfo, legacyErr := os.Stat(actual)
		canonicalInfo, canonicalErr := os.Stat(expected)
		if os.IsNotExist(legacyErr) || (legacyErr == nil && canonicalErr == nil && os.SameFile(legacyInfo, canonicalInfo)) {
			actual = expected
		}
	}
	if defaultEdition == "cloud" && !strings.EqualFold(filepath.Clean(actual), filepath.Clean(expected)) {
		return nil, fmt.Errorf("版本隔离拒绝：%s GM只管理 %s，配置却指向 %s", defaultEdition, expected, actual)
	}
	if defaultEdition == "local" {
		if strings.EqualFold(filepath.Clean(actual), filepath.Clean(historical)) && !strings.EqualFold(filepath.Clean(actual), filepath.Clean(expected)) {
			return nil, fmt.Errorf("旧目录存在不同存档，请在活动配置中明确选择实际数据库")
		}
		if strings.EqualFold(filepath.Clean(actual), filepath.Join(p.root, "runtime", "storage", "cloud-test.sqlite3")) {
			return nil, fmt.Errorf("单机GM拒绝联网版专用存档")
		}
	}
	check := ""
	if len(expectedDatabase) > 0 {
		check = expectedDatabase[0]
	}
	actual, err = database.ExistingSQLitePath(actual, check)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(actual)
	if err != nil || info.IsDir() {
		return nil, fmt.Errorf("找不到现有存档 %s；不会创建空库", actual)
	}
	cfg.SQLitePath = actual
	if defaultEdition == "cloud" {
		cfg.DurableWrites = true
	}
	return &editionManager{kind: defaultEdition, config: cfg, backups: filepath.Join(filepath.Dir(actual), "gm-backups", defaultEdition), progression: progression}, nil
}

func (e *editionManager) label() string {
	if e.kind == "cloud" {
		return "115US 联网服主 GM"
	}
	return "115US 单机 GM"
}

func (e *editionManager) acquire(ctx context.Context, store *database.Store) (func(), error) {
	// Never reclaim even a stale lease here: a stalled game process can still own
	// an in-memory save. The existing stop/clear-guard workflow owns recovery.
	if _, err := os.Stat(e.config.SQLitePath + ".admin-guard"); err == nil {
		return nil, fmt.Errorf("游戏服务或其他GM仍持有存档管理租约")
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	return store.HoldAdminGuard(ctx)
}

func (e *editionManager) backup(ctx context.Context) (database.BackupEntry, error) {
	b, err := database.Backup(ctx, database.SQLiteTarget(e.config.SQLitePath), e.backups, nil)
	if err == nil {
		err = database.VerifyBackup(b)
	}
	return b, err
}

func (s *server) registerEditionRoutes(mux *http.ServeMux) {
	s.registerGMOperations(mux)
	mux.HandleFunc("/api/gm/status", s.auth(s.handleEditionStatus))
	mux.HandleFunc("/api/gm/level", s.auth(s.handleEditionLevel))
	mux.HandleFunc("/api/gm/backup", s.auth(s.handleEditionBackup))
	mux.HandleFunc("/api/gm/backups", s.auth(s.handleEditionBackups))
}

func (s *server) handleEditionStatus(w http.ResponseWriter, r *http.Request) {
	_, err := os.Stat(s.edition.config.SQLitePath + ".admin-guard")
	writeJSON(w, map[string]any{"edition": s.edition.kind, "label": s.edition.label(), "database": s.edition.config.SQLitePath, "online": s.edition.kind == "local" && onlineReady(s.edition.config.SQLitePath), "write_ready": os.IsNotExist(err) || (s.edition.kind == "local" && onlineReady(s.edition.config.SQLitePath)), "backup_directory": s.edition.backups, "level_max": len(s.edition.progression.Thresholds) + 1, "version": "1.2.0", "note": "单机在线支持发物品、金币/点券和等级；其他修改需停服。每次修改前自动备份。等级与累计经验同步，不赠送技能点或完成任务。"})
}

func (s *server) handleEditionBackup(w http.ResponseWriter, r *http.Request) {
	if !dashboardRequest(w, r, http.MethodPost, nil) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	b, err := s.edition.backup(ctx)
	if err != nil {
		writeErr(w, 500, "备份失败：%v", err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "backup": b})
}

func (s *server) handleEditionBackups(w http.ResponseWriter, r *http.Request) {
	rows, err := database.ListBackups(s.edition.backups)
	if err != nil {
		writeErr(w, 500, "%v", err)
		return
	}
	writeJSON(w, map[string]any{"backups": rows})
}

func (s *server) handleEditionLevel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Account   int64  `json:"account"`
		Character int64  `json:"character"`
		Level     int    `json:"level"`
		GrantID   string `json:"grant_id"`
		Reason    string `json:"reason"`
	}
	if !dashboardRequest(w, r, http.MethodPost, &req) {
		return
	}
	if req.Account <= 0 || req.Character <= 0 || req.Level < 2 || req.Level > 255 || len(req.GrantID) < 8 || len(req.GrantID) > 128 || strings.TrimSpace(req.Reason) == "" {
		writeErr(w, 400, "账号、角色、等级、操作编号或原因无效")
		return
	}
	threshold, err := setlevel.LevelThreshold(s.edition.progression, byte(req.Level))
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	result, err := s.admin.Store.ApplyGrant(ctx, database.Grant{ID: req.GrantID, AccountID: req.Account, Character: req.Character, Reason: req.Reason, Operator: "gm-" + s.edition.kind}, func(current database.Character) (json.RawMessage, json.RawMessage, error) {
		return setlevel.ApplyLevel(current, byte(req.Level), threshold)
	})
	if err != nil {
		writeErr(w, 400, "%v", err)
		return
	}
	writeJSON(w, map[string]any{"ok": true, "applied": result.Applied, "grant_id": req.GrantID, "receipt": result.Receipt})
}
