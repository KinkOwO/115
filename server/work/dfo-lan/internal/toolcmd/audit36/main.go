// audit36 statically surveys the current quest, reward and equipment source
// for the 36 gameplay-closure work. Read-only: it reads generated catalogs
// and never writes or mutates any game or character state.
package audit36

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/gamedata"
	"dfolan/internal/inventory"
	"dfolan/internal/quest"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
)

func section(c []pvf.Token, name string) []pvf.Token {
	var out []pvf.Token
	on := false
	for _, t := range c {
		if t.Type == 3 {
			on = t.Text == name
			continue
		}
		if on {
			out = append(out, t)
		}
	}
	return out
}

func brief(c []pvf.Token) string {
	var out []string
	for i, t := range c {
		if i >= 28 {
			out = append(out, "...")
			break
		}
		switch t.Type {
		case 0:
			out = append(out, fmt.Sprintf("%d", t.Value))
		case 2:
			out = append(out, fmt.Sprintf("%g", t.Number))
		case 3, 6:
			out = append(out, t.Text)
		default:
			out = append(out, fmt.Sprintf("<t%d>", t.Type))
		}
	}
	return strings.Join(out, " ")
}

type kindRow struct {
	Kind                    string
	Total, OK, Pending      int
	LowTotal, LowOK, LowBad int
}

