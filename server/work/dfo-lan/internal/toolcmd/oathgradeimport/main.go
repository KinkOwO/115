// oathgradeimport 从内层 PVF 导出「誓约（oath）／引子（primer）」装备的稀有度表，
// 供服务端把角色实际穿戴的装备换算成 noti 2838 要下发的机制档位。
//
// 为什么要这张表：客户端 `getPrimerGrade()` / `getOathGrade()` 读的是单例
// `qword_14E6388B8` 的 +88/+92，而该单例只由 noti 2838 写入（构造器里是 72 兜底）。
// 也就是说**档位完全由服务端决定**。恒发 45 会让每场都满足
// `oath_now == 44 -> summon_orthaire`，隐藏 BOSS 场场登场 —— 而它代表必出太初。
// 档位的真正来源是玩家的誓约装备稀有度，见
// docs/protocol/endkeeper-of-order-primer-20260926.md §10.1/§10.2。
//
// 用法：
//
//	go run ./cmd/dfo-tool oathgradeimport -source <Script.inner.pvf> -output configs/oath-grades.json
package oathgradeimport

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"

	"dfolan/internal/catalog/pvf"
)

// families 是两类装备的路径标记与其在输出里的名字。
var families = []struct {
	dir  string // 归档路径里的目录片段
	name string // 输出里的 family 名
}{
	{"equipment/character/common/oath/", "oath"},
	{"equipment/character/common/primer/", "primer"},
}

// Entry 是一件誓约/引子装备的原始事实。
type Entry struct {
	Family string `json:"family"`
	Rarity int32  `json:"rarity"`
	Grade  int32  `json:"grade,omitempty"`
	// Type 是脚本里 [equipment type] 的原文，用来交叉验证目录分类。
	Type string `json:"type"`
	Path string `json:"path"`
}

type doc struct {
	Source  pvf.ArchiveSnapshot `json:"source"`
	Entries map[string]Entry    `json:"entries"`
}

func Run() {
	source := flag.String("source", "D:/115us/server/work/client-build/Script.inner.pvf", "inner archive")
	output := flag.String("output", "configs/oath-grades.json", "output json")
	flag.Parse()

	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 2 << 30})
	if err != nil {
		log.Fatal(err)
	}

	out := doc{Source: a.Snapshot(), Entries: map[string]Entry{}}
	stats := map[string]int{}
	for _, f := range a.Files() {
		p := strings.ToLower(f.ArchivePath)
		fam := ""
		for _, m := range families {
			if strings.HasPrefix(p, m.dir) {
				fam = m.name
				break
			}
		}
		if fam == "" {
			continue
		}
		if strings.ToLower(path.Ext(f.ArchivePath)) != ".equ" {
			continue
		}
		tokens, err := a.Tokens(f.ArchivePath)
		if err != nil {
			log.Fatalf("%s: %v", f.ArchivePath, err)
		}
		e := Entry{Family: fam, Rarity: -1, Path: f.ArchivePath}
		for i, t := range tokens {
			if i+1 >= len(tokens) {
				break
			}
			switch t.Text {
			case "[rarity]":
				e.Rarity = tokens[i+1].Value
			case "[grade]":
				e.Grade = tokens[i+1].Value
			case "[equipment type]":
				e.Type = tokens[i+1].Text
			}
		}
		if e.Rarity < 0 {
			log.Fatalf("%s: no [rarity]", f.ArchivePath)
		}
		// 目录分类必须与脚本自报的 [equipment type] 一致，否则表是错的。
		want := "[" + fam + "]"
		if e.Type != want {
			log.Fatalf("%s: [equipment type] is %q, want %q", f.ArchivePath, e.Type, want)
		}
		id := path.Base(f.ArchivePath)
		id = strings.TrimSuffix(id, path.Ext(id))
		if _, err := strconv.ParseUint(id, 10, 32); err != nil {
			log.Fatalf("%s: file name is not a decimal item id", f.ArchivePath)
		}
		if _, dup := out.Entries[id]; dup {
			log.Fatalf("duplicate id %s", id)
		}
		out.Entries[id] = e
		stats[fmt.Sprintf("%s rarity=%d", fam, e.Rarity)]++
	}

	if len(out.Entries) == 0 {
		log.Fatal("no oath/primer equipment found; wrong -source?")
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %-4d %s\n", stats[k], k)
	}

	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(*output, b, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("OATH_GRADES_WRITTEN entries=%d output=%s source_checksum=%s\n",
		len(out.Entries), *output, out.Source.Checksum)
}
