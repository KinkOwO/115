package main

// init-storage 子命令：首次初始化本地存储（数据库自举）。
//
// 它取代 scripts/bootstrap_local.py（业主 2026-10-05 决策：最小预装环境、没有 python、
// 不留回退路线）。语义与落点见 internal/launcher/initstorage.go；这一层只负责参数、
// 输出分流与退出码：
//
//   - 进度写 **stderr**，stdout 只留一行结果 JSON —— 与 bootstrap_local.py 的 stdout 契约
//     一致（相邻启动器 115us-dfolauncher/internal/storeboot 按行解析它）；
//   - --dry-run 把步骤清单打到 stdout（它本身就是这次运行的产物），并且不落盘、不起进程；
//   - 退出码：参数错 2、初始化失败 1、成功 0。

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"dfolan/internal/launcher"
)

func runInitStorage(args []string) int {
	flags := flag.NewFlagSet("init-storage", flag.ContinueOnError)
	root := flags.String("root", ".", "repository root")
	postgresBin := flags.String("postgres-bin", "", "PostgreSQL bin directory (default: the bundled portable runtime)")
	postgresPort := flags.Int("postgres-port", launcher.InitStorageDefaultPort, "database port")
	dryRun := flags.Bool("dry-run", false, "print the actions without performing them")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	absolute, err := filepathAbs(*root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve root: %v\n", err)
		return 1
	}

	// dry-run 的步骤清单就是这个子命令的产物，写到 stdout；真正的初始化把进度写到 stderr，
	// 让 stdout 保持"一行 JSON"的机器契约。
	progress := os.Stderr
	if *dryRun {
		progress = os.Stdout
	}
	logf := func(format string, args ...any) { fmt.Fprintf(progress, format+"\n", args...) }

	result, err := launcher.InitStorage(context.Background(), launcher.InitStorageOptions{
		Root:         absolute,
		PostgresBin:  *postgresBin,
		PostgresPort: *postgresPort,
		DryRun:       *dryRun,
	}, logf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init-storage: %v\n", err)
		return 1
	}
	if *dryRun {
		fmt.Printf("Dry run: would initialize %s on port %d; nothing was written.\n",
			result.ConfigPath, result.PostgresPort)
		return 0
	}

	body, err := json.Marshal(result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init-storage: 序列化结果失败：%v\n", err)
		return 1
	}
	fmt.Println(string(body))
	return 0
}
