package loot

import (
	"encoding/binary"
	"fmt"
	"sort"

	"dfolan/internal/catalog"
)

// [MERGE-20260928-DUNGEON-GROUP-DROP] 按 etc/dungeondropinfo.cos 的索引发放副本掉落。
//
// 这是「对标客户端」的实际执行端。它把三层数据串起来：
//
//	etc/dungeondropinfo.cos       副本 → [(品级, 副本类型, 掉落组, 率)]
//	etc/dungeondroptablebygroup   掉落组 → [物品 + 权重]
//	etc/itemdropinfo_monster_hell 深渊专用的 rarity 判定（导入进 Rules，供上层选用）
//
// 与 RollWithBonus 的分工：RollWithBonus 走**全局**表（`[drop prob]` + `[basis of
// rarity dicision]` + `[item drop ref table]`），是所有副本共用的底；本函数走**副本
// 显式声明**的组索引，只在副本确实带了 `[difficulty dropitem group list]` 时启用。
// 两者不互相替代。
//
// 语义依据（全部来自只读观察，不做发明）：
//
//   - `[type]`        品级标签（epic / stackable / unique / legendary / rare / special）
//   - `[drop group]`  组号，索引 DropGroups
//   - `[rate list]`   键是怪物类别（`boss` / `named` / `normal` ...），值是若干档率
//   - 判定顺序：先按怪物类别选行，再按难度索引进该行的档位；档位是百万空间阈值。
//
// 未确立的部分一律不猜：档位个数不假设（照抄源里的宽度），超出宽度返回错误而不是
// 取模回绕。

// DungeonGroupDropRequest 是一次副本掉落判定的输入。
type DungeonGroupDropRequest struct {
	// DungeonID 是副本 ID，用于查 dungeondropinfo。
	DungeonID uint32
	// MonsterKind 是怪物类别，对应 [rate list] 的键（如 "boss" / "named" / "normal"）。
	MonsterKind string
	// Difficulty 是难度档索引（0 起），索引 [rate list] 的档位。
	Difficulty int
	// Rarity 是本只怪物实际 roll 出的品级索引；用于挑选同品级的组。
	// 传 -1 表示不按品级过滤（取该副本的全部组）。
	Rarity int
}

// groupRateBase 是 `[rate list]` 的率空间。
//
// dungeondropinfo 的率用**百万**空间：深渊组 10900 的率写的是 110000，即 11%。它和
// `[drop prob]` 的 10000 空间**不是**同一套 —— 两者只差一个乘数，混用会让深渊掉落
// 要么整体报错（rate 超分母）、要么率被放大 100 倍。所以这里写死百万，不借用
// Rules.Denominator。
const groupRateBase = 1000000

// RollDungeonGroups 按副本声明的组索引发放一次掉落。
//
// 返回值：
//   - Outcome.Awards 是本次产出的物品（Template + Amount）
//   - Outcome.SkippedKinds 记录跳过的原因，便于从日志定位
//   - 副本没有 dungeondropinfo 条目（大多数副本走全局表）时返回 ok=false
func RollDungeonGroups(cat catalog.LootCatalog, r Rules, seed uint32, req DungeonGroupDropRequest) (Outcome, bool, error) {
	return rollDungeonGroups(cat, r, seed, req, false)
}

