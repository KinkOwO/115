package cashshop

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"testing"
)

func TestContractForProductResolvesPackageData(t *testing.T) {
	entry := OrdinaryProduct{
		Row: []pvf.Token{{Value: 3400200}, {Value: 590709315}},
		Item: catalog.ScriptRecord{Cells: []pvf.Token{
			{Type: 3, Text: "[package data]"},
			{Value: 10000390}, {Value: 1},
			{Type: 3, Text: "[/package data]"},
		}},
	}
	typ, duration, ok, err := ContractForProduct(entry)
	if err != nil || !ok || typ != PremiumCube || duration != 15*86400 {
		t.Fatalf("contract=%d duration=%d ok=%v err=%v", typ, duration, ok, err)
	}
}

func TestContractForProductResolvesNeoAliases(t *testing.T) {
	entry := OrdinaryProduct{Row: []pvf.Token{{Value: 3002837}, {Value: 50002918}}}
	typ, duration, ok, err := ContractForProduct(entry)
	if err != nil || !ok || typ != PremiumNeoPlus || duration != 23*86400 {
		t.Fatalf("contract=%d duration=%d ok=%v err=%v", typ, duration, ok, err)
	}
}

func TestContractForProductIgnoresOrdinaryItem(t *testing.T) {
	entry := OrdinaryProduct{Row: []pvf.Token{{Value: 3000118}, {Value: 6003}}}
	if _, _, ok, err := ContractForProduct(entry); err != nil || ok {
		t.Fatalf("ordinary item classified as contract ok=%v err=%v", ok, err)
	}
}
