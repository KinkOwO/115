package catalog

import (
	"dfolan/internal/catalog/pvf"
	"os"
	"testing"
)

func asrahanMonsterFixture(id, x int32) ScriptRecord {
	return ScriptRecord{Path: "fixture.cos", SHA256: "fixture", Cells: []pvf.Token{
		{Type: 3, Text: "[MONSTERS]"}, {Type: 3, Text: "[MONSTER INFO]"}, {Type: 3, Text: "[MONSTER TYPE]"}, {Type: 6, Text: "boss"},
		{Type: 3, Text: "[MONSTER INDEX]"}, {Type: 0, Value: id}, {Type: 3, Text: "[POSITION]"}, {Type: 0, Value: x}, {Type: 0, Value: 320},
		{Type: 3, Text: "[/MONSTER INFO]"}, {Type: 3, Text: "[/MONSTERS]"},
	}}
}

func TestAsrahanMonsterSourceFollowsChangedTemplateAndPosition(t *testing.T) {
	for _, v := range [][2]int32{{41, 780}, {42, 793}, {43, 70000}} {
		s, err := ParseAsrahanMonsterSource(asrahanMonsterFixture(v[0], v[1]))
		if err != nil {
			t.Fatal(err)
		}
		m := s.Monsters["boss"]
		if m.Template != uint32(v[0]) || m.X != v[1] || m.Y != 320 {
			t.Fatal("source change ignored", m)
		}
	}
	for _, v := range [][2]int32{{-1, 780}} {
		if _, err := ParseAsrahanMonsterSource(asrahanMonsterFixture(v[0], v[1])); err == nil {
			t.Fatal("bad source accepted", v)
		}
	}
	bad := asrahanMonsterFixture(41, 780)
	bad.Cells[7].Type = 2
	if _, err := ParseAsrahanMonsterSource(bad); err == nil {
		t.Fatal("float position accepted")
	}
	if s, err := ParseAsrahanMonsterSource(asrahanMonsterFixture(0, 0)); err != nil || s.Monsters["boss"].Template != 0 {
		t.Fatal("source placeholder fabricated", s, err)
	}
}

func TestAsrahanMonsterSourceRejectsAmbiguousBosses(t *testing.T) {
	first := asrahanMonsterFixture(41, 780)
	second := asrahanMonsterFixture(41, 790)
	second.Cells[3].Text = "other"
	first.Cells = append(first.Cells[:len(first.Cells)-1], second.Cells[1:len(second.Cells)-1]...)
	first.Cells = append(first.Cells, pvf.Token{Type: 3, Text: "[/MONSTERS]"})
	if _, err := ParseAsrahanMonsterSource(first); err == nil {
		t.Fatal("same positive monster index bound to two kinds")
	}
	second.Cells[5].Value = 42
	first = asrahanMonsterFixture(41, 780)
	first.Cells = append(first.Cells[:len(first.Cells)-1], second.Cells[1:len(second.Cells)-1]...)
	first.Cells = append(first.Cells, pvf.Token{Type: 3, Text: "[/MONSTERS]"})
	if _, err := ParseAsrahanMonsterSource(first); err != nil {
		t.Fatal("distinct source monster identities rejected", err)
	}
}

func TestAsrahanNativeMonsterSpawnBindings(t *testing.T) {
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("native PVF required")
	}
	a, err := OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := importAsrahanRaidRules(a, "contents/2024/asrahanraid/etc/asrahan.etc")
	if err != nil {
		t.Fatal(err)
	}
	for mode, p := range map[string]AsrahanRaidPhase{"normal": r.Normal, "hard": r.Hard} {
		groups, err := CompileAsrahanCombatGroups(r, p)
		if err != nil {
			t.Fatal(err)
		}
		bound := 0
		for _, g := range groups {
			_, m, err := r.MonsterForGroup(p, g.Index)
			if err != nil {
				t.Logf("mode=%s group=%d dungeon=%d unresolved_spawn=%v", mode, g.Index, g.Dungeon, err)
				continue
			}
			if m.Template != g.Monster {
				t.Fatal("foreign source boss")
			}
			bound++
			t.Logf("mode=%s group=%d dungeon=%d kind=%s template=%d position=%d,%d", mode, g.Index, g.Dungeon, m.Kind, m.Template, m.X, m.Y)
		}
		if bound == 0 {
			t.Fatal("no native boss spawns resolved")
		}
		t.Logf("mode=%s bound_spawns=%d/%d source=%s COS=%s", mode, bound, len(groups), a.Snapshot().Checksum, r.MonsterSource.SHA256)
	}
}