func Run() {
	questPath := flag.String("quests", "", "deprecated quest path; quests are read from native PVF")
	archive := flag.String("pvf-archive", "../client-build/Script.inner.pvf", "read-only inner PVF")
	low := flag.Int("low-level", 20, "low-level reporting threshold")
	samples := flag.Int("samples", 6, "per-kind unimplemented samples to print")
	worldPath := flag.String("world", "", "nonempty enables NPC presence scan; world is read from native PVF")
	skillManifest := flag.String("skill-manifest", "", "archer layout patch manifest")
	verify := flag.Bool("verify", false, "load every 36 startup catalog and policy")
	featureSource := flag.String("feature-source", "", "survey mail/avatar/shop/item-use source data in this PVF")
	tutorialRoutes := flag.String("tutorial-routes", "", "tutorial route catalog")
	tutorialDungeons := flag.String("tutorial-dungeons", "runtime/tutorial-source35/dungeons.json", "tutorial dungeon catalog")
	detailMin := flag.Int("detail-min", 0, "print full detail for quests from this id")
	detailMax := flag.Int("detail-max", 0, "print full detail for quests up to this id")
	flag.Parse()

	native, e := gamedata.Open(gamedata.Options{Mode: gamedata.PVF, ArchivePath: *archive})
	if e != nil {
		log.Fatal(e)
	}
	defer native.Close()
	q, e := native.Quests("")
	if e != nil {
		log.Fatal(e)
	}
	rows := map[string]*kindRow{}
	perKind := map[string][]string{}
	rewardTypes := map[string]int{}
	selectCount, selectLow := 0, 0
	var selectSamples []string
	var ids []uint32
	for id := range q.Quests {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	for _, id := range ids {
		d := q.Quests[id]
		k := d.Kind
		if k == "" {
			k = "(no type)"
		}
		r := rows[k]
		if r == nil {
			r = &kindRow{Kind: k}
			rows[k] = r
		}
		lowLevel := d.MinimumLevel > 0 && int(d.MinimumLevel) <= *low
		r.Total++
		if lowLevel {
			r.LowTotal++
		}
		_, _, err := quest.InitialProgress(d)
		if err == nil {
			r.OK++
			if lowLevel {
				r.LowOK++
			}
		} else {
			r.Pending++
			if lowLevel {
				r.LowBad++
			}
		}
		if err != nil && lowLevel && len(perKind[k]) < *samples {
			perKind[k] = append(perKind[k], fmt.Sprintf(
				"    quest %d lvl=%d-%d jobs=%v cells=[%s] pending=%v",
				id, d.MinimumLevel, d.MaximumLevel, d.Jobs, brief(d.ObjectiveCells), d.Pending))
		}
		sel := section(d.Script.Cells, "[reward select int data]")
		sel2 := section(d.Script.Cells, "[reward selection int data]")
		if len(sel) > 0 || len(sel2) > 0 {
			selectCount++
			if lowLevel {
				selectLow++
				if len(selectSamples) < *samples {
					selectSamples = append(selectSamples, fmt.Sprintf(
						"    quest %d lvl=%d type=%s\n      select=[%s]\n      selection=[%s]\n      reward_int=[%s]\n      reward_type=[%s]",
						id, d.MinimumLevel, k, brief(sel), brief(sel2),
						brief(d.RewardCells), brief(section(d.Script.Cells, "[reward type]"))))
				}
			}
		}
		for _, t := range section(d.Script.Cells, "[reward type]") {
			if t.Type == 6 {
				rewardTypes[t.Text]++
			}
		}
		if *detailMax > 0 && int(id) >= *detailMin && int(id) <= *detailMax {
			fmt.Printf("-- quest %d type=%s lvl=%d-%d jobs=%v pre=%v impl=%v\n", id, k,
				d.MinimumLevel, d.MaximumLevel, d.Jobs, d.Prerequisites, err == nil)
			fmt.Printf("     int data      = [%s]\n", brief(d.ObjectiveCells))
			fmt.Printf("     reward int    = [%s]\n", brief(d.RewardCells))
			fmt.Printf("     reward type   = [%s]\n", brief(section(d.Script.Cells, "[reward type]")))
			fmt.Printf("     reward select = [%s]\n", brief(sel))
			fmt.Printf("     reward selct2 = [%s]\n", brief(sel2))
			fmt.Printf("     grow type     = [%s]\n", brief(section(d.Script.Cells, "[grow type]")))
			fmt.Printf("     ignore level  = [%s]\n", brief(section(d.Script.Cells, "[ignore level]")))
			fmt.Printf("     difficulty    = [%s]\n", brief(section(d.Script.Cells, "[difficulty]")))
			fmt.Printf("     grade         = [%s]\n", brief(section(d.Script.Cells, "[grade]")))
		}
	}

	var kinds []string
	for k := range rows {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return rows[kinds[i]].LowTotal > rows[kinds[j]].LowTotal })
	fmt.Printf("== quest objective types (total=%d, low-level threshold=%d)\n", len(q.Quests), *low)
	fmt.Printf("%-28s %7s %7s %7s | %7s %7s %7s\n", "kind", "total", "ok", "pending", "low", "lowOK", "lowBad")
	for _, k := range kinds {
		r := rows[k]
		fmt.Printf("%-28s %7d %7d %7d | %7d %7d %7d\n", k, r.Total, r.OK, r.Pending, r.LowTotal, r.LowOK, r.LowBad)
	}
	fmt.Printf("\n== unimplemented low-level samples\n")
	for _, k := range kinds {
		if len(perKind[k]) == 0 {
			continue
		}
		fmt.Printf("  %s\n", k)
		for _, s := range perKind[k] {
			fmt.Println(s)
		}
	}
	fmt.Printf("\n== reward selection: total=%d low-level=%d\n", selectCount, selectLow)
	for _, s := range selectSamples {
		fmt.Println(s)
	}
	fmt.Printf("\n== reward types\n")
	for k, n := range rewardTypes {
		fmt.Printf("  %-20s %d\n", k, n)
	}

	index, e := native.ItemIndex("")
	if e != nil {
		log.Fatal(e)
	}
	dropPolicy, e := inventory.ReadDropPolicy("configs/pvf-drop-policy.json")
	if e != nil {
		log.Fatal(e)
	}
	questCatalog := catalog.QuestCatalog{Source: native.Snapshot(), Quests: map[uint32]catalog.QuestDefinition{}}
	gear, e := native.EquipmentSelection(index, questCatalog, dropPolicy)
	if e != nil {
		log.Fatal(e)
	}
	// Same acceptance test as inventory.EquipmentCatalog.Basic: only free,
	// ordinary-rarity gear with resolvable durability can enter a bag today.
	reasons := map[string]int{}
	byGrade := map[int32]int{}
	byLevel := map[int32]int{}
	usable := 0
	for _, r := range gear.Rows {
		attach, rarity := r.Fields["[attach type]"], r.Fields["[rarity]"]
		kind, dur := r.Fields["[equipment type]"], r.Fields["[durability]"]
		switch {
		case len(attach) != 1 || attach[0].Text != "[free]":
			reasons["attach type is not [free]"]++
			continue
		case len(rarity) != 1 || rarity[0].Type != 0 || rarity[0].Value < 0 || rarity[0].Value > 1:
			reasons["rarity outside 0..1"]++
			continue
		case len(kind) == 0:
			reasons["missing equipment type"]++
			continue
		}
		if len(dur) == 0 {
			t := kind[0].Text
			if t != "[amulet]" && t != "[wrist]" && t != "[ring]" {
				reasons["missing durability"]++
				continue
			}
		} else if len(dur) != 1 || dur[0].Type != 0 || dur[0].Value < 0 || dur[0].Value > 65535 {
			reasons["invalid durability"]++
			continue
		}
		usable++
		if g := r.Fields["[grade]"]; len(g) == 1 && g[0].Type == 0 {
			byGrade[g[0].Value]++
		}
		if l := r.Fields["[minimum level]"]; len(l) == 1 && l[0].Type == 0 {
			byLevel[l[0].Value]++
		}
	}
	fmt.Printf("\n== equipment drop pool candidates (native selected rows=%d bag-usable=%d)\n", len(gear.Rows), usable)
	for _, m := range []struct {
		name string
		data map[int32]int
	}{{"grade", byGrade}, {"minimum level", byLevel}} {
		var keys []int32
		for k := range m.data {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%d:%d", k, m.data[k]))
		}
		fmt.Printf("  by %s: %s\n", m.name, strings.Join(parts, " "))
	}
	fmt.Printf("  rejected:\n")
	for k, n := range reasons {
		fmt.Printf("    %-28s %d\n", k, n)
	}
	if *worldPath != "" {
		w, err := native.World("")
		if err != nil {
			log.Fatal(err)
		}
		reportNPC(w, []int{38, 40}, []uint32{1, 12, 29, 358})
	}
	if *tutorialRoutes != "" {
		reportTutorial(*tutorialRoutes, *tutorialDungeons)
	}
	if *skillManifest != "" {
		reportSkillPanel(*skillManifest, *questPath, 16)
	}
	if *featureSource != "" {
		reportFeatureSource(*featureSource)
		dumpPaths(*featureSource, []string{
			"stackable/potion_sharpeye.stk",
			"stackable/ptn_nbstunrecovery.stk",
			"stackable/cash/river_lethe.stk",
			"equipment/character/swordman/at_avatar/aura/107590017.equ",
		})
	}
	if *verify {
		if n := verifyStartup(native, q); n > 0 {
			os.Exit(1)
		}
	}
}
