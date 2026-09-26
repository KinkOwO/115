// Command attunementimport decodes the per-dungeon reward tables that drive the
// "boundary of attunement" (调律之边界) abyss dungeons and writes the truth the
// server consumes:
//
//	configs/attunement-rewards.generated.json
//
// The input is read straight out of the frozen client PVF, so the generated
// file can always be reproduced and diffed against the source hash. Nothing is
// invented: every field is a cell read, and the one field whose meaning is not
// yet proven ([hidden drop table]'s middle number, see hiddenEntry.Key) keeps
// its positional name plus a note.
//
// The container stores its values as positional triples, not as a nested
// structure, so the shape below is the shape the boundary-of-attunement tables
// actually use (verified on unique/legendary/epic alike):
//
//	[dungeon index]          1 float        which dungeon owns this table
//	[fixed drop table]       per maze       [maze] + [drop list]
//	[additional drop table]  x N            [effect index] + [select prob]
//	                                         + [drop count] + [drop list]
//	[hidden drop table]      per maze       [maze] + N x [drop list]
//
// [drop list] inside a fixed/additional table is (tier, weight, item) repeated,
// and the weights always sum to exactly one million. That invariant is the
// reason a decoder can be shipped at all: it makes the reading falsifiable, so
// the importer refuses to emit a table that breaks it.
package main

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
)

const (
	model = "rewardboostinfo-ctp-v1"

	dungeonIndexColumn = "[dungeon index]"
	fixedTableColumn   = "[fixed drop table]"
	additionalColumn   = "[additional drop table]"
	hiddenTableColumn  = "[hidden drop table]"
	mazeColumn         = "[maze]"
	dropListColumn     = "[drop list]"
	effectIndexColumn  = "[effect index]"
	selectProbColumn   = "[select prob]"
	dropCountColumn    = "[drop count]"

	// weightSpace is the denominator every [drop list] and the whole
	// [additional drop table] set add up to. It is a fact about the data, not a
	// tunable: both sums are checked before anything is written.
	weightSpace = 1000000
)

type rewardEntry struct {
	Tier   string `json:"tier"`
	Weight uint32 `json:"weight"`
	Item   uint32 `json:"item"`
}

type fixedTable struct {
	Maze    uint32        `json:"maze"`
	Entries []rewardEntry `json:"entries"`
}

type additionalTable struct {
	EffectIndex uint32        `json:"effectIndex"`
	SelectProb  uint32        `json:"selectProb"`
	DropCount   uint32        `json:"dropCount"`
	Entries     []rewardEntry `json:"entries"`
}

// hiddenEntry carries the middle number verbatim as Key. In the fixed and
// additional tables that slot is a weight, but the hidden tables fill it with
// 0,1,2,.. in step with the item ids, which a weight cannot do (the weights
// would have to sum to a million). No decoder is shipped for it.
type hiddenEntry struct {
	Tier string `json:"tier"`
	Key  uint32 `json:"key"`
	Item uint32 `json:"item"`
}

type hiddenTable struct {
	Maze    uint32        `json:"maze"`
	Index   uint32        `json:"index"`
	Entries []hiddenEntry `json:"entries"`
}

type dungeonRewards struct {
	Path        string            `json:"path"`
	SHA256      string            `json:"sha256"`
	Bytes       int               `json:"bytes"`
	Version     uint32            `json:"version"`
	RecordCount uint32            `json:"recordCount"`
	Dungeon     uint32            `json:"dungeon"`
	Fixed       []fixedTable      `json:"fixed"`
	Additional  []additionalTable `json:"additional"`
	Hidden      []hiddenTable     `json:"hidden"`
}

