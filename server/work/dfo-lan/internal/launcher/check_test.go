package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildTree creates the layout the launcher expects, so Check can be verified without
// depending on the machine's real client, profile or binaries.
func buildTree(t *testing.T) string {
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
	// launcher.local.json sits above the module, and its relative paths resolve from there.
	write(filepath.Join("server", "launcher.local.json"),
		`{"client_dir":"client","server_binary":"work/dfo-lan/bin/wireprobe-pvf.exe"}`)
	write(filepath.Join("server", "client", "DFO.exe"), "stub")
	write(filepath.Join("server", "client", "Script.pvf"), "stub")
	write(filepath.Join("server", "client", "sk.dat"), "stub")
	write(filepath.Join("server", "work", "dfo_probe_tools", "probe.exe"), "stub")
	write(filepath.Join("server", "work", "dfo-lan", "bin", "wireprobe-pvf.exe"), "stub")
	write(filepath.Join("server", "work", "dfo-lan", "bin", "wireprobe-handoff-source.exe"), "stub")
	// The default run applies the PVF default profile, whose own files are required too.
	write(filepath.Join("server", "work", "dfo-lan", "configs", "pvf-default.json"),
		`{"binary":"bin/wireprobe-pvf.exe","environment":{`+
			`"DFO_PVF_CATALOGS":"world",`+
			`"DFO_PVF_ARCHIVE":"../client-build/Script.inner.pvf",`+
			`"DFO_PVF_DROP_POLICY":"configs/pvf-drop-policy.json",`+
			`"DFO_PVF_VERIFY_BASELINES":"0"}}`)
	write(filepath.Join("server", "work", "client-build", "Script.inner.pvf"), "stub")
	write(filepath.Join("server", "work", "dfo-lan", "configs", "pvf-drop-policy.json"), "{}")
	return root
}

func TestCheckFindsEveryDependency(t *testing.T) {
	root := buildTree(t)
	report, err := Check(root, CheckOptions{})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if missing := report.Missing(); len(missing) != 0 {
		t.Fatalf("check reported %d missing paths on a complete tree: %v", len(missing), missing)
	}
	if report.DataMode != "PVF direct" {
		t.Errorf("data mode = %q, want PVF direct", report.DataMode)
	}
	// client_dir is written relative to server/, which is the fact this test pins down.
	want := filepath.Join(root, "server", "client")
	if report.ClientDir != want {
		t.Errorf("client dir = %q, want %q", report.ClientDir, want)
	}
	if filepath.Base(report.Binary) != "wireprobe-pvf.exe" {
		t.Errorf("default binary = %q, want the PVF default", report.Binary)
	}
	// The profile supplied the binary and its own required files, including the archive.
	if filepath.Base(report.ProfilePath) != "pvf-default.json" {
		t.Errorf("profile = %q, want the PVF default profile", report.ProfilePath)
	}
	if got := report.ProfileEnv["DFO_PVF_ARCHIVE"]; filepath.Base(got) != "Script.inner.pvf" {
		t.Errorf("profile archive = %q, want the resolved inner PVF", got)
	}
}

