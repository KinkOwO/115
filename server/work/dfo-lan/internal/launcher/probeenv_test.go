package launcher

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const probeSource = "../../../dfo_probe_tools/channel_probe.py"

// The table in probeenv.go is only trustworthy while it matches the probe it mirrors, and
// the failure mode of drift is silent: a missing table degrades a feature in game instead
// of failing at start-up. So the Python source is re-read here and compared.
//
// The comparison is driven by the probe's own environment assignments - VAR = str(<path
// variable>.resolve()) - rather than by guessing at variable naming, because the two do
// not correspond mechanically (the default is `weapon_box` while the override is
// `box_override`).
func TestProbeEnvironmentTableMatchesTheProbe(t *testing.T) {
	source, err := os.ReadFile(probeSource)
	if err != nil {
		t.Skipf("channel_probe.py is not present (%v); the mirror cannot be checked", err)
	}
	text := strings.ReplaceAll(string(source), "\r\n", "\n")

	paths := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^\s*(\w+)\s*=\s*project\s*/\s*"(configs/[^"]+)"`).
		FindAllStringSubmatch(text, -1) {
		paths[match[1]] = match[2]
	}

	assignments := map[string]string{} // environment variable -> path variable
	for _, match := range regexp.MustCompile(`os\.environ\[\"(DFO_\w+)\"\]\s*=\s*str\(\s*(\w+)\.resolve\(`).
		FindAllStringSubmatch(text, -1) {
		assignments[match[1]] = match[2]
	}

	expected := map[string]string{} // environment variable -> relative default path
	for variable, pathVariable := range assignments {
		if relative, ok := paths[pathVariable]; ok {
			expected[variable] = relative
		}
	}
	if len(expected) < 6 {
		t.Fatalf("only %d config-backed variables were found in the probe; the extraction is wrong", len(expected))
	}

	mirrored := map[string]bool{}
	for _, rule := range probeEnvRules {
		if rule.Fixed {
			if !strings.Contains(text, `"`+rule.Relative+`"`) {
				t.Errorf("fixed path %s no longer appears in the probe", rule.Relative)
			}
			continue
		}
		want, ok := expected[rule.Variable]
		if !ok {
			t.Errorf("%s is mirrored but the probe no longer decides it from a config path", rule.Variable)
			continue
		}
		mirrored[rule.Variable] = true
		if rule.DefaultRelative != want {
			t.Errorf("%s default = %q, the probe says %q", rule.Variable, rule.DefaultRelative, want)
		}
		if rule.OverrideEnv != "" && !strings.Contains(text, `os.environ["`+rule.OverrideEnv+`"]`) {
			t.Errorf("%s names override variable %q, which the probe does not read",
				rule.Variable, rule.OverrideEnv)
		}
	}
	for variable := range expected {
		if !mirrored[variable] {
			t.Errorf("the probe decides %s but the mirror table does not", variable)
		}
	}
}

func TestProbeEnvironmentDecisions(t *testing.T) {
	module := t.TempDir()
	write := func(relative string) string {
		path := filepath.Join(module, relative)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}

	// With nothing on disk only the three fixed variables are set: they are absolute paths
	// the gateway needs regardless, while the rest stay unset instead of pointing nowhere.
	env := ProbeEnvironment(module, nil)
	if len(env) != 3 {
		t.Errorf("with an empty tree, %d variables were set: %v", len(env), env)
	}
	for _, variable := range []string{"DFO_ATTUNEMENT_REWARDS", "DFO_EQUIPMENT_JOURNAL_RULES", "DFO_EQUIPMENT_CREATE_COST"} {
		value, ok := env[variable]
		if !ok {
			t.Errorf("%s was not set", variable)
			continue
		}
		if !filepath.IsAbs(value) {
			t.Errorf("%s = %q, which is not absolute; the gateway's cwd is the package root", variable, value)
		}
	}

	// A default file that exists is used...
	wantDefault := write("configs/odyssey-currency.json")
	env = ProbeEnvironment(module, nil)
	if env["DFO_ODYSSEY_COIN_RULES"] != wantDefault {
		t.Errorf("coin rules = %q, want the default %q", env["DFO_ODYSSEY_COIN_RULES"], wantDefault)
	}

	// ...and an override that exists wins over it.
	wantOverride := write("configs/custom-currency.json")
	env = ProbeEnvironment(module, map[string]string{"DFO_ODYSSEY_COIN_RULES": wantOverride})
	if env["DFO_ODYSSEY_COIN_RULES"] != wantOverride {
		t.Errorf("coin rules = %q, want the override %q", env["DFO_ODYSSEY_COIN_RULES"], wantOverride)
	}

	// An override naming a file that does not exist must fall back, not be passed through.
	env = ProbeEnvironment(module, map[string]string{
		"DFO_ODYSSEY_COIN_RULES": filepath.Join(module, "configs", "absent.json"),
	})
	if env["DFO_ODYSSEY_COIN_RULES"] != wantDefault {
		t.Errorf("a missing override was honoured: %q", env["DFO_ODYSSEY_COIN_RULES"])
	}

	// The wear-rules rule keys off a file, not an environment variable.
	wantWear := write("configs/equipment-wear.full-candidate.json")
	env = ProbeEnvironment(module, nil)
	if env["DFO_EQUIPMENT_WEAR_RULES"] != wantWear {
		t.Errorf("wear rules = %q, want %q", env["DFO_EQUIPMENT_WEAR_RULES"], wantWear)
	}
}

// Folding the additions in must keep the KEY-unique, last-wins contract of the server
// environment, because the server reads the last value it sees.
func TestApplyProbeEnvironmentOverridesWithoutDuplicating(t *testing.T) {
	base := []string{"PATH=C:\\bin", "DFO_ODYSSEY_COIN_RULES=old", "NOT_A_PAIR"}
	merged := ApplyProbeEnvironment(base, map[string]string{"DFO_ODYSSEY_COIN_RULES": "new"})

	var values []string
	for _, entry := range merged {
		if name, value, found := cutEntry(entry); found && name == "DFO_ODYSSEY_COIN_RULES" {
			values = append(values, value)
		}
	}
	if len(values) != 1 || values[0] != "new" {
		t.Errorf("coin rules entries = %v, want exactly one with the new value", values)
	}
	if !strings.Contains(strings.Join(merged, "|"), "NOT_A_PAIR") {
		t.Error("a malformed entry was dropped")
	}
}
