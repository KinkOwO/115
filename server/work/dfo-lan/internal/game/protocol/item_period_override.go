package protocol

import "sync/atomic"

type itemPeriodSet struct {
	templates map[uint32]struct{}
}

var maxItemPeriodTemplates atomic.Pointer[itemPeriodSet]

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
