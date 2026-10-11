package main

import (
	"bytes"
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalEditionUsesConfiguredExistingDatabase(t *testing.T) {
	previous := defaultEdition
	defaultEdition = "local"
	defer func() { defaultEdition = previous }()
	root := t.TempDir()
	custom := filepath.Join(root, "saves", "我的单机.sqlite3")
	if err := os.MkdirAll(filepath.Dir(custom), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(custom, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias.sqlite3")
	if err := os.Link(custom, alias); err != nil {
		t.Fatal(err)
	}
	p := editionPaths(root, "local")
	cfg := database.Config{Driver: "sqlite", SQLitePath: custom}
	m, err := newEditionManager(p, cfg, catalog.Progression{}, alias)
	if err != nil || m.config.SQLitePath != custom || m.backups != filepath.Join(filepath.Dir(custom), "gm-backups", "local") {
		t.Fatal(m, err)
	}
	other := filepath.Join(root, "other.sqlite3")
	if err := os.WriteFile(other, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := newEditionManager(p, cfg, catalog.Progression{}, other); err == nil {
		t.Fatal("expected DB mismatch accepted")
	}
	cfg.SQLitePath = filepath.Join(root, "missing.sqlite3")
	if _, err := newEditionManager(p, cfg, catalog.Progression{}, cfg.SQLitePath); err == nil {
		t.Fatal("missing DB accepted")
	}
	if _, err := os.Stat(cfg.SQLitePath); !os.IsNotExist(err) {
		t.Fatal("empty DB created")
	}
}

func TestLocalEditionResolvesRelativeConfigInTwoPackages(t *testing.T) {
	previous := defaultEdition
	defaultEdition = "local"
	defer func() { defaultEdition = previous }()
	base := t.TempDir()
	for _, relative := range []string{"runtime/storage/dfolan.sqlite3", "../自定义 存档/角色.sqlite3"} {
		for _, name := range []string{"原包", "迁移 包"} {
			root := filepath.Join(base, name)
			p := editionPaths(root, "local")
			target := filepath.Clean(filepath.Join(moduleDir(root), relative))
			for _, dir := range []string{filepath.Dir(p.storage), filepath.Dir(target)} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(target, []byte("isolated identity fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			body, _ := json.Marshal(map[string]any{"driver": "sqlite", "sqlite_path": relative, "keep": "unchanged"})
			if err := os.WriteFile(p.storage, body, 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := database.LoadConfig(p.storage)
			if err != nil {
				t.Fatal(err)
			}
			manager, err := newEditionManager(p, cfg, catalog.Progression{}, target)
			if err != nil || manager.config.SQLitePath != target {
				t.Fatal(manager, err)
			}
			other := filepath.Join(t.TempDir(), "other.sqlite3")
			if err := os.WriteFile(other, []byte("isolated identity fixture"), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := newEditionManager(p, cfg, catalog.Progression{}, other); err == nil {
				t.Fatal("other database accepted")
			}
			after, err := os.ReadFile(p.storage)
			if err != nil || !bytes.Equal(body, after) {
				t.Fatal("configuration changed", err)
			}
		}
	}
}
