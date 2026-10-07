// setpointdiag 是只读诊断：把「套装积分」在一件模板上的完整计算链打出来。
//
// 为什么需要它：套装积分有两层映射，任一层搞错都会让积分静默变 0 ——
//
//	模板 --(etc/equipmentgrouping.etc 的 [ability group])--> 能力组号
//	能力组号 + 调适档位 --(etc/115lvability/setpointinfo.cos 的 [info])--> 每件积分
//
// 第二层的 `[part set index]` 是**积分归属的套装号**，值为 -1 时用该件 `.equ` 自己的
// `[part set index]` 补上（internal/character/fame.go：`if id == -1 { id = set }`）。
// 所以「一件装备算几分」不能只看 setpointinfo.cos 的 `[group]` 字面量。
package setpointdiag

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

type options struct {
	source  string
	tokens  []uint32
	verbose bool
}

// Run 实现 dfo-tool setpointdiag。与其它子命令一致，用全局 flag 包
// （调度器已把命令名从 os.Args 里切掉）。
func Run() {
	source := flag.String("source", "runtime/pvf_source/Script.inner.pvf", "read-only inner archive")
	list := flag.String("templates", "", "comma separated equipment templates to explain")
	verbose := flag.Bool("v", false, "list every ability group membership")
	flag.Parse()
	if *list == "" {
		fmt.Fprintln(os.Stderr, "setpointdiag: -templates is required (comma separated template ids)")
		os.Exit(2)
	}
	var tokens []uint32
	for _, part := range strings.Split(*list, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseUint(part, 10, 32)
		if err != nil {
			fmt.Fprintf(os.Stderr, "setpointdiag: bad template %q: %v\n", part, err)
			os.Exit(2)
		}
		tokens = append(tokens, uint32(id))
	}
	if err := run(options{source: *source, tokens: tokens, verbose: *verbose}); err != nil {
		fmt.Fprintf(os.Stderr, "setpointdiag: %v\n", err)
		os.Exit(1)
	}
}

func run(o options) error {
	a, err := pvf.LoadArchive(pvf.Options{Path: o.source, MaxBytes: 1024 * 1024 * 1024})
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

	fmt.Printf("来源 %s\n", rules.Source.Checksum)
	fmt.Printf("setpointinfo.cos %d 字节 sha256=%s\n", rules.SetPointBytes, rules.SetPointSHA256)
	fmt.Printf("规则 %d 行，能力组 %d 个\n\n", len(rules.Set.Rules), len(groups))

	for _, id := range o.tokens {
		explain(equipment, rules, groups, id, o.verbose)
	}
	return nil
}

func explain(equipment *inventory.FullEquipmentCatalog, rules catalog.PointRules, groups map[uint32][]uint32, id uint32, verbose bool) {
	fmt.Printf("=== %d ===\n", id)
	d, err := equipment.Definition(id)
	if err != nil {
		fmt.Printf("  源里没有这件装备：%v\n\n", err)
		return
	}
	fmt.Printf("  rarity=%s  part-set-index=%s  awakening-option=%d  type=%s\n",
		firstNumeric(d.Fields["[rarity]"]), firstNumeric(d.Fields["[part set index]"]),
		len(d.Fields["[equipment awakening option]"]), firstText(d.Fields["[equipment type]"]))

	member := groups[id]
	if len(member) == 0 {
		fmt.Printf("  能力组：**不在任何 [ability group] 里** ⇒ 套装积分恒 0（无论调适写几）\n\n")
		return
	}
	sort.Slice(member, func(i, j int) bool { return member[i] < member[j] })
	fmt.Printf("  能力组：%v\n", member)

	for _, rank := range []int32{0, 1, 2, 3} {
		total := map[int32]int{}
		hits := 0
		for _, g := range member {
			for _, r := range rules.Set.Rules {
				if r.Group != g || r.Awakening != rank {
					continue
				}
				owner := r.PartSetIndex
				if owner == -1 {
					owner = int32(firstNumericValue(d.Fields["[part set index]"]))
				}
				if owner <= 0 {
					continue
				}
				total[owner] += int(r.Value)
				hits++
			}
		}
		if hits == 0 {
			fmt.Printf("  调适 %d：能力组在表里**没有**该档位的行 ⇒ 0 分\n", rank)
			continue
		}
		owners := make([]int, 0, len(total))
		for owner := range total {
			owners = append(owners, int(owner))
		}
		sort.Ints(owners)
		for _, owner := range owners {
			fmt.Printf("  调适 %d：归属套装 %d → 每件 %d 分\n", rank, owner, total[int32(owner)])
		}
	}
	if verbose {
		fmt.Printf("  全部命中行：\n")
		for _, g := range member {
			for _, r := range rules.Set.Rules {
				if r.Group == g {
					fmt.Printf("    group=%d awakening=%d part-set-index=%d value=%d\n",
						r.Group, r.Awakening, r.PartSetIndex, r.Value)
				}
			}
		}
	}
	fmt.Println()
}

// importAbilityGroups 读 etc/equipmentgrouping.etc 的 `[ability group]` 块，
// 返回 模板 -> 能力组号。字段布局与 internal/character/fame_native.go 一致：
// `[index] <组号>` 后面跟 `[list] <模板…> [/list]`。
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
	if len(out) == 0 {
		return nil, fmt.Errorf("equipmentgrouping.etc has no [ability group] membership")
	}
	return out, nil
}

func firstNumeric(ts []pvf.Token) string {
	if len(ts) == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", ts[0].Value)
}

func firstNumericValue(ts []pvf.Token) int64 {
	if len(ts) == 0 {
		return -1
	}
	return int64(ts[0].Value)
}

func firstText(ts []pvf.Token) string {
	if len(ts) == 0 {
		return "-"
	}
	return ts[0].Text
}
