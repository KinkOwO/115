package launcher

import (
	"encoding/json"
	"errors"
	"fmt"
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
//
// Stage 4 upgraded the inner-PVF step from "check only" to "generate when missing/stale",
// which is what launch_local.py always did (its _ensure_inner_pvf runs before --check
// prints). The gate, the manifest and the generation itself live in innerpvf.go.

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

	// The inner archive has to be checked (and, since Stage 4, generated) before the
	// profile's own required files: the default profile lists it as DFO_PVF_ARCHIVE, and
	// launch_local.py prepared it here so that check could pass.
	innerPVF := filepath.Join(clientBuild, "Script.inner.pvf")
	manifest := filepath.Join(clientBuild, "Script.inner.manifest.json")
	report.InnerPVF = InnerPVFStatus{Path: innerPVF, Manifest: manifest}
	switch {
	case opts.ClientOnly:
		// --client-only serves no local content, so it needs no inner archive.
	case profile.Env["DFO_PVF_CATALOGS"] == "":
		// No native profile: the gateway reads JSON and never opens the inner archive.
	default:
		// --dry-run is a Go-only switch that must stay read-only, so it reports the verdict
		// without building; --check is NOT read-only, because the Python's --check was not
		// either (its _ensure_inner_pvf ran first and rebuilt the 760 MB archive).
		report.InnerPVF = ensureInnerPVFForLaunch(client, innerPVF, manifest, !opts.DryRun)
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
		// Stage 4: the generation is Go now (innerpvf.go), not scripts/prepare_inner_pvf.py.
		// --dry-run never writes, so this line stays a plan.
		return PlanStep{
			Step:   step,
			Target: "(dfolauncher 现场生成)",
			Detail: fmt.Sprintf("%s → %s（清单 %s；%s；--dry-run 不生成）",
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
