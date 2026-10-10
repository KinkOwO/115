// dfo115gmtool —— 115us 单机服 GM 工具。
//
// 用法：
//
//	DFO115GMTool.exe -root D:\115us
//
// 启动后只在 127.0.0.1 上监听一个随机端口，并自动打开浏览器界面。
// 背包发放走既有事务、幂等与审计路径（与 cmd/admin 一致）。
// Dashboard 邮件保留独立管理队列；主金库操作复用 storage 的事务。
package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"dfolan/internal/admin"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/localgm"
	"dfolan/internal/managementdata"
	"dfolan/internal/toolcmd/gmicons"
	"dfolan/internal/toolcmd/gmremote"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

type paths struct {
	root           string
	storage        string
	configs        string
	namesZH        string
	namesEN        string
	namesClient    string
	itemIndex      string
	equipmentSlots string
	lootCatalog    string
	bagRules       string
	equipCatalog   string
	clientPVF      string
	equipmentFull  string
}

// 独立发布包 D:\115us\gm-tool 里的默认路径。
// 运行物品目录从原生 PVF 准备，外部名字表仅补翻译。
const (
	// defaultItemIndex 是全量物品库（386230 件）。
	defaultItemIndex = `D:\115us\gm-tool\configs\items.index.json`
	// defaultNamesClient 是 names.go 从客户端内层 PVF 的 string/*.uv.str 文本表
	// 导出的名字表 —— 这才是游戏里显示的名字，优先于 runtime/l10n/names.zh.json。
	defaultNamesClient = `D:\115us\gm-tool\configs\names.client.json`
	// defaultEquipmentSlots 是 names.go 从装备目录导出的 id -> [部位cell, 最低等级] 表。
	defaultEquipmentSlots = `D:\115us\gm-tool\configs\equipment.slots.json`
	// defaultClientPVF 是客户端当前汉化补丁解密出来的内层 PVF（-build-data 的输入）。
	defaultClientPVF = `D:\115us\work-cn\verify\clientpatch\Script.inner.pvf`
	// defaultEquipmentFull 是对客户端内层 PVF 全量遍历得到的装备目录（121726 行），
	// 用来补齐 configs/equipment.current37.json（19955 行）没覆盖到的物品。
	// 由上游仓库的 cmd/equipmentfull 产出；缺失时 -build-data 只用 current37。
	defaultEquipmentFull = `D:\115us\upstream\repo\server\work\dfo-lan\configs\equipment-full.json`
)

func resolvePaths(root string) paths {
	base := filepath.Join(root, "server", "work", "dfo-lan")
	l10n := filepath.Join(base, "runtime", "l10n")
	configs := filepath.Join(base, "configs")
	return paths{
		root:           root,
		storage:        filepath.Join(base, "runtime", "storage", "local.json"),
		configs:        configs,
		namesZH:        filepath.Join(l10n, "names.zh.json"),
		namesEN:        filepath.Join(l10n, "names.en.json"),
		namesClient:    defaultNamesClient,
		itemIndex:      defaultItemIndex,
		equipmentSlots: defaultEquipmentSlots,
		lootCatalog:    "",
		bagRules:       filepath.Join(configs, "inventory.next29.json"),
		equipCatalog:   "",
		clientPVF:      defaultClientPVF,
		equipmentFull:  defaultEquipmentFull,
	}
}

// applyOverrides 用命令行显式给出的目录覆盖默认路径。
// 独立发布包 D:\115us\gm-tool 通过 scripts/gmweb.py 传入它自己的 configs 路径。
func (p *paths) applyOverrides(lootCatalog, bagRules, equipCatalog, itemIndex, namesClient, equipmentSlots string) {
	for _, o := range []struct {
		v   string
		dst *string
	}{
		{lootCatalog, &p.lootCatalog},
		{bagRules, &p.bagRules},
		{equipCatalog, &p.equipCatalog},
		{itemIndex, &p.itemIndex},
		{namesClient, &p.namesClient},
		{equipmentSlots, &p.equipmentSlots},
	} {
		if strings.TrimSpace(o.v) != "" {
			*o.dst = o.v
		}
	}
}

// resolveDataFile 按候选顺序挑一个真实存在的文件，让发布包可以整体搬到任意路径。
// 显式给出的路径优先；否则依次尝试传入的候选目录（物品库同目录 / exe 的 ../configs /
// exe 同目录 configs / -root 环境的 configs），最后才是本机的历史默认路径。
// 全都不存在时返回第一个候选，方便报错时指向它。
func resolveDataFile(explicit, name string, dirs []string, fallback string) string {
	if strings.TrimSpace(explicit) != "" {
		return explicit
	}
	var candidates []string
	for _, d := range dirs {
		if strings.TrimSpace(d) == "" {
			continue
		}
		candidates = append(candidates, filepath.Join(d, name))
	}
	if fallback != "" {
		candidates = append(candidates, fallback)
	}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c
		}
	}
	if len(candidates) > 0 {
		return candidates[0]
	}
	return ""
}

