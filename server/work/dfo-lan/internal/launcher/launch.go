package launcher

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// This file is Stage 1 of the Python-orchestration migration (see
// docs/go-launch-migration-plan.md): everything launch_local.py did before it started
// anything. Configuration parsing, profile validation, the dependency checks and the
// inner-PVF gate are ported, and the result is printed; no process is started and no file
// is written. The real launch (gateway argv, client, WFP) is Stage 2/3.

// innerFormat is the manifest format prepare_inner_pvf.py writes. The gate accepts
// nothing else; kept in sync with scripts/ensure_inner_pvf.py and internal/pvfprep.
const innerFormat = "dfo_20260901_inner"

// innerClientInputs is the client triple the inner archive is derived from. DFO.exe alone
// carries the wrapper keys, so all three decide whether the archive still matches.
var innerClientInputs = []string{"DFO.exe", "sk.dat", "Script.pvf"}

// launcherTagPrefix is the session tag launch_local.py builds. The tag decides the output
// directory and, through the probe's downgrade chain, which payload the client gets.
const launcherTagPrefix = "roles_persist_select_actor_town_world_live_detail_dungeon_manual_"

// probePayload7001 is the payload the session tag produces after the probe's
// _next37 -> _next34 downgrade (channel_probe.py falls back to the 7001 form for tags
// ending _next30.._next34). It is a plan line, not an executed argument.
const probePayload7001 = "3?127.0.0.1?7001?probe?00000000000000000000000000000000?0?0?30?0?0?0"

// LaunchOptions is the launch_local.py command line as data. The Python had no --root: it
// derived the root from its own location. A Go binary can be started from anywhere, so the
// root is explicit and the .cmd entries pass the directory they cd to.
type LaunchOptions struct {
	Check         bool
	DryRun        bool
	ServerOnly    bool
	ClientOnly    bool
	StorageOnly   bool
	JSONMode      bool
	SourceBuild   bool
	RepairProfile string
	// Tag pins the session tag. The Python always built one from the clock, which makes a
	// byte-for-byte comparison against its own output impossible; --tag exists for that
	// comparison (and for reproducing a session) and is empty for a normal launch.
	Tag string
}

// LaunchReport is everything launch --check / --dry-run reports, plus the raw material the
// tests compare against the Python's own output.
type LaunchReport struct {
	Storage     string // "SQLite <path>" or "PostgreSQL: True|False", worded as the Python
	Binary      string
	DataMode    string
	ClientDir   string
	InnerPVF    InnerPVFStatus
	ProfilePath string
	ProfileEnv  map[string]string
	Required    []string
	Plan        []PlanStep
	// ChannelIdentity is launcher.local.json's channel_identity. The real launch needs it:
	// it decides both the DFO_CHANNEL_IDENTITY switch and the gateway's -channel-identity.
	ChannelIdentity bool
}

// InnerPVFStatus is the read-only verdict of the four-state gate in
// scripts/ensure_inner_pvf.py (decide()). Message is the line the launcher prints for it:
// the Python's own wording while the archive is reusable, and an explicit
// "checked but not built" note while it is not.
//
// ⚠️ The Python's --check is NOT read-only: _ensure_inner_pvf runs before the --check
// print and rebuilds the 760 MB archive when the gate says so. This stage never writes it
// (that generation is Stage 4 of the plan), so a missing archive is reported as missing
// and the profile's DFO_PVF_ARCHIVE check then fails exactly as the Python's would if its
// generation attempt had failed.
type InnerPVFStatus struct {
	Path       string
	Manifest   string
	Checked    bool
	NeedsBuild bool
	Reason     string
	Message    string
}

// PlanStep is one line of the --dry-run command plan.
type PlanStep struct {
	Step   string // 存储 / 内层 PVF / 网关 / 客户端
	Target string // 要执行的可执行文件；(...) 表示该步被跳过或有特殊结论
	Detail string // 关键参数与说明
}

// Line renders the step as the single line --dry-run prints.
func (s PlanStep) Line() string {
	if s.Target == "" {
		return fmt.Sprintf("[%s] %s", s.Step, s.Detail)
	}
	return fmt.Sprintf("[%s] %s %s", s.Step, s.Target, s.Detail)
}

