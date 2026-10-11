package cashshop

import (
	"strings"
	"testing"
)

func TestNativeContractTermsAreSourceDriven(t *testing.T) {
	text := `[type]
117
[target item]
999
[item]
100
[term]
5
[/term]
[/item]
[item]
101
[term]
3
[/term]
[is term unit minute]
1
[/is term unit minute]
[/item]
[item]
102
[term]
7
[/term]
[restrict]
1
[/restrict]
[/item]
[name]
` + "`7 Days`"
	r, e := ParseContractRules(strings.Repeat("a", 64), strings.Repeat("b", 64), text)
	if e != nil {
		t.Fatal(e)
	}
	if r.Items[100].DurationSecond != 5*86400 || r.Items[101].DurationSecond != 180 || !r.Restricted[102] {
		t.Fatal(r)
	}
	if _, ok := r.Items[999]; ok {
		t.Fatal("display icon became an alias")
	}
	restore, e := InstallContractRules(r)
	if e != nil {
		t.Fatal(e)
	}
	defer restore()
	r.Items[100] = Contract{Type: 22, DurationSecond: 99}
	got, _ := ResolveContract(100)
	if got.DurationSecond != 5*86400 {
		t.Fatal("runtime registry aliases input map")
	}
	if _, ok := ResolveContract(102); ok {
		t.Fatal("restricted contract activated")
	}
	changed, e := ParseContractRules(r.Source, r.SHA256, strings.Replace(text, "5\n[/term]", "9\n[/term]", 1))
	if e != nil {
		t.Fatal(e)
	}
	if changed.Items[100].DurationSecond != 9*86400 {
		t.Fatal("changed source duration was ignored")
	}
}
func TestNativeContractRejectsConflictingAndIncompleteTerms(t *testing.T) {
	for _, text := range []string{`[type]
22
[item]
10
[/item]`, `[type]
22
[item]
10
[term]
1
[/term]
[/item]
[type]
27
[item]
10
[term]
2
[/term]
[/item]`} {
		if _, e := ParseContractRules("", "", text); e == nil {
			t.Fatal("invalid source accepted")
		}
	}
}
