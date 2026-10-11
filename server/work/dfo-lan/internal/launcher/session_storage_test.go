package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"dfolan/internal/database"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestSessionStorageGatewayArgumentOpensOnlyMovedPackageDatabase(t *testing.T) {
	for _, relative := range []string{"runtime/storage/dfolan.sqlite3", "../中文 自定义/人物.sqlite3"} {
		t.Run(relative, func(t *testing.T) {
			base := t.TempDir()
			ctx := context.Background()
			oldRoot := filepath.Join(base, "仍然存在 的旧包")
			newRoot := filepath.Join(base, "迁移 新包")
			var oldDB string
			var oldDigest [32]byte
			for _, root := range []string{oldRoot, newRoot} {
				module := filepath.Join(root, "server", "work", "dfo-lan")
				active := filepath.Join(module, "runtime", "storage", "local.json")
				body, _ := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": relative, "max_connections": 2, "busy_timeout_ms": 500, "max_read_connections": 2, "sqlite_busy_timeout_ms": 500, "durable_writes": true, "postgres_dsn": "DO-NOT-COPY-OBSOLETE-SECRET", "unknown": 42})
				if err := os.MkdirAll(filepath.Dir(active), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(active, body, 0600); err != nil {
					t.Fatal(err)
				}
				in := launcherGatewayInput(t, module)
				if err := os.MkdirAll(in.Session.Out, 0700); err != nil {
					t.Fatal(err)
				}
				resolved, err := prepareSessionStorage(module, in.Session.Out)
				if err != nil {
					t.Fatal(err)
				}
				in.StorageConfig = resolved
				command, err := buildGatewayCommand(in)
				if err != nil {
					t.Fatal(err)
				}
				arg := ""
				for i, v := range command.Args {
					if v == "-character-storage" && i+1 < len(command.Args) {
						arg = command.Args[i+1]
					}
				}
				if arg != resolved || arg == active || filepath.Dir(arg) != in.Session.Out {
					t.Fatalf("actual gateway config: %q", arg)
				}
				cfg, err := database.LoadConfig(arg)
				if err != nil {
					t.Fatal(err)
				}
				want := filepath.Clean(filepath.Join(module, relative))
				if cfg.SQLitePath != want || !filepath.IsAbs(cfg.SQLitePath) || cfg.MaxConnections != 2 || !cfg.DurableWrites || cfg.BusyTimeoutMS != 500 || cfg.SQLiteBusyTimeoutMS != 500 || cfg.MaxReadConnections != 2 {
					t.Fatalf("configuration lost fields: %+v", cfg)
				}
				raw, err := os.ReadFile(arg)
				if err != nil || bytes.Contains(raw, []byte("SECRET")) || bytes.Contains(raw, []byte("unknown")) {
					t.Fatal("unnecessary sensitive/unknown data leaked", err)
				}
				// These are the exact API calls bootstrap uses for character-storage.
				if err := os.MkdirAll(filepath.Dir(want), 0755); err != nil {
					t.Fatal(err)
				}
				store, err := database.Open(ctx, cfg)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := store.DatabaseName(ctx)
				if err != nil {
					store.Close()
					t.Fatal(err)
				}
				if filepath.Clean(actual) != want {
					store.Close()
					t.Fatalf("opened %s want %s", actual, want)
				}
				if _, err := store.DevelopmentAccount(ctx, "isolated-"+filepath.Base(root)); err != nil {
					store.Close()
					t.Fatal(err)
				}
				store.Close()
				after, err := os.ReadFile(active)
				if err != nil || !bytes.Equal(after, body) {
					t.Fatal("active configuration changed", err)
				}
				if root == oldRoot {
					oldDB = want
					raw, err := os.ReadFile(oldDB)
					if err != nil {
						t.Fatal(err)
					}
					oldDigest = sha256.Sum256(raw)
				} else {
					raw, err := os.ReadFile(oldDB)
					if err != nil || sha256.Sum256(raw) != oldDigest {
						t.Fatal("new root touched old database", err)
					}
					a, _ := os.Stat(oldDB)
					b, _ := os.Stat(want)
					if os.SameFile(a, b) {
						t.Fatal("new root reopened old save")
					}
				}
			}
		})
	}
}

func TestSessionStorageKeepsAbsoluteCustomConfigAndPreviewsReadOnly(t *testing.T) {
	root := buildLaunchTree(t)
	module := filepath.Join(root, "server", "work", "dfo-lan")
	custom := filepath.Join(t.TempDir(), "外置 绝对", "自定义.sqlite3")
	body, _ := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": custom, "unknown": "preserve active"})
	path := filepath.Join(module, "runtime", "storage", "local.json")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	before := firstPreflightSnapshot(t, root)
	for _, opts := range []LaunchOptions{{Check: true}, {DryRun: true}} {
		if _, err := LaunchPlan(root, opts); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(before, firstPreflightSnapshot(t, root)) {
		t.Fatal("check/plan created a session config")
	}
	session := newSessionTag(module, "roles_persist_test")
	if err := os.MkdirAll(session.Out, 0700); err != nil {
		t.Fatal(err)
	}
	resolved, err := prepareSessionStorage(module, session.Out)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := database.LoadConfig(resolved)
	if err != nil || cfg.SQLitePath != custom {
		t.Fatal(cfg, err)
	}
	if err := os.MkdirAll(filepath.Dir(custom), 0755); err != nil {
		t.Fatal(err)
	}
	store, err := database.Open(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(body, after) {
		t.Fatal("absolute custom config changed", err)
	}
	if _, err := os.Stat(filepath.Join(module, "runtime", "storage", "dfolan.sqlite3")); !os.IsNotExist(err) {
		t.Fatal("unrelated default DB created")
	}
}

func TestSessionStorageRejectsInvalidInputsBeforeWritingSnapshot(t *testing.T) {
	for _, body := range []string{`{"driver":"postgres","sqlite_path":"runtime/storage/a.sqlite3"}`, `{"driver":"sqlite","sqlite_path":""}`, `{"driver":"sqlite","sqlite_path":"  "}`, `{"driver":"sqlite","sqlite_path":"C:relative.sqlite3"}`, `{`} {
		module := t.TempDir()
		out := filepath.Join(module, "runtime", "session")
		if err := os.MkdirAll(out, 0700); err != nil {
			t.Fatal(err)
		}
		writeLaunchFile(t, module, "runtime/storage/local.json", body)
		if _, err := prepareSessionStorage(module, out); err == nil {
			t.Fatal("invalid config accepted", body)
		}
		entries, err := os.ReadDir(out)
		if err != nil || len(entries) != 0 {
			t.Fatal("invalid config created snapshot", err)
		}
	}
	module := t.TempDir()
	writeLaunchFile(t, module, "runtime/storage/local.json", `{"driver":"sqlite","sqlite_path":"runtime/storage/a.sqlite3"}`)
	if _, err := prepareSessionStorage(module, t.TempDir()); err == nil || !strings.Contains(err.Error(), "private runtime session") {
		t.Fatal("outside session accepted", err)
	}
}
