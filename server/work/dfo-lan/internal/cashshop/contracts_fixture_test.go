package cashshop

import (
	"strings"
	"testing"
)

func fixtureContractRules(t *testing.T) *ContractRules {
	t.Helper()
	rules, err := ParseContractRules(strings.Repeat("a", 64), strings.Repeat("b", 64), `
[type]
22
[item]
34
[term]
15
[/term]
[/item]
[type]
27
[item]
45
[term]
7
[/term]
[/item]
[type]
92
[item]
10000389
[term]
7
[/term]
[/item]
[item]
10000390
[term]
15
[/term]
[/item]
[type]
118
[item]
50002918
[term]
23
[/term]
[/item]
`)
	if err != nil {
		t.Fatal(err)
	}
	return rules
}