func defaultRootDir() string {
	exe, err := os.Executable()
	if err != nil {
		return `D:\115us`
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(exe)))
}

type server struct {
	store    gmStore
	admin    *admin.Service
	index    *ItemIndex
	paths    paths
	token    string
	loot     catalog.LootCatalog
	started  time.Time
	operator string
	edition  *editionManager
}

func main() {
	sourceFlags := managementdata.Register(flag.CommandLine)
	root := flag.String("root", editionRoot(), "115us 环境根目录（默认从程序所在位置推导）")
	remoteConfig := flag.String("remote-controller", "", "联网GM远程维护控制器配置（不准备物品目录、不打开玩家库）")
	remoteInit := flag.String("remote-init", "", "创建远程GM服务器和服主连接文件的目录")
	remoteHost := flag.String("remote-host", "", "远程GM服务器IP（仅用于remote-init）")
	listen := flag.String("listen", "127.0.0.1:0", "本地监听地址")
	openBrowser := flag.Bool("open", true, "启动后自动打开浏览器")
	expectedDatabase := flag.String("expected-database", "", "启动器已解析的现有SQLite文件；必须是同一文件")
	operator := flag.String("operator", "gm-tool", "审计里记录的操作用户")
	// 兼容独立发布包 D:\115us\gm-tool 的启动参数（scripts/gmweb.py 会全部传进来）。
	storagePath := flag.String("storage", "", "local.json 路径（默认 -root 下的 runtime/storage/local.json）")
	itemIndexPath := flag.String("item-index", "", "兼容旧参数；运行内容只读PVF，不读取该JSON")
	lootCatalog := flag.String("loot-catalog", "", "已退休兼容参数；运行内容使用 PVF")
	bagRules := flag.String("bag-rules", "", "背包规则路径（默认 -root 下的 configs/inventory.next29.json）")
	equipCatalog := flag.String("equipment-catalog", "", "已退休兼容参数；运行内容使用 PVF")
	// 名字与部位这两个"和游戏对齐"的数据源，见 names.go 的包注释。
	namesClient := flag.String("names-client", "",
		"客户端 PVF 文本表导出的名字表 names.client.json（游戏里显示的名字；留空则自动查找，缺失时回落到 runtime/l10n/names.zh.json）")
	equipmentSlots := flag.String("equipment-slots", "",
		"装备部位/等级表 equipment.slots.json（留空则自动查找，缺失时回落到只用 -equipment-catalog 分部位）")
	// -build-data 是离线的一次性数据准备步骤，不需要数据库，也不碰 runtime/l10n/*。
	buildData := flag.Bool("build-data", false,
		"只生成 names.client.json 与 equipment.slots.json 后退出（不需要数据库）")
	clientPVF := flag.String("client-pvf", defaultClientPVF, "-build-data：客户端内层 PVF（字符串表来源）")
	equipmentFull := flag.String("equipment-full", defaultEquipmentFull,
		"-build-data：可选的补充装备目录（用于提高部位覆盖率）")
	// 下面三个参数独立包会传，但本工具的发放路径（admin.Service + inventory.Awarder）
	// 不使用角色模板/成长/经验规则，因此只接受不生效。
	charCatalog := flag.String("character-catalog", "", "（可接受但忽略）角色模板目录")
	progCatalog := flag.String("progression-catalog", "", "（可接受但忽略）成长目录")
	progRules := flag.String("progression-rules", "", "（可接受但忽略）经验规则")
	desktopIcons := flag.String("desktop-icons", "", "read-only local ImagePacks2 stdio renderer; no player storage")
	flag.Parse()
	if *desktopIcons != "" {
		if err := gmicons.Run(sourceFlags.ArchivePath, *desktopIcons, os.Stdin, os.Stdout); err != nil {
			log.Fatal(err)
		}
		return
	}
	if *remoteInit != "" {
		if err := gmremote.Provision(*remoteInit, *remoteHost); err != nil {
			log.Fatal(err)
		}
		fmt.Println("Remote owner files created; credentials were not printed")
		return
	}
	if *remoteConfig != "" {
		if defaultEdition != "cloud" {
			log.Fatal("远程控制器只允许联网GM版本")
		}
		if err := gmremote.Run(*remoteConfig); err != nil {
			log.Fatal(err)
		}
		return
	}

	// 发布包可能被解压到任意路径（不是 D:\115us），所以这几个数据文件按候选目录
	// 自动定位：物品库同目录 → exe 的 ../configs（发布包里就是它自己的 configs）
	// → exe 同目录 configs → -root 环境的 configs。显式传入的路径永远优先。
	exeDir := ""
	if exe, e := os.Executable(); e == nil {
		exeDir = filepath.Dir(exe)
	}
	defaultDirs := func(extra ...string) []string {
		dirs := append([]string{}, extra...)
		if exeDir != "" {
			dirs = append(dirs, filepath.Join(exeDir, "..", "configs"), filepath.Join(exeDir, "configs"))
		}
		dirs = append(dirs, filepath.Join(*root, "server", "work", "dfo-lan", "configs"))
		return dirs
	}
	itemIndexResolved := *itemIndexPath
	namesClientResolved := resolveDataFile(*namesClient, "names.client.json",
		defaultDirs(filepath.Dir(itemIndexResolved)), defaultNamesClient)
	equipmentSlotsResolved := resolveDataFile(*equipmentSlots, "equipment.slots.json",
		defaultDirs(filepath.Dir(itemIndexResolved)), defaultEquipmentSlots)
	if defaultEdition != "" {
		// A packaged edition belongs to its selected root. Missing optional
		// display tables must never fall through to cwd or a developer install.
		dirs := []string{filepath.Join(*root, "gm-tool", "configs"), filepath.Join(moduleDir(*root), "configs")}
		namesClientResolved = resolveDataFile(*namesClient, "names.client.json", dirs, "")
		equipmentSlotsResolved = resolveDataFile(*equipmentSlots, "equipment.slots.json", dirs, "")
	}

	p := resolvePaths(*root)
	if defaultEdition != "" {
		p = editionPaths(*root, defaultEdition)
		if *bagRules == "" {
			*bagRules = p.bagRules
		}
		if sourceFlags.DropPolicy == "configs/pvf-drop-policy.json" {
			sourceFlags.DropPolicy = filepath.Join(p.configs, "pvf-drop-policy.json")
		}
	}
	if strings.TrimSpace(*storagePath) != "" {
		p.storage = *storagePath
	}
	p.applyOverrides(*lootCatalog, *bagRules, *equipCatalog, itemIndexResolved, namesClientResolved, equipmentSlotsResolved)
	// 显式给了配置路径时，configs 目录以用户给的为准（独立包不在 -root 下面）。
	p.configs = filepath.Dir(p.bagRules)
	if defaultEdition != "" && *clientPVF == defaultClientPVF {
		*clientPVF = p.clientPVF
	}
	if defaultEdition != "" && *equipmentFull == defaultEquipmentFull {
		*equipmentFull = p.equipmentFull
	}
	p.clientPVF, p.equipmentFull = *clientPVF, *equipmentFull

	// 数据准备模式：只读 PVF/装备目录，写两个新文件，不连数据库。
	if *buildData {
		if err := buildDataFiles(p); err != nil {
			log.Fatalf("-build-data 失败：%v", err)
		}
		return
	}

	for _, o := range []struct{ name, v string }{
		{"-character-catalog", *charCatalog},
		{"-progression-catalog", *progCatalog},
		{"-progression-rules", *progRules},
	} {
		if strings.TrimSpace(o.v) != "" {
			log.Printf("说明：%s 已接受但本工具不使用（发放只依赖掉落/装备目录与背包规则），忽略 %s", o.name, o.v)
		}
	}

	if sourceFlags.ArchivePath == "" {
		if defaultEdition != "" {
			sourceFlags.ArchivePath = p.clientPVF
		} else {
			sourceFlags.ArchivePath = filepath.Join(*root, "server", "work", "client-build", "Script.inner.pvf")
		}
	}
	if err := validateLoopback(*listen); err != nil {
		log.Fatal(err)
	}
	if defaultEdition != "" && !sourceFlags.CheckOnly {
		preCfg, e := database.LoadConfig(p.storage)
		if e != nil {
			log.Fatalf("读取存储档失败：%v", e)
		}
		pre, e := newEditionManager(p, preCfg, catalog.Progression{}, *expectedDatabase)
		if e != nil {
			log.Fatal(e)
		}
		if _, e = os.Stat(pre.config.SQLitePath + ".admin-guard"); e == nil && !(pre.kind == "local" && onlineReady(pre.config.SQLitePath)) {
			log.Fatal("请先停止对应的游戏服务端，再启动GM；不会自动停服或清理管理租约")
		} else if e != nil && !os.IsNotExist(e) {
			log.Fatal(e)
		}
	}
	log.Printf("正在准备GM原生物品目录，首次读取可能需要约一分钟：%s", sourceFlags.ArchivePath)
	if defaultEdition == "cloud" {
		// The cloud host has 4 GiB RAM. Bound GC's working target while walking
		// the full source instead of retaining all intermediate projections.
		debug.SetMemoryLimit(1200 << 20)
	}
	prepared, err := prepareNativeGMData(p, *sourceFlags)
	if err != nil {
		log.Fatalf("PVF目录准备失败：%v", err)
	}
	defer prepared.awarder.Equipment.Full.Close()
	if defaultEdition != "" {
		debug.FreeOSMemory()
	}
	if sourceFlags.CheckOnly {
		if err := managementdata.Report(map[string]any{"source": prepared.awarder.Catalog.Source.Checksum, "items": prepared.index.Count(), "grant_stackables": len(prepared.awarder.Catalog.Items), "equipment_rows": len(prepared.awarder.Equipment.Rows), "storage_accessed": false}); err != nil {
			log.Fatal(err)
		}
		return
	}

	for _, f := range []string{p.storage} {
		if _, err := os.Stat(f); err != nil {
			log.Fatalf("路径不存在：%s（用 -root 指定 115us 根目录，或用 -storage 指定 local.json）", f)
		}
	}

	cfg, err := database.LoadConfig(p.storage)
	if err != nil {
		log.Fatalf("读取存储配置失败：%v", err)
	}
	var edition *editionManager
	if defaultEdition != "" {
		edition, err = newEditionManager(p, cfg, prepared.progression, *expectedDatabase)
		if err != nil {
			log.Fatal(err)
		}
		cfg = edition.config
		edition.quests = prepared.quests
		edition.skills = prepared.skills
	}
	// 存储档校验：SQLite 是唯一引擎，没有服务可起，这里只回报数据库文件位置。
	// PostgreSQL 的 pg_ctl 自动拉起随引擎一起移除（2026-10-05，见根 AGENTS.md §0.6）。
	if sc, e := loadStorageConfig(p.storage); e != nil {
		log.Printf("读取存储档失败（跳过存储档校验）：%v", e)
	} else if note, e := startStorage(sc); e != nil {
		log.Printf("存储档校验失败：%v", e)
	} else if note != "" {
		log.Printf("%s", note)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	store, err := database.Open(ctx, cfg)
	if err != nil {
		if driver, driverErr := database.EngineForConfig(cfg); driverErr == nil && driver == database.DriverSQLite {
			log.Fatalf("连接 SQLite 失败：%v\n\n处理办法：\n"+
				"  1. 确认 local.json 里的 sqlite_path 是**绝对路径**；\n"+
				"  2. 服务端会自己创建该文件，路径上的目录必须存在。\n", err)
		}
		log.Fatalf("连接 PostgreSQL 失败：%v\n\n"+
			"处理办法（任选其一）：\n"+
			"  1. 双击 游戏根目录下 scripts\\启动服务端.cmd 启动数据库；\n"+
			"  2. 检查 DFO 服务端目录下 runtime\\storage\\postgres.log。\n", err)
	}
	defer store.Close()
	if defaultEdition == "" {
		if err := store.MigrateGMMail(ctx); err != nil {
			log.Fatalf("初始化管理台邮件失败：%v", err)
		}
	}

	svc := &admin.Service{Store: store, Operator: *operator}
	svc.Awarder = prepared.awarder
	loot, index := prepared.awarder.Catalog, prepared.index
	log.Printf("GM原生目录：%d个LIST绑定；源%s；中文文本按外部显示覆盖读取", index.Count(), loot.Source.Checksum)

	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Fatal(err)
	}
	s := &server{
		store: store, admin: svc, index: index, paths: p,
		token: hex.EncodeToString(buf), loot: loot, started: time.Now(), operator: *operator,
		edition: edition,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/overview", s.auth(s.handleOverview))
	mux.HandleFunc("/api/characters", s.auth(s.handleCharacters))
	mux.HandleFunc("/api/character", s.auth(s.handleCharacter))
	mux.HandleFunc("/api/items", s.auth(s.handleItems))
	mux.HandleFunc("/api/catalog-metadata", s.auth(s.handleCatalogMetadata))
	mux.HandleFunc("/api/types", s.auth(s.handleTypes))
	mux.HandleFunc("/api/filters", s.auth(s.handleFilters))
	mux.HandleFunc("/api/grant", s.auth(s.handleGrant))
	mux.HandleFunc("/api/history", s.auth(s.handleHistory))
	s.registerDashboardRoutes(mux)
	if edition != nil {
		s.registerEditionRoutes(mux)
	}
	mux.HandleFunc("/api/backup/list", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/backup/save", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/backup/restore", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/world/areas", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/world/position", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/world/teleport", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/state/quests", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/quests/accepted", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/state/level", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/state/skills", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/state/bag", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/vault/account", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/vault/account/gold", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/vault/character", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/skin/catalog", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/skin/account", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/skin/unlock", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/skin/character", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/equip/list", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/equip/apply", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/fatigue", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/fatigue/write", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/materials", s.auth(s.handleUnavailableAPI))
	mux.HandleFunc("/api/materials/write", s.auth(s.handleUnavailableAPI))

	ln, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatalf("监听失败：%v", err)
	}
	url := fmt.Sprintf("http://%s/?token=%s", ln.Addr().String(), s.token)
	if edition != nil {
		url = fmt.Sprintf("http://%s/#%s", ln.Addr().String(), s.token)
	}
	log.Printf("DFO 115 GM 工具已启动：%s", url)
	log.Printf("物品索引 %d 件（客户端名字表 %d 条，汉化兜底表 %d 条）", index.Count(), len(index.namesClient), len(index.namesZH))
	if *openBrowser {
		go func() {
			time.Sleep(300 * time.Millisecond)
			openURL(url)
		}()
	}
	if err := (&http.Server{Handler: mux, ReadHeaderTimeout: 5 * time.Second, IdleTimeout: 60 * time.Second}).Serve(ln); err != nil {
		log.Fatal(err)
	}
}

