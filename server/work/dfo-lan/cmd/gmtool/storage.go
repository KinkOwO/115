package main

// 存储自举：GM 工具不要求管理员权限，自己把 PostgreSQL 拉起来（SQLite 档不需要）。
//
// launch_local.py 之所以要求管理员，是因为它后面要装 WFP 防火墙规则来隔离游戏客户端；
// 单纯启动存储（pg_ctl）是完全不需要提权的，所以这里自己实现，
// 避免"点了启动脚本但什么也没发生"。
//
// 引擎判定不在这里重写：它走 database.EngineForConfig（与服务端同一条规则），
// 于是 GM 工具、启动器与服务端对同一份 local.json 得出同一个答案。

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"dfolan/internal/database"
)

// storageConfig 是 local.json 里 GM 工具需要的那部分（比 internal/database 多几个路径字段）。
type storageConfig struct {
	Driver       string `json:"driver"`
	SQLitePath   string `json:"sqlite_path"`
	PostgresDSN  string `json:"postgres_dsn"`
	PostgresBin  string `json:"postgres_bin"`
	PostgresData string `json:"postgres_data"`
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

// engine 返回这个配置选中的引擎（"sqlite" / "postgres"），未知值原样返回。
func (c storageConfig) engine() (string, error) {
	return database.EngineForConfig(database.Config{
		Driver:      c.Driver,
		PostgresDSN: c.PostgresDSN,
		SQLitePath:  c.SQLitePath,
	})
}

func portOpen(host, port string) bool {
	c, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 800*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}

func dsnHostPort(dsn string) (string, string, error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return "", "", err
	}
	host := u.Hostname()
	port := u.Port()
	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		port = "5432"
	}
	return host, port, nil
}

// startStorage 在存储没起来时按 local.json 里的路径把它拉起来，并等待端口可用。
// 返回一段可读的说明，写进启动日志。
//
// SQLite 档直接返回说明：数据库是引擎自己要打开（必要时创建）的文件，没有服务可起。
// 此前这里无条件按 PostgreSQL 处理，SQLite 档会先报一句「找不到 pg_ctl.exe」的
// 误导性警告（2026-10-05 双库兼容排查）。
func startStorage(cfg storageConfig, storageDir string) (string, error) {
	driver, err := cfg.engine()
	if err != nil {
		return "", err
	}
	if driver == "sqlite" {
		path := strings.TrimSpace(cfg.SQLitePath)
		if path == "" {
			return "", fmt.Errorf("存储档 driver=sqlite 但缺少 sqlite_path")
		}
		return fmt.Sprintf("SQLite 档：数据库文件 %s 由服务端自己打开，无需启动 PostgreSQL", path), nil
	}
	if driver != "postgres" {
		return "", fmt.Errorf("unsupported storage driver %q", driver)
	}
	if strings.TrimSpace(cfg.PostgresDSN) == "" {
		return "", fmt.Errorf("存储档 driver=postgres 但缺少 postgres_dsn")
	}

	var notes []string
	host, port, err := dsnHostPort(cfg.PostgresDSN)
	if err != nil {
		return "", fmt.Errorf("postgres_dsn 无法解析：%w", err)
	}
	if !portOpen(host, port) {
		pgctl := filepath.Join(cfg.PostgresBin, "pg_ctl.exe")
		data := cfg.PostgresData
		if _, err := os.Stat(pgctl); err != nil {
			return "", fmt.Errorf("PostgreSQL 没在跑，并且找不到 %s", pgctl)
		}
		if _, err := os.Stat(filepath.Join(data, "PG_VERSION")); err != nil {
			return "", fmt.Errorf("PostgreSQL 没在跑，并且数据目录无效：%s", data)
		}
		logPath := filepath.Join(storageDir, "postgres.log")
		cmd := exec.Command(pgctl, "-D", data, "-l", logPath, "-w", "-t", "60", "start")
		cmd.Dir = storageDir
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("启动 PostgreSQL 失败：%v\n%s", err, strings.TrimSpace(string(out)))
		}
		notes = append(notes, "已自动启动 PostgreSQL")
	}
	if !waitPort(host, port, 40*time.Second) {
		return "", fmt.Errorf("PostgreSQL 端口 %s 仍未就绪，请看 %s", port, filepath.Join(storageDir, "postgres.log"))
	}

	return strings.Join(notes, "；"), nil
}

func waitPort(host, port string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if portOpen(host, port) {
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}
