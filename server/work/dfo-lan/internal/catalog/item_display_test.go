package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestDisplayScalarKeepsFirstFieldBeforeNestedConditions(t *testing.T) {
	cells := []pvf.Token{{Type: 3, Text: "[grade]"}, {Type: 0, Value: 1}, {Type: 3, Text: "[rarity]"}, {Type: 0, Value: 2}, {Type: 3, Text: "[grade]"}, {Type: 0, Value: 1}, {Type: 0, Value: 70}, {Type: 3, Text: "[rarity]"}, {Type: 0, Value: 5}}
	if n, ok := itemDisplayScalar(cells, "[grade]"); !ok || n != 1 {
		t.Fatal(n, ok)
	}
	if n, ok := itemDisplayScalar(cells, "[rarity]"); !ok || n != 2 {
		t.Fatal(n, ok)
	}
}

func TestDisplayTableKeepsExactFirstValue(t *testing.T) {
	d := parseDisplayTable("// ignored\r\nname_1>first>value\r\nname_1>second\nchn_name_2>中文\nname_empty>\n")
	if d["name_1"] != "first>value" || d["chn_name_2"] != "中文" || len(d) != 3 {
		t.Fatal(d)
	}
}

func TestDisplayRejectsMissingArchiveOrVisitor(t *testing.T) {
	if err := VisitItemDisplay(nil, ItemIndex{}, nil); err == nil {
		t.Fatal("unbound display source accepted")
	}
}
