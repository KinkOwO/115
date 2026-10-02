package inventory

import "testing"

func TestBoxWindowCountersUsePersistedStackOrder(t *testing.T) {
	svc := &ItemService{Boxes: &BoxCatalog{Tables: map[string]BoxTable{
		"590712474": {PointStacks: []BoxPointStack{{Type: "section"}, {Type: "bonus"}}},
	}}}
	state := []byte(`{"box_points":{"590712474":{"0":27,"1":3}},"inventory":{}}`)
	bonus, section, err := svc.BoxWindowCounters(state, 590712474)
	if err != nil {
		t.Fatal(err)
	}
	if bonus != 3 || section != 27 {
		t.Fatalf("window counters = (%d,%d), want (3,27)", bonus, section)
	}
	bonus, section, err = svc.BoxWindowCounters([]byte(`{"inventory":{}}`), 590712474)
	if err != nil || bonus != 0 || section != 0 {
		t.Fatalf("fresh character counters = (%d,%d), err %v", bonus, section, err)
	}
}