// Lines renders the check output. The four lines are worded exactly as
// launch_local.py printed them, so the two can be compared line by line; --dry-run
// appends the command plan. The inner-PVF status line is not included: the Python printed
// it before its fatal checks, so the caller prints it even when this function never runs.
func (r LaunchReport) Lines(dryRun bool) []string {
	lines := []string{
		"Paths OK. Storage: " + r.Storage,
		"Binary: " + r.Binary,
		"Data mode: " + r.DataMode,
		"Client: " + r.ClientDir,
	}
	if dryRun {
		for _, step := range r.Plan {
			lines = append(lines, step.Line())
		}
	}
	return lines
}

// LaunchPlan performs the read-only half of launch_local.py: configuration, profile
// validation, dependency checks and the inner-PVF gate. It writes nothing and starts
// nothing, which is what makes --check and --dry-run safe to run before a session.
//
// The error is the Python's fatal error: a missing dependency, a rejected profile or an
// unusable storage configuration. The report is returned even then, because the inner-PVF
// status line is printed before the Python raises (its _ensure_inner_pvf runs before the
// profile's own required-file check).
func LaunchPlan(root string, opts LaunchOptions) (LaunchReport, error) {
	report := LaunchReport{ProfileEnv: map[string]string{}}

	// Two path bases, and they are not the same one:
	//   ROOT    = server/              (the Python's PROJECT.parent.parent) for
	//                                  launcher.local.json and its relative paths,
	//   PROJECT = server/work/dfo-lan  for the profile and runtime/storage.
	// client-build sits next to dfo-lan under server/work, not under the package root, and
	// configs/pvf-default.json's "../client-build/..." is written against PROJECT too.
	serverRoot := filepath.Join(root, "server")
	module := filepath.Join(serverRoot, "work", "dfo-lan")
	probeDir := filepath.Join(serverRoot, "work", "dfo_probe_tools")
	storageDir := filepath.Join(module, "runtime", "storage")
	clientBuild := filepath.Join(serverRoot, "work", "client-build")

	settings, err := loadLaunchSettings(filepath.Join(serverRoot, "launcher.local.json"))
	if err != nil {
		return report, err
	}
	storage, err := loadStorageFile(filepath.Join(storageDir, "local.json"))
	if err != nil {
		return report, err
	}
	// The Python checked the type before it read anything else out of the settings file.
	if settings.ChannelIdentity != nil {
		identity, err := jsonBoolean(settings.ChannelIdentity)
		if err != nil {
			return report, err
		}
		report.ChannelIdentity = identity
	}
	if settings.ClientDir == nil || *settings.ClientDir == "" {
		return report, fmt.Errorf("launcher.local.json must set client_dir")
	}
	client := resolveUnder(serverRoot, *settings.ClientDir)
	report.ClientDir = client

	pg, err := storageEndpoint(storage)
	if err != nil {
		return report, err
	}
	if pg == nil {
		// SQLite: no server to start, and every PostgreSQL step is skipped rather than
		// attempted. The path is echoed verbatim, as the Python's f-string did.
		report.Storage = "SQLite " + *storage.SQLitePath
	} else {
		// pythonBool, not %t: this line is compared against the Python's output, and the
		// Python printed the bool's repr.
		report.Storage = "PostgreSQL: " + pythonBool(PortListening(pg.Port, portProbeTimeout))
	}

	binary := ""
	switch {
	case opts.SourceBuild:
		binary = filepath.Join(module, "bin", "wireprobe-handoff-source.exe")
	default:
		if settings.ServerBinary == nil || *settings.ServerBinary == "" {
			return report, fmt.Errorf("launcher.local.json must set server_binary")
		}
		binary = resolveUnder(serverRoot, *settings.ServerBinary)
	}

	// The default profile applies to a normal, server-only and storage-only run; JSON
	// mode, client-only and an explicit --repair-profile are the three ways not to use it.
	// Note that server-only is NOT an exclusion, and that the Python passed
	// --repair-profile to pathlib as written, so a relative one resolved against the
	// caller's directory rather than against the module. Both are kept as they are.
	profilePath := opts.RepairProfile
	if profilePath == "" && !opts.JSONMode && !opts.ClientOnly && !opts.StorageOnly {
		profilePath = filepath.Join(module, "configs", "pvf-default.json")
	}
	var profile Profile
	if profilePath != "" {
		loaded, err := LoadProfile(profilePath, module)
		if err != nil {
			return report, err
		}
		profile = loaded
		report.ProfilePath = loaded.Path
		report.ProfileEnv = loaded.Env
		if opts.SourceBuild && opts.RepairProfile == "" {
			// --source-build keeps the source binary and swaps it into the required list,
			// so the profile's binary is neither launched nor required.
			profile.Required = replacePath(profile.Required, loaded.Binary, binary)
		} else {
			// The profile's binary is authoritative: the default profile names the gateway
			// the official 启动游戏.cmd / 启动服务端.cmd also run.
			binary = loaded.Binary
		}
	}
	report.Binary = binary
	report.DataMode = "JSON / explicit profile"
	if profile.Env["DFO_PVF_CATALOGS"] != "" {
		report.DataMode = "PVF direct"
	}

	// The required list is the Python's, branch for branch. --client-only wins over
	// --server-only because it is tested first there too (argparse let both through).
	helper := filepath.Join(probeDir, "channel_probe.py")
	var required []string
	switch {
	case opts.ClientOnly:
		// A client-only run reaches a server on another machine: no local gateway, but the
		// probe runtime and the client triple are still needed.
		required = []string{
			helper,
			filepath.Join(probeDir, "probe.exe"),
			filepath.Join(client, "DFO.exe"),
			filepath.Join(client, "Script.pvf"),
			filepath.Join(client, "sk.dat"),
		}
	case opts.ServerOnly:
		required = []string{helper, binary}
	default:
		required = []string{
			helper,
			filepath.Join(probeDir, "probe.exe"),
			binary,
			filepath.Join(client, "DFO.exe"),
			filepath.Join(client, "Script.pvf"),
			filepath.Join(client, "sk.dat"),
		}
	}
	// Unconditional, including --server-only and --client-only, as in the Python.
	required = append(required, filepath.Join(probeDir, "catalog_startup.py"))
	report.Required = required
	for _, path := range required {
		if !regularFile(path) {
			// The Python raised on the first missing entry, so this reports the same path.
			return report, fmt.Errorf("Missing dependency: %s", path)
		}
	}

	// The inner archive has to be checked before the profile's own required files: the
	// default profile lists it as DFO_PVF_ARCHIVE, and the Python generated it here so
	// that check could pass. This stage only inspects (Stage 4 builds it).
	innerPVF := filepath.Join(clientBuild, "Script.inner.pvf")
	manifest := filepath.Join(clientBuild, "Script.inner.manifest.json")
	report.InnerPVF = InnerPVFStatus{Path: innerPVF, Manifest: manifest}
	switch {
	case opts.ClientOnly:
		// --client-only serves no local content, so it needs no inner archive.
	case profile.Env["DFO_PVF_CATALOGS"] == "":
		// No native profile: the gateway reads JSON and never opens the inner archive.
	default:
		status := inspectInnerPVF(client, innerPVF, manifest)
		status.Checked = true
		if status.NeedsBuild {
			status.Message = fmt.Sprintf(
				"WARNING: 内层 PVF 未就绪：%s（本阶段只校验不生成；生成见 go-launch-migration-plan.md Stage 4）",
				status.Reason)
		} else {
			status.Message = "内层 PVF 无需重建：" + status.Reason
		}
		report.InnerPVF = status
	}

	for _, path := range profile.Required {
		if !regularFile(path) {
			return report, fmt.Errorf("Missing repair profile dependency: %s", path)
		}
	}

	if opts.DryRun {
		input := planInput{
			module:      module,
			storageDir:  storageDir,
			probeDir:    probeDir,
			innerPVF:    innerPVF,
			manifest:    manifest,
			client:      client,
			binary:      binary,
			storage:     storage,
			pg:          pg,
			opts:        opts,
			catalogs:    profile.Env["DFO_PVF_CATALOGS"],
			innerStatus: report.InnerPVF,
			now:         time.Now(),
		}
		if report.Plan, err = launchPlanSteps(input); err != nil {
			return report, err
		}
	}
	return report, nil
}