func openURL(url string) {
	switch runtime.GOOS {
	case "windows":
		_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		_ = exec.Command("open", url).Start()
	default:
		_ = exec.Command("xdg-open", url).Start()
	}
}

func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := r.URL.Query().Get("token")
		if tok == "" {
			tok = r.Header.Get("X-GM-Token")
		}
		if s.token == "" || subtle.ConstantTimeCompare([]byte(tok), []byte(s.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if s.edition != nil && s.edition.kind == "local" && r.Method != http.MethodGet {
			if _, e := os.Stat(s.edition.config.SQLitePath + ".admin-guard"); e == nil {
				d, e := localgm.Read(s.edition.config.SQLitePath)
				if e != nil {
					writeErr(w, 409, "当前服务端未开放在线GM，请使用本次配套启动器：%v", e)
					return
				}
				body, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 65536))
				if e != nil {
					writeErr(w, 400, "请求过大：%v", e)
					return
				}
				result, e := localgm.Call(r.Context(), d, &localgm.Request{Path: r.URL.Path, Body: body})
				if e != nil {
					writeErr(w, 503, "在线操作结果待确认，请保留操作编号重试：%v", e)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(result.Status)
				w.Write(result.Body)
				return
			}
		}
		if s.edition != nil && r.Method != http.MethodGet {
			// Only implemented, audited mutation endpoints are exposed by the two editions.
			if r.URL.Path != "/api/grant" && r.URL.Path != "/api/gm/level" && r.URL.Path != "/api/gm/backup" && r.URL.Path != "/api/gm/command" {
				writeErr(w, 404, "此版本未开放该操作")
				return
			}
			s.edition.writeMu.Lock()
			defer s.edition.writeMu.Unlock()
			ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
			defer cancel()
			release, err := s.edition.acquire(ctx, s.admin.Store)
			if err != nil {
				writeErr(w, 409, "请先停止对应服务端，再修改存档：%v", err)
				return
			}
			defer release()
			if r.URL.Path != "/api/gm/backup" {
				if _, err := s.edition.backup(ctx); err != nil {
					writeErr(w, 500, "自动备份失败，已取消修改：%v", err)
					return
				}
			}
		}
		next(w, r)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, format string, args ...any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": fmt.Sprintf(format, args...)})
}

