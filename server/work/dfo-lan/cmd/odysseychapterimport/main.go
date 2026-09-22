// odysseychapterimport exports the Arad Odyssey chapter journal into two JSON
// catalogs:
//
//   - chapters: the seven chapters, each with its dungeon list (in source
//     order), its Final dungeon, and its [reward] lines (template, count).
//   - chapter drop: the per-chapter selection box handed out by the chapter's
//     final lord. Chapter 7 has none (both of its reward boxes are plain
//     [booster] items, not EquipmentSelectionBox), which the source expresses
//     as template 0.
//
// The source is contents/2026/aradodyssey/etc/aradodysseyjournal.cos inside
// ../client-build/Script.inner.pvf. That entry is data_type=3 (UTF-16), so it
// is read through Archive.ReadText, which decodes it; the raw bytes are hashed
// separately to pin definition_sha256.
//
// Everything is asserted, never assumed: seven chapters, fifty dungeons, no
// dungeon in two chapters, Final = the chapter's last dungeon, fifteen distinct
// reward templates, and (chapters 1..6) a first reward line that really is a
// [booster selection] box.
//
//	go run ./cmd/odysseychapterimport -source ../client-build/Script.inner.pvf \
//	    -chapters configs/odyssey-chapters-candidate.json \
//	    -drop configs/odyssey-chapter-drop-candidate.json
package main

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
)

const journalPath = "contents/2026/aradodyssey/etc/aradodysseyjournal.cos"

// Journal raw sha256 of the 115-source build this tool was written against.
const expectedJournalSHA = "d4654fa9a50ddd582077f5f7a0a19835ec4fb6b777f032d1a66be7f288eff67e"

const chaptersModel = "source-odyssey-chapters-v1"
const dropModel = "source-odyssey-chapter-drop-v1"

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

// RewardLine is one line of a chapter's [reward] block.
type RewardLine struct {
	Template uint32 `json:"template"`
	Count    uint32 `json:"count"`
}

// Chapter is one [chapter] block of the journal.
type Chapter struct {
	Number   uint8        `json:"number"`
	Dungeons []uint32     `json:"dungeons"`
	Final    uint32       `json:"final"`
	Rewards  []RewardLine `json:"rewards"`
}

type chaptersDoc struct {
	Model          string              `json:"model"`
	Source         pvf.ArchiveSnapshot `json:"source"`
	JournalPath    string              `json:"journal_path"`
	DefinitionSHA  string              `json:"definition_sha256"`
	ChapterCount   int                 `json:"chapter_count"`
	DungeonCount   int                 `json:"dungeon_count"`
	RewardTemplate int                 `json:"reward_templates"`
	Chapters       []Chapter           `json:"chapters"`
}

// DropLine is the selection box a chapter's final lord drops. Template 0 means
// the source gives that chapter no equipment selection box.
type DropLine struct {
	Chapter  uint8  `json:"chapter"`
	Final    uint32 `json:"final"`
	Template uint32 `json:"template"`
	Rate     uint32 `json:"rate"`
	Enabled  bool   `json:"enabled"`
}

type dropDoc struct {
	Model         string              `json:"model"`
	Source        pvf.ArchiveSnapshot `json:"source"`
	JournalPath   string              `json:"journal_path"`
	DefinitionSHA string              `json:"definition_sha256"`
	Drops         []DropLine          `json:"drops"`
}

var (
	chapterSplit = regexp.MustCompile(`(?m)^\[chapter\]\s*$`)
	rewardBlock  = regexp.MustCompile(`(?s)\[reward\](.*?)\[/reward\]`)
	dungeonBlock = regexp.MustCompile(`(?s)\[dungeon\](.*?)\[/dungeon\]`)
	indexLine    = regexp.MustCompile(`\[index\]\s*(\d+)`)
	rewardPair   = regexp.MustCompile(`(\d+)\s+(\d+)`)
)

