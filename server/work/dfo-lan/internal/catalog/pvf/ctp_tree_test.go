package pvf

import "testing"

func TestCTPTreePreservesNestedAndRepeatedColumns(t *testing.T) {
	table := &CTPTable{Path: "fixture.ctp", Records: []CTPRecord{
		{Index: 0, Parent: -1, Name: "[TRIGGER]", Refs: []CTPRef{{Name: "[OR]", Values: []uint64{1}}}},
		{Index: 1, Parent: 0, Name: "[OR]", Refs: []CTPRef{{Name: "[IF]", Values: []uint64{2, 3}}}},
		{Index: 2, Parent: 1, Name: "[IF]", Cells: []CTPCell{{Kind: "name", Name: "[A]"}, {Kind: "float", Float: 2}}},
		{Index: 3, Parent: 1, Name: "[IF]", Cells: []CTPCell{{Kind: "name_indexed", Name: "localized", Index: 6}, {Kind: "opaque", Raw: "0123"}}},
	}}
	tree, e := NewCTPTree(table)
	if e != nil {
		t.Fatal(e)
	}
	if len(tree.Children(0, "[IF]")) != 0 {
		t.Fatal("nested IF flattened into trigger")
	}
	rows := tree.Children(1, "[IF]")
	if len(rows) != 2 || rows[1].Cells[0].Index != 6 || rows[1].Cells[1].Raw != "0123" {
		t.Fatal("mixed cells/order lost")
	}
	if _, e = tree.One(1, "[IF]"); e == nil {
		t.Fatal("duplicate field accepted as singleton")
	}
	if _, e = tree.One(0, "[MISSING]"); e == nil {
		t.Fatal("absent field accepted as zero")
	}
}
func TestCTPTreeRejectsCorruptOwnershipAndParents(t *testing.T) {
	for _, which := range []string{"cycle", "foreign child", "wrong column", "duplicate reference", "out of range", "missing reference"} {
		t.Run(which, func(t *testing.T) {
			table := &CTPTable{Path: "fixture.ctp", Records: []CTPRecord{{Index: 0, Parent: -1, Name: "[ROOT]", Refs: []CTPRef{{Name: "[VALUE]", Values: []uint64{1}}}}, {Index: 1, Parent: 0, Name: "[VALUE]"}}}
			switch which {
			case "cycle":
				table.Records[0].Parent = 1
			case "foreign child":
				table.Records[1].Parent = -1
			case "wrong column":
				table.Records[1].Name = "[OTHER]"
			case "duplicate reference":
				table.Records[0].Refs[0].Values = append(table.Records[0].Refs[0].Values, 1)
			case "out of range":
				table.Records[0].Refs[0].Values[0] = 2
			case "missing reference":
				table.Records[0].Refs = nil
			}
			if _, e := NewCTPTree(table); e == nil {
				t.Fatal("corrupt CTP tree accepted")
			}
		})
	}
}
