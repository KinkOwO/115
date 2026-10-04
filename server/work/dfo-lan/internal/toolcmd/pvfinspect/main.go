// pvfinspect reads source data and writes an index or one requested text entry.
package pvfinspect

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func Run() {
	source := flag.String("source", "", "source PVF, opened read-only")
	out := flag.String("output", "runtime/pvf", "output directory")
	find := flag.String("find", "character/", "case-insensitive path substring")
	prefix := flag.Bool("prefix", false, "match only paths starting with find")
	suffix := flag.String("suffix", "", "optional case-insensitive archive path suffix")
	file := flag.String("file", "", "exact entry to read")
	files := flag.String("files", "", "comma separated exact entries, exported with raw bytes and hashes")
	tokens := flag.Bool("tokens", false, "export all typed cells for each requested script, including unknown types")
	flag.Parse()
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatal(err)
	}
	if err = os.MkdirAll(*out, 0700); err != nil {
		log.Fatal(err)
	}
	b, _ := json.MarshalIndent(a.Snapshot(), "", "  ")
	if err = os.WriteFile(filepath.Join(*out, "archive.json"), b, 0600); err != nil {
		log.Fatal(err)
	}
	var matches []pvf.File
	for _, f := range a.Files() {
		match := strings.Contains(strings.ToLower(f.ArchivePath), strings.ToLower(*find))
		if *prefix {
			match = strings.HasPrefix(strings.ToLower(f.ArchivePath), strings.ToLower(*find))
		}
		if match && strings.HasSuffix(strings.ToLower(f.ArchivePath), strings.ToLower(*suffix)) {
			matches = append(matches, f)
		}
	}
	b, _ = json.MarshalIndent(matches, "", "  ")
	if err = os.WriteFile(filepath.Join(*out, "matches.json"), b, 0600); err != nil {
		log.Fatal(err)
	}
	if *file != "" {
		s, err := a.ReadText(*file)
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(*out, "entry.txt"), []byte(s), 0600); err != nil {
			log.Fatal(err)
		}
	}
	if *files != "" {
		var manifest []map[string]any
		for i, path := range strings.Split(*files, ",") {
			entry, ok := a.FindFile(path)
			if !ok {
				manifest = append(manifest, map[string]any{"path": path, "error": "not found"})
				continue
			}
			raw, err := a.ReadRaw(path)
			if err != nil {
				log.Fatal(err)
			}
			txt, err := a.ReadText(path)
			if err != nil {
				log.Fatal(err)
			}
			base := fmt.Sprintf("%02d-%s", i, filepath.Base(path))
			if *tokens && entry.DataType == 1 {
				cells, tokenErr := a.Tokens(path)
				if tokenErr != nil {
					log.Fatal(tokenErr)
				}
				encoded, tokenErr := json.MarshalIndent(cells, "", "  ")
				if tokenErr != nil {
					log.Fatal(tokenErr)
				}
				if tokenErr = os.WriteFile(filepath.Join(*out, base+".tokens.json"), encoded, 0600); tokenErr != nil {
					log.Fatal(tokenErr)
				}
			}
			if err = os.WriteFile(filepath.Join(*out, base+".bin"), raw, 0600); err != nil {
				log.Fatal(err)
			}
			if err = os.WriteFile(filepath.Join(*out, base+".txt"), []byte(txt), 0600); err != nil {
				log.Fatal(err)
			}
			manifest = append(manifest, map[string]any{"entry": entry, "raw_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "text_file": base + ".txt", "raw_file": base + ".bin"})
		}
		b, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(*out, "exports.json"), b, 0600); err != nil {
			log.Fatal(err)
		}
	}
	log.Printf("archive files=%d matches=%d output=%s", a.FileCount(), len(matches), *out)
}