func (s *server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if s.edition != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(strings.ReplaceAll(editionHTML, "__EDITION__", s.edition.label())))
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("token")), []byte(s.token)) != 1 || s.token == "" {
		http.Error(w, "unauthorized", 401)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(strings.ReplaceAll(indexHTML, "__TOKEN__", s.token)))
}

type accountRow struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

func (s *server) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	rows, err := s.store.Accounts(ctx)
	if err != nil {
		writeErr(w, 500, "读取账号失败：%v", err)
		return
	}
	accounts := make([]accountRow, len(rows))
	for i, row := range rows {
		accounts[i] = accountRow{ID: row.ID, Username: row.Username}
	}
	writeJSON(w, map[string]any{
		"accounts":      accounts,
		"items":         s.index.Count(),
		"types":         s.index.Types(),
		"index_source":  s.index.Source(),
		"grade_cap":     s.loot.MaximumGrade,
		"root":          s.paths.root,
		"operator":      s.operator,
		"started":       s.started.Format(time.RFC3339),
		"engine_online": true,
	})
}

type characterRow struct {
	ID         int64           `json:"id"`
	WireID     int             `json:"wire_id"`
	Name       string          `json:"name"`
	Profession int             `json:"profession"`
	ClassName  string          `json:"class_name"`
	Level      int             `json:"level"`
	Experience uint64          `json:"experience"`
	Gold       uint32          `json:"gold"`
	CreatedAt  time.Time       `json:"created_at"`
	State      json.RawMessage `json:"state,omitempty"`
}

