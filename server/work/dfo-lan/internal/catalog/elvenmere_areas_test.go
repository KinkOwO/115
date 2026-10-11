package catalog

import (
	"strings"
	"testing"
)

func TestElvenmereAreaRoutingFollowsSourceAndRejectsGaps(t *testing.T) {
	text := `[area group]
[group]
[area index] 9
[floor] 1 1
[waiting map index] 111
[/group]
[group]
[area index] 12
[floor] 2 2
[waiting map index] 222
[/group]
[/area group]`
	r, err := parseElvenmereRules(elvenmereTestSource + "\n" + text)
	if err != nil {
		t.Fatal(err)
	}
	area, err := r.AreaForFloor(2)
	if err != nil || area.Index != 12 || area.WaitingMap != 222 {
		t.Fatal(area, err)
	}
	r, err = parseElvenmereRules(elvenmereTestSource + "\n" + strings.ReplaceAll(text, "222", "333"))
	if err != nil {
		t.Fatal(err)
	}
	area, err = r.AreaForFloor(2)
	if err != nil || area.WaitingMap != 333 {
		t.Fatal("ignored source map", area, err)
	}
	for _, bad := range []string{strings.ReplaceAll(text, "[floor] 2 2", "[floor] 1 2"), strings.ReplaceAll(text, "[area index] 12", "[area index] 9"), strings.ReplaceAll(text, "[waiting map index] 222", "[waiting map index] 0"), strings.ReplaceAll(text, "[/area group]", "")} {
		if _, err := parseElvenmereRules(elvenmereTestSource + "\n" + bad); err == nil {
			t.Fatal("invalid area routing accepted", bad)
		}
	}
}
