// Package accountlist 打印存储里各账号的只读概览：点券 + 每个角色的 id / 名字 /
// 等级 / 经验 / 金币。
//
// 为什么要有这个工具：GM 命令行（scripts/GM.cmd + scripts/gm.py）此前直接用 Python
// 的 sqlite3 读存档，于是**只认 SQLite 档**，PostgreSQL 档只能去开 Web GM。读取本身
// 与引擎无关，所以这里走 database.Open 的引擎缝：同一份 local.json，SQLite 与
// PostgreSQL 给出同一份结论；写操作仍走 cmd/admin / dfo-tool setlevel（本来就引擎中立）。
//
// 只读：不建表、不迁移、不写审计。等级/经验取自角色 state；金币用
// inventory.ReadBag（写路径的权威读取器），旧存档读不动时该角色金币报 null 并在
// 文本输出里说明原因，而不是把「读不出来」显示成 0。
package accountlist

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"text/tabwriter"
	"time"
)

// characterRow 是一个角色的概览行。Gold 为 nil 表示读不出来（旧存档或形状不符），
// 与「金币是 0」区分开。
type characterRow struct {
	ID         int64   `json:"id"`
	WireID     uint16  `json:"wire_id"`
	Name       string  `json:"name"`
	Level      byte    `json:"level"`
	Experience uint64  `json:"experience"`
	Gold       *uint64 `json:"gold"`
}

type accountRow struct {
	ID         int64          `json:"id"`
	Username   string         `json:"username"`
	Cera       uint64         `json:"cera"`
	Characters []characterRow `json:"characters"`
}

type report struct {
	Driver   string       `json:"driver"`
	Target   string       `json:"target"`
	Config   string       `json:"config"`
	Accounts []accountRow `json:"accounts"`
}

func Run() {
	config := flag.String("config", "runtime/storage/local.json", "local storage configuration")
	accountName := flag.String("account", "", "only this account (default: every account)")
	jsonOut := flag.Bool("json", false, "machine-readable JSON (used by scripts/gm.py)")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	cfg, err := database.LoadConfig(*config)
	if err != nil {
		log.Fatal(err)
	}
	store, err := database.Open(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	driver, err := database.EngineForConfig(cfg)
	if err != nil {
		log.Fatal(err)
	}
	target := "(unknown)"
	if name, nameErr := store.DatabaseName(ctx); nameErr == nil {
		target = name
	}

	accounts, err := store.Accounts(ctx)
	if err != nil {
		log.Fatal(err)
	}
	out := report{Driver: driver, Target: target, Config: *config, Accounts: []accountRow{}}
	for _, account := range accounts {
		if *accountName != "" && account.Username != *accountName {
			continue
		}
		cera, err := store.AccountCera(ctx, account.ID)
		if err != nil {
			log.Fatal(err)
		}
		roles, err := store.AdminCharacters(ctx, account.ID)
		if err != nil {
			log.Fatal(err)
		}
		row := accountRow{ID: account.ID, Username: account.Username, Cera: cera, Characters: []characterRow{}}
		for _, role := range roles {
			row.Characters = append(row.Characters, describe(role))
		}
		out.Accounts = append(out.Accounts, row)
	}
	if *accountName != "" && len(out.Accounts) == 0 {
		fmt.Fprintf(os.Stderr, "accountlist: no account named %s in %s\n", *accountName, target)
		os.Exit(2)
	}

	if *jsonOut {
		encoded, err := json.Marshal(out)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println(string(encoded))
		return
	}
	printText(out)
}

// describe 把一个角色存档投影成概览行。等级/经验只读 state 的两个键；金币优先用
// inventory.ReadBag（校验过的读取器），失败则该行金币为 null。
func describe(role database.Character) characterRow {
	row := characterRow{ID: role.ID, WireID: role.WireID, Name: role.Name}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(role.State, &fields); err == nil {
		_ = decodeNumeric(fields["level"], &row.Level)
		_ = decodeNumeric(fields["experience"], &row.Experience)
	}
	if bag, err := inventory.ReadBag(role.State); err == nil {
		gold := uint64(bag.Gold)
		row.Gold = &gold
	}
	return row
}

// decodeNumeric 兼容 JSON 数字与历史字符串写法；读不动就保持零值。
func decodeNumeric(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err == nil {
		return nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return err
	}
	return json.Unmarshal([]byte(text), out)
}

func printText(out report) {
	fmt.Printf("存储: %s  %s\n", out.Driver, out.Target)
	fmt.Printf("配置: %s\n", out.Config)
	if len(out.Accounts) == 0 {
		fmt.Println("（没有账号）")
		return
	}
	for _, account := range out.Accounts {
		fmt.Printf("\n账号 %s (id=%d)   点券: %d\n", account.Username, account.ID, account.Cera)
		writer := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(writer, "  id\t名字\t等级\t经验\t金币")
		for _, role := range account.Characters {
			gold := "-"
			if role.Gold != nil {
				gold = fmt.Sprintf("%d", *role.Gold)
			}
			fmt.Fprintf(writer, "  %d\t%s\t%d\t%d\t%s\n", role.ID, role.Name, role.Level, role.Experience, gold)
		}
		_ = writer.Flush()
	}
}
