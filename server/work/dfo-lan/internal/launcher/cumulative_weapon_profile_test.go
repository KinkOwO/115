package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCumulativeWeaponAdapterProfileAndMemory(t *testing.T) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	p, err := LoadProfile(filepath.Join(module, "configs", "pvf-default.json"), module)
	if err != nil {
		t.Fatal(err)
	}
	if p.Env["DFO_WEAPON_SKIN_ANIMATION"] != "r1" {
		t.Fatal("default animation adapter missing")
	}
	if got, ok := lookup(BuildServerEnv(nil, p.Env, false), "DFO_WEAPON_SKIN_ANIMATION"); !ok || got != "r1" {
		t.Fatal("adapter lost during environment assembly")
	}
	if clientMemoryBudget != 16<<30 || serverMemoryBudget != 4<<30 {
		t.Fatal("confirmed memory budgets lost")
	}
	for _, raw := range []string{`"unknown"`, `" r1"`, `"r2"`, `1`, `true`, `null`} {
		dir := t.TempDir()
		path := filepath.Join(dir, "profile.json")
		if err := os.WriteFile(path, []byte(`{"binary":"bin/x.exe","environment":{"DFO_WEAPON_SKIN_ANIMATION":`+raw+`}}`), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadProfile(path, dir); err == nil {
			t.Fatalf("accepted invalid adapter %s", raw)
		}
	}
}
