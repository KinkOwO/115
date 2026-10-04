package inventory

import (
	"dfolan/internal/catalog"
	"fmt"
	"sort"
)

// 装备库（图鉴）账本的**对账修复**。
//
// 背景（2026-10-04 用户实机报告）：图鉴里的晶石份数"被搞乱"——不单纯是增加，也可能变成
// 更高等级、甚至减少。根因是本文件之外的两条**记账路径各自漂移**：
//
//	CMD2381（晶体/誓约变换）：旧实现按来源分三套记账，且"先判上限再登记"会把同模板换回挡掉
//	                          ⇒ 既会凭空增加，也会**只扣不还**（图鉴变少）。
//	CMD2259（装备变换）      ：只登记换下的源装备、**不扣**被换成的那件 ⇒ 每换一次净 +1。
//
// 两条路径的**记账口径**已在本轮统一（见 primer_transform.go 的 primerSourceKind 与
// equipment_journal_operations.go 的 PrepareEquipmentTransform）。本文件只负责一件事：
// 把**已经漂掉的历史数据**拉回一个可解释的状态。
//
// 为什么需要"对账"而不是简单重算：
//   - 图鉴份数在客户端是**绝对份数**（不是布尔标记），"0 份但已登记"与"从未登记"是两种
//     状态（2610 规格），所以不能把条目删掉，只能改份数；
//   - 图鉴是"能不能被选为变换目标"的**唯一池子**。份数被清零 ⇒ 玩家在窗口里再也选不到它，
//     **这是不可逆的功能损失**（除非再次分解一件同样的装备）；
//   - 因此修复的默认方向是"**不减少、不静默**"：只把"身上带着、图鉴里却没有"的漏记补上，
//     其余异常只报告、交给业主决定。
//
// 判据（唯一真源 = 角色实际带着的东西）：
//
//	carried(tpl)  = 背包装备区 + 穿戴 里该模板的件数
//	journal(tpl)  = 图鉴账本里的绝对份数
//
//	① 漏记：carried > 0 且（journal 缺席或为 0）⇒ 把份数补到上限内（**可逆地找回功能**）；
//	② 虚计：journal > 0 但 carried == 0 ⇒ **只报告**（默认不动；`-zero-uncarried` 才清零）；
//	③ 其余保持原样。
type JournalReconcileMode int

const (
	// JournalReconcileReportOnly 只对账、不写（默认）。
	JournalReconcileReportOnly JournalReconcileMode = iota
	// JournalReconcileRestoreCarried 补上"身上带着却没登记"的条目（不动虚计条目）。
	JournalReconcileRestoreCarried
	// JournalReconcileZeroUncarried 在上一档基础上，把"登记了却一件都不在手边"的条目清零。
	//
	// ⚠️ 会**不可逆地**从变换窗口里移除这些条目（0 份 = 客户端不可选），只在业主明确要求时用。
	JournalReconcileZeroUncarried
)

// JournalReconcileChange 是一条对账改动（诊断与回执都用它）。
type JournalReconcileChange struct {
	Template uint32 `json:"template"`
	Before   uint32 `json:"before"`
	After    uint32 `json:"after"`
	Carried  int    `json:"carried"`
	// Capped 表示"补登记"被上限挡住，After 只能到上限。
	Capped bool   `json:"capped,omitempty"`
	Reason string `json:"reason"`
}

// JournalReconcileReport 是一次对账的完整结果。
type JournalReconcileReport struct {
	Character int64                    `json:"character,omitempty"`
	Mode      string                   `json:"mode"`
	Changes   []JournalReconcileChange `json:"changes,omitempty"`
	// Missing 是"身上带着、图鉴里从未出现过"的模板（补登记后也会出现在 Changes 里）。
	Missing []uint32 `json:"missing,omitempty"`
	// Orphans 是"图鉴里有份数、但身上一件都没有"的模板（默认不动，只报告）。
	Orphans []uint32 `json:"orphans,omitempty"`
	// Zeroed 记录被清零的虚计条目（只有 ZeroUncarried 模式才会非空）。
	Zeroed []JournalReconcileChange `json:"zeroed,omitempty"`
	// Applied 表示这次真的写回了账本。
	Applied bool `json:"applied"`
}