func parseJournal(text string) ([]Chapter, error) {
	parts := chapterSplit.Split(text, -1)
	if len(parts) < 2 {
		return nil, fmt.Errorf("journal has no [chapter] block")
	}
	var out []Chapter
	seen := map[uint32]uint8{}
	for i, body := range parts[1:] {
		ch := Chapter{Number: uint8(i + 1)}
		if m := rewardBlock.FindStringSubmatch(body); m != nil {
			for _, p := range rewardPair.FindAllStringSubmatch(m[1], -1) {
				t, e1 := strconv.ParseUint(p[1], 10, 32)
				n, e2 := strconv.ParseUint(p[2], 10, 32)
				if e1 != nil || e2 != nil || n == 0 {
					return nil, fmt.Errorf("chapter %d: bad reward line %q", ch.Number, p[0])
				}
				ch.Rewards = append(ch.Rewards, RewardLine{Template: uint32(t), Count: uint32(n)})
			}
		}
		if len(ch.Rewards) == 0 {
			return nil, fmt.Errorf("chapter %d: empty [reward] block", ch.Number)
		}
		for _, blk := range dungeonBlock.FindAllStringSubmatch(body, -1) {
			m := indexLine.FindStringSubmatch(blk[1])
			if m == nil {
				return nil, fmt.Errorf("chapter %d: [dungeon] without [index]", ch.Number)
			}
			id, e := strconv.ParseUint(m[1], 10, 32)
			if e != nil || id == 0 {
				return nil, fmt.Errorf("chapter %d: bad dungeon index %q", ch.Number, m[1])
			}
			if prev, dup := seen[uint32(id)]; dup {
				return nil, fmt.Errorf("dungeon %d appears in chapters %d and %d", id, prev, ch.Number)
			}
			seen[uint32(id)] = ch.Number
			ch.Dungeons = append(ch.Dungeons, uint32(id))
		}
		if len(ch.Dungeons) == 0 {
			return nil, fmt.Errorf("chapter %d: no dungeons", ch.Number)
		}
		// Final is the chapter's last dungeon in source order — the journal
		// lists the story path, and chapter 5 genuinely runs 968,967,969,...
		ch.Final = ch.Dungeons[len(ch.Dungeons)-1]
		out = append(out, ch)
	}
	return out, nil
}

func main() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF, opened read-only")
	indexPath := flag.String("index", "configs/items.index.json", "item index used to classify reward templates")
	chaptersOut := flag.String("chapters", "configs/odyssey-chapters-candidate.json", "exported chapter catalog")
	dropOut := flag.String("drop", "configs/odyssey-chapter-drop-candidate.json", "exported chapter drop catalog")
	rate := flag.Uint("rate", 10000, "drop rate in basis points for an enabled chapter box (default 100%)")
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
	raw, err := a.ReadRaw(journalPath)
	if err != nil {
		log.Fatalf("read journal raw: %v", err)
	}
	sum := sha256.Sum256(raw)
	definition := hex.EncodeToString(sum[:])
	if definition != expectedJournalSHA {
		log.Fatalf("journal sha256 %s, expected %s", definition, expectedJournalSHA)
	}
	text, err := a.ReadText(journalPath)
	if err != nil {
		log.Fatalf("read journal text: %v", err)
	}

	chapters, err := parseJournal(text)
	if err != nil {
		log.Fatal(err)
	}
	if len(chapters) != 7 {
		log.Fatalf("journal has %d chapters, expected 7", len(chapters))
	}
	dungeons := 0
	templates := map[uint32]bool{}
	for _, ch := range chapters {
		dungeons += len(ch.Dungeons)
		for _, r := range ch.Rewards {
			templates[r.Template] = true
		}
	}
	if dungeons != 50 {
		log.Fatalf("journal has %d dungeons, expected 50", dungeons)
	}
	if len(templates) != 15 {
		log.Fatalf("journal has %d distinct reward templates, expected 15", len(templates))
	}

	// Which chapters hand out an equipment selection box, and on which line.
	drops := make([]DropLine, 0, len(chapters))
	for _, ch := range chapters {
		line := DropLine{Chapter: ch.Number, Final: ch.Final, Rate: uint32(*rate), Enabled: false}
		for _, r := range ch.Rewards {
			entry, ok := idx.Items[strconv.FormatUint(uint64(r.Template), 10)]
			if !ok {
				log.Fatalf("chapter %d: reward template %d missing from item index", ch.Number, r.Template)
			}
			if entry.StackableType != "[booster selection]" {
				continue
			}
			// The first selection box line is the chapter box; later selection
			// lines are shared side rewards (e.g. 10419741 / 10419743).
			line.Template = r.Template
			break
		}
		if ch.Number <= 6 && line.Template == 0 {
			log.Fatalf("chapter %d has no [booster selection] reward", ch.Number)
		}
		// Chapter 7 must not have one (both boxes are plain [booster]) — that is
		// the source shape, so record template 0 rather than inventing a box.
		drops = append(drops, line)
	}

	snap := a.Snapshot()
	cdoc := chaptersDoc{
		Model: chaptersModel, Source: snap, JournalPath: journalPath,
		DefinitionSHA: definition, ChapterCount: len(chapters),
		DungeonCount: dungeons, RewardTemplate: len(templates), Chapters: chapters,
	}
	ddoc := dropDoc{
		Model: dropModel, Source: snap, JournalPath: journalPath,
		DefinitionSHA: definition, Drops: drops,
	}
	for path, doc := range map[string]any{*chaptersOut: cdoc, *dropOut: ddoc} {
		b, err := json.MarshalIndent(doc, "", " ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, '\n'), 0600); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s", path)
	}
	for _, ch := range chapters {
		log.Printf("chapter %d: %d dungeons, final %d, rewards %v", ch.Number, len(ch.Dungeons), ch.Final, ch.Rewards)
	}
	for _, d := range drops {
		log.Printf("drop chapter %d: final %d box %d", d.Chapter, d.Final, d.Template)
	}
}
