// equipmentfull 从 PVF 里**直接枚举** equipment/**/*.equ 导出完整装备目录。
//
// 为什么不用 cmd/equipmentaudit：
//   它是按 `list/equipment.lst` 索引遍历的，而该索引**不含 115 级新装备**
//   （实测：115 武器 equipment/character/swordman/weapon/ssword/101001153.equ
//     文件存在，但导出结果里没有它，等级>=110 的条目只有 2 个）。
//   客户端能显示这些装备，服务端却因为查不到定义而报
//   "equipment definition missing"（表现为装备脱下来就穿不回去）。
//
// 本工具改用 Archive.Files() 枚举全部 565 万个文件条目，
// 按路径前缀/后缀筛选，因此不依赖任何索引文件是否完整。
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
)

// 与 cmd/equipmentaudit 保持一致的字段集合，便于复用同一套解析代码。
var wanted = map[string]bool{
	"[name]": true, "[grade]": true, "[rarity]": true, "[creation rate]": true,
	"[minimum level]": true, "[equipment type]": true, "[durability]": true,
	"[attach type]": true, "[usable job]": true, "[item category]": true,
}

type row struct {
	ID     uint32
	Path   string
	Fields map[string][]pvf.Token
	SHA256 string
}

func main() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner archive")
	out := flag.String("output", "configs/equipment-full.json", "output catalog")
	minLevel := flag.Int("min-level", 0, "only keep entries whose [minimum level] >= this (0 = keep all)")
	prefix := flag.String("prefix", "equipment/", "path prefix to enumerate")
	flag.Parse()

	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	files := a.Files()
	log.Printf("archive files=%d", len(files))

	counts := map[string]int{}
	var selected []row
	sampled := 0
	if len(files) > 0 {
		for i := 0; i < len(files) && i < 12; i++ {
			log.Printf("FILE[%d] path=%q name=%q archive=%q type=%d size=%d",
				i, files[i].Path, files[i].Name, files[i].ArchivePath, files[i].DataType, files[i].Size)
		}
	}
	suffixes := map[string]int{}
	for _, f := range files {
		if i := strings.LastIndex(f.ArchivePath, "."); i >= 0 {
			suffixes[strings.ToLower(f.ArchivePath[i:])]++
		} else {
			suffixes["<none>"]++
		}
	}
	log.Printf("path suffix histogram (top 15) = %v", topN(suffixes, 15))
	for _, f := range files {
		// File.Path 是目录名（不带文件名/扩展名），完整路径在 ArchivePath 里
		p := f.ArchivePath
		if !strings.HasSuffix(p, ".equ") {
			continue
		}
		if strings.Contains(p, "/avatar/") {
			counts["skip_avatar"]++
			continue
		}
		counts["candidate"]++
		// Files() 的 Path 不带 equipment/ 前缀（参考 equipmentaudit 里的补前缀逻辑），
		// 两种写法都试一下，谁解析成功用谁。
		tries := []string{p}
		if !strings.HasPrefix(p, *prefix) {
			tries = append(tries, *prefix+p)
		}
		var s catalog.ScriptRecord
		var ok bool
		for _, t := range tries {
			if got, e := catalog.ResolveScript(a, t); e == nil {
				s, ok = got, true
				break
			}
		}
		if sampled < 5 {
			log.Printf("sample path=%q name=%q archive=%q -> resolved=%v", f.Path, f.Name, f.ArchivePath, ok)
			sampled++
		}
		_ = sampled
		if !ok {
			counts["unreadable"]++
			continue
		}
		fields := map[string][]pvf.Token{}
		name := ""
		for _, t := range s.Cells {
			if t.Type == 3 {
				name = t.Text
				continue
			}
			if wanted[name] {
				fields[name] = append(fields[name], t)
			}
		}
		counts["read"]++
		if *minLevel > 0 {
			lv := fields["[minimum level]"]
			if len(lv) == 0 || lv[0].Type != 0 || int(lv[0].Value) < *minLevel {
				counts["below_min_level"]++
				continue
			}
		}
		// ID 取自文件名（101001153.equ -> 101001153）
		base := strings.TrimSuffix(path.Base(p), ".equ")
		id, e := strconv.ParseUint(base, 10, 32)
		if e != nil {
			counts["bad_name"]++
			continue
		}
		selected = append(selected, row{uint32(id), s.Path, fields, s.SHA256})
	}
	data := map[string]any{
		"source":  a.Snapshot(),
		"counts":  counts,
		"rows":    selected,
		"minLevel": *minLevel,
	}
	b, e := json.Marshal(data)
	if e != nil {
		log.Fatal(e)
	}
	if e = os.WriteFile(*out, b, 0600); e != nil {
		log.Fatal(e)
	}
	log.Printf("equipmentfull counts=%v selected=%d", counts, len(selected))
}

// topN 返回计数最大的 n 项，便于日志里快速看分布。
func topN(m map[string]int, n int) map[string]int {
	type kv struct {
		k string
		v int
	}
	var all []kv
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	for i := 0; i < len(all); i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].v > all[i].v {
				all[i], all[j] = all[j], all[i]
			}
		}
	}
	out := map[string]int{}
	for i := 0; i < len(all) && i < n; i++ {
		out[all[i].k] = all[i].v
	}
	return out
}