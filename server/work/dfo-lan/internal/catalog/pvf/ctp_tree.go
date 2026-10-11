package pvf

import "fmt"

// CTPTree indexes the native parent/ref relations. Rows and their mixed cells
// remain in the original table; consumers must not flatten children into tokens.
type CTPTree struct {
	Table    *CTPTable
	children map[int][]int
}

func NewCTPTree(t *CTPTable) (*CTPTree, error) {
	if t == nil {
		return nil, fmt.Errorf("CTP tree source missing")
	}
	out := &CTPTree{Table: t, children: map[int][]int{}}
	for i, r := range t.Records {
		if r.Index != i || r.Parent < -1 || r.Parent >= i {
			return nil, fmt.Errorf("%s row %d: invalid index/parent %d/%d", t.Path, i, r.Index, r.Parent)
		}
		out.children[r.Parent] = append(out.children[r.Parent], i)
	}
	for _, r := range t.Records {
		seen := map[uint64]bool{}
		for _, ref := range r.Refs {
			for _, index := range ref.Values {
				if index >= uint64(len(t.Records)) || seen[index] {
					return nil, fmt.Errorf("%s row %d: invalid/duplicate child reference %d", t.Path, r.Index, index)
				}
				child := t.Records[index]
				if child.Parent != r.Index || child.Name != ref.Name {
					return nil, fmt.Errorf("%s row %d: child reference %d owner/name mismatch", t.Path, r.Index, index)
				}
				seen[index] = true
			}
		}
		if len(seen) != len(out.children[r.Index]) {
			return nil, fmt.Errorf("%s row %d: parent/ref child sets differ", t.Path, r.Index)
		}
	}
	return out, nil
}

// Children returns direct rows in source order. Parent -1 selects roots;
// an empty name selects every child, preserving repeated columns.
func (t *CTPTree) Children(parent int, name string) []CTPRecord {
	var out []CTPRecord
	for _, i := range t.children[parent] {
		r := t.Table.Records[i]
		if name == "" || r.Name == name {
			out = append(out, r)
		}
	}
	return out
}

// One distinguishes absence from duplicate fields; neither can become zero.
func (t *CTPTree) One(parent int, name string) (CTPRecord, error) {
	rows := t.Children(parent, name)
	if len(rows) != 1 {
		return CTPRecord{}, fmt.Errorf("%s row %d: expected one %s, got %d", t.Table.Path, parent, name, len(rows))
	}
	return rows[0], nil
}
