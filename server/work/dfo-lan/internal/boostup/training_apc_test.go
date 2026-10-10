package boostup

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"testing"
)

type apcFixture map[string][]pvf.Token

func (s apcFixture) Tokens(p string) ([]pvf.Token, error) {
	v, ok := s[p]
	if !ok {
		return nil, fmt.Errorf("missing %s", p)
	}
	return v, nil
}
func TestTrainingAPCBindsSelectedSourceInsteadOfManualTemplate(t *testing.T) {
	tag := func(s string) pvf.Token { return pvf.Token{Type: 3, Text: s} }
	txt := func(s string) pvf.Token { return pvf.Token{Type: 6, Text: s} }
	num := func(n int32) pvf.Token { return pvf.Token{Type: 0, Value: n} }
	for _, pair := range [][2]int32{{7, 3577}, {19, 8123}, {31, 3577}} {
		sourceIndex, id := pair[0], pair[1]
		s := apcFixture{TrainingAPCPath: {tag("[character]"), tag("[index]"), num(sourceIndex), tag("[apc index]"), num(id), tag("[usable contents]"), txt("boost up event"), tag("[/usable contents]"), tag("[type]"), txt("event"), num(int32(EventID)), tag("[/character]")}, "list/aicharacter.lst": {num(id), txt("aicharacter/custom/fixture.aic")}, "aicharacter/custom/fixture.aic": {tag("[warlike]"), num(0), tag("[quick skill]"), tag("[/quick skill]")}}
		c := &Catalog{}
		if e := c.BindTrainingAPCs(s); e != nil {
			t.Fatal(e)
		}
		if len(c.TrainingAPCs) != 1 || c.TrainingAPCs[0].Template != uint32(id) || c.TrainingAPCs[0].SourceIndex != uint32(sourceIndex) {
			t.Fatal("did not follow source")
		}
		// The source identity must never fall back to the AIC template.
		s[TrainingAPCPath][2] = num(0)
		if e := c.BindTrainingAPCs(s); e == nil {
			t.Fatal("missing special APC identity accepted")
		}
		s[TrainingAPCPath][2] = num(sourceIndex)
		s["aicharacter/custom/fixture.aic"][1] = num(1)
		if e := c.BindTrainingAPCs(s); e == nil {
			t.Fatal("unverified combat NPC accepted")
		}
	}
}
