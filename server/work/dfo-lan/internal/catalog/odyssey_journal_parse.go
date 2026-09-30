package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"regexp"
	"strconv"
)

const OdysseyJournalPath = "contents/2026/aradodyssey/etc/aradodysseyjournal.cos"

func ImportOdysseyChapters(a *pvf.Archive) (*OdysseyChapters, error) {
	if a == nil || a.Snapshot().Checksum != OdysseySource {
		return nil, fmt.Errorf("Odyssey source mismatch")
	}
	raw, e := a.ReadRaw(OdysseyJournalPath)
	if e != nil {
		return nil, e
	}
	text, e := a.ReadText(OdysseyJournalPath)
	if e != nil {
		return nil, e
	}
	chapters, e := parseOdysseyJournal(text)
	if e != nil {
		return nil, e
	}
	c := OdysseyChapters{Model: OdysseyChaptersModel, Source: a.Snapshot(), JournalPath: OdysseyJournalPath, DefinitionSHA: fmt.Sprintf("%x", sha256.Sum256(raw)), Chapters: chapters, ChapterCount: len(chapters)}
	refs := map[uint32]bool{}
	for _, ch := range chapters {
		c.DungeonCount += len(ch.Dungeons)
		for _, r := range ch.Rewards {
			refs[r.Template] = true
		}
	}
	c.RewardTemplate = len(refs)
	if c.RewardTemplate != 15 {
		return nil, fmt.Errorf("Odyssey journal reward template count %d", c.RewardTemplate)
	}
	return NewOdysseyChapters(c)
}

var (
	chapterSplit = regexp.MustCompile(`(?m)^\[chapter\]\s*$`)
	rewardBlock  = regexp.MustCompile(`(?s)\[reward\](.*?)\[/reward\]`)
	dungeonBlock = regexp.MustCompile(`(?s)\[dungeon\](.*?)\[/dungeon\]`)
	indexLine    = regexp.MustCompile(`\[index\]\s*(\d+)`)
	rewardPair   = regexp.MustCompile(`(\d+)\s+(\d+)`)
)

func parseOdysseyJournal(text string) ([]OdysseyChapter, error) {
	parts := chapterSplit.Split(text, -1)
	if len(parts) < 2 {
		return nil, fmt.Errorf("journal has no [chapter] block")
	}
	var out []OdysseyChapter
	seen := map[uint32]uint8{}
	for i, body := range parts[1:] {
		ch := OdysseyChapter{Number: uint8(i + 1)}
		if m := rewardBlock.FindStringSubmatch(body); m != nil {
			for _, p := range rewardPair.FindAllStringSubmatch(m[1], -1) {
				t, e1 := strconv.ParseUint(p[1], 10, 32)
				n, e2 := strconv.ParseUint(p[2], 10, 32)
				if e1 != nil || e2 != nil || n == 0 {
					return nil, fmt.Errorf("chapter %d: bad reward line %q", ch.Number, p[0])
				}
				ch.Rewards = append(ch.Rewards, ChapterReward{Template: uint32(t), Count: uint32(n)})
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
