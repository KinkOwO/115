package protocol

import "sync/atomic"

type itemPeriodSet struct {
	templates map[uint32]struct{}
}

var maxItemPeriodTemplates atomic.Pointer[itemPeriodSet]

// skinStoragePeriodTemplates holds the `[add skin storage]` consumable templates.
var skinStoragePeriodTemplates atomic.Pointer[itemPeriodSet]

// liftStoredPeriods 兜底开关：装载不到 PVF 期限模板表时，至少保证**存档里已有的
// 非零期限**不会被当成过期（按永不过期下发）。
//
// 为什么需要它：一键启动器（`DFO-115US单机一键启动器.exe`）自己拉起 launch_local.py，
// 从不设置 DFO_MAX_ITEM_PERIOD（三个 .cmd 入口都设了）⇒ 走启动器时
// ConfigureMaxItemPeriods 一次都没被调用，脚本声明过期限的模板（如银增幅书，
// 到期日 2022-11-08）一律按 0 下发，客户端显示「剩余期限已过」并拒绝使用。
var liftStoredPeriods atomic.Bool

// ConfigureStoredPeriodLifting 打开/关闭上述兜底。与 ConfigureMaxItemPeriods
// 互不冲突：两者都开时以模板表的规则为准。
func ConfigureStoredPeriodLifting(on bool) { liftStoredPeriods.Store(on) }

// ConfigureMaxItemPeriods installs the PVF-derived templates that should be
// sent with the largest client period. A nil slice disables the override.
// Configure once before accepting clients; the copy prevents later mutations.
func ConfigureMaxItemPeriods(templates []uint32) {
	if len(templates) == 0 {
		maxItemPeriodTemplates.Store(nil)
		return
	}
	set := &itemPeriodSet{templates: make(map[uint32]struct{}, len(templates))}
	for _, template := range templates {
		set.templates[template] = struct{}{}
	}
	maxItemPeriodTemplates.Store(set)
}

// ConfigureSkinStoragePeriods installs the `[add skin storage]` consumable
// templates only. The client refuses to use a period-declaring item whose
// offset-56 cell is 0 (「剩余期限已过」, and it sends no C2S at all), so those rows
// carry the sentinel while a real remaining period is unknown. It is deliberately
// separate from ConfigureMaxItemPeriods: a row that does hold a countdown keeps
// showing that countdown.
func ConfigureSkinStoragePeriods(templates []uint32) {
	if len(templates) == 0 {
		skinStoragePeriodTemplates.Store(nil)
		return
	}
	set := &itemPeriodSet{templates: make(map[uint32]struct{}, len(templates))}
	for _, template := range templates {
		set.templates[template] = struct{}{}
	}
	skinStoragePeriodTemplates.Store(set)
}

// ItemPeriodForWire changes only the period sent to the client. A nonzero
// instance period is also covered even when its template has no PVF marker.
// Stored item state remains intact, including old saves whose period is zero.
func ItemPeriodForWire(template, stored uint32) uint32 {
	if set := maxItemPeriodTemplates.Load(); set != nil {
		if stored != 0 {
			return MaxItemPeriod
		}
		if _, ok := set.templates[template]; ok {
			return MaxItemPeriod
		}
	}
	if stored != 0 && liftStoredPeriods.Load() {
		return MaxItemPeriod
	}
	if stored == 0 {
		if set := skinStoragePeriodTemplates.Load(); set != nil {
			if _, ok := set.templates[template]; ok {
				return MaxItemPeriod
			}
		}
	}
	return stored
}

// StoredItemExpired applies the same opt-in policy to server-side use checks.
// A zero stored expiry is unspecified and cannot establish that an item expired.
func StoredItemExpired(stored uint32, now int64) bool {
	if stored == 0 || maxItemPeriodTemplates.Load() != nil || liftStoredPeriods.Load() {
		return false
	}
	return int64(stored) <= now
}
