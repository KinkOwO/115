package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"sort"
	"strconv"
)

// SelectionScopeAudit measures what the existing parser can read from the
// archive. It does not select the runtime scope or make items grantable.
type SelectionScopeAudit struct {
	Candidates int                   `json:"candidates"`
	Parsed     int                   `json:"parsed"`
	Fixed      int                   `json:"fixed"`
	Unparsed   int                   `json:"unparsed"`
	Rejected   int                   `json:"rejected"`
	Issues     []SelectionScopeIssue `json:"issues,omitempty"`
}

type SelectionScopeIssue struct {
	Template uint32 `json:"template"`
	Path     string `json:"path"`
	SHA256   string `json:"sha256,omitempty"`
	Reason   string `json:"reason"`
}

// AuditSelectionScope discovers candidates from native LIST identities and
// [stackable type], without a maintained template whitelist or JSON baseline.
// Rejections are recorded per item so one unsupported script does not hide the
// rest of the evidence. issueLimit caps report details, not the counts.
func AuditSelectionScope(a *pvf.Archive, index ItemIndex, issueLimit int) (SelectionScopeAudit, error) {
	var out SelectionScopeAudit
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || issueLimit < 0 {
		return out, fmt.Errorf("selection scope audit requires matching source and non-negative issue limit")
	}
	if err := index.Validate(); err != nil {
		return out, err
	}
	var ids []uint32
	for id, item := range index.Items {
		if item.Kind == "stackable" && item.StackableType == "[booster selection]" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out.Candidates = len(ids)
	issue := func(id uint32, path, hash, reason string) {
		if len(out.Issues) < issueLimit {
			out.Issues = append(out.Issues, SelectionScopeIssue{Template: id, Path: path, SHA256: hash, Reason: reason})
		}
	}
	for _, id := range ids {
		item := index.Items[id]
		script, err := ReadScript(a, item.Path)
		if err != nil {
			out.Rejected++
			issue(id, item.Path, "", err.Error())
			continue
		}
		categories, fixed := ParseSelectionCells(script.Cells)
		if len(categories) == 0 {
			if fixed {
				out.Fixed++
			} else {
				out.Unparsed++
				issue(id, script.Path, script.SHA256, "no modeled selection categories or fixed booster block")
			}
			continue
		}
		box := SelectionBox{Template: id, Path: script.Path, SHA256: script.SHA256, Categories: categories}
		_, err = NewSelectionBoxes(SelectionBoxes{
			Model: SelectionBoxModel, Bounded: true, Source: index.Source,
			Boxes: map[string]SelectionBox{strconv.FormatUint(uint64(id), 10): box},
		})
		if err != nil {
			out.Rejected++
			issue(id, script.Path, script.SHA256, err.Error())
		} else {
			out.Parsed++
		}
	}
	return out, nil
}
