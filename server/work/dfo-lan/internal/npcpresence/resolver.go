package npcpresence

// State describes the composed entity result, not quest authorization.
type State string

const (
	StateUnknown State = "unknown"
	Present      State = "present"
	Hidden       State = "hidden"
	Absent       State = "absent"
)

type SourcePlacement struct {
	MapPath   string `json:"map_path"`
	MapSHA256 string `json:"map_sha256"`
	Placement
}

// MapEvidence is scoped to the final selected root and the current instance
// generation. Rows cannot be a union of arbitrary phases. Selected remains
// unknown if phase selection or a later map override was not reconstructed.
// Instances must come from closed creation/lifecycle evidence; an eligible
// source row is insufficient (145B5F520 has special constructors/variants).
type MapEvidence struct {
	RootPath   string
	Phase      *int32
	Selected   Truth
	Placements []SourcePlacement
	Instances  map[uint32]Truth
	Gaps       []string
}

type PlacementResult struct {
	SourcePlacement
	Gate Truth `json:"gate"`
}

type Result struct {
	NPC             uint32            `json:"npc"`
	State           State             `json:"state"`
	RootPath        string            `json:"root_path"`
	Phase           *int32            `json:"phase_index"`
	MapSelected     Truth             `json:"map_selected"`
	InstancePresent Truth             `json:"instance_present"`
	Visibility      Visibility        `json:"visibility"`
	Placements      []PlacementResult `json:"placements"`
	Reasons         []string          `json:"reasons"`
}

// Resolve composes independently established map, placement, instance and
// visibility facts. It never authorizes an NPC from an ancestor show's mere
// existence, an NPC's default-hidden membership, or a phase coordinate.
func Resolve(npc uint32, m MapEvidence, accepted, completed QuestSet, visibility *VisibilityReplay) Result {
	r := Result{NPC: npc, State: StateUnknown, RootPath: m.RootPath, Phase: m.Phase,
		MapSelected: m.Selected, Reasons: append([]string(nil), m.Gaps...)}
	if visibility != nil {
		r.Visibility = visibility.Snapshot(npc)
	}
	if npc == 0 || npc > 0x7fffffff {
		r.Reasons = append(r.Reasons, "NPC identity outside the confirmed signed source scope")
		return r
	}
	for _, row := range m.Placements {
		if row.NPC == int32(npc) {
			r.Placements = append(r.Placements, PlacementResult{row, row.Gate(accepted, completed)})
		}
	}
	if m.Selected != True || m.RootPath == "" {
		r.Reasons = append(r.Reasons, "final selected map and override order are not established")
		return r
	}
	r.InstancePresent = m.Instances[npc]
	switch r.InstancePresent {
	case False:
		r.State = Absent
		r.Reasons = append(r.Reasons, "NPC instance is absent in the selected map generation")
	case True:
		switch r.Visibility.EntityVisible {
		case True:
			r.State = Present
		case False:
			r.State = Hidden
		default:
			r.Reasons = append(r.Reasons, "NPC instance exists but visibility history is incomplete")
		}
	default:
		r.Reasons = append(r.Reasons, "native NPC creation and instance lifecycle are not established")
	}
	return r
}