// ReconcileEquipmentJournal 按 `mode` 对账账本。
//
// 纯函数：不改传入的 `ledger`，返回新账本与报告。`gear`/`rules` 用来判"能不能登记、上限多少"
// （与分解、变换走**同一条** `JournalLimit` 判据，不另立口径）；任一为 nil 时只做"补 1 份"这种
// 不依赖上限的最小修复，并在报告里说明（`Capped` 保持 false）。
func ReconcileEquipmentJournal(
	bag Bag,
	ledger EquipmentJournal,
	gear EquipmentDefinitioner,
	rules *catalog.EquipmentJournalRules,
	mode JournalReconcileMode,
) (EquipmentJournal, JournalReconcileReport) {
	out := ledger.clone()
	if out.Counts == nil {
		out.Counts = map[uint32]uint32{}
	}
	report := JournalReconcileReport{Mode: mode.String()}

	carried := map[uint32]int{}
	for _, row := range bag.Equipment {
		if row.Template != 0 {
			carried[row.Template]++
		}
	}
	for _, row := range bag.Worn {
		if row.Template != 0 {
			carried[row.Template]++
		}
	}

	limitOf := func(template uint32) (uint32, bool) {
		if gear == nil || rules == nil {
			return 1, true // 拿不到规则表时只保证"至少 1 份"，绝不凭空加量
		}
		return JournalLimit(gear, rules, template)
	}

	// ① 漏记：身上带着、图鉴里却没有（或为 0）。
	for template, count := range carried {
		if count == 0 || out.Counts[template] > 0 {
			continue
		}
		if _, ok := out.Counts[template]; !ok {
			report.Missing = append(report.Missing, template)
		}
		if mode == JournalReconcileReportOnly {
			continue
		}
		limit, ok := limitOf(template)
		if !ok {
			// 不在收录范围 ⇒ 不能登记（与分解/变换同一条判据）。
			report.Changes = append(report.Changes, JournalReconcileChange{
				Template: template, Before: 0, After: 0, Carried: count,
				Reason: "不在收录范围（minimum level / rarity 不符）",
			})
			continue
		}
		want := uint32(count)
		capped := false
		if want > limit {
			want, capped = limit, true
		}
		next, _, e := out.Add(template, want, limit)
		if e != nil {
			report.Changes = append(report.Changes, JournalReconcileChange{
				Template: template, Before: 0, After: 0, Carried: count, Reason: e.Error(),
			})
			continue
		}
		out = next
		report.Changes = append(report.Changes, JournalReconcileChange{
			Template: template, Before: 0, After: want, Carried: count, Capped: capped,
			Reason: "身上带着却没登记 ⇒ 补回（否则窗口里再也选不到它）",
		})
	}

	// ② 虚计：图鉴里有份数、身上一件都没有。
	for template, count := range out.Counts {
		if count == 0 || carried[template] > 0 {
			continue
		}
		report.Orphans = append(report.Orphans, template)
		if mode != JournalReconcileZeroUncarried {
			continue
		}
		change := JournalReconcileChange{
			Template: template, Before: count, After: 0, Carried: 0,
			Reason: "账上有份数、手上一件都没有 ⇒ 清零（不可逆：窗口里将不再可选）",
		}
		out.Counts[template] = 0
		report.Zeroed = append(report.Zeroed, change)
		report.Changes = append(report.Changes, change)
	}

	sort.Slice(report.Changes, func(i, j int) bool { return report.Changes[i].Template < report.Changes[j].Template })
	sort.Slice(report.Orphans, func(i, j int) bool { return report.Orphans[i] < report.Orphans[j] })
	sort.Slice(report.Missing, func(i, j int) bool { return report.Missing[i] < report.Missing[j] })
	sort.Slice(report.Zeroed, func(i, j int) bool { return report.Zeroed[i].Template < report.Zeroed[j].Template })
	return out, report
}

// String 给出模式的稳定名字（写进回执与日志）。
func (m JournalReconcileMode) String() string {
	switch m {
	case JournalReconcileRestoreCarried:
		return "restore-carried"
	case JournalReconcileZeroUncarried:
		return "zero-uncarried"
	default:
		return "report"
	}
}

// JournalReconcileModeFromString 解析命令行传入的模式名。
func JournalReconcileModeFromString(s string) (JournalReconcileMode, error) {
	switch s {
	case "report", "":
		return JournalReconcileReportOnly, nil
	case "restore-carried":
		return JournalReconcileRestoreCarried, nil
	case "zero-uncarried":
		return JournalReconcileZeroUncarried, nil
	default:
		return JournalReconcileReportOnly, fmt.Errorf("unknown journal reconcile mode %q (report|restore-carried|zero-uncarried)", s)
	}
}
