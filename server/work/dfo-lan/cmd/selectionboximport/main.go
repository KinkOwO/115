// selectionboximport exports the source selection boxes ([booster selection]
// boxes carrying [booster select category] blocks) into a JSON catalog.
//
// It is the read-only sibling of cmd/boosterexport: that tool models the fixed
// [booster info] boxes, while this one models the boxes whose contents the
// player picks from. The two are disjoint in the source, and the item index
// mislabels a couple of fixed boxes as [booster selection] — those are reported
// as "fixed" instead of being dropped or panicking.
//
// By default the export is bounded to the boxes the server's own configuration
// can actually hand out (references found in the booster catalog, the monster
// drop catalog and the other configs/*.json files), which keeps the artifact at
// ~13 MB instead of the ~78 MB a full dump of every indexed template costs. The
// bound is only an artifact-size decision: it never changes how a box is parsed.
//
//	go run ./cmd/selectionboximport -source ../client-build/Script.inner.pvf \
//	    -index configs/items.index.json -output configs/selection-boxes-candidate.json
//
//	go run ./cmd/selectionboximport -bounded=false ...      # every indexed box
//	go run ./cmd/selectionboximport -debug 10417789,10307659
package main

import (
	"bytes"
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type itemIndexEntry struct {
	ID            uint32 `json:"id"`
	Path          string `json:"path"`
	Kind          string `json:"kind"`
	StackableType string `json:"stackable_type"`
}

type itemIndex struct {
	Source pvf.ArchiveSnapshot       `json:"source"`
	Items  map[string]itemIndexEntry `json:"items"`
}

// SelectionItem is one entry of a box's [equipment] list: the source writes
// (id, count) pairs, and the client hands the picked id back in its request.
type SelectionItem struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// SelectionCategory is one [booster select category] block. Category is the
// (job, growtype) pair the client sends with the request; Items is the closed
// set the server validates that pick against, and Sections records which
// content blocks the source carried (only [equipment] is modelled so far — the
// [avatar]/[etc] blocks still go down the generic destination path).
type SelectionCategory struct {
	Category  [2]byte         `json:"category"`
	Grade     uint32          `json:"grade,omitempty"`
	Recommend []uint32        `json:"recommend,omitempty"`
	Items     []SelectionItem `json:"items"`
	Sections  []string        `json:"sections,omitempty"`
}

type selectionBox struct {
	Template   uint32              `json:"template"`
	Path       string              `json:"path"`
	SHA256     string              `json:"sha256"`
	Categories []SelectionCategory `json:"categories"`
}

type document struct {
	Model    string                  `json:"model"`
	Bounded  bool                    `json:"bounded"`
	Source   pvf.ArchiveSnapshot     `json:"source"`
	Boxes    map[string]selectionBox `json:"boxes"`
	Fixed    []uint32                `json:"fixed"`
	Unparsed []uint32                `json:"unparsed,omitempty"`
}

const selectionBoxModel = "source-selection-boxes-v1"

// scanLimit skips the multi-hundred-MB exports (equipment-full, dungeons.full,
// shop-vault, skills) while scanning configs for template references. Those
// files describe equipment/skills, not boxes, so nothing is lost.
const scanLimit = 8 << 20

// parseSelection walks one script's typed cells. It returns the box's category
// blocks and whether the script carries a fixed [booster info] block, which is
// how a mislabeled fixed box is recognised.
func parseSelection(cells []pvf.Token) ([]SelectionCategory, bool) {
	var out []SelectionCategory
	fixed := false
	for i := 0; i < len(cells); i++ {
		if cells[i].Type != 3 {
			continue
		}
		switch cells[i].Text {
		case "[booster info]":
			fixed = true
		case "[booster select category]":
			cat, next, ok := parseCategory(cells, i+1)
			if ok {
				out = append(out, cat)
			}
			i = next
		}
	}
	return out, fixed
}

func parseCategory(cells []pvf.Token, i int) (SelectionCategory, int, bool) {
	var cat SelectionCategory
	nums := make([]int32, 0, 2)
	for i < len(cells) && len(nums) < 2 {
		if cells[i].Type == 3 {
			return cat, i, false
		}
		if cells[i].Type == 0 {
			nums = append(nums, cells[i].Value)
		}
		i++
	}
	if len(nums) != 2 {
		return cat, i, false
	}
	cat.Category = [2]byte{byte(nums[0]), byte(nums[1])}
	for i < len(cells) {
		c := cells[i]
		if c.Type != 3 {
			i++
			continue
		}
		if c.Text == "[/booster select category]" {
			return cat, i, true
		}
		switch c.Text {
		case "[booster equipment grade]":
			i++
			if i < len(cells) && cells[i].Type == 0 {
				cat.Grade = uint32(cells[i].Value)
				i++
			}
		case "[recommend]":
			i++
			if i < len(cells) && cells[i].Type == 0 {
				n := int(cells[i].Value)
				i++
				for k := 0; k < n && i < len(cells) && cells[i].Type == 0; k++ {
					cat.Recommend = append(cat.Recommend, uint32(cells[i].Value))
					i++
				}
			}
		case "[equipment]":
			cat.Sections = append(cat.Sections, c.Text)
			i++
			for i < len(cells) && !(cells[i].Type == 3 && cells[i].Text == "[/equipment]") {
				if cells[i].Type != 0 {
					i++
					continue
				}
				id := uint32(cells[i].Value)
				i++
				count := uint32(1)
				if i < len(cells) && cells[i].Type == 0 {
					count = uint32(cells[i].Value)
					i++
				}
				cat.Items = append(cat.Items, SelectionItem{Template: id, Count: count})
			}
		case "[avatar]", "[creature]", "[etc]", "[stackable]", "[cera]":
			// 尚未建模的内容段：只记名并跳过。Resolve 见到这些类别时不校验
			// （它们的条目结构各不相同，先把装备段做对再说）。
			cat.Sections = append(cat.Sections, c.Text)
			end := "[/" + c.Text[1:]
			i++
			for i < len(cells) && !(cells[i].Type == 3 && cells[i].Text == end) {
				i++
			}
		default:
			i++
		}
	}
	return cat, i, false
}

// collectTemplateIDs walks a decoded JSON document and records every integer
// that names a template. Booleans and floats are ignored: template ids are
// always written as integers in these configs.
func collectTemplateIDs(v any, out map[uint32]bool) {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			collectTemplateIDs(k, out)
			collectTemplateIDs(val, out)
		}
	case []any:
		for _, val := range t {
			collectTemplateIDs(val, out)
		}
	case string:
		if n, err := strconv.ParseUint(strings.TrimSpace(t), 10, 32); err == nil {
			out[uint32(n)] = true
		}
	case json.Number:
		if n, err := strconv.ParseUint(t.String(), 10, 64); err == nil && n <= 0xffffffff {
			out[uint32(n)] = true
		}
	}
}

