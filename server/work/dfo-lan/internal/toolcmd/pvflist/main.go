// Temporary helper: dump full file listing (TSV) + summary from the inner PVF.
package pvflist

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"dfolan/internal/catalog/pvf"
)

type dirStat struct {
	Count int   `json:"count"`
	Bytes int64 `json:"bytes"`
}

func Run() {
	source := flag.String("source", "", "inner PVF")
	out := flag.String("out", "", "output directory")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 2 * 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(*out, 0700); err != nil {
		log.Fatal(err)
	}
	tsv, err := os.Create(filepath.Join(*out, "filelist.tsv"))
	if err != nil {
		log.Fatal(err)
	}
	defer tsv.Close()
	w := bufio.NewWriterSize(tsv, 1<<20)

	dirs := map[string]*dirStat{}
	exts := map[string]int{}
	types := map[int]int{}
	var total int64
	for _, f := range a.Files() {
		fmt.Fprintf(w, "%d\t%d\t%d\t%s\n", f.Index, f.DataType, f.Size, f.ArchivePath)
		total += int64(f.Size)
		top := f.ArchivePath
		if i := strings.IndexByte(top, '/'); i >= 0 {
			top = top[:i]
		}
		d := dirs[top]
		if d == nil {
			d = &dirStat{}
			dirs[top] = d
		}
		d.Count++
		d.Bytes += int64(f.Size)
		if i := strings.LastIndexByte(f.ArchivePath, '.'); i >= 0 {
			exts[f.ArchivePath[i:]]++
		} else {
			exts["(none)"]++
		}
		types[f.DataType]++
	}
	if err := w.Flush(); err != nil {
		log.Fatal(err)
	}

	ordered := make([]string, 0, len(dirs))
	for k := range dirs {
		ordered = append(ordered, k)
	}
	sort.Slice(ordered, func(i, j int) bool { return dirs[ordered[i]].Count > dirs[ordered[j]].Count })

	summary := map[string]any{
		"format":      a.Format(),
		"file_count":  a.FileCount(),
		"total_bytes": total,
		"dirs_top":    40,
		"top_dirs":    nil,
		"ext_top":     nil,
		"data_types":  types,
	}
	type kv struct {
		Key   string `json:"key"`
		Count int    `json:"count"`
		Bytes int64  `json:"bytes"`
	}
	td := make([]kv, 0, len(ordered))
	for i, k := range ordered {
		if i >= 40 {
			break
		}
		td = append(td, kv{Key: k, Count: dirs[k].Count, Bytes: dirs[k].Bytes})
	}
	summary["top_dirs"] = td

	extOrdered := make([]string, 0, len(exts))
	for k := range exts {
		extOrdered = append(extOrdered, k)
	}
	sort.Slice(extOrdered, func(i, j int) bool { return exts[extOrdered[i]] > exts[extOrdered[j]] })
	te := make([]kv, 0, len(extOrdered))
	for i, k := range extOrdered {
		if i >= 20 {
			break
		}
		te = append(te, kv{Key: k, Count: exts[k]})
	}
	summary["ext_top"] = te

	b, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "summary.json"), b, 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("files=%d total_bytes=%d tsv=%s", a.FileCount(), total, filepath.Join(*out, "filelist.tsv"))
}