func (s *server) classOf(profession int) string {
	key := "growtype_name_" + strconv.Itoa(profession)
	if v := s.index.namesZH[key]; v != "" {
		return v
	}
	if v := s.index.namesEN[key]; v != "" {
		return v
	}
	return "职业 " + strconv.Itoa(profession)
}

func (s *server) characters(ctx context.Context, account int64) ([]characterRow, error) {
	rows, err := s.store.AdminCharacters(ctx, account)
	if err != nil {
		return nil, err
	}
	out := []characterRow{}
	for _, role := range rows {
		c := characterRow{ID: role.ID, WireID: int(role.WireID), Name: role.Name, Profession: int(role.Profession), CreatedAt: role.CreatedAt}
		state := role.State
		var fields struct {
			Level      byte   `json:"level"`
			Experience uint64 `json:"experience"`
			Inventory  struct {
				Gold uint32 `json:"gold"`
			} `json:"inventory"`
		}
		_ = json.Unmarshal(state, &fields)
		c.Level = int(fields.Level)
		c.Experience = fields.Experience
		c.Gold = fields.Inventory.Gold
		c.ClassName = s.classOf(c.Profession)
		c.State = state
		out = append(out, c)
	}
	return out, nil
}

func (s *server) handleCharacters(w http.ResponseWriter, r *http.Request) {
	account, err := strconv.ParseInt(r.URL.Query().Get("account"), 10, 64)
	if err != nil || account == 0 {
		writeErr(w, 400, "缺少或不合法的一 account 参数")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	list, err := s.characters(ctx, account)
	if err != nil {
		writeErr(w, 500, "读取角色失败：%v", err)
		return
	}
	cera, _ := s.store.AccountCera(ctx, account)
	writeJSON(w, map[string]any{"characters": list, "cera": cera})
}

// bagView 是给界面看的背包快照。
type bagView struct {
	Slot  uint16 `json:"slot"`
	ID    uint32 `json:"template"`
	Name  string `json:"name"`
	Count uint32 `json:"count"`
	Worn  bool   `json:"worn"`
}

func (s *server) handleCharacter(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil || id == 0 {
		writeErr(w, 400, "缺少或不合法的一 id 参数")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	role, err := s.store.AdminCharacter(ctx, id)
	c := characterRow{ID: role.ID, WireID: int(role.WireID), Name: role.Name, CreatedAt: role.CreatedAt}
	account, state, profession := role.AccountID, role.State, int(role.Profession)
	if err != nil {
		writeErr(w, 404, "角色不存在：%v", err)
		return
	}
	c.Profession = profession
	c.ClassName = s.classOf(profession)

	var summary struct {
		Level           byte                 `json:"level"`
		Experience      uint64               `json:"experience"`
		SkillPoints     [2]uint16            `json:"skill_points"`
		TechniquePoints [2]uint16            `json:"technique_points"`
		CurrencySlot2   uint32               `json:"currency_slot2"`
		Advancement     byte                 `json:"advancement"`
		Attributes      map[string]float32   `json:"attributes"`
		LearnedSkills   [2]map[uint16]byte   `json:"learned_skills"`
		SkillSlots      [2]map[uint16]uint16 `json:"skill_slots"`
		Inventory       struct {
			Gold  uint32 `json:"gold"`
			Items []struct {
				Slot             uint16 `json:"slot"`
				Template, Amount uint32
			} `json:"items"`
			Equipment []struct {
				Slot       uint16 `json:"slot"`
				Template   uint32 `json:"template"`
				Durability uint16 `json:"durability"`
			} `json:"equipment"`
			Worn []struct {
				Slot       uint16 `json:"slot"`
				Template   uint32 `json:"template"`
				Durability uint16 `json:"durability"`
			} `json:"worn"`
		} `json:"inventory"`
	}
	if err := json.Unmarshal(state, &summary); err != nil {
		writeErr(w, 500, "角色存档不是可识别的一 JSON：%v", err)
		return
	}
	c.Level, c.Experience, c.Gold = int(summary.Level), summary.Experience, summary.Inventory.Gold
	c.State = nil

	bag := []bagView{}
	for _, it := range summary.Inventory.Items {
		name := "ID " + strconv.FormatUint(uint64(it.Template), 10)
		if e, ok := s.index.Get(it.Template); ok && e.Name != "" {
			name = e.Name
		}
		bag = append(bag, bagView{Slot: it.Slot, ID: it.Template, Name: name, Count: it.Amount})
	}
	for _, it := range summary.Inventory.Equipment {
		name := "ID " + strconv.FormatUint(uint64(it.Template), 10)
		if e, ok := s.index.Get(it.Template); ok && e.Name != "" {
			name = e.Name
		}
		bag = append(bag, bagView{Slot: it.Slot, ID: it.Template, Name: name, Count: 1})
	}
	for _, it := range summary.Inventory.Worn {
		name := "ID " + strconv.FormatUint(uint64(it.Template), 10)
		if e, ok := s.index.Get(it.Template); ok && e.Name != "" {
			name = e.Name
		}
		bag = append(bag, bagView{Slot: it.Slot, ID: it.Template, Name: name, Count: 1, Worn: true})
	}
	cera, _ := s.store.AccountCera(ctx, account)
	writeJSON(w, map[string]any{
		"character": c,
		"account":   account,
		"cera":      cera,
		"summary": map[string]any{
			"level":            summary.Level,
			"experience":       summary.Experience,
			"skill_points":     summary.SkillPoints,
			"technique_points": summary.TechniquePoints,
			"currency_slot2":   summary.CurrencySlot2,
			"advancement":      summary.Advancement,
			"attributes":       summary.Attributes,
			"learned_skills":   summary.LearnedSkills,
			"skill_slots":      summary.SkillSlots,
		},
		"bag": bag,
	})
}

func (s *server) handleTypes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]any{"types": s.index.Types()})
}

