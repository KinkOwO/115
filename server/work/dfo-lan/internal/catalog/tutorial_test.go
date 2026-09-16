package catalog

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCurrentSourceTutorialsKeepJobAndEventBoundaries(t *testing.T) {
	p, e := os.ReadFile("../../configs/tutorial-routes.current35.json")
	if e != nil {
		t.Fatal(e)
	}
	var c TutorialCatalog
	if e = json.Unmarshal(p, &c); e != nil {
		t.Fatal(e)
	}
	c.Flows, e = ParseTutorialFlows(c.Script)
	if e != nil {
		t.Fatal(e)
	}
	if len(c.Flows) != 16 {
		t.Fatalf("source route count=%d", len(c.Flows))
	}
	for _, sample := range []struct {
		job   string
		id    uint32
		after bool
	}{
		{"[swordman]", 7115, false}, {"[archer]", 100003327, true}, {"[gunner]", 7110, true},
	} {
		r, e := c.Normal(sample.job, 0)
		if e != nil || r.Dungeon != sample.id || r.IntroAfter != sample.after || r.EventOnly || r.Town != 38 || r.Area != 0 || r.Position != [2]uint16{1677, 222} {
			t.Fatalf("%s: %+v %v", sample.job, r, e)
		}
	}
	// The source's gunner grow5 event has a different movie, town and quest.
	// It must not silently replace the normal route even for matching grow5.
	r, e := c.Normal("[gunner]", 5)
	if e != nil || r.Dungeon != 7110 || r.FirstQuest != 0 {
		t.Fatalf("event route leaked: %+v %v", r, e)
	}
	if _, e = c.Normal("[absent job]", 0); e == nil {
		t.Fatal("invented missing-job tutorial")
	}
	c.Flows = append(c.Flows, c.Flows[0])
	if _, e = c.Normal("[swordman]", 0); e == nil {
		t.Fatal("ambiguous route accepted")
	}
	broken := c.Script
	broken.Cells = broken.Cells[:len(broken.Cells)-1]
	if _, e = ParseTutorialFlows(broken); e == nil {
		t.Fatal("unfinished route accepted")
	}
}