type document struct {
	Model   string              `json:"model"`
	Archive pvf.ArchiveSnapshot `json:"archive"`
	Tables  []dungeonRewards    `json:"tables"`
}

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF")
	baseDir := flag.String("base", "etc/rewardboostinfo/skyofathousandseasofborder", "directory holding the difficulty tables")
	output := flag.String("output", "configs/attunement-rewards.generated.json", "generated config")
	flag.Parse()

	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		log.Fatalf("load %s: %v", *source, err)
	}

	doc := document{Model: model, Archive: a.Snapshot()}
	for _, name := range []string{"unique.ctp", "legendary.ctp", "epic.ctp"} {
		table, err := readTable(a, path.Join(*baseDir, name))
		if err != nil {
			log.Fatalf("read %s: %v", name, err)
		}
		fmt.Printf("%-14s dungeon=%d fixed=%d additional=%d hidden=%d\n",
			name, table.Dungeon, len(table.Fixed), len(table.Additional), len(table.Hidden))
		doc.Tables = append(doc.Tables, table)
	}
	if len(doc.Tables) == 0 {
		log.Fatal("no attunement reward table found")
	}

	b, err := json.MarshalIndent(doc, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(*output, b, 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("\nwrote %s (%d bytes, %d tables)\n", *output, len(b), len(doc.Tables))
}

// readTable decodes one difficulty table and checks every invariant the reading
// depends on before returning. A table that breaks one is not written at all:
// a half-believed reward table would pay the wrong items silently.
func readTable(a *pvf.Archive, entry string) (dungeonRewards, error) {
	var out dungeonRewards
	t, err := a.CTP(entry)
	if err != nil {
		return out, err
	}
	out.Path = t.Path
	out.SHA256 = t.SHA256
	out.Bytes = t.Bytes
	out.Version = t.Version
	out.RecordCount = t.RecordCount

	children := map[int][]int{}
	for i, r := range t.Records {
		if r.Parent >= 0 {
			children[r.Parent] = append(children[r.Parent], i)
		}
	}
	childNamed := func(parent int, name string) (int, bool) {
		for _, i := range children[parent] {
			if t.Records[i].Name == name {
				return i, true
			}
		}
		return 0, false
	}

	var dungeonSet bool
	for i, r := range t.Records {
		if r.Parent >= 0 {
			continue
		}
		switch r.Name {
		case dungeonIndexColumn:
			v, err := singleNumber(r)
			if err != nil {
				return out, fmt.Errorf("%s: %w", dungeonIndexColumn, err)
			}
			if dungeonSet {
				return out, fmt.Errorf("repeated %s", dungeonIndexColumn)
			}
			out.Dungeon = v
			dungeonSet = true
		case fixedTableColumn:
			var ft fixedTable
			if j, ok := childNamed(i, mazeColumn); ok {
				if ft.Maze, err = singleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			j, ok := childNamed(i, dropListColumn)
			if !ok {
				return out, fmt.Errorf("%s at %d carries no %s", fixedTableColumn, i, dropListColumn)
			}
			if ft.Entries, err = entries(t.Records[j]); err != nil {
				return out, err
			}
			if err := checkWeights(ft.Entries, fmt.Sprintf("%s maze %d", fixedTableColumn, ft.Maze)); err != nil {
				return out, err
			}
			out.Fixed = append(out.Fixed, ft)
		case additionalColumn:
			var at additionalTable
			for _, col := range []struct {
				name string
				dst  *uint32
			}{{effectIndexColumn, &at.EffectIndex}, {selectProbColumn, &at.SelectProb}, {dropCountColumn, &at.DropCount}} {
				j, ok := childNamed(i, col.name)
				if !ok {
					return out, fmt.Errorf("%s at %d carries no %s", additionalColumn, i, col.name)
				}
				if *col.dst, err = singleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			j, ok := childNamed(i, dropListColumn)
			if !ok {
				return out, fmt.Errorf("%s at %d carries no %s", additionalColumn, i, dropListColumn)
			}
			if at.Entries, err = entries(t.Records[j]); err != nil {
				return out, err
			}
			if err := checkWeights(at.Entries, fmt.Sprintf("%s effect %d", additionalColumn, at.EffectIndex)); err != nil {
				return out, err
			}
			out.Additional = append(out.Additional, at)
		case hiddenTableColumn:
			var ht hiddenTable
			if j, ok := childNamed(i, mazeColumn); ok {
				if ht.Maze, err = singleNumber(t.Records[j]); err != nil {
					return out, err
				}
			}
			for _, j := range children[i] {
				if t.Records[j].Name != dropListColumn {
					continue
				}
				row, err := readHiddenRow(t.Records[j])
				if err != nil {
					return out, err
				}
				ht.Index = row.Index
				ht.Entries = row.Entries
				out.Hidden = append(out.Hidden, ht)
			}
		}
	}
	if !dungeonSet {
		return out, fmt.Errorf("missing %s", dungeonIndexColumn)
	}
	if len(out.Fixed) == 0 && len(out.Additional) == 0 {
		return out, fmt.Errorf("table carries no reward list")
	}

	// The additional tables are a single draw from the same million-space: their
	// [select prob] values are exclusive weights, not independent chances.
	var prob uint32
	for _, at := range out.Additional {
		prob += at.SelectProb
	}
	if len(out.Additional) > 0 && prob != weightSpace {
		return out, fmt.Errorf("%s select prob sums to %d, want %d", additionalColumn, prob, weightSpace)
	}
	return out, nil
}

type hiddenRow struct {
	Index   uint32
	Entries []hiddenEntry
}

// readHiddenRow decodes one [drop list] of a hidden table: a leading index
// followed by (tier, key, item) triples.
func readHiddenRow(r pvf.CTPRecord) (hiddenRow, error) {
	var out hiddenRow
	var cells []pvf.CTPCell
	for _, c := range r.Cells {
		if c.Kind != "" {
			cells = append(cells, c)
		}
	}
	if len(cells) == 0 {
		return out, fmt.Errorf("empty hidden %s", dropListColumn)
	}
	v, err := number(cells[0])
	if err != nil {
		return out, fmt.Errorf("hidden %s index: %w", dropListColumn, err)
	}
	out.Index = v
	rest := cells[1:]
	if len(rest)%3 != 0 {
		return out, fmt.Errorf("hidden %s holds %d cells, want a triple count", dropListColumn, len(rest)+1)
	}
	for i := 0; i < len(rest); i += 3 {
		tier, err := text(rest[i])
		if err != nil {
			return out, err
		}
		key, err := number(rest[i+1])
		if err != nil {
			return out, err
		}
		item, err := number(rest[i+2])
		if err != nil {
			return out, err
		}
		out.Entries = append(out.Entries, hiddenEntry{Tier: tier, Key: key, Item: item})
	}
	return out, nil
}

// entries decodes a fixed/additional [drop list]: (tier, weight, item) triples.
func entries(r pvf.CTPRecord) ([]rewardEntry, error) {
	var cells []pvf.CTPCell
	for _, c := range r.Cells {
		if c.Kind != "" {
			cells = append(cells, c)
		}
	}
	if len(cells)%3 != 0 {
		return nil, fmt.Errorf("%s holds %d cells, want a triple count", dropListColumn, len(cells))
	}
	var out []rewardEntry
	for i := 0; i < len(cells); i += 3 {
		tier, err := text(cells[i])
		if err != nil {
			return nil, err
		}
		weight, err := number(cells[i+1])
		if err != nil {
			return nil, err
		}
		item, err := number(cells[i+2])
		if err != nil {
			return nil, err
		}
		if item == 0 || item > 0x2ffffff {
			return nil, fmt.Errorf("%s item %d is outside the item id space", dropListColumn, item)
		}
		out = append(out, rewardEntry{Tier: tier, Weight: weight, Item: item})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s is empty", dropListColumn)
	}
	return out, nil
}

func checkWeights(list []rewardEntry, what string) error {
	var sum uint64
	for _, e := range list {
		if e.Weight == 0 {
			return fmt.Errorf("%s holds a zero weight for item %d", what, e.Item)
		}
		sum += uint64(e.Weight)
	}
	if sum != weightSpace {
		return fmt.Errorf("%s weights sum to %d, want %d", what, sum, weightSpace)
	}
	return nil
}

func singleNumber(r pvf.CTPRecord) (uint32, error) {
	var n uint32
	for _, c := range r.Cells {
		if c.Kind == "" {
			continue
		}
		v, err := number(c)
		if err != nil {
			return 0, fmt.Errorf("%s: %w", r.Name, err)
		}
		n = v
	}
	return n, nil
}

func number(c pvf.CTPCell) (uint32, error) {
	switch c.Kind {
	case "float":
		if c.Float < 0 || c.Float > float64(^uint32(0)) {
			return 0, fmt.Errorf("value %g does not fit a u32", c.Float)
		}
		return uint32(c.Float), nil
	case "byte":
		return uint32(c.Byte), nil
	default:
		return 0, fmt.Errorf("cell kind %q is not numeric", c.Kind)
	}
}

func text(c pvf.CTPCell) (string, error) {
	switch c.Kind {
	case "name", "name_indexed":
		if strings.TrimSpace(c.Name) == "" {
			return "", fmt.Errorf("empty name cell")
		}
		return c.Name, nil
	default:
		return "", fmt.Errorf("cell kind %q is not a name", c.Kind)
	}
}