// advanceSelection is enabled for ordinary drops. The existing Abyss reward
// sequence remains unchanged until that separately confirmed pool is audited.
func rollDungeonGroups(cat catalog.LootCatalog, r Rules, seed uint32, req DungeonGroupDropRequest, advanceSelection bool) (Outcome, bool, error) {
	var out Outcome
	out.NextSeed = seed

	entries, ok := cat.DropInfoByID(req.DungeonID)
	if !ok || len(entries) == 0 {
		// 该副本不声明组索引，交给全局表。这不是错误。
		return out, false, nil
	}
	_ = r

	rng := RNG{seed}
	produced := false
	for _, en := range entries {
		if req.Rarity >= 0 && en.DropGroup != 0 {
			// 品级过滤：只挑与本次 roll 出的品级一致的组。
			if !gradeMatchesRarity(en.Grade, req.Rarity) {
				continue
			}
		}
		rates, ok := en.RateList[req.MonsterKind]
		if !ok || len(rates) == 0 {
			out.SkippedKinds = append(out.SkippedKinds,
				fmt.Sprintf("no rate for kind %q (group %d grade %s)", req.MonsterKind, en.DropGroup, en.Grade))
			continue
		}
		if req.Difficulty < 0 || req.Difficulty >= len(rates) {
			// 档位宽度照抄源里的个数，不取模回绕。
			return out, true, fmt.Errorf("dungeon %d group %d kind %q: difficulty %d outside %d steps",
				req.DungeonID, en.DropGroup, req.MonsterKind, req.Difficulty, len(rates))
		}
		threshold := rates[req.Difficulty]
		if threshold > groupRateBase {
			return out, true, fmt.Errorf("dungeon %d group %d: rate %d exceeds the %d-space",
				req.DungeonID, en.DropGroup, threshold, groupRateBase)
		}
		if threshold == 0 {
			continue
		}
		if rng.Next(groupRateBase) >= threshold {
			continue
		}
		if en.DropGroup == 0 {
			// 组号 0 表示这一档不指向具体组，跳过而不是猜。
			out.SkippedKinds = append(out.SkippedKinds,
				fmt.Sprintf("group 0 with non-zero rate (grade %s)", en.Grade))
			continue
		}
		group, ok := cat.DropGroupByID(en.DropGroup)
		if !ok {
			// 组不可读时不发任何东西 —— 宁可不掉，也不从一个猜测里发奖励。
			out.SkippedKinds = append(out.SkippedKinds,
				fmt.Sprintf("group %d not readable", en.DropGroup))
			continue
		}
		pick, ok := selectGroupItem(&rng, group, advanceSelection)
		if !ok {
			out.SkippedKinds = append(out.SkippedKinds,
				fmt.Sprintf("group %d has no weighted item", en.DropGroup))
			continue
		}
		out.Awards = append(out.Awards, Award{Template: pick, Amount: 1})
		produced = true
	}
	_ = produced
	out.NextSeed = rng.Seed
	return out, true, nil
}

// replaceItemAwards 保留全局表给出的金币（Template==0），把它的物品产出换成组产出。
//
// 为什么必须换而不是叠加：全局表和组是从**同一份**掉落预算里二选一的两种算法。叠加会
// 变成双倍掉落（全局表的中低概率物品 + 组的自选物品）。金币是全局表独有的（组表里没有
// 金币条目），所以留下。
//
// from 为空时**原样返回 base**：这说明组一个物品都没抽出来（组不可读、组是空组、或目录
// 里根本没有组表 —— loot.next25.json 就是最后这种）。此时清掉全局表的物品等于把掉落
// 变成「只掉金币」，比不启用组还糟。回退到全局表是安全方向：宁可多给，不可清零。
//
// 用新 slice 而不是 base[:0]：from 有可能与 base 共享底层数组（组产出若直接复用），
// 原地覆写会把还没读到的元素冲掉。
func replaceItemAwards(base, from []Award) []Award {
	if len(from) == 0 {
		return base
	}
	return replaceDeclaredItemAwards(base, from)
}

// A declared ordinary pool owns the item result, including a probability miss
// or an empty pool. Retaining generic items here bypasses the map's drop rate.
func replaceDeclaredItemAwards(base, from []Award) []Award {
	kept := make([]Award, 0, len(base)+len(from))
	for _, a := range base {
		if a.Template == 0 {
			kept = append(kept, a)
		}
	}
	return append(kept, from...)
}

// RollDeclaredGroups 从副本自己声明的组号里抽物品。
// 与 dungeondropinfo 的区别：后者带品级/率，是「什么品级、什么怪、按什么率出哪个组」；
// 前者只是一串组号，没有品级和率 —— 率和品级由**怪物侧**的全局表（RollWithBonus）先
// 定，组只负责把「出什么物品」从全局池换成副本自选的池。
//
// **count 是必须的**：组不决定「掉几件」，只决定「掉什么」。没有这层预算约束时，每只
// 小怪都会把每个声明的组各抽一件（100005014 声明 2 组 → 每只小怪必掉 2 件，实机
// 2026-09-28 表现为「普通小怪爆了一地」，13 只小怪 26 件/把）。件数由全局表先 roll
// 出来，这里只按那个件数抽。count <= 0 时不产出任何东西。
func RollDeclaredGroups(cat catalog.LootCatalog, groupIDs []uint32, count int, seed uint32) (Outcome, int, error) {
	return rollDeclaredGroups(cat, groupIDs, count, seed, false)
}

