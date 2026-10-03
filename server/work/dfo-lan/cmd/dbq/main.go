// dbq: 临时只读查询器——从 runtime/storage/local.json 读 DSN，执行 -c 的 SQL 并打印。
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
)

func main() {
	local := flag.String("local", "runtime/storage/local.json", "storage local.json")
	sql := flag.String("c", "", "sql to run")
	flag.Parse()
	cfg := map[string]interface{}{}
	b, err := os.ReadFile(*local)
	if err != nil {
		panic(err)
	}
	if err = json.Unmarshal(b, &cfg); err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := pgx.Connect(ctx, fmt.Sprint(cfg["postgres_dsn"]))
	if err != nil {
		panic(err)
	}
	defer conn.Close(ctx)
	rows, err := conn.Query(ctx, *sql)
	if err != nil {
		panic(err)
	}
	defer rows.Close()
	fields := rows.FieldDescriptions()
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			panic(err)
		}
		line := ""
		for i, v := range vals {
			line += fmt.Sprintf("%s=%v | ", fields[i].Name, v)
		}
		fmt.Println(line)
	}
	fmt.Println("done")
}
