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
package equipmentfull

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

// normPath 归一化路径用于查表：小写、正斜杠、去首尾空白。
func normPath(p string) string {
	return strings.ToLower(strings.ReplaceAll(strings.TrimSpace(p), "\\", "/"))
}

// equipmentIDByPath 读 list/equipment.lst，返回归一化 path -> template id。
//
// 为什么不用文件名：早期装备的文件名不是纯数字（`vest_owool.equ`、`robe_cfiber.equ`…），
// 从文件名解析 ID 会把它们整条丢掉（作者侧实测 **43,349 条**），而它们的 ID 就在这份索引里。
func equipmentIDByPath(a *pvf.Archive) (map[string]uint32, error) {
	rec, err := catalog.ResolveScript(a, "list/equipment.lst")
	if err != nil {
		return nil, err
	}
	ents, err := catalog.ParseIndex(rec.Cells)
	if err != nil {
		return nil, err
	}
	out := make(map[string]uint32, len(ents))
	for _, e := range ents {
		out[normPath(e.Path)] = e.ID
	}
	return out, nil
}

func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner archive")
	out := flag.String("output", "", "required diagnostic output")
	minLevel := flag.Int("min-level", 0, "only keep entries whose [minimum level] >= this (0 = keep all)")
	prefix := flag.String("prefix", "equipment/", "path prefix to enumerate")
	flag.Parse()
	if *out == "" {
		log.Fatal("explicit -output is required for diagnostic export")
	}

	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	// list/equipment.lst 是**客户端自己的装备索引**，ID 的唯一权威来源。
	byPath, idxErr := equipmentIDByPath(a)
	if idxErr != nil {
		log.Fatalf("读 list/equipment.lst：%v", idxErr)
	}
	log.Printf("list/equipment.lst: %d 条 path->id", len(byPath))

	files := a.Files()
	log.Printf("archive files=%d", len(files))

	counts := map[string]int{}
	var selected []row
	// byID 记录已经收下的行，用于同 ID 去重（见下方规则）。
	type seenRow struct {
		path      string
		fromIndex bool
	}
	byID := map[uint32]seenRow{}
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
		// 时装判定用「路径含 avatar」而不是 "/avatar/"：还有 `at_avatar` 这种变体
		// （equipment/character/fighter/at_avatar/...），只匹配前者会漏 10 万条。
		if strings.Contains(strings.ToLower(p), "avatar") {
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
		// ID 优先取自 list/equipment.lst（权威）；文件名只在索引查不到时兜底。
		// 这样 vest_owool.equ 这类非数字文件名也能拿到正确 ID 而不是被丢弃。
		var id uint32
		fromIndex := false
		for _, cand := range tries {
			if v, ok := byPath[normPath(cand)]; ok {
				id, fromIndex = v, true
				break
			}
		}
		if !fromIndex {
			base := strings.TrimSuffix(path.Base(p), ".equ")
			v, pe := strconv.ParseUint(base, 10, 32)
			if pe != nil {
				counts["bad_name"]++
				continue
			}
			id = uint32(v)
			counts["id_from_filename"]++
		} else {
			counts["id_from_index"]++
		}
		// 同一 ID 可能有多条 `.equ`（实测 100990965 同时存在于 equipment/creature/ 与
		// equipment/creature/arcadecenter/）。服务端按 index[ID] 存，重复会让「哪条胜出」
		// 取决于遍历顺序 —— 不确定行为。规则：**list/equipment.lst 认可的那条优先**；
		// 两条都被认可或都不被认可时取路径字典序较小者，保证结果可复现。
		if prev, dup := byID[id]; dup {
			counts["dup_id"]++
			keepNew := false
			switch {
			case fromIndex && !prev.fromIndex:
				keepNew = true
			case fromIndex == prev.fromIndex && s.Path < prev.path:
				keepNew = true
			}
			if !keepNew {
				continue
			}
		}
		byID[id] = seenRow{path: s.Path, fromIndex: fromIndex}
		selected = append(selected, row{id, s.Path, fields, s.SHA256})
	}
	// 二次去重：上面的 byID 只能「跳过后来的」，早收进去的落选行仍在 selected 里，
	// 所以按 ID 再筛一遍，保留 byID 记录的那条。
	{
		final := selected[:0]
		seen := map[uint32]bool{}
		for _, r := range selected {
			keep := byID[r.ID]
			if keep.path != r.Path || seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			final = append(final, r)
		}
		selected = final
	}
	data := map[string]any{
		"source":   a.Snapshot(),
		"counts":   counts,
		"rows":     selected,
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
