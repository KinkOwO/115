// setpointscan 是只读诊断：把「能力组 → 每件积分」的两层映射反过来打一遍。
//
// 用途：某个分数（例如星蕴石的 195）到底来自哪个模板 / 能力组。
//
//	能力组 --(setpointinfo.cos 的 [info])--> 每件积分
//	模板   --(equipmentgrouping.etc 的 [ability group])--> 能力组
package setpointscan

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// Run 实现 dfo-tool setpointscan。
func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner archive")
	point := flag.Int("point", 0, "only report templates whose awakening-3 per-piece score equals this")
	group := flag.Int("group", -1, "only report membership of this ability group")
	templates := flag.String("templates", "", "extra templates to explain verbatim")
	profitable := flag.Bool("profitable", false, "list every template that gains points from awakening 3 (the only ones worth writing tune 3)")
	flag.Parse()

	if err := run(*source, *point, *group, *templates, *profitable); err != nil {
		fmt.Fprintf(os.Stderr, "setpointscan: %v\n", err)
		os.Exit(1)
	}
}

func run(source string, wantPoint, wantGroup int, templateList string, profitableOnly bool) error {
	a, err := pvf.LoadArchive(pvf.Options{Path: source, MaxBytes: 1024 * 1024 * 1024})
	if err != nil {
		return err
	}
	defer a.Close()
	rules, err := catalog.ImportPointRules(a)
	if err != nil {
		return err
	}
	groups, err := importAbilityGroups(a)
	if err != nil {
		return err
	}
	index, err := catalog.ImportItemIndex(a)
	if err != nil {
		return err
	}
	equipment, err := inventory.OpenPVFEquipmentCatalog(a, index)
	if err != nil {
		return err
	}
	defer equipment.Close()

	// 每个能力组在调适 0..3 下、按 `-1 → 用本件 [part set index]` 之后能拿到的分。
	byGroup := map[uint32]map[int32][]catalog.SetPointRule{}
	for _, r := range rules.Set.Rules {
		if byGroup[r.Group] == nil {
			byGroup[r.Group] = map[int32][]catalog.SetPointRule{}
		}
		byGroup[r.Group][r.Awakening] = append(byGroup[r.Group][r.Awakening], r)
	}

	if wantGroup >= 0 {
		ids := make([]uint32, 0)
		for id, gs := range groups {
			for _, g := range gs {
				if int(g) == wantGroup {
					ids = append(ids, id)
					break
				}
			}
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		fmt.Printf("能力组 %d 的成员（%d 件）：\n", wantGroup, len(ids))
		for _, id := range ids {
			report(equipment, id, groups, byGroup)
		}
		fmt.Println()
	}

	if wantPoint > 0 {
		fmt.Printf("调适 3 时每件 == %d 分的模板：\n", wantPoint)
		found := 0
		ids := make([]uint32, 0, len(groups))
		for id := range groups {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			d, err := equipment.Definition(id)
			if err != nil {
				continue
			}
			owner := int32(0)
			if len(d.Fields["[part set index]"]) > 0 {
				owner = d.Fields["[part set index]"][0].Value
			}
			if scoreFor(groups[id], byGroup, 3, owner) == wantPoint {
				found++
				report(equipment, id, groups, byGroup)
			}
		}
		fmt.Printf("共 %d 件\n\n", found)
	}

	if profitableOnly {
		fmt.Printf("「写调适 3 能加分」的全部模板（按 部位/稀有度/能力组家族 汇总）：\n")
		ids := sortedGroupTemplates(groups)
		type key struct {
			part   string
			rarity int32
			family uint32
		}
		count := map[key]int{}
		samples := map[key]uint32{}
		total := 0
		for _, id := range ids {
			d, err := equipment.Definition(id)
			if err != nil {
				continue
			}
			owner := partSetOf(d)
			if scoreFor(groups[id], byGroup, 3, owner) <= scoreFor(groups[id], byGroup, 0, owner) {
				continue
			}
			rarity := int32(-1)
			if len(d.Fields["[rarity]"]) > 0 {
				rarity = d.Fields["[rarity]"][0].Value
			}
			// family = 该件在 setpointinfo 里命中的最小能力组（190 家族 = 265 分那一族）。
			fam := uint32(0)
			for _, g := range groups[id] {
				if _, ok := byGroup[g]; ok {
					if fam == 0 || g < fam {
						fam = g
					}
				}
			}
			k := key{text(d.Fields["[equipment type]"]), rarity, fam}
			count[k]++
			if _, ok := samples[k]; !ok {
				samples[k] = id
			}
			total++
		}
		keys := make([]key, 0, len(count))
		for k := range count {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if keys[i].part != keys[j].part {
				return keys[i].part < keys[j].part
			}
			if keys[i].rarity != keys[j].rarity {
				return keys[i].rarity < keys[j].rarity
			}
			return keys[i].family < keys[j].family
		})
		for _, k := range keys {
			d, _ := equipment.Definition(samples[k])
			fmt.Printf("  %-22s rarity=%-2d 家族=%-4d n=%-4d 例=%d 调适0=%d 调适3=%d\n",
				k.part, k.rarity, k.family, count[k], samples[k],
				scoreFor(groups[samples[k]], byGroup, 0, partSetOf(d)),
				scoreFor(groups[samples[k]], byGroup, 3, partSetOf(d)))
		}
		fmt.Printf("共 %d 件\n\n", total)
	}

	for _, part := range strings.Split(templateList, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			return err
		}
		report(equipment, uint32(id), groups, byGroup)
	}
	return nil
}