// launchSettings is the part of server/launcher.local.json the launcher reads. The
// pointers keep "key absent" distinguishable from "key empty", because the Python raised
// KeyError for a missing client_dir and only defaulted channel_identity.
type launchSettings struct {
	ClientDir       *string         `json:"client_dir"`
	ServerBinary    *string         `json:"server_binary"`
	ChannelIdentity json.RawMessage `json:"channel_identity"`
}

// loadLaunchSettings reads launcher.local.json below server/.
func loadLaunchSettings(path string) (launchSettings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return launchSettings{}, fmt.Errorf(
				"Copy launcher.example.json to launcher.local.json and set client_dir.")
		}
		return launchSettings{}, err
	}
	var settings launchSettings
	if err := json.Unmarshal(stripBOM(data), &settings); err != nil {
		return launchSettings{}, fmt.Errorf("launcher.local.json: %w", err)
	}
	return settings, nil
}

// jsonBoolean enforces what the Python's `type(value) is not bool` did: the value must be
// the literal true or false, not 0/1 and not the string "true".
func jsonBoolean(raw json.RawMessage) (bool, error) {
	switch strings.TrimSpace(string(raw)) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("channel_identity must be a JSON boolean")
	}
}

// storageFile is runtime/storage/local.json. Pointers distinguish "absent" from "empty",
// which is the difference between the Python's KeyError and its own error messages.
type storageFile struct {
	Driver       string  `json:"driver"`
	SQLitePath   *string `json:"sqlite_path"`
	PostgresDSN  *string `json:"postgres_dsn"`
	PostgresBin  string  `json:"postgres_bin"`
	PostgresData string  `json:"postgres_data"`
}