// handleFilters 返回部位 / 等级档 / 品级三个筛选维度的可选值与数量。
//
// 与 /api/types 的分工：/api/types 是"客户端页签 + 装备部位"的树，
// /api/filters 是筛选控件的数据源，另外把覆盖率与品级名来源一并说清楚，
// 免得界面上的数量看起来没有出处。
func (s *server) handleFilters(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, s.index.Filters())
}

func (s *server) handleItems(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	f := ItemFilter{
		Kind: query.Get("kind"), RawType: query.Get("raw_type"), Offset: atoiDefault(query.Get("offset"), 0),
		Q:        query.Get("q"),
		Type:     query.Get("type"),
		Slot:     query.Get("slot"),
		LevelMin: atoiDefault(query.Get("level_min"), 0),
		LevelMax: atoiDefault(query.Get("level_max"), 0),
		Limit:    atoiDefault(query.Get("limit"), 60),
	}
	// rarity= 接受品级序号（8）或中文名（太初）。写错的值直接 400，
	// 不要静默给出空结果 —— 否则"筛不出来"会被当成工具坏了。
	if raw := strings.TrimSpace(query.Get("rarity")); raw != "" {
		n, ok := s.index.RarityIndex(raw)
		if !ok {
			writeErr(w, 400, "rarity 只能是品级序号 0~%d 或品级中文名（如 太初 / 史诗）", len(defaultRarityLabels)-1)
			return
		}
		f.Rarity, f.RaritySet = n, true
	}
	if f.LevelMin > 0 && f.LevelMax > 0 && f.LevelMin > f.LevelMax {
		writeErr(w, 400, "level_min 不能大于 level_max")
		return
	}
	items := s.index.Search(f)
	writeJSON(w, map[string]any{
		"items":     items,
		"type":      f.Type,
		"slot":      f.Slot,
		"q":         f.Q,
		"rarity":    f.Rarity,
		"level_min": f.LevelMin,
		"level_max": f.LevelMax,
		"count":     len(items),
	})
}

