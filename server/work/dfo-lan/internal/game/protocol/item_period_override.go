package protocol

import "sync/atomic"

type itemPeriodSet struct {
	templates map[uint32]struct{}
}

var maxItemPeriodTemplates atomic.Pointer[itemPeriodSet]

// skinStoragePeriodTemplates holds the `[add skin storage]` consumable templates.
var skinStoragePeriodTemplates atomic.Pointer[itemPeriodSet]

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
	if stored == 0 || maxItemPeriodTemplates.Load() != nil {
		return false
	}
	return int64(stored) <= now
}