func TestCheckSelectsTheSourceBinaryAndServerOnlyScope(t *testing.T) {
	root := buildTree(t)
	// An explicit repair profile means the profile's binary is NOT overridden.
	explicit := filepath.Join(root, "server", "work", "dfo-lan", "configs", "profile.json")
	if err := os.WriteFile(explicit,
		[]byte(`{"binary":"bin/wireprobe-pvf.exe","environment":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Check(root, CheckOptions{
		SourceBuild: true, ServerOnly: true, RepairProfile: explicit})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	// With an explicit profile the profile wins, exactly as gateway_configuration did.
	if filepath.Base(report.Binary) != "wireprobe-pvf.exe" {
		t.Errorf("explicit profile did not win: %q", report.Binary)
	}
	for _, dependency := range report.Dependencies {
		switch dependency.Label {
		case "client", "client script archive", "client key data", "probe runtime":
			t.Errorf("a server-only check required %s", dependency.Label)
		}
	}

	// Without an explicit profile, --source-build keeps the source binary and swaps it
	// into the profile's required list.
	report, err = Check(root, CheckOptions{SourceBuild: true})
	if err != nil {
		t.Fatalf("check: %v", err)
	}
	if filepath.Base(report.Binary) != "wireprobe-handoff-source.exe" {
		t.Errorf("--source-build selected %q", report.Binary)
	}
	if len(report.Missing()) != 0 {
		t.Errorf("source build on a complete tree reported missing: %v", report.Missing())
	}

	// Removing a profile file must be reported, not ignored.
	if err := os.Remove(filepath.Join(root, "server", "work", "client-build", "Script.inner.pvf")); err != nil {
		t.Fatal(err)
	}
	report, err = Check(root, CheckOptions{})
	if err != nil {
		t.Fatalf("check after removal: %v", err)
	}
	missing := report.Missing()
	if len(missing) != 1 || filepath.Base(missing[0].Path) != "Script.inner.pvf" {
		t.Errorf("missing = %v, want exactly the inner PVF archive", missing)
	}
}

func TestCheckRequiresClientDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "server", "launcher.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"server_binary":"x"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Check(root, CheckOptions{}); err == nil {
		t.Error("a settings file without client_dir was accepted")
	}
	if _, err := Check(t.TempDir(), CheckOptions{}); err == nil {
		t.Error("a missing launcher.local.json was accepted")
	}
}

// A profile must be exactly {binary, environment}; anything else is a profile someone
// edited into something the launcher cannot honour.
func TestLoadProfileRejectsMalformedProfiles(t *testing.T) {
	module := t.TempDir()
	cases := map[string]string{
		"missing environment": `{"binary":"bin/x.exe"}`,
		"missing binary":      `{"environment":{}}`,
		"extra field":         `{"binary":"bin/x.exe","environment":{},"extra":1}`,
		"empty binary":        `{"binary":"","environment":{}}`,
	}
	for name, body := range cases {
		path := filepath.Join(module, "profile.json")
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProfile(path, module); err == nil {
			t.Errorf("%s: was accepted", name)
		}
	}

	// A well-formed profile resolves its paths against the module and requires them.
	path := filepath.Join(module, "profile.json")
	body := `{"binary":"bin/x.exe","environment":{"DFO_PVF_ARCHIVE":"../a/b.pvf","DFO_SHOP_OPEN_ALL":"1"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadProfile(path, module)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	wantBinary := filepath.Join(module, "bin", "x.exe")
	if loaded.Binary != wantBinary {
		t.Errorf("binary = %q, want %q", loaded.Binary, wantBinary)
	}
	if len(loaded.Required) != 2 {
		t.Errorf("required = %v, want the binary and the archive", loaded.Required)
	}
	if loaded.Env["DFO_SHOP_OPEN_ALL"] != "1" {
		t.Errorf("flag env = %q", loaded.Env["DFO_SHOP_OPEN_ALL"])
	}
}

// The storage line is how the owner confirms which engine a run will use, so its wording
// is pinned: swapping in a sqlite profile must be visible before anything starts.
func TestCheckReportsTheStorageDriver(t *testing.T) {
	root := buildTree(t)
	storageDir := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		t.Fatal(err)
	}
	storagePath := filepath.Join(storageDir, "local.json")

	// No storage config at all means the shared SQLite fallback (2026-10-05 业主口径
	// 「默认 sqlite」), which the report must say.
	report, err := Check(root, CheckOptions{ServerOnly: true})
	if err != nil {
		t.Fatalf("check without a storage config: %v", err)
	}
	if !strings.HasPrefix(report.Storage, "SQLite ") {
		t.Errorf("storage = %q, want the SQLite form (the shared fallback)", report.Storage)
	}

	// A sqlite profile must name the database file instead, so the owner can see at a
	// glance that no PostgreSQL is involved.
	dbPath := filepath.Join(root, "save.sqlite3")
	body := `{"driver":"sqlite","sqlite_path":"` + filepath.ToSlash(dbPath) + `"}`
	if err := os.WriteFile(storagePath, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err = Check(root, CheckOptions{ServerOnly: true})
	if err != nil {
		t.Fatalf("check with a sqlite profile: %v", err)
	}
	// The report echoes the configured path verbatim, so compare against the same form.
	want := "SQLite " + filepath.ToSlash(dbPath)
	if report.Storage != want {
		t.Errorf("storage = %q, want %q", report.Storage, want)
	}
}
