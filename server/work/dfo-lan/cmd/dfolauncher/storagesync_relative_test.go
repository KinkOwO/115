package main

import (
	"bytes"
	"dfolan/internal/database"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestStorageSyncRelativePathsBindToExplicitModuleAndKeepConfig(t *testing.T) {
	for _, name := range []string{"default-relative", "parent-relative", "absolute-custom", "absolute-external"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			roots := []string{filepath.Join(base, "原包 中文"), filepath.Join(base, "迁移 包")}
			for _, root := range roots {
				module := filepath.Join(root, "server", "work", "dfo-lan")
				rel := "runtime/storage/dfolan.sqlite3"
				if name == "parent-relative" {
					rel = "../自定义 存档/我的库.sqlite3"
				}
				target := filepath.Clean(filepath.Join(module, rel))
				if name == "absolute-external" {
					target = filepath.Join(t.TempDir(), "外置 位置", "个人.sqlite3")
				}
				configured := rel
				if name == "absolute-custom" || name == "absolute-external" {
					configured = target
				}
				writeStorageFile(t, target, testSQLiteHeader)
				body, err := json.MarshalIndent(map[string]any{"driver": "sqlite", "sqlite_path": configured, "保留字段": map[string]any{"value": 42}}, "", "  ")
				if err != nil {
					t.Fatal(err)
				}
				body = append([]byte{0xef, 0xbb, 0xbf}, body...)
				path := filepath.Join(storageSyncDir(root), "local.json")
				writeStorageFile(t, path, string(body))
				alias := filepath.Join(filepath.Dir(target), "同文件 别名.sqlite3")
				if err := os.Link(target, alias); err != nil {
					t.Fatal(err)
				}
				for _, source := range []string{"storage", "gm-local"} {
					result := storageSyncResult{Action: "list"}
					if err := storageSyncRun(root, &result, storageSyncOptions{Source: source, ExpectedDatabase: alias}); err != nil {
						t.Fatal(err)
					}
					wantBackups := filepath.Join(filepath.Dir(target), database.BackupDirName)
					if source == "gm-local" {
						wantBackups = filepath.Join(filepath.Dir(target), "gm-backups", "local")
					}
					if result.Database != target || result.StorageDir != filepath.Dir(target) || result.BackupsDir != wantBackups {
						t.Fatalf("wrong root/source: %+v want %s", result, target)
					}
				}
				foreign := filepath.Join(t.TempDir(), "另一个 同内容.sqlite3")
				writeStorageFile(t, foreign, testSQLiteHeader)
				if err := storageSyncRun(root, &storageSyncResult{Action: "list"}, storageSyncOptions{ExpectedDatabase: foreign}); err == nil {
					t.Fatal("cross-database identity accepted")
				}
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := storageSyncRun(root, &storageSyncResult{Action: "list"}, storageSyncOptions{ExpectedDatabase: alias}); err == nil {
					t.Fatal("missing configured database accepted")
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Fatal("missing database was created")
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(body, after) {
					t.Fatal("config rewritten", err)
				}
			}
		})
	}
}

func TestStorageSyncRelativeDefaultRejectsAnotherExistingPackage(t *testing.T) {
	base := t.TempDir()
	roots := []string{filepath.Join(base, "旧包"), filepath.Join(base, "新包")}
	for _, root := range roots {
		writeStorageFile(t, filepath.Join(storageSyncDir(root), "dfolan.sqlite3"), testSQLiteHeader)
		writeStorageFile(t, filepath.Join(storageSyncDir(root), "local.json"), `{"driver":"sqlite","sqlite_path":"runtime/storage/dfolan.sqlite3"}`)
	}
	if err := storageSyncRun(roots[1], &storageSyncResult{Action: "list"}, storageSyncOptions{ExpectedDatabase: filepath.Join(storageSyncDir(roots[0]), "dfolan.sqlite3")}); err == nil {
		t.Fatal("copied package followed the old database")
	}
}
