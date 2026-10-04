// dbq: 临时只读查询器——从 runtime/storage/local.json 读 DSN，执行 -c 的 SQL 并打印。
package dbq

import (
	"context"
	"flag"
	"fmt"
	"time"

	"dfolan/internal/database"
)

func Run() {
	local := flag.String("local", "runtime/storage/local.json", "storage local.json")
	sql := flag.String("c", "", "sql to run")
	flag.Parse()
	cfg, err := database.LoadConfig(*local)
	if err != nil {
		panic(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := database.Open(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer s.Close()
	err = s.DiagnosticQuery(ctx, *sql, func(fields []string, vals []any) error {
		line := ""
		for i, v := range vals {
			line += fmt.Sprintf("%s=%v | ", fields[i], v)
		}
		fmt.Println(line)
		return nil
	})
	if err != nil {
		panic(err)
	}
	fmt.Println("done")
}
