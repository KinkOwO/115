package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"testing"
)

func TestTownRobotRosterSourceDetails(t *testing.T) {
	out := os.Getenv("DFO_ROBOT_ROSTER_DETAILS")
	if out == "" {
		t.Skip("explicit source audit")
	}
	a, e := pvf.OpenReadOnly(pvf.Options{Path: `D:\115us\server\work\client-build\Script.inner.pvf`, MaxBytes: 1 << 30}, "45c8db452291abf5f912d23515d1774aa56ced23a5b61ec95ef1f3ec8263c0dc")
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	c, e := catalog.ImportCharacters(a)
	if e != nil {
		t.Fatal(e)
	}
	scripts := []catalog.ScriptRecord{}
	for _, p := range c.Professions {
		v, e := catalog.ResolveScript(a, p.Path)
		if e != nil {
			t.Fatal(e)
		}
		for i := range v.Cells {
			if v.Cells[i].Type == 8 {
				ref, ok := pvf.ParseLocalizedRef(v.Cells[i].Reference)
				if ok {
					name, found, err := a.LocalizedText(ref)
					if err != nil {
						t.Fatal(err)
					}
					if found {
						v.Cells[i].Text = name
					}
				}
			}
		}
		scripts = append(scripts, v)
	}
	for _, path := range []string{"etc/mycharactersapc/mycharacters_apc_skill_autoset_info.etc", "stackable/cash/amplification/strong_amplification.stk"} {
		v, e := catalog.ResolveScript(a, path)
		if e == nil {
			scripts = append(scripts, v)
		}
	}
	raw, _ := json.MarshalIndent(scripts, "", "  ")
	if e = os.WriteFile(out, raw, 0600); e != nil {
		t.Fatal(e)
	}
}