// atoiDefault 解析整数，失败或为空时给默认值。
func atoiDefault(s string, def int) int {
	if strings.TrimSpace(s) == "" {
		return def
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return def
	}
	return n
}

// buildDataFiles 是本工具的离线数据准备步骤（-build-data，见 names.go）：
// 从客户端内层 PVF 的文本表导出游戏显示名，并把装备目录里的
// [equipment type] / [minimum level] 抽成一张紧凑表。两个产物都写在
// D:\115us\gm-tool\configs 下，不动 runtime/l10n/*（汉化流水线还在用）。
func buildDataFiles(p paths) error {
	log.Printf("从客户端内层 PVF 导出显示名：%s -> %s", p.clientPVF, p.namesClient)
	n, err := buildClientNames(p.clientPVF, p.namesClient)
	if err != nil {
		return err
	}
	log.Printf("名字表导出完成：%d 条（name_/growtype_name_/common_rarity_）", n)

	log.Printf("从原生 PVF 导出部位与最低等级：%s -> %s", p.clientPVF, p.equipmentSlots)
	m, used, err := buildEquipmentSlots(p.equipmentSlots, p.clientPVF)
	if err != nil {
		return err
	}
	log.Printf("部位表导出完成：%d 件（来源 %s）", m, strings.Join(used, " + "))
	return nil
}