// templateLiteral matches ids spelled out in Go source. Several boxes are handed
// out by code rather than by a config file — the Odyssey creation flow grants
// 10417789/10417790 directly — so "referenced by the server" has to include the
// source tree. The scan is intersected with the item index afterwards, so the
// unrelated numbers every program is full of cannot leak in.
var templateLiteral = regexp.MustCompile(`\b[1-9][0-9]{6,9}\b`)

// scanSourceTemplates collects the integers written in the Go sources under
// root, skipping generated/runtime trees.
func scanSourceTemplates(root string) (map[uint32]bool, error) {
	out := map[uint32]bool{}
	if root == "" {
		return out, nil
	}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", "runtime", "bin", "vendor", "node_modules", "update-backup", "pgdata", "docs":
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(entry.Name(), ".go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		for _, m := range templateLiteral.FindAllString(string(raw), -1) {
			if n, err := strconv.ParseUint(m, 10, 32); err == nil {
				out[uint32(n)] = true
			}
		}
		return nil
	})
	return out, err
}

func decodeIDs(path string) (map[uint32]bool, error) {
	out := map[uint32]bool{}
	raw, err := os.ReadFile(path)
	if err != nil {
		return out, err
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var doc any
	if err := dec.Decode(&doc); err != nil {
		return out, err
	}
	collectTemplateIDs(doc, out)
	return out, nil
}

// boundedTemplates collects the templates the server's own configuration can
// reference: the fixed-content booster catalog, the monster drop catalog and
// every reasonably sized configs/*.json.
func boundedTemplates(catalogs []string, configDir string) (map[uint32]bool, error) {
	out := map[uint32]bool{}
	for _, p := range catalogs {
		ids, err := decodeIDs(p)
		if err != nil {
			return nil, fmt.Errorf("reference %s: %w", p, err)
		}
		for id := range ids {
			out[id] = true
		}
	}
	entries, err := os.ReadDir(configDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(configDir, e.Name())
		info, err := e.Info()
		if err != nil || info.Size() > scanLimit {
			continue
		}
		ids, err := decodeIDs(path)
		if err != nil {
			log.Printf("scan %s: %v", path, err)
			continue
		}
		for id := range ids {
			out[id] = true
		}
	}
	return out, nil
}

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF, opened read-only")
	indexPath := flag.String("index", "configs/items.index.json", "item index used to enumerate [booster selection] templates")
	output := flag.String("output", "configs/selection-boxes-candidate.json", "exported catalog")
	bounded := flag.Bool("bounded", true, "export only templates the server's own configuration references")
	configDir := flag.String("configs", "configs", "directory scanned for template references when -bounded")
	boosterCatalog := flag.String("booster-catalog", "configs/booster-catalog.json", "fixed-content booster catalog (reference source)")
	lootCatalog := flag.String("loot-catalog", "configs/loot.next25.json", "monster drop catalog (reference source)")
	extraIDs := flag.String("extra-ids", "", "comma separated templates that must always be exported")
	extraPaths := flag.String("extra-paths", "stackable/10417001/", "comma separated archive path prefixes whose indexed boxes are always exported; the Odyssey item directory is handed out by the Odyssey flow, so those templates never appear as a config reference")
	scanSources := flag.String("scan-sources", ".", "Go source tree scanned for hard-coded template literals; several boxes (the Odyssey creation grants) are handed out by code rather than by a config file")
	debug := flag.String("debug", "", "comma separated templates: print their parsed categories and exit")
	flag.Parse()

	data, err := os.ReadFile(*indexPath)
	if err != nil {
		log.Fatalf("read item index: %v", err)
	}
	var idx itemIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		log.Fatalf("unmarshal item index: %v", err)
	}

	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatalf("load pvf: %v", err)
	}

	type candidate struct {
		id   uint32
		path string
	}
	byID := map[uint32]string{}
	for _, item := range idx.Items {
		if item.StackableType != "[booster selection]" {
			continue
		}
		byID[item.ID] = item.Path
	}

	keep := map[uint32]bool{}
	if *bounded {
		refs, err := boundedTemplates([]string{*boosterCatalog, *lootCatalog}, *configDir)
		if err != nil {
			log.Fatalf("collect references: %v", err)
		}
		for id := range byID {
			if refs[id] {
				keep[id] = true
			}
		}
		for _, prefix := range strings.Split(*extraPaths, ",") {
			prefix = strings.TrimSpace(prefix)
			if prefix == "" {
				continue
			}
			hits := 0
			for id, path := range byID {
				if strings.HasPrefix(path, prefix) {
					keep[id] = true
					hits++
				}
			}
			log.Printf("path prefix %s: %d indexed selection boxes", prefix, hits)
		}
		if *scanSources != "" {
			scanned, err := scanSourceTemplates(*scanSources)
			if err != nil {
				log.Printf("scan sources: %v", err)
			} else {
				hits := 0
				for id := range byID {
					if scanned[id] {
						keep[id] = true
						hits++
					}
				}
				log.Printf("source scan: %d template literals in %s, %d of them are indexed selection boxes", len(scanned), *scanSources, hits)
			}
		}
	}
	for _, part := range strings.Split(*extraIDs, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			log.Fatalf("bad -extra-ids template %q", part)
		}
		keep[uint32(n)] = true
	}

	var candidates []candidate
	for id, path := range byID {
		if *bounded && !keep[id] {
			continue
		}
		candidates = append(candidates, candidate{id: id, path: path})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].id < candidates[j].id })

	if *debug != "" {
		wanted := map[uint32]bool{}
		for _, part := range strings.Split(*debug, ",") {
			n, err := strconv.ParseUint(strings.TrimSpace(part), 10, 32)
			if err != nil {
				log.Fatalf("bad -debug template %q", part)
			}
			wanted[uint32(n)] = true
		}
		for id, path := range byID {
			if !wanted[id] {
				continue
			}
			cells, err := a.Tokens(path)
			if err != nil {
				log.Printf("%d %s: %v", id, path, err)
				continue
			}
			cats, fixed := parseSelection(cells)
			fmt.Printf("%d %s fixed=%v categories=%d\n", id, path, fixed, len(cats))
			for _, cat := range cats {
				fmt.Printf("  category=%v grade=%d recommend=%v items=%v\n", cat.Category, cat.Grade, cat.Recommend, cat.Items)
			}
		}
		return
	}

	doc := document{
		Model:   selectionBoxModel,
		Bounded: *bounded,
		Source:  a.Snapshot(),
		Boxes:   map[string]selectionBox{},
	}
	start := time.Now()
	boxes, fixed := 0, 0
	for _, cand := range candidates {
		cells, err := a.Tokens(cand.path)
		if err != nil {
			log.Printf("skip %d (%s): %v", cand.id, cand.path, err)
			doc.Unparsed = append(doc.Unparsed, cand.id)
			continue
		}
		cats, hasFixed := parseSelection(cells)
		if len(cats) == 0 {
			if hasFixed {
				doc.Fixed = append(doc.Fixed, cand.id)
				fixed++
			} else {
				doc.Unparsed = append(doc.Unparsed, cand.id)
			}
			continue
		}
		raw, err := a.ReadRaw(cand.path)
		if err != nil {
			log.Printf("skip %d (%s): raw read: %v", cand.id, cand.path, err)
			doc.Unparsed = append(doc.Unparsed, cand.id)
			continue
		}
		sum := sha256.Sum256(raw)
		doc.Boxes[strconv.FormatUint(uint64(cand.id), 10)] = selectionBox{
			Template:   cand.id,
			Path:       cand.path,
			SHA256:     hex.EncodeToString(sum[:]),
			Categories: cats,
		}
		boxes++
	}
	sort.Slice(doc.Fixed, func(i, j int) bool { return doc.Fixed[i] < doc.Fixed[j] })
	sort.Slice(doc.Unparsed, func(i, j int) bool { return doc.Unparsed[i] < doc.Unparsed[j] })

	out, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(*output, out, 0o644); err != nil {
		log.Fatalf("write %s: %v", *output, err)
	}
	fmt.Printf("selection boxes: %d (fixed mislabeled: %d, unparsed: %d; templates considered %d, bounded=%v) -> %s (%d bytes) in %v\n",
		boxes, fixed, len(doc.Unparsed), len(candidates), *bounded, *output, len(out), time.Since(start))
}
