// equipmentjournalimport 导出内层 PVF 里的装备库（装备图鉴）规则表：
//
//	contents/2025/equipmentsetjournal/etc/equipmentsetjournal.cos
//	    → configs/equipment-journal.generated.json
//	    → configs/equipment-create-cost.generated.json（`-create-cost`，装备生成/制作的成本表）
//
// 这张表是「分解/添加能否入库、上限多少、收藏类别有几类」的真源，而不是 id 白名单：
//   - [max equipment count] 99 / [max equipment count by equipment type]（誓约按 rarity 2/3/4/6/8 各 1）
//   - [new peculiar group] 的 5 个 [group]：[set mark] 就是收藏类别 0..4
//   - [part set index] / [oath group] / [primer group] / [common primer item index] 等
//
// 为什么**不用**分享版那种 1,527 条显式收录目录：那是别的客户端版本派生的产物。
// 本版本的收录判据是「minimum level == 115 且 rarity ∈ {2,3,4,6,8}」（规格 0026-DISJOINTITEM），
// 上限来自本文件。可随时从同一份 PVF 重新生成并与 source hash 对照。
package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"flag"
	"log"
	"os"
	"path/filepath"
)

func writeAtomic(name string, b []byte) error {
	f, e := os.CreateTemp(filepath.Dir(name), ".equipment-journal-import-*")
	if e != nil {
		return e
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return e
	}
	if e = f.Close(); e != nil {
		return e
	}
	return os.Rename(tmp, name)
}

func main() {
	source := flag.String("source", "", "read-only inner PVF")
	output := flag.String("output", "configs/equipment-journal.generated.json", "output catalog")
	imageCostOutput := flag.String("create-cost", "", "also export the [create cost] (装备生成成本) table to this path")
	replace := flag.Bool("replace", false, "explicitly replace an existing output")
	flag.Parse()
	if *source == "" {
		log.Fatal("-source is required")
	}
	if !*replace {
		for _, p := range []string{*output, *imageCostOutput} {
			if p == "" {
				continue
			}
			if _, e := os.Stat(p); !os.IsNotExist(e) {
				log.Fatalf("output %s already exists; use a fresh path or explicit -replace", p)
			}
		}
	}
	a, e := pvf.LoadArchive(pvf.Options{Path: *source, MaxBytes: 1024 * 1024 * 1024})
	if e != nil {
		log.Fatal(e)
	}
	out, e := catalog.ImportEquipmentJournalRules(a)
	if e != nil {
		log.Fatal(e)
	}
	b, e := json.MarshalIndent(out, "", " ")
	if e != nil {
		log.Fatal(e)
	}
	if e = writeAtomic(*output, b); e != nil {
		log.Fatal(e)
	}
	log.Printf("source=%s path=%s sha256=%s bytes=%d", out.Source.Checksum, out.Path, out.SHA256, out.Bytes)
	log.Printf("  maximum=%d maxAwakening=%d limits=%d partSet=%d oathGroup=%d primerGroup=%d primerItem=%d",
		out.Maximum, out.MaxAwakening, len(out.MaximumByType), len(out.PartSetIndex),
		len(out.OathGroup), len(out.PrimerGroup), len(out.CommonPrimerItem))
	if *imageCostOutput != "" {
		cc, e := catalog.ImportEquipmentCreateCost(a)
		if e != nil {
			log.Fatal(e)
		}
		cb, e := json.MarshalIndent(cc, "", " ")
		if e != nil {
			log.Fatal(e)
		}
		if e = writeAtomic(*imageCostOutput, cb); e != nil {
			log.Fatal(e)
		}
		items := 0
		for _, g := range cc.Groups {
			items += len(g.Items)
		}
		log.Printf("create cost: source=%s sha256=%s bytes=%d groups=%d itemRows=%d templates=%d",
			cc.Source.Checksum, cc.SHA256, cc.Bytes, len(cc.Groups), items, len(cc.Templates()))
		for _, g := range cc.Groups {
			log.Printf("  group %d: items=%d costs=%d", g.Index, len(g.Items), len(g.Costs))
		}
	}
	for _, c := range out.Categories {
		log.Printf("  category mark=%d index=%d button=%q members=%d name=%s",
			c.Mark, c.Index, c.Button, len(c.Members), c.Name)
	}
	log.Printf("  weaponGroups=%d(%d ids) peculiarGroups=%d(%d ids)",
		len(out.WeaponGroups), journalMembers(out.WeaponGroups),
		len(out.PeculiarGroups), journalMembers(out.PeculiarGroups))
}

func journalMembers(groups []catalog.JournalInfoGroup) int {
	n := 0
	for _, g := range groups {
		n += len(g.Members)
	}
	return n
}
