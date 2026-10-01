package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strings"
)

// Unsupported pools stay unavailable as a whole: dropping individual rewards
// would change their odds. Issues retain the exact content that needs support.
type LotteryScopeIssue struct {
	Template uint32 `json:"template"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256"`
	Reason   string `json:"reason"`
}

type LotteryScope struct {
	Candidates int                 `json:"candidates"`
	Issues     []LotteryScopeIssue `json:"issues"`
}

// DiscoverLotteryTables derives the supported scope from source types and the
// existing grant capabilities, without an ID allowlist or exported reward JSON.
func DiscoverLotteryTables(a *pvf.Archive, index ItemIndex) (LotteryTables, LotteryScope, error) {
	var out LotteryTables
	var scope LotteryScope
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum {
		return out, scope, fmt.Errorf("lottery source/index mismatch")
	}
	if err := index.Validate(); err != nil {
		return out, scope, err
	}
	out.Items.SourcePVFSHA256 = index.Source.Checksum
	out.Equipment.SourcePVFSHA256 = index.Source.Checksum
	ids := make([]uint32, 0)
	for id, item := range index.Items {
		if item.Kind == "stackable" && item.StackableType == "[upgradable legacy]" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	scope.Candidates = len(ids)
	for _, id := range ids {
		script, err := ReadScript(a, index.Items[id].Path)
		if err != nil {
			return out, scope, fmt.Errorf("lottery %d: %w", id, err)
		}
		rows, equipment, err := classifyLotteryRewards(ParseLotteryCells(script.Cells), index.Items)
		if err != nil {
			scope.Issues = append(scope.Issues, LotteryScopeIssue{Template: id, Path: script.Path, SHA256: script.SHA256, Reason: err.Error()})
			continue
		}
		pool := LotterySourcePool{SourceItem: id, SourceScript: script.Path, SourceScriptSHA256: script.SHA256, Candidates: rows}
		if equipment {
			out.Equipment.Pools = append(out.Equipment.Pools, pool)
		} else {
			out.Items.Pools = append(out.Items.Pools, pool)
		}
	}
	return out, scope, nil
}

func classifyLotteryRewards(values []int32, index map[uint32]ItemIndexEntry) ([][3]uint32, bool, error) {
	if len(values) == 0 || len(values)%3 != 0 {
		return nil, false, fmt.Errorf("unsupported source triples")
	}
	rows := make([][3]uint32, 0, len(values)/3)
	equipment := false
	for j := 0; j < len(values); j += 3 {
		if values[j] < 0 || values[j+1] <= 0 || values[j+2] <= 0 {
			return nil, false, fmt.Errorf("invalid source triple at row %d", j/3)
		}
		id, weight, count := uint32(values[j]), uint32(values[j+1]), uint32(values[j+2])
		if id != 0 {
			item, ok := index[id]
			if !ok {
				return nil, false, fmt.Errorf("reward %d absent from native item index", id)
			}
			switch item.Kind {
			case "stackable":
			case "equipment", "avatar":
				if count != 1 || (item.Kind == "equipment" && strings.Contains(item.Path, "equipment/creature/")) {
					return nil, false, fmt.Errorf("reward %d outside supported grant capability", id)
				}
				equipment = true
			default:
				return nil, false, fmt.Errorf("reward %d outside supported grant capability", id)
			}
		}
		rows = append(rows, [3]uint32{id, weight, count})
	}
	return rows, equipment, nil
}
