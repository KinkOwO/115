package main

import (
	"strings"
	"testing"
)

func TestCatalogCheckRequiresPreparedSourceAndExplicitDomains(t *testing.T) {
	if _, err := (pvfCoreCatalogs{}).checkReport("items"); err == nil {
		t.Fatal("unprepared source accepted")
	}
	c := pvfCoreCatalogs{sourceChecksum: strings.Repeat("a", 64)}
	if _, err := c.checkReport(""); err == nil {
		t.Fatal("empty selection accepted")
	}
	if _, err := c.checkReport("unverified"); err == nil {
		t.Fatal("unknown source domain accepted")
	}
	got, err := c.checkReport("items,characters")
	if err != nil || got["domain_count"] != 2 || got["storage_accessed"] != false || got["runtime_started"] != false {
		t.Fatal(got, err)
	}
}
