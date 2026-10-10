package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

// Loading the shipped profile catches drift between local server policy and the
// replacement Go launcher before the player tries the default game entry.
func TestLocalDefaultProfileSurvivesGoLauncher(t *testing.T) {
	module, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	profile, err := LoadProfile(filepath.Join(module, "configs", "pvf-default.json"), module)
	if err != nil {
		t.Fatal(err)
	}
	env := BuildServerEnv(nil, profile.Env, false)
	for key, want := range map[string]string{
		"DFO_LOCAL_GM":                            "1",
		"DFO_ATTUNEMENT_QUANTITY_MULTIPLIER":      "5",
		"DFO_ATTUNEMENT_RARITY_WEIGHT_MULTIPLIER": "5",
		"DFO_BAKAL_MODE":                          "unlimited",
		"DFO_ISPINS_MODE":                         "unlimited",
		"DFO_FATIGUE_FREE":                        "1",
	} {
		if got, ok := lookup(env, key); !ok || got != want {
			t.Errorf("%s = %q, present %v; want %q", key, got, ok, want)
		}
	}
}

func TestLocalPolicyProfileValidation(t *testing.T) {
	for _, key := range []string{"DFO_ATTUNEMENT_QUANTITY_MULTIPLIER", "DFO_ATTUNEMENT_RARITY_WEIGHT_MULTIPLIER", "DFO_BAKAL_MODE"} {
		cases := map[string]string{`"1"`: "1", `"005"`: "5", `"100"`: "100", `"0"`: "", `"101"`: "", `"1000"`: "", `"-1"`: "", `5`: "", `null`: "", `true`: "", `" 5"`: ""}
		if key == "DFO_BAKAL_MODE" {
			cases = map[string]string{`"unlimited"`: "unlimited", `"weekly"`: "weekly", `"sometimes"`: "", `1`: "", `null`: ""}
		}
		for raw, want := range cases {
			t.Run(key+"/"+raw, func(t *testing.T) {
				module := t.TempDir()
				path := filepath.Join(module, "profile.json")
				data := `{"binary":"bin/x.exe","environment":{"` + key + `":` + raw + `}}`
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
				profile, err := LoadProfile(path, module)
				if want == "" {
					if err == nil {
						t.Fatalf("accepted invalid value %s", raw)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if got := profile.Env[key]; got != want {
					t.Fatalf("value %q, want %q", got, want)
				}
			})
		}
	}
}
