package catalog

import (
	"fmt"
	"math"
)

// ApocalypseRoleLimits is derived from apocalypse.ctp, never from the request
// tracker's former PartySize=1 default. Keys are real member counts and native
// role IDs; values are the maximum seats for each role.
type ApocalypseRoleLimits map[int]map[uint32]int

// CompileRoleLimits reads [role per member limit] -> [member]. Current client
// 1476E7370 (1476E75F7..1476E7837) reads an odd-width row: member count followed
// by (role, limit) pairs. Each pair creates a cumulative map snapshot;
// 140698D50 uses the final snapshot for the supported one/two-role layouts.
// This view retains that final map, not a second hand-maintained quota table.
func (c *ApocalypseCatalog) CompileRoleLimits() (ApocalypseRoleLimits, error) {
	if c == nil {
		return nil, fmt.Errorf("apocalypse role source unavailable")
	}
	root := -1
	var membersRef []uint64
	memberRefFound := false
	for _, r := range c.Records {
		if r.Name == "[role per member limit]" && r.Parent == -1 {
			if root != -1 {
				return nil, fmt.Errorf("duplicate apocalypse role limit container")
			}
			root = r.Index
			for _, ref := range r.Refs {
				if ref.Name == "[member]" {
					if memberRefFound {
						return nil, fmt.Errorf("duplicate apocalypse member reference")
					}
					membersRef = ref.Values
					memberRefFound = true
				}
			}
		}
	}
	if root == -1 {
		return nil, fmt.Errorf("apocalypse source has no [role per member limit]")
	}
	out := ApocalypseRoleLimits{}
	seenRefs := map[uint64]bool{}
	for _, index := range membersRef {
		if index >= uint64(len(c.Records)) || seenRefs[index] {
			return nil, fmt.Errorf("invalid or repeated apocalypse member reference %d", index)
		}
		seenRefs[index] = true
		r := c.Records[index]
		if r.Index != int(index) || r.Parent != root || r.Name != "[member]" {
			return nil, fmt.Errorf("apocalypse member reference %d has inconsistent identity/parent", index)
		}
		// The native consumer supports one or two role-map snapshots. Do not
		// silently accept a third pair which its lookup would return -1 for.
		if len(r.Cells) != 3 && len(r.Cells) != 5 {
			return nil, fmt.Errorf("apocalypse role row %d has unsupported width %d", r.Index, len(r.Cells))
		}
		values := make([]int, len(r.Cells))
		for i, cell := range r.Cells {
			v := cell.Float
			if cell.Kind != "float" || math.IsNaN(v) || math.IsInf(v, 0) || v != math.Trunc(v) || v < 1 || v > 4 {
				return nil, fmt.Errorf("apocalypse role row %d cell %d is outside native seat range", r.Index, i)
			}
			values[i] = int(v)
		}
		members := values[0]
		if _, exists := out[members]; exists {
			return nil, fmt.Errorf("duplicate apocalypse member count %d", members)
		}
		roles, total := map[uint32]int{}, 0
		for i := 1; i < len(values); i += 2 {
			role, limit := uint32(values[i]), values[i+1]
			// 140690F90 implements role IDs 0..2; zero cancels a role and
			// therefore is not a selectable quota row in the source.
			if role > 2 {
				return nil, fmt.Errorf("apocalypse source role %d has no native executor", role)
			}
			if _, exists := roles[role]; exists {
				return nil, fmt.Errorf("duplicate apocalypse role %d for %d members", role, members)
			}
			roles[role] = limit
			total += limit
		}
		if total > 4 {
			return nil, fmt.Errorf("apocalypse role quotas exceed the native four seats")
		}
		out[members] = roles
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("apocalypse role limit container has no member rows")
	}
	return out, nil
}

// Quota requires an authoritative roster size; missing rows/roles have no
// implicit quota. In particular, solo guardian is absent in the current source.
func (r ApocalypseRoleLimits) Quota(members int, role uint32) (int, bool) {
	v, ok := r[members][role]
	return v, ok
}
