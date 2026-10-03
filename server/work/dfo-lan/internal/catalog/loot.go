package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

type LootItem struct {
	ID            uint32 `json:"id"`
	Kind          string `json:"kind"`
	Grade, Rarity int32
	Weight        uint32
	StackableType string `json:"stackable_type,omitempty"`
	StackLimit    uint32 `json:"stack_limit,omitempty"`
	Script        ScriptRecord
}
type LootCatalog struct {
	details      *ScriptDetails[uint32, ScriptRecord]
	Source       pvf.ArchiveSnapshot     `json:"source"`
	MaximumGrade uint32                  `json:"maximum_grade"`
	Rules        map[string]ScriptRecord `json:"rules"`
	Items        map[uint32]LootItem     `json:"items"`
	IndexHashes  map[string]string       `json:"index_hashes"`
	// DropGroups is the typed projection of etc/dungeondroptablebygroup.etc; a
	// dungeon script's [difficulty dropitem group list] indexes into it by id.
	DropGroupSource DropGroupSource `json:"drop_group_source"`
	DropGroups      []DropGroup     `json:"drop_groups,omitempty"`
	// DropGroupsUnreadable lists group ids the parser refused to decode. One
	// group in the shipped table (21469) declares [drop item] with an odd count -
	// eleven bare templates and no weights - which no reading explains, so the id
	// is recorded instead of a weight being invented for it. A consumer that needs
	// such a group has to fail loudly rather than award from a guess.
	DropGroupsUnreadable []uint32 `json:"drop_groups_unreadable,omitempty"`
	// [MERGE-20260928-DUNGEON-DROPINFO] DungeonDropInfo 是 etc/dungeondropinfo.cos
	// 的类型化投影：副本 → 若干条 (品级, 副本类型, 掉落组, 掉落率)。它和 DropGroups
	// 一起构成完整的发放链：本表选组，DropGroups 给物品。
	//
	// 它同时是「哪些副本走深渊」的权威依据：[dungeon type] 只有 `dgn_normal` 和
	// `dgn_hell` 两种取值，深渊只落在 100003295/6/7 三个副本上。
	DungeonDropInfoSource DungeonDropInfoSource             `json:"dungeon_dropinfo_source"`
	DungeonDropInfo       map[uint32][]DungeonDropRateEntry `json:"dungeon_drop_info,omitempty"`
	Skipped               []string                          `json:"skipped,omitempty"`
	Pending               []string                          `json:"pending,omitempty"`
}

func lootInt(c []pvf.Token, name string) (int32, bool) {
	a := sectionCells(c, name)
	if len(a) != 1 || a[0].Type != 0 {
		return 0, false
	}
	return a[0].Value, true
}

