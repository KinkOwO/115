package launcher

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Capture the whole isolated fixture: no config/database/cache writes or archive
// rotations may hide behind an otherwise successful first-install preview.
func firstPreflightSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if entry.IsDir() {
			files[rel] = "directory"
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[rel] = fmt.Sprintf("%x %d %d", sha256.Sum256(body), info.Size(), info.ModTime().UnixNano())
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestLaunchFirstPreflightPlansMissingStorageWithoutWrites(t *testing.T) {
	for _, option := range []struct {
		name string
		opts LaunchOptions
	}{
		{"check", LaunchOptions{Check: true}},
		{"dry-run", LaunchOptions{DryRun: true}},
		{"check-and-dry-run", LaunchOptions{Check: true, DryRun: true}},
	} {
		for _, missingDir := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/missing-directory=%v", option.name, missingDir), func(t *testing.T) {
				root := buildLaunchTree(t)
				storage := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage")
				if err := os.Remove(filepath.Join(storage, "local.json")); err != nil {
					t.Fatal(err)
				}
				if missingDir {
					if err := os.Remove(storage); err != nil {
						t.Fatal(err)
					}
				}
				// Make the inner cache stale: preview must not repair or rotate it.
				writeLaunchFile(t, root, "server/work/client-build/Script.inner.manifest.json", `{"format":"stale"}`)
				before := firstPreflightSnapshot(t, root)
				report, err := LaunchPlan(root, option.opts)
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Join(storage, "dfolan.sqlite3")
				if !report.StorageUninitialized || !strings.Contains(report.Storage, want) || !strings.Contains(report.Storage, "尚未初始化") {
					t.Fatalf("storage plan: %+v", report)
				}
				if !report.InnerPVF.NeedsBuild {
					t.Fatal("stale archive was not identified")
				}
				if option.opts.DryRun && (len(report.Plan) != 4 || report.Plan[0].Target != "(待首次初始化)" || !strings.Contains(report.Plan[0].Detail, want)) {
					t.Fatalf("plan: %+v", report.Plan)
				}
				if !reflect.DeepEqual(before, firstPreflightSnapshot(t, root)) {
					t.Fatal("preview changed fixture files")
				}
				for _, path := range []string{filepath.Join(storage, "local.json"), want, want + "-wal", want + "-shm"} {
					if _, err := os.Lstat(path); !os.IsNotExist(err) {
						t.Fatalf("preview created %s: %v", path, err)
					}
				}
			})
		}
	}
}

func TestLaunchFirstPreflightDoesNotMaskExistingConfiguration(t *testing.T) {
	for _, fixture := range []struct {
		name, body       string
		directory, valid bool
	}{
		{name: "malformed", body: `{"driver":`},
		{name: "postgres", body: `{"driver":"postgres","sqlite_path":"other.sqlite3"}`},
		{name: "unknown-driver", body: `{"driver":"mysql","sqlite_path":"other.sqlite3"}`},
		{name: "missing-path", body: `{"driver":"sqlite"}`},
		{name: "null-path", body: `{"driver":"sqlite","sqlite_path":null}`},
		{name: "empty-path", body: `{"driver":"sqlite","sqlite_path":""}`},
		{name: "whitespace-path", body: `{"driver":"sqlite","sqlite_path":"  "}`},
		{name: "directory", directory: true},
		{name: "custom-existing", body: `{"driver":"sqlite","sqlite_path":"custom saves/kept.sqlite3","unrelated":42}`, valid: true},
	} {
		for _, opts := range []LaunchOptions{{Check: true}, {DryRun: true}} {
			t.Run(fmt.Sprintf("%s/check=%v", fixture.name, opts.Check), func(t *testing.T) {
				root := buildLaunchTree(t)
				path := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
				if fixture.directory {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
					if err := os.Mkdir(path, 0700); err != nil {
						t.Fatal(err)
					}
				} else {
					if err := os.WriteFile(path, []byte(fixture.body), 0600); err != nil {
						t.Fatal(err)
					}
				}
				before := firstPreflightSnapshot(t, root)
				report, err := LaunchPlan(root, opts)
				if fixture.valid {
					if err != nil || report.Storage != "SQLite custom saves/kept.sqlite3" {
						t.Fatalf("custom config changed: %+v %v", report, err)
					}
				} else if err == nil {
					t.Fatal("invalid existing configuration accepted")
				}
				if report.StorageUninitialized {
					t.Fatal("existing config received first-install fallback")
				}
				if !reflect.DeepEqual(before, firstPreflightSnapshot(t, root)) {
					t.Fatal("existing config/files changed")
				}
			})
		}
	}
}

func TestLaunchFirstPreflightStillRequiresConfigurationForActualLaunch(t *testing.T) {
	root := buildLaunchTree(t)
	path := filepath.Join(root, "server", "work", "dfo-lan", "runtime", "storage", "local.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	before := firstPreflightSnapshot(t, root)
	for _, opts := range []LaunchOptions{{}, {ServerOnly: true}, {StorageOnly: true}, {ClientOnly: true}} {
		report, err := LaunchPlan(root, opts)
		if err != errStorageMissing || report.StorageUninitialized {
			t.Fatalf("actual launch bypassed initialization: %+v %v", report, err)
		}
	}
	if !reflect.DeepEqual(before, firstPreflightSnapshot(t, root)) {
		t.Fatal("rejected launch changed fixture")
	}
}
