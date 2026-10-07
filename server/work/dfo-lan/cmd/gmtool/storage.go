package main

// 存储自举：GM 工具不要求管理员权限，自己把 PostgreSQL 拉起来（SQLite 档不需要）。
//
// 启动链之所以要求管理员，是因为它后面要装 WFP 防火墙规则来隔离游戏客户端；
// 单纯启动存储（pg_ctl）是完全不需要提权的，所以这里自己实现，
// 避免"点了启动脚本但什么也没发生"。
//
// 引擎判定不在这里重写：它走 database.EngineForConfig（与服务端同一条规则），
// 于是 GM 工具、启动器与服务端对同一份 local.json 得出同一个答案。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"dfolan/internal/database"
)

// storageConfig 是 local.json 里 GM 工具需要的那部分。
type storageConfig struct {
	Driver     string `json:"driver"`
	SQLitePath string `json:"sqlite_path"`
}

// loadStorageConfig 读 local.json 的 GM 视图。BOM 与缺失的字段一样要容忍：
// Windows 编辑器写的 BOM 曾让服务端在这个文件上直接启动失败（见 database.LoadConfig）。
func loadStorageConfig(path string) (storageConfig, error) {
	var cfg storageConfig
	raw, err := os.ReadFile(path)
	if err != nil {
		return cfg, err
	}
	raw = bytes.TrimPrefix(raw, []byte{0xEF, 0xBB, 0xBF})
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("storage config %s: %w", path, err)
	}
	return cfg, nil
}

// engine 返回这个配置选中的引擎。SQLite 是唯一引擎，所以成功时只会得到 "sqlite"；
// 档里写了别的引擎时 EngineForConfig 会给出明确报错（含"PostgreSQL 支持已移除"）。
func (c storageConfig) engine() (string, error) {
	return database.EngineForConfig(database.Config{
		Driver:     c.Driver,
		SQLitePath: c.SQLitePath,
	})
}

// startStorage 校验存储档并给出一句可读的说明，写进启动日志。
//
// SQLite 是唯一引擎：数据库是引擎自己要打开（必要时创建）的文件，没有服务可起。
// PostgreSQL 的 pg_ctl 拉起逻辑（连同 postgres_bin/postgres_data/postgres_dsn 三个
// 字段）随引擎一起移除（2026-10-05 业主口径，见根 AGENTS.md §0.6）。
func startStorage(cfg storageConfig) (string, error) {
	driver, err := cfg.engine()
	if err != nil {
		return "", err
	}
	if driver != "sqlite" {
		return "", fmt.Errorf("unsupported storage driver %q", driver)
	}
	path := strings.TrimSpace(cfg.SQLitePath)
	if path == "" {
		return "", fmt.Errorf("存储档 driver=sqlite 但缺少 sqlite_path")
	}
	return fmt.Sprintf("SQLite 档：数据库文件 %s 由服务端自己打开（必要时创建），无需启动数据库服务", path), nil
}