// ImportLoot preserves current typed item definitions. Only item grades inside
// the explicitly requested import range are projected into this runtime pool.
func ImportLoot(a *pvf.Archive, maxGrade uint32) (LootCatalog, error) {
	c := LootCatalog{Source: a.Snapshot(), MaximumGrade: maxGrade, Rules: map[string]ScriptRecord{}, Items: map[uint32]LootItem{}, IndexHashes: map[string]string{}}
	if maxGrade == 0 || maxGrade > 200 {
		return c, fmt.Errorf("invalid loot import grade range")
	}
	// [MERGE-20260928-ABYSS-HELL-TABLE] etc/itemdropinfo_monster_hell.etc 是深渊
	// (hell party) 专用的掉落支持表。它自 2026-09-28 起就在 client-build 的 PVF 里
	// (index 5055835，645 cells)，但导入清单一直漏了它 —— 于是深渊副本只能回落到
	// 普通怪的 monseter 表，实测表现是「深渊只掉紫装」。
	//
	// 与 monseter 的差异（只读观察，已实测）：
	//   monseter:      [basis of rarity dicision] 4 行 × 9 列
	//   monster_hell:  [basis of rarity dicision] 2 行 × 9 列，且**没有** [drop prob] 段
	// 即深渊有独立的一套 rarity 判定，不能套用普通怪的表。
	//
	// 这里只做「导入」，不改变任何现有判定：把它放进 Rules 供后续按副本类型选用。
	for _, name := range []string{"etc/itemdropinfo_monseter.etc", "etc/itemdropinfo_common.etc", "etc/itemdropinfo_control.etc", "etc/dungeonbossdrop.etc", "etc/itemdropinfo_monster_hell.etc"} {
		s, e := ResolveScript(a, name)
		if e != nil {
			if name == "etc/itemdropinfo_monster_hell.etc" {
				// 旧快照没有这张表时跳过，而不是让整个 loot 导入失败。
				continue
			}
			return c, e
		}
		c.Rules[name] = s
	}
	// etc/dungeondroptablebygroup.etc is the per-dungeon drop group table a
	// dungeon's [difficulty dropitem group list] indexes into. It is stored as a
	// typed projection instead of raw cells: the cell stream is 85k entries, and
	// repeating it would grow every loot catalog sixfold. Importing it is not the
	// same as awarding from it - the award semantics are still not established, so
	// nothing consumes these groups yet.
	groupTable, e := ResolveScript(a, "etc/dungeondroptablebygroup.etc")
	if e != nil {
		return c, e
	}
	c.DropGroupSource = DropGroupSource{Path: groupTable.Path, SHA256: groupTable.SHA256}
	c.DropGroups, c.DropGroupsUnreadable, e = ParseDropGroups(groupTable.Cells)
	if e != nil {
		return c, e
	}
	// [MERGE-20260928-DUNGEON-DROPINFO] etc/dungeondropinfo.cos 是「副本 → 掉落组」
	// 的索引表。它是 DataType=3 的**文本**（UTF-16LE），不是脚本，所以不能走
	// ResolveScript/Tokens —— 用 ReadRaw 取原始字节再解码。
	//
	// 与 DropGroups 的分工：本表决定「哪个副本走哪些组、什么品级、什么率」，
	// DropGroups 决定「那个组里有哪些物品」。两者拼起来才是完整发放链。
	const dungeonDropInfoPath = "etc/dungeondropinfo.cos"
	if idx := a.FindFileIndex(dungeonDropInfoPath); idx >= 0 {
		raw, e := a.ReadRaw(dungeonDropInfoPath)
		if e != nil {
			return c, e
		}
		info, e := ParseDungeonDropInfo(raw)
		if e != nil {
			return c, e
		}
		sum := sha256.Sum256(raw)
		c.DungeonDropInfo = info
		c.DungeonDropInfoSource = DungeonDropInfoSource{
			Path:   dungeonDropInfoPath,
			SHA256: hex.EncodeToString(sum[:]),
		}
	}
	refs := map[string]map[uint32]string{}
	for _, kind := range []string{"stackable", "equipment"} {
		index, e := ResolveScript(a, "list/"+kind+".lst")
		if e != nil {
			return c, e
		}
		c.IndexHashes[index.Path] = index.SHA256
		rows, e := ParseIndex(index.Cells)
		if e != nil {
			return c, e
		}
		refs[kind] = map[uint32]string{}
		for _, r := range rows {
			refs[kind][r.ID] = r.Path
		}
	}
	read := func(kind string, id uint32) (ScriptRecord, error) {
		p, ok := refs[kind][id]
		if !ok {
			return ScriptRecord{}, fmt.Errorf("item missing source list")
		}
		if !strings.HasPrefix(p, kind+"/") {
			p = path.Join(kind, p)
		}
		return ResolveScript(a, p)
	}
	var ids []uint32
	for id := range refs["stackable"] {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for n, id := range ids {
		if n%1024 == 0 {
			a.ReleaseReadCaches()
		}
		p := strings.TrimPrefix(refs["stackable"][id], "stackable/")
		excluded := false
		for _, prefix := range []string{"cash/", "quest/", "recipe/", "temp/", "event/", "emblem/", "monstercard/"} {
			excluded = excluded || strings.HasPrefix(p, prefix)
		}
		if excluded {
			continue
		}
		s, e := read("stackable", id)
		if e != nil {
			c.Skipped = append(c.Skipped, fmt.Sprintf("stackable %d unreadable", id))
			continue
		}
		rate, ok := lootInt(s.Cells, "[creation rate]")
		if !ok || rate <= 0 {
			continue
		}
		grade, ok := lootInt(s.Cells, "[grade]")
		if !ok || grade <= 0 || uint32(grade) > maxGrade {
			continue
		}
		rarity, _ := lootInt(s.Cells, "[rarity]")
		if rarity < 0 || rarity > 6 {
			continue
		}
		typeCells := sectionCells(s.Cells, "[stackable type]")
		if len(typeCells) == 0 || typeCells[0].Type != 6 {
			continue
		}
		limit, _ := lootInt(s.Cells, "[stack limit]")
		if limit < 0 {
			continue
		}
		c.Items[id] = LootItem{ID: id, Kind: "stackable", Grade: grade, Rarity: rarity, Weight: 1, Script: s, StackableType: typeCells[0].Text, StackLimit: uint32(limit)}
	}
	dictionary, e := ResolveScript(a, "etc/itemdictionary/itemdictionary.etc")
	if e != nil {
		return c, e
	}
	c.IndexHashes[dictionary.Path] = dictionary.SHA256
	// Current dictionary rows contain 16..37 integers: category is column3,
	// whereas 90CN uses column4. Its generation column is not established.
	// Preserve provenance and refuse equipment projection instead of silently
	// treating flags as weights and awarding unrelated items.
	c.Pending = []string{"equipment dictionary generation semantics", "independent/world/explicit monster item pools"}
	return c, nil
}

func LoadLoot(path string) (LootCatalog, error) {
	var c LootCatalog
	b, e := os.ReadFile(path)
	if e != nil {
		return c, e
	}
	if e = json.Unmarshal(b, &c); e != nil {
		return c, e
	}
	return ValidateLoot(c)
}

func ValidateLoot(c LootCatalog) (LootCatalog, error) {
	// [MERGE-20260928-ABYSS-HELL-TABLE] 原来是 len(c.Rules) != 4：加入深渊专用的
	// etc/itemdropinfo_monster_hell.etc 后变成 5 张。放宽为「至少 4 张」，这样旧快照
	// （没有 hell 表）与新快照都能装载，不会因为多导一张支持表就拒绝整份目录。
	if len(c.Source.Checksum) != 64 || c.MaximumGrade == 0 || len(c.Rules) < 4 {
		return c, fmt.Errorf("invalid loot catalog")
	}
	for id, item := range c.Items {
		if id == 0 || id != item.ID || item.Weight == 0 || item.Grade <= 0 || uint32(item.Grade) > c.MaximumGrade || len(item.Script.SHA256) != 64 {
			return c, fmt.Errorf("invalid loot item projection")
		}
	}
	// A catalog that declares a group table source must carry a well-formed
	// projection of it. Catalogs written before the projection existed have
	// neither and still load, so the transition does not break stored artifacts.
	if c.DropGroupSource.SHA256 != "" || len(c.DropGroups) > 0 {
		if c.DropGroupSource.Path == "" || len(c.DropGroupSource.SHA256) != 64 {
			return c, fmt.Errorf("drop groups without source provenance")
		}
		if len(c.DropGroups) == 0 {
			return c, fmt.Errorf("drop group source without groups")
		}
		seen := map[uint32]bool{}
		empty := 0
		for _, g := range c.DropGroups {
			// [MERGE-20260928-EMPTY-DROP-GROUP] 空组（两段都没有物品）是**源表的正常
			// 形态**，不是损坏：dungeondroptablebygroup.etc 里有 479 个这样的块，形如
			//
			//	[group] 20363
			//	[creation rate] 0 0
			//	[smart drop item]    ← 空
			//	[/smart drop item]
			//
			// 原校验把它们判成 invalid drop group，于是**只要目录里带组就装载失败** ——
			// 旧文件 drop_groups=0 所以从未暴露，重新导入（1221 组）后立刻炸。
			// 空组只表示「这一组不产出物品」，与 ID==0 或重复 ID 这类真损坏不同。
			if g.ID == 0 || seen[g.ID] {
				return c, fmt.Errorf("invalid drop group %d", g.ID)
			}
			if len(g.Explicit) == 0 && len(g.Smart) == 0 {
				empty++
			}
			seen[g.ID] = true
			for _, row := range [2][]DropWeight{g.Explicit, g.Smart} {
				for _, w := range row {
					if w.Template == 0 {
						return c, fmt.Errorf("invalid drop group item")
					}
				}
			}
		}
		if empty > 0 {
			// 空组数量写进 Skipped 供排查，不当作错误。
			c.Skipped = append(c.Skipped, fmt.Sprintf("empty_drop_groups:%d", empty))
		}
	}
	return c, nil
}

// SupplementStackables supplements the catalog with stackable definitions
// from an items.index.json file for inventory/quickslot/consume operations,
// without overriding any existing monster drop items.
func (c *LootCatalog) SupplementStackables(path string) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	var doc struct {
		Source struct {
			Checksum string `json:"checksum"`
		} `json:"source"`
		Items map[string]struct {
			ID            uint32 `json:"id"`
			Kind          string `json:"kind"`
			Path          string `json:"path"`
			StackableType string `json:"stackable_type"`
			StackLimit    uint32 `json:"stack_limit"`
		} `json:"items"`
	}
	dec := json.NewDecoder(f)
	if err := dec.Decode(&doc); err != nil {
		return err
	}
	if doc.Source.Checksum != "" && c.Source.Checksum != "" && doc.Source.Checksum != c.Source.Checksum {
		return fmt.Errorf("items index source mismatch: got %s want %s", doc.Source.Checksum, c.Source.Checksum)
	}

	if c.Items == nil {
		c.Items = make(map[uint32]LootItem, len(doc.Items))
	}
	for _, it := range doc.Items {
		if it.Kind != "stackable" || it.ID == 0 {
			continue
		}
		if _, exists := c.Items[it.ID]; !exists {
			c.Items[it.ID] = LootItem{
				ID:            it.ID,
				Kind:          "stackable",
				StackableType: it.StackableType,
				StackLimit:    it.StackLimit,
				Script:        ScriptRecord{Path: it.Path},
			}
		}
	}
	return nil
}
