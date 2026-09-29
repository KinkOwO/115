package inventory

import (
	"dfolan/internal/catalog"
	"encoding/json"
	"fmt"
	"sort"
)

// 装备库（装备图鉴）账本。
//
// 为什么放在角色 State JSON 里而不是新开 SQL 表：
//   - CMD26 的既有事务入口是 storage.CommitCharacterEvent(…, apply func(Character) (state, receipt, error))，
//     它**只**提交角色状态与回执。账本要与之"同生共死"（分解扣物、发材料、写收录、写回执一起提交），
//     放进 State 就自动在同一个事务里，不需要新的 Store 方法，也不会出现"扣了装备但收录没写"的半状态。
//   - 老存档没有这个键 ⇒ ReadEquipmentJournal 读出空账本，天然向前兼容，无需迁移脚本。
//
// 语义（两处真源：规格 CMD/0026-DISJOINTITEM、源文件 equipmentsetjournal.cos）：
//   - Counts 是**绝对份数**（不是增量），按模板号索引。
//   - 收录资格与上限由 catalog.EquipmentJournalRules 回答（minLevel==115 ∧ rarity∈{2,3,4,6,8}）。
//   - Favorites 是"按类别的界面分组 ID 列表"，与 Counts 无关：可以收藏尚未收录的分组。
const (
	EquipmentJournalKey     = "equipment_journal"
	equipmentJournalVersion = "equipment-journal-v1"

	// JournalFavoriteSlots 是 2610 尾部每类的槽位数（5 类 × 3 槽 × u32 = 60 B）。
	JournalFavoriteSlots = 3
	// journalFavoriteStorage 是我们**按原样保存**的槽位数。
	// 2264 请求每类带 4 个 u32（偏移 16/20/24/28），实测 8/8 样本第 4 槽恒 0；
	// 2610 只回填 3 槽。多出来的那一槽不丢（存下来），但不参与编码 —— 等知道它是什么再处理。
	journalFavoriteStorage = 4
	// journalMaxCategory 是收藏类别上限（源里 [set mark] 取 0..4，共 5 类）。
	journalMaxCategory = 4
)

// EquipmentJournal 是角色级的装备库账本。
type EquipmentJournal struct {
	Version string `json:"version"`
	// Counts 模板号 → 绝对收录份数。份数为 0 的记录**必须保留**：
	// 客户端那份 2048 行快照里"已登记但为 0"与"从未登记"是两种状态（规格 2610 §已推翻）。
	Counts map[uint32]uint32 `json:"counts,omitempty"`
	// Favorites 收藏类别 → 该类的界面分组 ID（升序、无重复、无 0）。
	Favorites map[uint32][]uint32 `json:"favorites,omitempty"`
}

// ReadEquipmentJournal 读角色 State 里的账本。缺键 = 空账本（老档）。
func ReadEquipmentJournal(state json.RawMessage) (EquipmentJournal, error) {
	out := EquipmentJournal{Version: equipmentJournalVersion}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return out, e
	}
	raw, ok := fields[EquipmentJournalKey]
	if !ok {
		return out, nil
	}
	if e := json.Unmarshal(raw, &out); e != nil {
		return out, fmt.Errorf("equipment journal: %w", e)
	}
	if out.Version == "" {
		out.Version = equipmentJournalVersion
	}
	if out.Version != equipmentJournalVersion {
		return out, fmt.Errorf("unsupported saved equipment journal %q", out.Version)
	}
	return out, nil
}

// SaveEquipmentJournal 把账本写回角色 State，**保留其它未知键**（与 SaveBag 同一约定）。
func SaveEquipmentJournal(state json.RawMessage, j EquipmentJournal) (json.RawMessage, error) {
	if j.Version == "" {
		j.Version = equipmentJournalVersion
	}
	var fields map[string]json.RawMessage
	if e := json.Unmarshal(state, &fields); e != nil {
		return nil, e
	}
	if fields == nil {
		return nil, fmt.Errorf("character state is not an object")
	}
	raw, e := json.Marshal(j)
	if e != nil {
		return nil, e
	}
	fields[EquipmentJournalKey] = raw
	return json.Marshal(fields)
}

// clone 深拷贝两个 map。值接收者的方法必须 clone：否则调用方拿到的"新账本"与原账本共享底层 map，
// 一处失败会污染另一处。
func (j EquipmentJournal) clone() EquipmentJournal {
	out := EquipmentJournal{Version: j.Version}
	if out.Version == "" {
		out.Version = equipmentJournalVersion
	}
	if j.Counts != nil {
		out.Counts = make(map[uint32]uint32, len(j.Counts))
		for k, v := range j.Counts {
			out.Counts[k] = v
		}
	}
	if j.Favorites != nil {
		out.Favorites = make(map[uint32][]uint32, len(j.Favorites))
		for k, v := range j.Favorites {
			out.Favorites[k] = append([]uint32(nil), v...)
		}
	}
	return out
}

