package main

// 存储自举：GM 工具不要求管理员权限，自己把 PostgreSQL / Redis 拉起来。
//
// launch_local.py 之所以要求管理员，是因为它后面要装 WFP 防火墙规则来隔离游戏客户端；
// 单纯启动存储（pg_ctl / redis-server）是完全不需要提权的，所以这里自己实现，
// 避免"点了启动脚本但什么也没发生"。

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// storageConfig 是 local.json 里 GM 工具需要的那部分（比 internal/storage 多几个路径字段）。
type storageConfig struct {
	PostgresDSN  string `json:"postgres_dsn"`
	RedisAddress string `json:"redis_address"`
	PostgresBin  string `json:"postgres_bin"`
	PostgresData string `json:"postgres_data"`
	RedisBin     string `json:"redis_bin"`
}

func portOpen(host string, port string) bool {
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
func startStorage(cfg storageConfig, storageDir string) (string, error) {
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

	rhost := "127.0.0.1"
	rport := "6379"
	if i := strings.LastIndex(cfg.RedisAddress, ":"); i > 0 {
		rhost, rport = cfg.RedisAddress[:i], cfg.RedisAddress[i+1:]
	}
	if !portOpen(rhost, rport) {
		redis := filepath.Join(cfg.RedisBin, "redis-server.exe")
		conf := filepath.Join(storageDir, "redis.conf")
		if _, err := os.Stat(redis); err != nil {
			return "", fmt.Errorf("Redis 没在跑，并且找不到 %s", redis)
		}
		if _, err := os.Stat(conf); err != nil {
			return "", fmt.Errorf("Redis 没在跑，并且找不到 %s", conf)
		}
		logf, _ := os.OpenFile(filepath.Join(storageDir, "redis.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		cmd := exec.Command(redis, "redis.conf")
		cmd.Dir = storageDir
		cmd.Stdout, cmd.Stderr = logf, logf
		if err := cmd.Start(); err != nil {
			return "", fmt.Errorf("启动 Redis 失败：%v", err)
		}
		notes = append(notes, "已自动启动 Redis")
		// 让子进程脱离本进程的生命周期
		go func() {
			_ = cmd.Wait()
			if logf != nil {
				_ = logf.Close()
			}
		}()
	}
	if !waitPort(rhost, rport, 20*time.Second) {
		return "", fmt.Errorf("Redis 端口 %s 仍未就绪，请看 %s", rport, filepath.Join(storageDir, "redis.log"))
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