// loadStorageFile reads runtime/storage/local.json. Unlike LoadStorageConfig (used by the
// stop path) it refuses to continue without the file: the launch chain cannot know which
// engine to use, and the Python said so with "Storage missing".
func loadStorageFile(path string) (storageFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return storageFile{}, fmt.Errorf(
				"Storage missing. Follow README first-time setup; no database was changed.")
		}
		return storageFile{}, err
	}
	var storage storageFile
	if err := json.Unmarshal(stripBOM(data), &storage); err != nil {
		return storageFile{}, fmt.Errorf("storage config %s: %w", path, err)
	}
	return storage, nil
}

// postgresEndpoint is what the Python kept from postgres_dsn: a loopback host and a port.
type postgresEndpoint struct {
	Host string
	Port int
}

// storageEndpoint classifies the storage configuration exactly as configuration() did: an
// explicit driver wins, an empty one means PostgreSQL, and SQLite means there is no
// service at all. A nil endpoint therefore means "skip every PostgreSQL step".
//
// It deliberately does not use StorageConfig.DriverName(): that method also infers the
// engine from the DSN and sqlite_path, while the launch path follows the driver field
// alone, and the two must not disagree about which database a start would use.
func storageEndpoint(storage storageFile) (*postgresEndpoint, error) {
	driver := strings.ToLower(storage.Driver)
	if driver == "" {
		driver = "postgres"
	}
	switch driver {
	case "sqlite":
		if storage.SQLitePath == nil || *storage.SQLitePath == "" {
			return nil, fmt.Errorf(
				"Storage driver 'sqlite' requires sqlite_path in runtime/storage/local.json.")
		}
		return nil, nil
	case "postgres":
		dsn := ""
		if storage.PostgresDSN != nil {
			dsn = *storage.PostgresDSN
		}
		endpoint, err := parseLoopbackDSN(dsn)
		if err != nil {
			return nil, err
		}
		return &endpoint, nil
	default:
		return nil, fmt.Errorf("Unsupported storage driver '%s'.", driver)
	}
}

// parseLoopbackDSN is urlparse plus the development rule that storage must be local. A
// missing port leaves 0, which cannot be listening - the Python passed None to
// create_connection and landed in the same place.
func parseLoopbackDSN(dsn string) (postgresEndpoint, error) {
	endpoint := postgresEndpoint{}
	if parsed, err := url.Parse(dsn); err == nil {
		endpoint.Host = parsed.Hostname()
		if port := parsed.Port(); port != "" {
			if value, convErr := strconv.Atoi(port); convErr == nil {
				endpoint.Port = value
			}
		}
	}
	if endpoint.Host != "127.0.0.1" {
		return endpoint, fmt.Errorf("This development profile requires local loopback storage.")
	}
	return endpoint, nil
}

// resolveUnder resolves a configured path, mirroring the Python's resolved(): an absolute
// path is kept, a relative one is joined to the base, and a leading "~" is expanded.
func resolveUnder(base, value string) string {
	value = expandUser(value)
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	return filepath.Clean(filepath.Join(base, value))
}

