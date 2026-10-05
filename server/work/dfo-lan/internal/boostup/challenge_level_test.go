package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBoostChallengeActualLevelMail(t *testing.T) {
	dir := os.Getenv("US115_TEST_BOOST_CHALLENGE")
	if dir == "" {
		t.Skip("explicit read-only challenge source required")
	}
	b, e := os.ReadFile(filepath.Join(dir, "00-boostupspecupchallenge.evt.tokens.json"))
	if e != nil {
		t.Fatal(e)
	}
	var cells []pvf.Token
	if e = json.Unmarshal(b, &cells); e != nil {
		t.Fatal(e)
	}
	rows, e := ParseChallengeLevelRewards(cells)
	if e != nil {
		t.Fatal(e)
	}
	if len(rows) != 1 || rows[115] != (Reward{590015933, 1}) {
		t.Fatal("source level bonus mixed with unlock/challenge rewards", rows)
	}
	c := &Catalog{GoalLevel: 115, ChallengeLevelRewards: rows}
	if c.CapsuleLevelMail(114) != nil {
		t.Fatal("under-level eligible")
	}
	if m := c.CapsuleLevelMail(115); m == nil || m.Item != 590015933 || m.Count != 1 || m.Validate() != nil {
		t.Fatal(m)
	}
}

func TestBoostChallengeLevelMailMalformedSource(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	n := func(v int32) pvf.Token { return pvf.Token{Type: 0, Value: v} }
	root := []pvf.Token{tag("[level up bonus info]"), tag("[mail]"), tag("[level bonus]"), n(115), n(590015933), n(1), tag("[/level bonus]"), tag("[/level up bonus info]")}
	if _, e := ParseChallengeLevelRewards(root); e != nil {
		t.Fatal(e)
	}
	for _, at := range []int{3, 4, 5} {
		bad := append([]pvf.Token(nil), root...)
		bad[at].Value = 0
		if _, e := ParseChallengeLevelRewards(bad); e == nil {
			t.Fatal("bad reward accepted", at)
		}
	}
	if _, e := ParseChallengeLevelRewards(append(root[:6:6], root[3:]...)); e == nil {
		t.Fatal("duplicate level accepted")
	}
	if _, e := ParseChallengeLevelRewards(append(root[:1:1], root[2:]...)); e == nil {
		t.Fatal("non-mail mode accepted")
	}
}