func rollDeclaredGroups(cat catalog.LootCatalog, groupIDs []uint32, count int, seed uint32, advanceSelection bool) (Outcome, int, error) {
	var out Outcome
	out.NextSeed = seed
	if count <= 0 || len(groupIDs) == 0 {
		return out, 0, nil
	}
	rng := RNG{seed}
	skipped := 0
	produced := 0
	for _, gid := range groupIDs {
		if produced >= count {
			break
		}
		group, ok := cat.DropGroupByID(gid)
		if !ok {
			// 组不可读时不发任何东西 —— 宁可不掉，也不从一个猜测里发奖励。
			out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("declared group %d not readable", gid))
			skipped++
			continue
		}
		pick, ok := selectGroupItem(&rng, group, advanceSelection)
		if !ok {
			// 空组（源表里有 479 个）就是「这一组不产出」，不是错误。
			out.SkippedKinds = append(out.SkippedKinds, fmt.Sprintf("declared group %d is empty", gid))
			skipped++
			continue
		}
		out.Awards = append(out.Awards, Award{Template: pick, Amount: 1})
		produced++
	}
	out.NextSeed = rng.Seed
	return out, skipped, nil
}

// gradeMatchesRarity 把 dungeondropinfo 的 [type] 标签映射到 rarity 索引。
//
// 映射来自三级交叉证据：
//   - omen.go 的注释明确写过「档位 粉/传说/史诗/太初 等于 unique/legendary/epic/primeval」
//   - dungeondropinfo 里出现的标签只有 rare/unique/epic/legendary/special/stackable
//   - equipment.current37.json 的 [rarity] 取值 0..6
//
// 只映射有把握的；没把握的返回 false，让调用方跳过而不是发错。
func gradeMatchesRarity(grade string, rarity int) bool {
	switch grade {
	case "rare":
		return rarity == 1
	case "unique":
		return rarity == 2
	case "legendary":
		return rarity == 3
	case "epic":
		return rarity == 4
	case "special":
		// special 是深渊/特殊装备标签，不与普通 rarity 档一一对应。
		return false
	case "stackable":
		// stackable 不是装备品级，任何 rarity 都可能配它。
		return true
	default:
		return false
	}
}

// pickFromGroup 按权重从组里挑一件。
//
// 这是既有兼容选择算法。115 reader 147426050 将 Explicit 的两种 creation
// 分支与 Smart 存入三个不同 map；直接合并并非已确认的原生行为。完整发奖
// 分支尚待取证，本轮仅修普通路径的随机状态传播，保留排除范围的历史序列。
func pickFromGroup(rng RNG, g catalog.DropGroup) (uint32, bool) {
	return pickFromGroupWithSeed(&rng, g)
}

func selectGroupItem(rng *RNG, g catalog.DropGroup, advance bool) (uint32, bool) {
	if !advance {
		return pickFromGroup(*rng, g)
	}
	return pickFromGroupWithSeed(rng, g)
}

func pickFromGroupWithSeed(rng *RNG, g catalog.DropGroup) (uint32, bool) {
	type row struct {
		tpl uint32
		w   uint32
	}
	var rows []row
	var total uint64
	for _, e := range g.Explicit {
		if e.Template == 0 || e.Weight == 0 {
			continue
		}
		rows = append(rows, row{e.Template, e.Weight})
		total += uint64(e.Weight)
	}
	for _, e := range g.Smart {
		if e.Template == 0 || e.Weight == 0 {
			continue
		}
		rows = append(rows, row{e.Template, e.Weight})
		total += uint64(e.Weight)
	}
	if len(rows) == 0 || total == 0 {
		return 0, false
	}
	// 稳定顺序：同权重下按 Template 升序，避免上层顺序影响结果。
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].w != rows[j].w {
			return rows[i].w < rows[j].w
		}
		return rows[i].tpl < rows[j].tpl
	})
	// RNG.Next 只吃 uint32；总权重超出时钳到 uint32 上限。源表的权重都是几百量级，
	// 组内物品数最多约 800，实际不会触到上限，钳位只是防御。
	span := total
	if span > 0xffffffff {
		span = 0xffffffff
	}
	pick := uint64(rng.Next(uint32(span)))
	var acc uint64
	for _, r := range rows {
		acc += uint64(r.w)
		if pick < acc {
			return r.tpl, true
		}
	}
	return rows[len(rows)-1].tpl, true
}

// dungeonGroupSeed 从 RunID + 怪物实体派生一个稳定种子，供 wireprobe 层调用。
func dungeonGroupSeed(runID string, entity uint16) uint32 {
	h := uint32(2166136261)
	for i := 0; i < len(runID); i++ {
		h ^= uint32(runID[i])
		h *= 16777619
	}
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], entity)
	h ^= uint32(b[0]) | uint32(b[1])<<8
	h *= 16777619
	return h
}

// DungeonGroupSeed 是 dungeonGroupSeed 的导出形式。
func DungeonGroupSeed(runID string, entity uint16) uint32 { return dungeonGroupSeed(runID, entity) }
