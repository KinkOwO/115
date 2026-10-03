// odysseychapterimport exports the native journal and chapter box projection.
// Exported drops remain disabled; runtime rates and activation are separate
// operator strategy. Both native startup and this tool share the same parser.
package odysseychapterimport

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/loot"
	"encoding/json"
	"flag"
	"log"
	"os"
)

func Run() {
	source := flag.String("source", "../client-build/Script.inner.pvf", "source PVF, opened read-only")
	chaptersOut := flag.String("chapters", "configs/odyssey-chapters.json", "exported chapter catalog")
	dropOut := flag.String("drop", "configs/odyssey-chapter-drop.json", "exported chapter drop catalog")
	rate := flag.Uint("rate", 10000, "drop rate in basis points for an enabled chapter box (default 100%)")
	flag.Parse()
	if *rate > 10000 {
		log.Fatal("drop rate must be in 0..10000")
	}
	a, err := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1 << 30})
	if err != nil {
		log.Fatal(err)
	}
	defer a.Close()
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		log.Fatal(err)
	}
	chapters, err := catalog.ImportOdysseyChapters(a)
	if err != nil {
		log.Fatal(err)
	}
	var policy []loot.OdysseyChapterDropPolicy
	for _, ch := range chapters.Chapters {
		policy = append(policy, loot.OdysseyChapterDropPolicy{Chapter: ch.Number, Rate: uint32(*rate)})
	}
	drops, err := loot.ImportOdysseyChapterDrop(chapters, index, policy)
	if err != nil {
		log.Fatal(err)
	}
	for path, doc := range map[string]any{*chaptersOut: chapters, *dropOut: drops} {
		b, err := json.MarshalIndent(doc, "", " ")
		if err != nil {
			log.Fatal(err)
		}
		if err := os.WriteFile(path, append(b, 10), 0600); err != nil {
			log.Fatal(err)
		}
		log.Printf("wrote %s", path)
	}
	for _, ch := range chapters.Chapters {
		log.Printf("chapter %d: %d dungeons, final %d, rewards %v", ch.Number, len(ch.Dungeons), ch.Final, ch.Rewards)
	}
	for _, d := range drops.Drops {
		log.Printf("drop chapter %d: final %d box %d", d.Chapter, d.Final, d.Template)
	}
}