// Add 把 template 的绝对份数增加 amount，上限 limit。
//
// 超上限**报错**，由调用方整批回滚 —— 不做截断。理由：截断会变成"装备扣了、收录没记满"的半提交，
// 而客户端那份快照是按绝对份数渲染的，半提交在界面上看不出来却已经吃掉了玩家的装备。
func (j EquipmentJournal) Add(template uint32, amount, limit uint32) (EquipmentJournal, uint32, error) {
	if template == 0 {
		return j, 0, fmt.Errorf("equipment journal: template 0 is never registrable")
	}
	cur := j.Counts[template]
	if amount == 0 {
		return j, cur, nil
	}
	if limit == 0 {
		return j, cur, fmt.Errorf("equipment journal: template %d is not registrable", template)
	}
	if uint64(cur)+uint64(amount) > uint64(limit) {
		return j, cur, fmt.Errorf("equipment journal: template %d would pass its %d cap (have %d, add %d)",
			template, limit, cur, amount)
	}
	out := j.clone()
	if out.Counts == nil {
		out.Counts = map[uint32]uint32{}
	}
	out.Counts[template] = cur + amount
	return out, cur + amount, nil
}

// ReplaceFavorites 覆盖一个类别的收藏列表（2264 是"全量替换"，不是增量）。
func (j EquipmentJournal) ReplaceFavorites(category uint32, slots []uint32) (EquipmentJournal, []uint32, error) {
	if category > journalMaxCategory {
		return j, nil, fmt.Errorf("equipment journal: favorite category %d is out of range", category)
	}
	normalized, e := normalizeJournalSlots(slots)
	if e != nil {
		return j, nil, e
	}
	out := j.clone()
	if out.Favorites == nil {
		out.Favorites = map[uint32][]uint32{}
	}
	out.Favorites[category] = normalized
	return out, normalized, nil
}

// FavoritesFor 取某类别的收藏列表（编码 2610 用；永远是 ≤3 项、升序、无 0）。
func (j EquipmentJournal) FavoritesFor(category uint32) []uint32 {
	slots := j.Favorites[category]
	if len(slots) > JournalFavoriteSlots {
		// 2610 每类只有 3 槽。客户端实测从不发第 4 个非零值；真出现时只记诊断，
		// 不在这里拒绝（拒绝会让 2264 无回包 = 玩家"点了没反应"）。
		slots = slots[:JournalFavoriteSlots]
	}
	return append([]uint32(nil), slots...)
}

// ExtraFavoriteSlots 报告"存下来了但 2610 编码不出去"的槽位，供调用方记诊断。
func (j EquipmentJournal) ExtraFavoriteSlots() map[uint32][]uint32 {
	var out map[uint32][]uint32
	for c, slots := range j.Favorites {
		if len(slots) > JournalFavoriteSlots {
			if out == nil {
				out = map[uint32][]uint32{}
			}
			out[c] = append([]uint32(nil), slots[JournalFavoriteSlots:]...)
		}
	}
	return out
}

func normalizeJournalSlots(slots []uint32) ([]uint32, error) {
	seen := map[uint32]bool{}
	out := make([]uint32, 0, len(slots))
	for _, v := range slots {
		if v == 0 || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) > journalFavoriteStorage {
		return nil, fmt.Errorf("equipment journal: %d favorite groups in one category (max %d)",
			len(out), journalFavoriteStorage)
	}
	sort.Slice(out, func(i, k int) bool { return out[i] < out[k] })
	return out, nil
}

// JournalLimit 回答"这个模板能否登记、上限多少"。
//
// 判据由 catalog.EquipmentJournalRules 提供；装备事实（minimum level / rarity / [equipment type]）
// 从装备定义里取 —— 与分解计算器用的是同一套字段（ExtractDisjointEquipmentInfo），不另立口径。
// EquipmentDefinitioner 是"能按模板号取装备定义"的最小接口：基础目录与
// 完整目录（FullEquipmentCatalog）都满足它，所以判据不绑死在其中一个上。
type EquipmentDefinitioner interface {
	Definition(uint32) (EquipmentDefinition, error)
}

func JournalLimit(c EquipmentDefinitioner, rules *catalog.EquipmentJournalRules, template uint32) (uint32, bool) {
	if c == nil || rules == nil || template == 0 {
		return 0, false
	}
	d, err := c.Definition(template)
	if err != nil {
		return 0, false
	}
	info := ExtractDisjointEquipmentInfo(d)
	if info.Impossible {
		return 0, false
	}
	if uint32(info.MinimumLevel) != journalMinimumLevel {
		return 0, false
	}
	if info.Rarity < 0 {
		return 0, false
	}
	return rules.LimitFor(equipmentTypeKind(d), uint32(info.Rarity))
}

// journalMinimumLevel 是源里的门槛：minimum level == 115 才登记（规格 CMD/0026-DISJOINTITEM）。
const journalMinimumLevel = 115

// equipmentTypeKind 取 [equipment type] 的首个文本，它就是
// [max equipment count by equipment type] 里的类型名（如 `[oath]`）。空串表示"普通"。
func equipmentTypeKind(d EquipmentDefinition) string {
	toks, ok := d.Fields["[equipment type]"]
	if !ok || len(toks) == 0 {
		return ""
	}
	return toks[0].Text
}
