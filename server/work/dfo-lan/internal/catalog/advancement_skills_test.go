package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func TestAdvancementSourceTriples(t *testing.T) {
	b, err := os.ReadFile("../../docs/evidence/skycastle-auto-skills-20260917/details/00-atswordman.chr.tokens.json")
	if err != nil {
		t.Fatal(err)
	}
	var tokens []pvf.Token
	if err = json.Unmarshal(b, &tokens); err != nil {
		t.Fatal(err)
	}
	grants, err := AdvancementSkillGrants(tokens)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := 0; i < len(grants[2]); i += 3 {
		if grants[2][i] == 62 {
			found = grants[2][i+1] == 1 && grants[2][i+2] == 1
		}
		if grants[2][i] == 75 {
			t.Fatal("awakening leaked into advancement grants")
		}
	}
	if !found || len(grants[0]) != 0 {
		t.Fatal(grants)
	}
	if _, err = AdvancementSkillGrants([]pvf.Token{{Type: 3, Text: "[growtype 3]"}, {Type: 3, Text: "[skill]"}, {Type: 0, Value: 62}}); err == nil {
		t.Fatal("accepted truncated triple")
	}
}
