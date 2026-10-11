package launcher

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"dfolan/internal/accountname"
	"dfolan/internal/gamedata"
)

// buildLaunchTree creates the layout `launch` reads, so check and plan output can be
// verified without the machine's real client, profile or binaries.
func buildLaunchTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(relative, body string) {
		path := filepath.Join(root, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// launcher.local.json sits above the module and its relative paths resolve from there.
	write("server/launcher.local.json",
		`{"client_dir":"client","server_binary":"work/dfo-lan/bin/wireprobe-pvf.exe"}`)
	write("server/client/DFO.exe", "client-exe")
	write("server/client/Script.pvf", "client-script")
	write("server/client/sk.dat", "client-sk")
	write("server/work/dfo_probe_tools/channel_probe.py", "stub")
	write("server/work/dfo_probe_tools/catalog_startup.py", "stub")
	write("server/work/dfo_probe_tools/probe.exe", "stub")
	write("server/work/dfo-lan/bin/wireprobe-pvf.exe", "stub")
	write("server/work/dfo-lan/bin/wireprobe-handoff-source.exe", "stub")
	write("server/work/dfo-lan/configs/pvf-drop-policy.json", "{}")
	write("server/work/dfo-lan/configs/pvf-default.json",
		`{"binary":"bin/wireprobe-pvf.exe","environment":{`+
			`"DFO_PVF_CATALOGS":"world, quests",`+
			`"DFO_PVF_ARCHIVE":"../client-build/Script.inner.pvf",`+
			`"DFO_PVF_DROP_POLICY":"configs/pvf-drop-policy.json",`+
			`"DFO_PVF_SHA256":"",`+
			`"DFO_FATIGUE_FREE":"1"}}`)
	// The inner archive is a local artifact; here it exists and matches its manifest, which
	// is the state the four-line check is compared against.
	write("server/work/client-build/Script.inner.pvf", "inner-archive")
	writeInnerManifest(t, root, true)
	write("server/work/dfo-lan/runtime/storage/local.json",
		`{"driver":"sqlite","sqlite_path":"C:\\data\\dfolan.sqlite3"}`)
	return root
}

// writeInnerManifest writes the manifest the inner-PVF gate reads, for the tree built by
// buildLaunchTree.
func writeInnerManifest(t *testing.T, root string, matchingCache bool) {
	t.Helper()
	writeInnerManifestAt(t,
		filepath.Join(root, "server", "client"),
		filepath.Join(root, "server", "work", "client-build", "Script.inner.pvf"),
		filepath.Join(root, "server", "work", "client-build", "Script.inner.manifest.json"),
		matchingCache)
}

// writeInnerManifestAt records the archive size, the client fingerprints and the
// (size, mtime_ns) cache. With matchingCache the cheap comparison hits; otherwise the
// fingerprints are correct but the cache is deliberately stale, which is the slow path.
func writeInnerManifestAt(t *testing.T, client, inner, manifestPath string, matchingCache bool) {
	t.Helper()
	info, err := os.Stat(inner)
	if err != nil {
		t.Fatal(err)
	}
	states, err := clientStates(client)
	if err != nil {
		t.Fatal(err)
	}
	if !matchingCache {
		for index := range states {
			states[index].MtimeNS++
		}
	}
	digest := func(name string) string {
		value, err := fileSHA256(filepath.Join(client, name))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	manifest := map[string]any{
		"format":     innerFormat,
		"inner":      map[string]any{"size": info.Size()},
		"client_exe": map[string]any{"sha256": digest("DFO.exe")},
		"sk_dat":     map[string]any{"sha256": digest("sk.dat")},
		"outer":      map[string]any{"sha256": digest("Script.pvf")},
		"cache":      states,
	}
	body, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

// The four lines are the acceptance criterion for Stage 1: they must read exactly as
// launch_local.py printed them, including the raw sqlite_path and the resolved paths.
func TestLaunchCheckPrintsThePythonFourLines(t *testing.T) {
	root := buildLaunchTree(t)
	report, err := LaunchPlan(root, LaunchOptions{Check: true})
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	want := []string{
		"Paths OK. Storage: SQLite C:\\data\\dfolan.sqlite3",
		"Binary: " + filepath.Join(root, "server", "work", "dfo-lan", "bin", "wireprobe-pvf.exe"),
		"Data mode: PVF direct",
		"Client: " + filepath.Join(root, "server", "client"),
	}
	if got := report.Lines(false); !reflect.DeepEqual(got, want) {
		t.Errorf("check output =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// client_dir is written relative to server/, which is the fact this test pins down.
	if report.ClientDir != filepath.Join(root, "server", "client") {
		t.Errorf("client dir = %q", report.ClientDir)
	}
	// The profile's own binary is authoritative for a default run.
	if filepath.Base(report.Binary) != "wireprobe-pvf.exe" {
		t.Errorf("binary = %q, want the profile's binary", report.Binary)
	}
}

// The inner archive is compared before the profile's own required files, and the reusable
// state prints the same sentence the Python printed.
func TestLaunchReportsTheInnerPVFState(t *testing.T) {
	root := buildLaunchTree(t)
	report, err := LaunchPlan(root, LaunchOptions{Check: true})
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	if !report.InnerPVF.Checked || report.InnerPVF.NeedsBuild {
		t.Fatalf("inner PVF = %+v, want a reusable archive", report.InnerPVF)
	}
	want := "内层 PVF 无需重建：客户端与内层 PVF 均未变化，复用现有产物"
	if report.InnerPVF.Message != want {
		t.Errorf("inner message = %q, want %q", report.InnerPVF.Message, want)
	}

	// Removing the archive is reported as missing, and because the default profile lists it
	// as DFO_PVF_ARCHIVE the run then fails with the Python's profile message. The Python
	// would have rebuilt it here; that generation is Stage 4.
	if err := os.Remove(report.InnerPVF.Path); err != nil {
		t.Fatal(err)
	}
	report, err = LaunchPlan(root, LaunchOptions{Check: true})
	if err == nil || !strings.Contains(err.Error(), "Missing repair profile dependency") {
		t.Fatalf("a missing inner PVF gave %v, want the profile dependency error", err)
	}
	if !strings.Contains(report.InnerPVF.Message, "内层 PVF 未就绪") {
		t.Errorf("inner message = %q, want the not-ready warning", report.InnerPVF.Message)
	}
}

// The gate has four states and each one has its own reason; only reuse skips the build.
func TestInnerPVFGateDecisions(t *testing.T) {
	cases := []struct {
		name   string
		setup  func(t *testing.T, client, inner, manifest string)
		build  bool
		reason string
	}{
		{
			name:   "reusable through the cache",
			setup:  func(t *testing.T, client, inner, manifest string) {},
			build:  false,
			reason: "客户端与内层 PVF 均未变化，复用现有产物",
		},
		{
			name: "reusable through the fingerprints",
			setup: func(t *testing.T, client, inner, manifest string) {
				writeInnerManifestAt(t, client, inner, manifest, false)
			},
			build:  false,
			reason: "客户端指纹与清单一致，复用现有产物",
		},
		{
			name: "missing archive",
			setup: func(t *testing.T, client, inner, manifest string) {
				if err := os.Remove(inner); err != nil {
					t.Fatal(err)
				}
			},
			build:  true,
			reason: "内层 PVF 不存在",
		},
		{
			name: "missing manifest",
			setup: func(t *testing.T, client, inner, manifest string) {
				if err := os.Remove(manifest); err != nil {
					t.Fatal(err)
				}
			},
			build:  true,
			reason: "缺少或无法解析清单，旧件不可信",
		},
		{
			name: "foreign manifest format",
			setup: func(t *testing.T, client, inner, manifest string) {
				if err := os.WriteFile(manifest, []byte(`{"format":"something-else"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			build:  true,
			reason: "缺少或无法解析清单",
		},
		{
			name: "size mismatch",
			setup: func(t *testing.T, client, inner, manifest string) {
				if err := os.WriteFile(inner, []byte("inner-archive-with-a-different-size"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			build:  true,
			reason: "内层 PVF 大小不符",
		},
		{
			name: "changed client",
			setup: func(t *testing.T, client, inner, manifest string) {
				writeInnerManifestAt(t, client, inner, manifest, false)
				if err := os.WriteFile(filepath.Join(client, "Script.pvf"), []byte("new-script"), 0o644); err != nil {
					t.Fatal(err)
				}
			},
			build:  true,
			reason: "已变化",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := buildLaunchTree(t)
			client := filepath.Join(root, "server", "client")
			clientBuild := filepath.Join(root, "server", "work", "client-build")
			inner := filepath.Join(clientBuild, "Script.inner.pvf")
			manifest := filepath.Join(clientBuild, "Script.inner.manifest.json")
			testCase.setup(t, client, inner, manifest)

			needsBuild, reason := decideInnerPVF(client, inner, manifest)
			if needsBuild != testCase.build {
				t.Errorf("needs build = %v, want %v (reason %q)", needsBuild, testCase.build, reason)
			}
			if !strings.Contains(reason, testCase.reason) {
				t.Errorf("reason = %q, want it to contain %q", reason, testCase.reason)
			}
		})
	}
}

// A missing client triple is what the Python's ensure() raised first. It is reported as a
// warning for a server-only run - where the client files are not required - and the
// existing archive keeps the run going, exactly as the Python's WARNING did.
func TestLaunchInnerPVFReportsMissingClientFiles(t *testing.T) {
	root := buildLaunchTree(t)
	for _, name := range []string{"DFO.exe", "sk.dat", "Script.pvf"} {
		if err := os.Remove(filepath.Join(root, "server", "client", name)); err != nil {
			t.Fatal(err)
		}
	}
	report, err := LaunchPlan(root, LaunchOptions{Check: true, ServerOnly: true})
	if err != nil {
		t.Fatalf("a server-only run does not need the client files: %v", err)
	}
	want := "客户端缺少 DFO.exe、sk.dat、Script.pvf，无法生成内层 PVF"
	if !strings.Contains(report.InnerPVF.Message, want) {
		t.Errorf("inner message = %q, want it to contain %q", report.InnerPVF.Message, want)
	}
}

// The required list is the Python's, branch for branch, including the unconditional
// catalog_startup.py and the client-only branch that omits the gateway binary.
func TestLaunchRequiredListPerMode(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	probe := filepath.Join(root, "server", "work", "dfo_probe_tools")
	client := filepath.Join(root, "server", "client")
	probeExe := filepath.Join(probe, "probe.exe")
	pvfBinary := filepath.Join(module, "bin", "wireprobe-pvf.exe")
	sourceBinary := filepath.Join(module, "bin", "wireprobe-handoff-source.exe")

	cases := []struct {
		name string
		opts LaunchOptions
		want []string
	}{
		{
			name: "default",
			opts: LaunchOptions{Check: true},
			want: []string{probeExe, pvfBinary,
				filepath.Join(client, "DFO.exe"), filepath.Join(client, "Script.pvf"),
				filepath.Join(client, "sk.dat")},
		},
		{
			name: "server only",
			opts: LaunchOptions{Check: true, ServerOnly: true},
			want: []string{pvfBinary},
		},
		{
			name: "client only",
			opts: LaunchOptions{Check: true, ClientOnly: true},
			want: []string{probeExe,
				filepath.Join(client, "DFO.exe"), filepath.Join(client, "Script.pvf"),
				filepath.Join(client, "sk.dat")},
		},
		{
			name: "storage only still needs the client",
			opts: LaunchOptions{Check: true, StorageOnly: true},
			want: []string{probeExe, pvfBinary,
				filepath.Join(client, "DFO.exe"), filepath.Join(client, "Script.pvf"),
				filepath.Join(client, "sk.dat")},
		},
		{
			name: "json mode keeps the settings binary",
			opts: LaunchOptions{Check: true, JSONMode: true},
			want: []string{probeExe, pvfBinary,
				filepath.Join(client, "DFO.exe"), filepath.Join(client, "Script.pvf"),
				filepath.Join(client, "sk.dat")},
		},
		{
			name: "source build swaps the profile binary",
			opts: LaunchOptions{Check: true, SourceBuild: true},
			want: []string{probeExe, sourceBinary,
				filepath.Join(client, "DFO.exe"), filepath.Join(client, "Script.pvf"),
				filepath.Join(client, "sk.dat")},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := LaunchPlan(root, testCase.opts)
			if err != nil {
				t.Fatalf("launch plan: %v", err)
			}
			if !reflect.DeepEqual(report.Required, testCase.want) {
				t.Errorf("required =\n%s\nwant\n%s",
					strings.Join(report.Required, "\n"), strings.Join(testCase.want, "\n"))
			}
		})
	}
}

// The modes decide the binary and the data mode, and the profile does not apply to JSON
// mode, client-only or storage-only.
func TestLaunchModesSelectTheBinaryAndDataMode(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")

	cases := []struct {
		name     string
		opts     LaunchOptions
		binary   string
		dataMode string
		profile  bool
	}{
		{
			name:     "default uses the profile",
			opts:     LaunchOptions{Check: true},
			binary:   filepath.Join(module, "bin", "wireprobe-pvf.exe"),
			dataMode: "PVF direct",
			profile:  true,
		},
		{
			name:     "server only still uses the profile",
			opts:     LaunchOptions{Check: true, ServerOnly: true},
			binary:   filepath.Join(module, "bin", "wireprobe-pvf.exe"),
			dataMode: "PVF direct",
			profile:  true,
		},
		{
			name:     "json mode uses the settings binary",
			opts:     LaunchOptions{Check: true, JSONMode: true},
			binary:   filepath.Join(module, "bin", "wireprobe-pvf.exe"),
			dataMode: "JSON / explicit profile",
			profile:  false,
		},
		{
			name:     "storage only skips the profile",
			opts:     LaunchOptions{Check: true, StorageOnly: true},
			binary:   filepath.Join(module, "bin", "wireprobe-pvf.exe"),
			dataMode: "JSON / explicit profile",
			profile:  false,
		},
		{
			name:     "client only skips the profile",
			opts:     LaunchOptions{Check: true, ClientOnly: true},
			binary:   filepath.Join(module, "bin", "wireprobe-pvf.exe"),
			dataMode: "JSON / explicit profile",
			profile:  false,
		},
		{
			name:     "source build without an explicit profile",
			opts:     LaunchOptions{Check: true, SourceBuild: true},
			binary:   filepath.Join(module, "bin", "wireprobe-handoff-source.exe"),
			dataMode: "PVF direct",
			profile:  true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			report, err := LaunchPlan(root, testCase.opts)
			if err != nil {
				t.Fatalf("launch plan: %v", err)
			}
			if report.Binary != testCase.binary {
				t.Errorf("binary = %q, want %q", report.Binary, testCase.binary)
			}
			if report.DataMode != testCase.dataMode {
				t.Errorf("data mode = %q, want %q", report.DataMode, testCase.dataMode)
			}
			if loaded := report.ProfilePath != ""; loaded != testCase.profile {
				t.Errorf("profile loaded = %v, want %v", loaded, testCase.profile)
			}
		})
	}
}

// An explicit --repair-profile always wins, including over --source-build, exactly as
// gateway_configuration decided it.
func TestLaunchRepairProfileWinsOverSourceBuild(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	explicit := filepath.Join(module, "configs", "profile-explicit.json")
	body := `{"binary":"bin/wireprobe-pvf.exe","environment":{"DFO_SHOP_OPEN_ALL":"1"}}`
	if err := os.WriteFile(explicit, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := LaunchPlan(root, LaunchOptions{
		Check: true, SourceBuild: true, RepairProfile: explicit})
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	if report.Binary != filepath.Join(module, "bin", "wireprobe-pvf.exe") {
		t.Errorf("binary = %q, want the profile's binary", report.Binary)
	}
	// A JSON profile has no DFO_PVF_CATALOGS, so the data mode follows it.
	if report.DataMode != "JSON / explicit profile" {
		t.Errorf("data mode = %q", report.DataMode)
	}
}

// The first missing dependency is the one reported（2026-10-05 起不再要求任何 .py，故这里缺的是 probe.exe）。
func TestLaunchReportsTheFirstMissingDependency(t *testing.T) {
	root := buildLaunchTree(t)
	probeExe := filepath.Join(root, "server", "work", "dfo_probe_tools", "probe.exe")
	if err := os.Remove(probeExe); err != nil {
		t.Fatal(err)
	}
	report, err := LaunchPlan(root, LaunchOptions{Check: true, ServerOnly: true})
	// A server-only run needs no probe runtime, so nothing is missing.
	if err != nil {
		t.Fatalf("server-only run: %v", err)
	}
	// server-only 只需要网关程序本身（以前还要 channel_probe.py / catalog_startup.py 两个 .py）。
	if len(report.Required) != 1 {
		t.Errorf("server-only required = %v", report.Required)
	}
	_, err = LaunchPlan(root, LaunchOptions{Check: true})
	if err == nil || err.Error() != "Missing dependency: "+probeExe {
		t.Fatalf("error = %v, want the probe runtime", err)
	}
}

// A malformed configuration is rejected the way the Python rejected it, with its wording.
func TestLaunchRejectsUnusableConfiguration(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(t *testing.T, root string)
		want   string
	}{
		{
			name: "missing launcher.local.json",
			mutate: func(t *testing.T, root string) {
				if err := os.Remove(filepath.Join(root, "server", "launcher.local.json")); err != nil {
					t.Fatal(err)
				}
			},
			want: "Copy launcher.example.json to launcher.local.json and set client_dir.",
		},
		{
			name: "missing storage config",
			mutate: func(t *testing.T, root string) {
				storage := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
				if err := os.Remove(storage); err != nil {
					t.Fatal(err)
				}
			},
			want: "Storage missing. Follow README first-time setup; no database was changed.",
		},
		{
			name: "sqlite without a path",
			mutate: func(t *testing.T, root string) {
				writeLaunchFile(t, root, "server/work/dfo-lan/runtime/storage/local.json", `{"driver":"sqlite"}`)
			},
			want: "Storage driver 'sqlite' requires sqlite_path in runtime/storage/local.json.",
		},
		{
			name: "unknown driver",
			mutate: func(t *testing.T, root string) {
				writeLaunchFile(t, root, "server/work/dfo-lan/runtime/storage/local.json", `{"driver":"MySQL"}`)
			},
			want: "Unsupported storage driver 'mysql'.",
		},
		{
			name: "the removed PostgreSQL driver is refused",
			mutate: func(t *testing.T, root string) {
				writeLaunchFile(t, root, "server/work/dfo-lan/runtime/storage/local.json",
					`{"driver":"postgres","postgres_dsn":"postgresql://user@10.0.0.5:25438/dfo"}`)
			},
			want: "PostgreSQL support was removed (2026-10-05, see root AGENTS.md §0.6); " +
				"runtime/storage/local.json must name sqlite_path.",
		},
		{
			name: "channel_identity is not a boolean",
			mutate: func(t *testing.T, root string) {
				writeLaunchFile(t, root, "server/launcher.local.json",
					`{"client_dir":"client","server_binary":"work/dfo-lan/bin/wireprobe-pvf.exe","channel_identity":1}`)
			},
			want: "channel_identity must be a JSON boolean",
		},
		{
			name: "client_dir is missing",
			mutate: func(t *testing.T, root string) {
				writeLaunchFile(t, root, "server/launcher.local.json",
					`{"server_binary":"work/dfo-lan/bin/wireprobe-pvf.exe"}`)
			},
			want: "launcher.local.json must set client_dir",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := buildLaunchTree(t)
			testCase.mutate(t, root)
			_, err := LaunchPlan(root, LaunchOptions{Check: true})
			if err == nil || err.Error() != testCase.want {
				t.Fatalf("error = %v, want %q", err, testCase.want)
			}
		})
	}
}

// The storage line is how the owner sees which engine a run would use. SQLite is the only
// engine since 2026-10-05 (owner decision, see root AGENTS.md §0.6), so the line is always
// the database file the engine opens.
func TestLaunchStorageLineNamesTheSQLiteFile(t *testing.T) {
	root := buildLaunchTree(t)
	report, err := LaunchPlan(root, LaunchOptions{Check: true})
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	if !strings.HasPrefix(report.Storage, "SQLite ") {
		t.Errorf("storage = %q, want the SQLite file the engine opens", report.Storage)
	}
}

// --dry-run adds the 存储 -> 内层 PVF -> 网关 -> 客户端 plan and starts nothing.
func TestLaunchDryRunPlan(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	probe := filepath.Join(root, "server", "work", "dfo_probe_tools")

	report, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true})
	if err != nil {
		t.Fatalf("launch plan: %v", err)
	}
	if len(report.Plan) != 4 {
		t.Fatalf("plan = %d steps, want 4: %v", len(report.Plan), report.Plan)
	}
	wantSteps := []string{"存储", "内层 PVF", "网关", "客户端"}
	for index, want := range wantSteps {
		if report.Plan[index].Step != want {
			t.Errorf("step %d = %q, want %q", index, report.Plan[index].Step, want)
		}
	}
	if !strings.Contains(report.Plan[0].Detail, "SQLite") {
		t.Errorf("storage step = %q, want the sqlite profile", report.Plan[0].Line())
	}
	if report.Plan[2].Target != filepath.Join(module, "bin", "wireprobe-pvf.exe") {
		t.Errorf("gateway = %q", report.Plan[2].Line())
	}
	if report.Plan[3].Target != filepath.Join(probe, "probe.exe") {
		t.Errorf("client = %q, want probe.exe", report.Plan[3].Line())
	}
	if !strings.Contains(report.Plan[3].Detail, "interactive-ui") {
		t.Errorf("client step = %q, want the interactive probe mode", report.Plan[3].Line())
	}
	if lines := report.Lines(true); len(lines) != 8 {
		t.Errorf("dry-run printed %d lines, want the four check lines plus four steps", len(lines))
	}

	// --server-only and --storage-only do not run the later steps.
	serverOnly, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, ServerOnly: true})
	if err != nil {
		t.Fatalf("server-only plan: %v", err)
	}
	if serverOnly.Plan[3].Target != "(跳过)" {
		t.Errorf("server-only client step = %q, want it skipped", serverOnly.Plan[3].Line())
	}
	storageOnly, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, StorageOnly: true})
	if err != nil {
		t.Fatalf("storage-only plan: %v", err)
	}
	for _, index := range []int{2, 3} {
		if storageOnly.Plan[index].Target != "(跳过)" {
			t.Errorf("storage-only step %d = %q, want it skipped", index, storageOnly.Plan[index].Line())
		}
	}

	// A JSON-mode plan has no inner archive step and says so.
	jsonPlan, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, JSONMode: true})
	if err != nil {
		t.Fatalf("json-mode plan: %v", err)
	}
	if jsonPlan.Plan[1].Target != "(跳过)" {
		t.Errorf("json-mode inner PVF step = %q, want it skipped", jsonPlan.Plan[1].Line())
	}
}

// 计划行的 payload 账号段跟着会话定型的名字走，而名字不合法必须在打印计划之前失败 ——
// 业主拿来做实机对照的就是这几行，不能出现「打印得出、启动却起不来」。
func TestLaunchDryRunPlanAccount(t *testing.T) {
	root := buildLaunchTree(t)
	// 计划走的是 os.Getenv，宿主环境里真带着 DFO_ACCOUNT 时默认段就不是 probe 了；这里把它
	// 钉成空，两条断言才只测 --account 这一条路。
	t.Setenv(accountEnvKey, "")
	// 会话目录名默认来自时钟，不钉住 tag 两次计划的 client.log 路径就不同，逐字节比对无从成立。
	pinned := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "account_plan_pin")

	defaultPlan, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, Tag: "account_plan_pin"})
	if err != nil {
		t.Fatalf("默认计划: %v", err)
	}
	defaultLine := defaultPlan.Plan[3].Detail
	if !strings.Contains(defaultLine, "?probe?") {
		t.Errorf("默认客户端行 = %q，payload 的账号段该是 %q", defaultLine, accountname.Default)
	}
	if !strings.Contains(defaultLine, pinned) {
		t.Errorf("默认客户端行 = %q，会话目录该钉在 %q", defaultLine, pinned)
	}

	renamed, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, Tag: "account_plan_pin", Account: "Tomeu-2"})
	if err != nil {
		t.Fatalf("改名计划: %v", err)
	}
	renamedLine := renamed.Plan[3].Detail
	if !strings.Contains(renamedLine, "?Tomeu-2?") {
		t.Errorf("改名客户端行 = %q", renamedLine)
	}
	// 除账号那一段外逐字节一致：这一条同时钉住「改名只动了账号段」。
	if got := strings.Replace(renamedLine, "?Tomeu-2?", "?probe?", 1); got != defaultLine {
		t.Errorf("改名后的客户端行除账号段外应一致：\n got %q\nwant %q", got, defaultLine)
	}

	for _, bad := range []string{"玩家", "probe?2", "a b", strings.Repeat("a", accountname.MaxLength+1)} {
		if _, err := LaunchPlan(root, LaunchOptions{Check: true, DryRun: true, Account: bad}); err == nil {
			t.Errorf("账号名 %q 该在打印计划前被拒", bad)
		}
	}
}

// Every profile value rule the Python enforced, in one table. The accepted cases also pin
// the normalisation (lower-cased checksum, canonical drop percent, trimmed domains) and
// the required-file list each key class contributes.
func TestLoadProfileValueRules(t *testing.T) {
	accepted := []struct {
		name         string
		body         string
		wantEnv      map[string]string
		wantRequired []string // "R:<relative>" resolves against the module
	}{
		{
			name:    "flags",
			body:    `{"DFO_SHOP_OPEN_ALL":"1","DFO_FATIGUE_FREE":"0"}`,
			wantEnv: map[string]string{"DFO_SHOP_OPEN_ALL": "1", "DFO_FATIGUE_FREE": "0"},
		},
		{
			name:    "hell drop percent is normalised",
			body:    `{"DFO_HELL_PARTY_DROP_PERCENT":"0100"}`,
			wantEnv: map[string]string{"DFO_HELL_PARTY_DROP_PERCENT": "100"},
		},
		{
			name:    "sha256 is lower-cased",
			body:    `{"DFO_PVF_SHA256":"` + strings.Repeat("A", 64) + `"}`,
			wantEnv: map[string]string{"DFO_PVF_SHA256": strings.Repeat("a", 64)},
		},
		{
			name:    "empty diagnostics are the normal path",
			body:    `{"DFO_PVF_SHA256":"","DFO_OMEN_INFO":"","DFO_OATH_GRADES":""}`,
			wantEnv: map[string]string{"DFO_PVF_SHA256": "", "DFO_OMEN_INFO": "", "DFO_OATH_GRADES": ""},
		},
		{
			name:    "omen payload and oath grades",
			body:    `{"DFO_OMEN_INFO":"1,2,3,4;5","DFO_OATH_GRADES":"45,45"}`,
			wantEnv: map[string]string{"DFO_OMEN_INFO": "1,2,3,4;5", "DFO_OATH_GRADES": "45,45"},
		},
		{
			name:    "ispins mode",
			body:    `{"DFO_ISPINS_MODE":"weekly"}`,
			wantEnv: map[string]string{"DFO_ISPINS_MODE": "weekly"},
		},
		{
			name:    "domains are trimmed and rejoined",
			body:    `{"DFO_PVF_CATALOGS":" world , quests "}`,
			wantEnv: map[string]string{"DFO_PVF_CATALOGS": "world,quests"},
		},
		{
			name: "path keys become required",
			body: `{"DFO_PVF_ARCHIVE":"../client-build/Script.inner.pvf"}`,
			wantEnv: map[string]string{
				"DFO_PVF_ARCHIVE": "R:../client-build/Script.inner.pvf",
			},
			wantRequired: []string{"R:../client-build/Script.inner.pvf"},
		},
		{
			name: "equipment catalog requires its data pair",
			body: `{"DFO_EQUIPMENT_FULL_CATALOG":"configs/full.json"}`,
			wantEnv: map[string]string{
				"DFO_EQUIPMENT_FULL_CATALOG": "R:configs/full.json",
			},
			wantRequired: []string{"R:configs/full.json.data", "R:configs/full.json.index.json"},
		},
	}
	for _, testCase := range accepted {
		t.Run("accepts "+testCase.name, func(t *testing.T) {
			module := t.TempDir()
			path := filepath.Join(module, "profile.json")
			writeProfile(t, path, `{"binary":"bin/x.exe","environment":`+testCase.body+`}`)
			profile, err := LoadProfile(path, module)
			if err != nil {
				t.Fatalf("load: %v", err)
			}
			resolve := func(value string) string {
				if strings.HasPrefix(value, "R:") {
					return filepath.Clean(filepath.Join(module, value[2:]))
				}
				return value
			}
			for key, want := range testCase.wantEnv {
				if got := profile.Env[key]; got != resolve(want) {
					t.Errorf("%s = %q, want %q", key, got, resolve(want))
				}
			}
			want := []string{filepath.Join(module, "bin", "x.exe")}
			for _, extra := range testCase.wantRequired {
				want = append(want, resolve(extra))
			}
			if !reflect.DeepEqual(profile.Required, want) {
				t.Errorf("required = %v, want %v", profile.Required, want)
			}
		})
	}

	rejected := []struct {
		name string
		body string
		want string
	}{
		{"unknown key", `{"DFO_NOT_A_KEY":"1"}`, "Unknown profile key or invalid value: DFO_NOT_A_KEY"},
		{"flag written as a JSON boolean", `{"DFO_SHOP_OPEN_ALL":true}`, "Unknown profile key or invalid value: DFO_SHOP_OPEN_ALL"},
		{"flag written as a number", `{"DFO_SHOP_OPEN_ALL":1}`, "Unknown profile key or invalid value: DFO_SHOP_OPEN_ALL"},
		{"unknown domain", `{"DFO_PVF_CATALOGS":"world,nowhere"}`, "Invalid PVF candidate domains"},
		{"duplicate domain", `{"DFO_PVF_CATALOGS":"world,world"}`, "Invalid PVF candidate domains"},
		{"empty domain list", `{"DFO_PVF_CATALOGS":""}`, "Invalid PVF candidate domains"},
		{"checksum too short", `{"DFO_PVF_SHA256":"deadbeef"}`, "Unknown profile key or invalid value: DFO_PVF_SHA256"},
		{"checksum is not a string", `{"DFO_PVF_SHA256":64}`, "Unknown profile key or invalid value: DFO_PVF_SHA256"},
		{"omen payload with letters", `{"DFO_OMEN_INFO":"hello"}`, "Unknown profile key or invalid value: DFO_OMEN_INFO"},
		{"oath grade out of range", `{"DFO_OATH_GRADES":"1234"}`, "Unknown profile key or invalid value: DFO_OATH_GRADES"},
		{"unknown ispins mode", `{"DFO_ISPINS_MODE":"sometimes"}`, "Unknown profile key or invalid value: DFO_ISPINS_MODE"},
		{"hell drop percent above the cap", `{"DFO_HELL_PARTY_DROP_PERCENT":"10001"}`, "Unknown profile key or invalid value: DFO_HELL_PARTY_DROP_PERCENT"},
		{"hell drop percent negative", `{"DFO_HELL_PARTY_DROP_PERCENT":"-1"}`, "Unknown profile key or invalid value: DFO_HELL_PARTY_DROP_PERCENT"},
		{"path key is a number", `{"DFO_PVF_ARCHIVE":7}`, "DFO_PVF_ARCHIVE: Expected nonempty file path"},
		{"path key is empty", `{"DFO_PVF_ARCHIVE":""}`, "DFO_PVF_ARCHIVE: Expected nonempty file path"},
		{"path key is null", `{"DFO_PVF_ARCHIVE":null}`, "DFO_PVF_ARCHIVE: Expected nonempty file path"},
		{"domains are not a string", `{"DFO_PVF_CATALOGS":["world"]}`, "Unknown profile key or invalid value: DFO_PVF_CATALOGS"},
	}
	for _, testCase := range rejected {
		t.Run("rejects "+testCase.name, func(t *testing.T) {
			module := t.TempDir()
			path := filepath.Join(module, "profile.json")
			writeProfile(t, path, `{"binary":"bin/x.exe","environment":`+testCase.body+`}`)
			_, err := LoadProfile(path, module)
			if err == nil {
				t.Fatal("the value was accepted")
			}
			if !strings.Contains(err.Error(), testCase.want) {
				t.Errorf("error = %v, want it to contain %q", err, testCase.want)
			}
		})
	}
}

// A profile is exactly {binary, environment}; the first bad entry in file order is the one
// reported, which is what makes the message stable.
func TestLoadProfileShapeAndOrder(t *testing.T) {
	module := t.TempDir()
	cases := map[string]string{
		`["binary"]`:             "Expected binary and environment fields",
		`{"binary":"bin/x.exe"}`: "Expected binary and environment fields",
		`{"environment":{}}`:     "Expected binary and environment fields",
		`{"binary":"bin/x.exe","environment":{},"x":1}`: "Expected binary and environment fields",
		`{"binary":"","environment":{}}`:                "Expected nonempty file path",
		`{"binary":null,"environment":{}}`:              "Expected nonempty file path",
		`{"binary":"bin/x.exe","environment":[]}`:       "environment must be an object",
	}
	for body, want := range cases {
		path := filepath.Join(module, "profile.json")
		writeProfile(t, path, body)
		_, err := LoadProfile(path, module)
		if err == nil {
			t.Errorf("%s: was accepted", body)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s: error = %v, want it to contain %q", body, err, want)
		}
	}

	// File order decides which of two bad entries is reported.
	path := filepath.Join(module, "profile.json")
	writeProfile(t, path,
		`{"binary":"bin/x.exe","environment":{"DFO_NOT_A_KEY":"1","DFO_ALSO_NOT":"2"}}`)
	if _, err := LoadProfile(path, module); err == nil || !strings.Contains(err.Error(), "DFO_NOT_A_KEY") {
		t.Errorf("error = %v, want the first bad key in file order", err)
	}

	// A relative binary resolves against the module; an absolute one is kept.
	writeProfile(t, path, `{"binary":"bin/x.exe","environment":{}}`)
	profile, err := LoadProfile(path, module)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if profile.Binary != filepath.Join(module, "bin", "x.exe") {
		t.Errorf("binary = %q", profile.Binary)
	}
	absolute := filepath.Join(module, "elsewhere", "y.exe")
	writeProfile(t, path, `{"binary":`+strconvQuote(absolute)+`,"environment":{}}`)
	profile, err = LoadProfile(path, module)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if profile.Binary != absolute {
		t.Errorf("absolute binary = %q, want %q", profile.Binary, absolute)
	}
}

// The domain whitelist must not drift between the two implementations: a domain the Python
// accepts but the server does not serve turns a working profile into a rejected one, and
// the reverse hides a typo until the gateway refuses it. repair_profile.py's own comment
// names gamedata.SupportedDomains, so this compares the two literals directly.
func TestPVFDomainsMatchThePythonWhitelist(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "scripts", "repair_profile.py"))
	if err != nil {
		t.Fatalf("read repair_profile.py: %v", err)
	}
	match := regexp.MustCompile(`allowed = \{([^}]*)\}`).FindSubmatch(body)
	if match == nil {
		t.Fatal("the Python whitelist was not found; repair_profile.py changed shape")
	}
	python := map[string]bool{}
	for _, name := range regexp.MustCompile(`'([a-z0-9-]+)'`).FindAllStringSubmatch(string(match[1]), -1) {
		python[name[1]] = true
	}
	goDomains := map[string]bool{}
	for _, domain := range strings.Split(gamedata.SupportedDomains, ",") {
		goDomains[strings.TrimSpace(domain)] = true
	}
	if !reflect.DeepEqual(python, goDomains) {
		t.Errorf("the Python whitelist has %d domains and the Go one %d; only in Python: %v; only in Go: %v",
			len(python), len(goDomains), missingFrom(python, goDomains), missingFrom(goDomains, python))
	}
	if !reflect.DeepEqual(pvfDomains, goDomains) {
		t.Error("the launcher's whitelist is not gamedata.SupportedDomains")
	}
}

func missingFrom(left, right map[string]bool) []string {
	var only []string
	for name := range left {
		if !right[name] {
			only = append(only, name)
		}
	}
	return only
}

func writeLaunchFile(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, relative)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeProfile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// strconvQuote renders a Windows path as a JSON string literal for the profile bodies.
func strconvQuote(path string) string {
	encoded, err := json.Marshal(path)
	if err != nil {
		return `""`
	}
	return string(encoded)
}

// 账号口径：显式 --account 赢，其次已合并进来的 DFO_ACCOUNT（GUI 启动器注入的就是这条），
// 最后才是历史默认 probe。
func TestSessionAccountPrecedence(t *testing.T) {
	lookup := func(value string) func(string) string {
		return func(key string) string {
			if key == accountEnvKey {
				return value
			}
			return ""
		}
	}
	if got := sessionAccount("Tomeu-2", lookup("from_env")); got != "Tomeu-2" {
		t.Errorf("给了显式参数时该用它，得到 %q", got)
	}
	if got := sessionAccount("", lookup("from_env")); got != "from_env" {
		t.Errorf("没有显式参数时该沿用环境，得到 %q", got)
	}
	if got := sessionAccount("", lookup("")); got != accountname.Default {
		t.Errorf("两处都没给时该回落 %q，得到 %q", accountname.Default, got)
	}
}

// 写环境策略：谁都没选账号时一个字节都不加，子进程环境与今天逐字节一致；选了才写，写的值
// 就是定型的名字；名字不合法当场失败。
func TestApplySessionAccountWritePolicy(t *testing.T) {
	base := []string{`SystemRoot=C:\Windows`, "DFO_ENABLE_OBSERVER=0"}
	withInherited := func(value string) []string {
		return append(append([]string{}, base...), accountEnvKey+"="+value)
	}
	assertEnv := func(t *testing.T, env *childEnv, want []string) {
		t.Helper()
		if got := env.List(); !reflect.DeepEqual(got, want) {
			t.Errorf("子进程环境 = %v, want %v", got, want)
		}
	}

	t.Run("默认一个字节都不加", func(t *testing.T) {
		env := newChildEnv(base)
		account, err := applySessionAccount(env, LaunchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if account != accountname.Default {
			t.Errorf("account = %q, want %q", account, accountname.Default)
		}
		if env.Has(accountEnvKey) {
			t.Errorf("默认启动不该写出 %s", accountEnvKey)
		}
		assertEnv(t, env, base)
	})

	t.Run("显式参数写进环境", func(t *testing.T) {
		env := newChildEnv(base)
		account, err := applySessionAccount(env, LaunchOptions{Account: "Tomeu-2"})
		if err != nil {
			t.Fatal(err)
		}
		if account != "Tomeu-2" {
			t.Errorf("account = %q", account)
		}
		assertEnv(t, env, withInherited("Tomeu-2"))
	})

	t.Run("继承值原样保住", func(t *testing.T) {
		env := newChildEnv(withInherited("gui_saved"))
		account, err := applySessionAccount(env, LaunchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if account != "gui_saved" {
			t.Errorf("account = %q, want gui_saved", account)
		}
		assertEnv(t, env, withInherited("gui_saved"))
	})

	t.Run("显式参数覆盖继承值且留在原位", func(t *testing.T) {
		env := newChildEnv(withInherited("gui_saved"))
		account, err := applySessionAccount(env, LaunchOptions{Account: "tomeu_new"})
		if err != nil {
			t.Fatal(err)
		}
		if account != "tomeu_new" {
			t.Errorf("account = %q", account)
		}
		assertEnv(t, env, withInherited("tomeu_new"))
	})

	t.Run("存在但为空也写回默认名", func(t *testing.T) {
		env := newChildEnv(withInherited(""))
		account, err := applySessionAccount(env, LaunchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if account != accountname.Default {
			t.Errorf("account = %q, want %q", account, accountname.Default)
		}
		assertEnv(t, env, withInherited(accountname.Default))
	})

	unsafe := []string{"probe?2", "玩家", "a b", "a=b", strings.Repeat("a", accountname.MaxLength+1)}
	for _, name := range unsafe {
		t.Run("拒绝 "+name, func(t *testing.T) {
			env := newChildEnv(base)
			if _, err := applySessionAccount(env, LaunchOptions{Account: name}); err == nil {
				t.Errorf("账号名 %q 该被拒", name)
			}
			assertEnv(t, env, base)

			inherited := newChildEnv(append([]string{}, withInherited(name)...))
			if _, err := applySessionAccount(inherited, LaunchOptions{}); err == nil {
				t.Errorf("环境里的账号名 %q 该被拒", name)
			}
		})
	}
}
