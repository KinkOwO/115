package inventory

import "os"

// equipmentParitySweepCap bounds the per-item deep-parity loop. The native
// equipment set is large (hundreds of thousands), so the default compares a
// deterministic stride sample while the full record counts and index hashes
// are still asserted in full. Set DFO_EQUIPMENT_FULL_SWEEP=1 to compare every
// row.
const equipmentParitySweepCap = 500

// paritySweep returns a deterministic stride sample of an already ordered
// slice, or the whole slice when it is small or a full sweep is requested.
func paritySweep[T any](all []T) []T {
	if os.Getenv("DFO_EQUIPMENT_FULL_SWEEP") == "1" || len(all) <= equipmentParitySweepCap {
		return all
	}
	step := (len(all) + equipmentParitySweepCap - 1) / equipmentParitySweepCap
	out := make([]T, 0, equipmentParitySweepCap)
	for i := 0; i < len(all); i += step {
		out = append(out, all[i])
	}
	return out
}