func (s *server) handleHistory(w http.ResponseWriter, r *http.Request) {
	account, err := strconv.ParseInt(r.URL.Query().Get("account"), 10, 64)
	if err != nil || account == 0 {
		writeErr(w, 400, "缺少或不合法的一 account 参数")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	rows, err := s.store.GrantHistory(ctx, account, 100)
	if err != nil {
		writeErr(w, 500, "读取发放记录失败：%v", err)
		return
	}
	type hist struct {
		GrantID   string          `json:"grant_id"`
		Character int64           `json:"character_id"`
		Operator  string          `json:"operator"`
		Reason    string          `json:"reason"`
		Request   json.RawMessage `json:"request"`
		At        time.Time       `json:"at"`
	}
	out := []hist{}
	for _, row := range rows {
		out = append(out, hist{GrantID: row.GrantID, Character: row.CharacterID, Operator: row.Operator,
			Reason: row.Reason, Request: row.Request, At: row.CreatedAt})
	}
	writeJSON(w, map[string]any{"history": out})
}

type grantRequest struct {
	GrantID   string `json:"grant_id"`
	Account   int64  `json:"account"`
	Character int64  `json:"character"`
	Cera      int64  `json:"cera"`
	Gold      uint32 `json:"gold"`
	Reason    string `json:"reason"`
	Items     []struct {
		Template uint32 `json:"template"`
		Amount   uint32 `json:"amount"`
	} `json:"items"`
}

func (s *server) handleGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, 405, "只接受 POST")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, 400, "读取请求失败：%v", err)
		return
	}
	// 浏览器 fetch 一定发 UTF-8；但用 GBK 控制台 curl 时中文会变成非法字节，
	// 这里做一次兜底转换，避免审计里的发放原因变成乱码。
	if !utf8.Valid(body) {
		if fixed, e := simplifiedchinese.GB18030.NewDecoder().Bytes(body); e == nil {
			body = fixed
		}
	}
	var req grantRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErr(w, 400, "请求体不是合法 JSON：%v", err)
		return
	}
	if req.Account == 0 {
		writeErr(w, 400, "缺少账号")
		return
	}
	if req.Reason == "" {
		req.Reason = "GM 工具发放"
	}
	g := database.Grant{
		ID:        fmt.Sprintf("gm-%d-%s", time.Now().UnixNano(), s.token[:6]),
		AccountID: req.Account,
		Character: req.Character,
		Cera:      req.Cera,
		Gold:      req.Gold,
		Reason:    req.Reason,
		Operator:  s.operator,
	}
	if s.edition != nil {
		if len(req.GrantID) < 8 || len(req.GrantID) > 128 || strings.TrimSpace(req.Reason) == "" {
			writeErr(w, 400, "需要8～128字符的操作编号和原因")
			return
		}
		g.ID = req.GrantID
		g.Operator = "gm-" + s.edition.kind
	}
	for _, it := range req.Items {
		if it.Template == 0 || it.Amount == 0 {
			continue
		}
		g.Items = append(g.Items, database.GrantItem{Template: it.Template, Amount: it.Amount})
	}
	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()
	receipt, applied, err := s.admin.Apply(ctx, g)
	if err != nil {
		writeErr(w, 400, "发放失败：%v", err)
		return
	}
	msg := "发放成功。"
	if !applied {
		msg = "该笔发放已存在，未重复发放。"
	}
	writeJSON(w, map[string]any{
		"ok": true, "applied": applied, "message": msg,
		"grant_id": g.ID, "receipt": receipt,
		"note": "游戏内的角色如果正在线上，需要重新选择角色（或小退再进）才能看到新的金币/物品。",
	})
}

var _ = strings.TrimSpace

// Missing management operations continue to return an authenticated failure.
func (s *server) handleUnavailableAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": false, "error": "当前源码包未包含此管理功能；请使用现有发布版GM工具。"})
}
