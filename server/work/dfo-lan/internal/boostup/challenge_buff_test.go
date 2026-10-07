package boostup

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestBoostChallengeActualNativeBuffSources(t *testing.T) {
	dir, defs := os.Getenv("US115_TEST_BOOST_CHALLENGE"), os.Getenv("US115_TEST_BOOST_BUFF_SOURCE")
	if dir == "" || defs == "" {
		t.Skip("explicit read-only source exports required")
	}
	read := func(path string) []pvf.Token {
		t.Helper()
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		var c []pvf.Token
		if e = json.Unmarshal(raw, &c); e != nil {
			t.Fatal(e)
		}
		return c
	}
	event := read(filepath.Join(dir, "00-boostupspecupchallenge.evt.tokens.json"))
	cells := read(filepath.Join(defs, "00-variousbufflist.etc.tokens.json"))
	buffs, e := ParseChallengeBuffs(event, cells)
	if e != nil {
		t.Fatal(e)
	}
	if len(buffs) != 2 {
		t.Fatal("source buff count", len(buffs))
	}
	for i, b := range buffs {
		if b.Index != uint32(423+i) || len(b.Effects) != 4 {
			t.Fatal(b)
		}
		for _, name := range []string{"attack_speed", "cast_speed", "move_speed"} {
			if p := b.Effects[name]; len(p) != 1 || p[0].Value != 10 {
				t.Fatal("speed source", name, p)
			}
		}
		want := int32(20)
		if i == 1 {
			want = 10
		}
		if p := b.Effects["skill_attack_bonus"]; len(p) != 1 || p[0].Value != want {
			t.Fatal("skill bonus source", p)
		}
	}
	if !reflect.DeepEqual(buffs[0].Contents, []uint32{175, 191, 174}) || !reflect.DeepEqual(buffs[1].Contents, []uint32{213}) {
		t.Fatal("confused content/raid/channel identifiers")
	}
	if !reflect.DeepEqual(buffs[0].ChannelTypes, []uint32{174, 175, 191}) || !reflect.DeepEqual(buffs[0].AlarmExcludedDungeons, []uint32{100004137, 100004133, 100004134, 100004477}) {
		t.Fatal("source channel/ALARM-only exceptions")
	}
	if _, e = ParseChallengeBuffs(event, nil); e == nil {
		t.Fatal("missing referenced definitions ignored")
	}
	duplicate := append(append([]pvf.Token(nil), cells...), cells...)
	if _, e = ParseChallengeBuffs(event, duplicate); e == nil {
		t.Fatal("duplicate buff definitions ignored")
	}
}
