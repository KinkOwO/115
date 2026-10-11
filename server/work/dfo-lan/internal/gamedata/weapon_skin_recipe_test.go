package gamedata

import (
	"os"
	"testing"
)

func TestWeaponShapeNativeRecipes(t *testing.T) {
	p := os.Getenv("DFO_WEAPON_SKIN_NATIVE_MATRIX")
	if p == "" {
		t.Skip("set current native archive")
	}
	s, e := Open(Options{Mode: PVF, ArchivePath: p})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	r, e := s.WeaponSkinRecipes()
	if e != nil {
		t.Fatal(e)
	}
	if len(r) != 2 || r[1].Material != 10308357 || r[1].Product != 10308358 || r[1].Attach != "[free]" || r[2].Material != 10308836 || r[2].Product != 10308800 || r[2].Attach != "[trade]" {
		t.Fatalf("current PVF recipes changed: %+v", r)
	}
	t.Logf("native recipes: %+v", r)
}
