package catalog

import (
	"os"
	"strings"
	"testing"
)

func TestOathTruthSourceRules(t *testing.T) {
	raw, err := os.ReadFile("testdata/oath_truth.cos")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ParseOathTruth(string(raw))
	if err != nil {
		t.Fatal(err)
	}
	if r.MaxLevel != 4 || len(r.Stages[0].Options) != 0 {
		t.Fatal("available source slots")
	}
	for n := 1; n <= 4; n++ {
		s := r.Stages[n]
		if len(s.Options) != 2 || s.Options[0].Ability != 4 || s.Options[1].Ability != 8 || s.Options[0].Buff != 800 || s.Costs[1][0].Amount != uint32(n*100) || s.Costs[2][0].Amount != uint32(n*500000) {
			t.Fatalf("stage %d %+v", n, s)
		}
	}
	altered, err := ParseOathTruth(strings.Replace(string(raw), "10401346 100", "10401346 137", 1))
	if err != nil || altered.Stages[1].Costs[1][0].Amount != 137 {
		t.Fatal("source changes must drive fees", err)
	}
	for _, bad := range []string{strings.Replace(string(raw), "[max level available] 4", "[max level available] 5", 1), strings.Replace(string(raw), "[/oath reinforce]", "", 1), strings.Replace(string(raw), "10401346 100", "10401346 0", 1)} {
		if _, err := ParseOathTruth(bad); err == nil {
			t.Fatal("incomplete source accepted")
		}
	}
}