// expandUser mirrors pathlib's expanduser for the "~" / "~/..." forms; a "~user" path is
// returned unchanged, as it is on Windows.
func expandUser(value string) string {
	if value != "~" && !strings.HasPrefix(value, "~\\") && !strings.HasPrefix(value, "~/") {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return value
	}
	if value == "~" {
		return home
	}
	return filepath.Join(home, value[2:])
}

// resolveFromWorkingDir resolves a path the way pathlib's Path(...).resolve() did when the
// value was written relative: against the caller's directory, not against the root. The
// Python used it for postgres_data, which is why an empty value means the current
// directory rather than the storage directory.
func resolveFromWorkingDir(value string) string {
	if value == "" {
		value = "."
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if absolute, err := filepath.Abs(value); err == nil {
		return absolute
	}
	return filepath.Clean(value)
}

// regularFile reports whether the path is a file, following symlinks - the same test
// Path.is_file() applied.
func regularFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// pythonBool renders a bool the way the Python's f-string printed it. The storage line is
// compared byte for byte against launch_local.py's output, so "True" is not a typo.
func pythonBool(value bool) string {
	if value {
		return "True"
	}
	return "False"
}

// inspectInnerPVF is the read-only half of scripts/ensure_inner_pvf.py: the same
// four-state gate, minus the rebuild.
func inspectInnerPVF(client, inner, manifestPath string) InnerPVFStatus {
	status := InnerPVFStatus{Path: inner, Manifest: manifestPath}
	missing := make([]string, 0, len(innerClientInputs))
	for _, name := range innerClientInputs {
		if !regularFile(filepath.Join(client, name)) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		// The Python's ensure() raised this before it looked at the archive at all.
		status.NeedsBuild = true
		status.Reason = fmt.Sprintf(
			"客户端缺少 %s，无法生成内层 PVF（需 DFO.exe + sk.dat + Script.pvf 三件套）",
			strings.Join(missing, "、"))
		return status
	}
	status.NeedsBuild, status.Reason = decideInnerPVF(client, inner, manifestPath)
	return status
}

// decideInnerPVF mirrors ensure_inner_pvf.decide: pure read-only inspection, with the same
// reasons in the same order. The cheap (size, mtime_ns) comparison runs first so a warm
// client is never hashed.
func decideInnerPVF(client, inner, manifestPath string) (bool, string) {
	info, err := os.Stat(inner)
	if err != nil {
		return true, "内层 PVF 不存在"
	}
	manifest := loadInnerManifest(manifestPath)
	if manifest == nil {
		return true, "缺少或无法解析清单，旧件不可信"
	}
	if manifest.Format != innerFormat {
		// Unreachable through loadInnerManifest, kept so the order matches decide().
		return true, fmt.Sprintf("清单格式不符（%q）", manifest.Format)
	}
	if manifest.Inner.Size != nil && *manifest.Inner.Size != info.Size() {
		return true, fmt.Sprintf("内层 PVF 大小不符（盘上 %d，清单 %d）", info.Size(), *manifest.Inner.Size)
	}
	if len(manifest.Cache) == len(innerClientInputs) {
		states, statErr := clientStates(client)
		if statErr != nil {
			return true, "无法读取客户端文件状态，按需重建"
		}
		if sameClientStates(states, manifest.Cache) {
			return false, "客户端与内层 PVF 均未变化，复用现有产物"
		}
	}
	// Slow path: the client triple changed (or the fast path was unavailable), so compare
	// full fingerprints.
	exe, err := fileSHA256(filepath.Join(client, "DFO.exe"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	sk, err := fileSHA256(filepath.Join(client, "sk.dat"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	script, err := fileSHA256(filepath.Join(client, "Script.pvf"))
	if err != nil {
		return true, "无法读取客户端文件状态，按需重建"
	}
	if exe == manifest.ClientExe.SHA256 && sk == manifest.SkDat.SHA256 && script == manifest.Outer.SHA256 {
		return false, "客户端指纹与清单一致，复用现有产物"
	}
	return true, "客户端 DFO.exe/sk.dat/Script.pvf 已变化"
}

// innerManifest is the part of the manifest the gate compares. Size is a pointer because
// an absent entry means "unknown", which the Python accepted.
type innerManifest struct {
	Format    string            `json:"format"`
	ClientExe innerFingerprint  `json:"client_exe"`
	SkDat     innerFingerprint  `json:"sk_dat"`
	Outer     innerFingerprint  `json:"outer"`
	Inner     innerSize         `json:"inner"`
	Cache     []innerCacheEntry `json:"cache"`
}

type innerFingerprint struct {
	SHA256 string `json:"sha256"`
}

type innerSize struct {
	Size *int64 `json:"size"`
}

// innerCacheEntry is one (size, mtime_ns) record of the client triple.
type innerCacheEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	MtimeNS int64  `json:"mtime_ns"`
}

// loadInnerManifest returns nil for a missing, unparseable or foreign-format manifest, so
// the caller treats it as "cannot prove provenance" and rebuilds.
func loadInnerManifest(path string) *innerManifest {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	// The Python read the manifest as plain utf-8 (no BOM tolerance), so a byte-order mark
	// makes it unparseable there too.
	var manifest innerManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil
	}
	if manifest.Format != innerFormat {
		return nil
	}
	return &manifest
}

// clientStates is the cheap key the gate compares first.
func clientStates(client string) ([]innerCacheEntry, error) {
	states := make([]innerCacheEntry, 0, len(innerClientInputs))
	for _, name := range innerClientInputs {
		info, err := os.Stat(filepath.Join(client, name))
		if err != nil {
			return nil, err
		}
		states = append(states, innerCacheEntry{
			Name:    name,
			Size:    info.Size(),
			MtimeNS: info.ModTime().UnixNano(),
		})
	}
	return states, nil
}

// sameClientStates compares the recorded and current triples entry by entry, in order, as
// the Python's list comparison did.
func sameClientStates(current, recorded []innerCacheEntry) bool {
	if len(current) != len(recorded) {
		return false
	}
	for index := range current {
		if current[index] != recorded[index] {
			return false
		}
	}
	return true
}

// fileSHA256 is the fingerprint the manifest stores.
func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	digest := sha256.New()
	if _, err := io.Copy(digest, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(digest.Sum(nil)), nil
}

// planInput is everything the --dry-run plan needs, so the plan builder stays testable
// without a live environment.
type planInput struct {
	module      string
	storageDir  string
	probeDir    string
	innerPVF    string
	manifest    string
	client      string
	binary      string
	storage     storageFile
	pg          *postgresEndpoint
	opts        LaunchOptions
	catalogs    string
	innerStatus InnerPVFStatus
	now         time.Time
}

// launchPlanSteps builds the 存储 -> 内层 PVF -> 网关 -> 客户端 plan. The order is the one
// the migration plan prescribes; note that the Python prepared the inner PVF before it
// started storage (launch_local.py L233 vs L257), it just never printed a plan.
func launchPlanSteps(in planInput) ([]PlanStep, error) {
	// The tag launch_local.py builds; it names the session directory the gateway and the
	// probe write into. sessionTagName carries the strftime("%Y%m%d_%H%M%S_%f") detail.
	tag := sessionTagName(in.now)
	out := filepath.Join(in.module, "runtime", tag)

	storage, err := storageStep(in)
	if err != nil {
		return nil, err
	}
	return []PlanStep{
		storage,
		innerPVFStep(in),
		gatewayStep(in, out),
		clientStep(in, out),
	}, nil
}

// storageStep mirrors start_storage read-only: SQLite has no service, and a PostgreSQL
// profile is started only from the data directory the launcher owns.
func storageStep(in planInput) (PlanStep, error) {
	const step = "存储"
	if in.pg == nil {
		path := ""
		if in.storage.SQLitePath != nil {
			path = *in.storage.SQLitePath
		}
		return PlanStep{
			Step:   step,
			Target: "(无需启动)",
			Detail: "SQLite 档没有服务要起；引擎自行打开并创建 " + path,
		}, nil
	}
	if PortListening(in.pg.Port, portProbeTimeout) {
		return PlanStep{
			Step:   step,
			Target: "(已在监听)",
			Detail: fmt.Sprintf("127.0.0.1:%d 已在监听，无需启动", in.pg.Port),
		}, nil
	}
	data := resolveFromWorkingDir(in.storage.PostgresData)
	if data != filepath.Join(in.storageDir, "pgdata") {
		// Same refusal as the Python: a data directory outside the launcher's own storage
		// area belongs to whoever configured it.
		return PlanStep{}, fmt.Errorf(
			"Database offline; external data directories must be started by their owner.")
	}
	pgCtl := filepath.Join(in.storage.PostgresBin, "pg_ctl.exe")
	if !regularFile(filepath.Join(data, "PG_VERSION")) || !regularFile(pgCtl) {
		return PlanStep{}, fmt.Errorf("Existing PostgreSQL data or pg_ctl missing.")
	}
	return PlanStep{
		Step:   step,
		Target: pgCtl,
		Detail: fmt.Sprintf("-D %s -l %s -w -t 30 start（stdout/stderr 追加到 launcher-postgres.log）",
			data, filepath.Join(in.storageDir, "postgres.log")),
	}, nil
}

// innerPVFStep is the inner-archive step the Python performed inline. It is skipped for a
// client-only run and for a profile without DFO_PVF_CATALOGS, which are the two cases the
// Python also skipped.
func innerPVFStep(in planInput) PlanStep {
	const step = "内层 PVF"
	switch {
	case in.opts.ClientOnly:
		return PlanStep{Step: step, Target: "(跳过)",
			Detail: "--client-only：本机不起服务端，不需要内层归档"}
	case in.catalogs == "":
		return PlanStep{Step: step, Target: "(跳过)",
			Detail: "profile 未声明 DFO_PVF_CATALOGS，JSON 模式不使用内层归档"}
	case !in.innerStatus.NeedsBuild:
		return PlanStep{Step: step, Target: "(复用)",
			Detail: in.innerStatus.Reason + "：" + in.innerPVF}
	default:
		return PlanStep{
			Step:   step,
			Target: filepath.Join(in.module, "scripts", "prepare_inner_pvf.py"),
			Detail: fmt.Sprintf("%s %s %s（%s；本阶段只校验不生成，生成见 Stage 4）",
				in.client, in.innerPVF, in.manifest, in.innerStatus.Reason),
		}
	}
}

// gatewayStep names the gateway and the two arguments the launcher itself decides. The
// rest of the argv comes from the session tag (channel_probe.py L227-449) and is Stage 2.
func gatewayStep(in planInput, out string) PlanStep {
	const step = "网关"
	if in.opts.StorageOnly {
		return PlanStep{Step: step, Target: "(跳过)",
			Detail: "--storage-only：起库后直接返回，不起网关"}
	}
	detail := fmt.Sprintf("-fixture %s -output %s；由 helper channel_probe.py 拉起（cwd=server\\）",
		filepath.Join(out, "channelinfo.bin"), out)
	if in.catalogs == "" {
		detail += "；JSON 模式：继承环境里的 DFO_PVF_* 会被全部清除"
	} else {
		detail += fmt.Sprintf("；PVF 直读只经环境变量下发：DFO_PVF_CATALOGS %d 个域，DFO_PVF_ARCHIVE=%s",
			len(strings.Split(in.catalogs, ",")), in.innerPVF)
	}
	return PlanStep{Step: step, Target: in.binary, Detail: detail}
}

// clientStep names the probe and its arguments. The client itself is started by probe.exe,
// never by the launcher, so probe.exe is the executable this step runs.
func clientStep(in planInput, out string) PlanStep {
	const step = "客户端"
	switch {
	case in.opts.StorageOnly:
		return PlanStep{Step: step, Target: "(跳过)",
			Detail: "--storage-only：起库后直接返回，不拉起客户端"}
	case in.opts.ServerOnly:
		return PlanStep{Step: step, Target: "(跳过)",
			Detail: "--server-only：只起服务端，探针不拉起客户端"}
	}
	// The Python matched the mode string exactly; "client-only" matches none of the four
	// names, so the probe falls back to trace-root-ui. Kept as it behaves.
	mode := "interactive-ui"
	if in.opts.ClientOnly {
		mode = "trace-root-ui"
	}
	return PlanStep{
		Step:   step,
		Target: filepath.Join(in.probeDir, "probe.exe"),
		Detail: fmt.Sprintf("%s %s 55 %s %s %s（由 helper 拉起；payload 为 _next37→_next34 降级后的 7001 形态）",
			in.client, filepath.Join(out, "client.log"), mode,
			filepath.Join(out, "breakpoints.txt"), probePayload7001),
	}
}
