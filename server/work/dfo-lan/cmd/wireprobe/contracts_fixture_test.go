package main

import (
	"dfolan/internal/cashshop"
	"strings"
	"testing"
)

func installBoosterContractFixture(t *testing.T) {
	t.Helper()
	r, e := cashshop.ParseContractRules(strings.Repeat("a", 64), strings.Repeat("b", 64), `
[type]
22
[item]
31
[term]
3
[/term]
[/item]
[type]
27
[item]
44
[term]
3
[/term]
[/item]
[type]
79
[item]
2660409
[term]
3
[/term]
[/item]
[type]
92
[item]
10000388
[term]
3
[/term]
[/item]
`)
	if e != nil {
		t.Fatal(e)
	}
	restore, e := cashshop.InstallContractRules(r)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(restore)
}
