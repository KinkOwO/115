package launcher

import (
	"slices"
	"testing"
)

func lookup(env []string, key string) (string, bool) {
	for _, entry := range env {
		if name, value, found := cut(entry); found && name == key {
			return value, true
		}
	}
	return "", false
}

func cut(entry string) (string, string, bool) {
	for i := 0; i < len(entry); i++ {
		if entry[i] == '=' {
			return entry[:i], entry[i+1:], true
		}
	}
	return entry, "", false
}

// The environment decides which content source the server reads, so each rule of
// launch_environment is pinned separately.
func TestBuildServerEnv(t *testing.T) {
	base := []string{
		"PATH=C:\\bin",
		"DFO_PVF_CATALOGS=world",
		"DFO_PVF_ARCHIVE=old.pvf",
		"DFO_SHOP_OPEN_ALL=1",
		"DFO_SKILL_RELEASE=1",
		"DFO_ODYSSEY_REWARDS_PILOT=1",
		"NOT_A_PAIR",
	}

	t.Run("json mode drops every DFO_PVF entry", func(t *testing.T) {
		env := BuildServerEnv(base, nil, true)
		for _, key := range []string{"DFO_PVF_CATALOGS", "DFO_PVF_ARCHIVE"} {
			if _, ok := lookup(env, key); ok {
				t.Errorf("JSON mode kept %s", key)
			}
		}
		// Everything else must survive, including the legacy switches: JSON mode is about
		// the content source, not about gameplay switches.
		for _, key := range []string{"PATH", "DFO_SHOP_OPEN_ALL", "DFO_SKILL_RELEASE"} {
			if _, ok := lookup(env, key); !ok {
				t.Errorf("JSON mode dropped %s", key)
			}
		}
	})

	t.Run("a non-native profile drops the legacy switches", func(t *testing.T) {
		env := BuildServerEnv(base, map[string]string{"DFO_SOMETHING": "x"}, false)
		for _, key := range LegacyGameplaySwitches {
			if _, ok := lookup(env, key); ok {
				t.Errorf("a profile without DFO_PVF_CATALOGS kept %s", key)
			}
		}
		if value, ok := lookup(env, "DFO_SOMETHING"); !ok || value != "x" {
			t.Errorf("profile entry = %q, %v; want x", value, ok)
		}
	})

	t.Run("a native profile keeps them and wins over the base", func(t *testing.T) {
		env := BuildServerEnv(base, map[string]string{
			"DFO_PVF_CATALOGS": "world,quests",
			"DFO_PVF_ARCHIVE":  "new.pvf",
			"DFO_SKILL_RELEASE": "0",
		}, false)
		for _, key := range LegacyGameplaySwitches {
			if _, ok := lookup(env, key); !ok {
				t.Errorf("a native profile dropped %s", key)
			}
		}
		if value, _ := lookup(env, "DFO_PVF_ARCHIVE"); value != "new.pvf" {
			t.Errorf("archive = %q, want the profile's value", value)
		}
		if value, _ := lookup(env, "DFO_SKILL_RELEASE"); value != "0" {
			t.Errorf("skill release = %q, want the profile's override", value)
		}
	})

	t.Run("last value wins and entries stay unique", func(t *testing.T) {
		env := BuildServerEnv([]string{"A=1", "A=2"}, map[string]string{"A": "3"}, false)
		var count int
		for _, entry := range env {
			if name, value, found := cut(entry); found && name == "A" {
				count++
				if value != "3" {
					t.Errorf("A = %q, want 3", value)
				}
			}
		}
		if count != 1 {
			t.Errorf("A appears %d times, want exactly one entry", count)
		}
	})

	t.Run("malformed entries are carried through", func(t *testing.T) {
		env := BuildServerEnv([]string{"NOT_A_PAIR"}, nil, false)
		if !slices.Contains(env, "NOT_A_PAIR") {
			t.Error("a malformed entry was dropped instead of carried through")
		}
	})
}