func sortedGroupTemplates(groups map[uint32][]uint32) []uint32 {
	ids := make([]uint32, 0, len(groups))
	for id := range groups {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func partSetOf(d inventory.EquipmentDefinition) int32 {
	if len(d.Fields["[part set index]"]) == 0 {
		return 0
	}
	return d.Fields["[part set index]"][0].Value
}

func scoreFor(gs []uint32, byGroup map[uint32]map[int32][]catalog.SetPointRule, rank int32, owner int32) int {
	total := 0
	for _, g := range gs {
		for _, r := range byGroup[g][rank] {
			o := r.PartSetIndex
			if o == -1 {
				o = owner
			}
			if o > 0 {
				total += int(r.Value)
			}
		}
	}
	return total
}

func report(equipment *inventory.FullEquipmentCatalog, id uint32, groups map[uint32][]uint32, byGroup map[uint32]map[int32][]catalog.SetPointRule) {
	d, err := equipment.Definition(id)
	if err != nil {
		fmt.Printf("  %d 源里没有这件装备: %v\n", id, err)
		return
	}
	owner := int32(0)
	if len(d.Fields["[part set index]"]) > 0 {
		owner = d.Fields["[part set index]"][0].Value
	}
	rarity := int32(-1)
	if len(d.Fields["[rarity]"]) > 0 {
		rarity = d.Fields["[rarity]"][0].Value
	}
	fmt.Printf("  %-10d type=%-16s rarity=%-2d part-set=%-6d 能力组=%-14v 调适0=%-5d 调适3=%-5d awakening-option=%d\n",
		id, text(d.Fields["[equipment type]"]), rarity, owner, groups[id],
		scoreFor(groups[id], byGroup, 0, owner), scoreFor(groups[id], byGroup, 3, owner),
		len(d.Fields["[equipment awakening option]"]))
}

func text(ts []pvf.Token) string {
	if len(ts) == 0 {
		return "-"
	}
	return ts[0].Text
}

// importAbilityGroups 与 setpointdiag 同一口径：`[index] <组号>` + `[list] <模板…> [/list]`。
func importAbilityGroups(a *pvf.Archive) (map[uint32][]uint32, error) {
	script, err := catalog.ResolveScript(a, "etc/equipmentgrouping.etc")
	if err != nil {
		return nil, err
	}
	out := map[uint32][]uint32{}
	var group int64 = -1
	pendingIndex, inList := false, false
	for _, t := range script.Cells {
		if t.Type == 3 {
			switch t.Text {
			case "[ability group]", "[/ability group]":
				group, pendingIndex, inList = -1, false, false
			case "[index]":
				pendingIndex, inList = true, false
			case "[list]":
				inList = true
			case "[/list]":
				inList = false
			}
			continue
		}
		if t.Type != 0 {
			continue
		}
		if pendingIndex {
			group, pendingIndex = int64(t.Value), false
			continue
		}
		if inList && group >= 0 && t.Value > 0 {
			id := uint32(t.Value)
			out[id] = append(out[id], uint32(group))
		}
	}
	return out, nil
}
