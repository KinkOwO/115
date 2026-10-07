package launcher

import (
	"os"
	"path/filepath"
)

// EnvRule is one environment decision the probe makes before starting the server.
//
// Two shapes occur, both measured from channel_probe.py rather than assumed:
//
//   - OverrideWins: the same variable may come from an override file (named by the
//     variable's own value, resolved against the module) or fall back to a default path;
//     whichever file exists is used.
//   - Fixed: the variable is always set to one path, whether or not the file exists,
//     because the gateway only ever finds these tables through an absolute path (its
//     working directory is the package root).
type EnvRule struct {
	// Variable is the environment variable being decided.
	Variable string
	// OverrideEnv names the variable whose value points at an override file, if any.
	OverrideEnv string
	// Relative paths are resolved against the module directory, mirroring the Python's
	// `project / ...`.
	OverrideRelative string
	DefaultRelative  string
	// Fixed marks the always-set shape; Relative then holds the single path.
	Fixed    bool
	Relative string
}

// probeEnvRules mirrors channel_probe.py's environment decisions. Paths are the ones that
// file uses; the accompanying test re-reads the Python source and fails if this table and
// it disagree, so the two cannot drift apart silently - a drift that would otherwise show
// up as a missing feature in game rather than as a start-up error.
var probeEnvRules = []EnvRule{
	{
		Variable:         "DFO_ODYSSEY_COIN_RULES",
		OverrideEnv:      "DFO_ODYSSEY_COIN_RULES",
		DefaultRelative:  "configs/odyssey-currency.json",
	},
	{
		Variable:         "DFO_ODYSSEY_WEAPON_BOX",
		OverrideEnv:      "DFO_ODYSSEY_WEAPON_BOX",
		DefaultRelative:  "configs/odyssey-weapon-box-release.json",
	},
	{
		Variable:         "DFO_ODYSSEY_GROWTH",
		OverrideEnv:      "DFO_ODYSSEY_GROWTH",
		DefaultRelative:  "configs/odyssey-growth-release.json",
	},
	{
		Variable:         "DFO_ODYSSEY_CHAPTERS",
		OverrideEnv:      "DFO_ODYSSEY_CHAPTERS",
		DefaultRelative:  "configs/odyssey-chapters-release.json",
	},
	{
		Variable:         "DFO_ODYSSEY_CHAPTER_DROP",
		OverrideEnv:      "DFO_ODYSSEY_CHAPTER_DROP",
		DefaultRelative:  "configs/odyssey-chapter-drop-release.json",
	},
	{
		// The one rule whose override is not named by its own variable: the Python tests
		// the wear-rules candidate file for existence and uses it when present.
		Variable:        "DFO_EQUIPMENT_WEAR_RULES",
		DefaultRelative: "configs/equipment-wear.full-candidate.json",
	},
	{Variable: "DFO_ATTUNEMENT_REWARDS", Fixed: true, Relative: "configs/attunement-rewards.generated.json"},
	{Variable: "DFO_EQUIPMENT_JOURNAL_RULES", Fixed: true, Relative: "configs/equipment-journal.generated.json"},
	{Variable: "DFO_EQUIPMENT_CREATE_COST", Fixed: true, Relative: "configs/equipment-create-cost.generated.json"},
}

// ProbeEnvironment computes the environment additions the probe makes before launching the
// server. base supplies the values an override variable may already carry, exactly as the
// Python read them from its own environment.
//
// An override is honoured only when the file it names actually exists, and a default only
// when that file exists too; a variable with neither is left unset rather than pointed at
// a missing file, which is what the Python's existence guards amount to.
func ProbeEnvironment(module string, base map[string]string) map[string]string {
	env := map[string]string{}
	resolve := func(value string) string {
		if filepath.IsAbs(value) {
			return filepath.Clean(value)
		}
		return filepath.Clean(filepath.Join(module, value))
	}
	exists := func(path string) bool {
		info, err := os.Stat(path)
		return err == nil && !info.IsDir()
	}

	for _, rule := range probeEnvRules {
		if rule.Fixed {
			env[rule.Variable] = resolve(rule.Relative)
			continue
		}
		if rule.OverrideEnv != "" {
			if raw, ok := base[rule.OverrideEnv]; ok && raw != "" {
				override := resolve(raw)
				if exists(override) {
					env[rule.Variable] = override
					continue
				}
			}
		}
		if rule.OverrideRelative != "" {
			override := resolve(rule.OverrideRelative)
			if exists(override) {
				env[rule.Variable] = override
				continue
			}
		}
		def := resolve(rule.DefaultRelative)
		if exists(def) {
			env[rule.Variable] = def
		}
	}
	return env
}

// ApplyProbeEnvironment folds the probe's decisions into a server environment, keeping the
// KEY-unique, last-wins property BuildServerEnv establishes.
func ApplyProbeEnvironment(serverEnv []string, additions map[string]string) []string {
	merged := make([]string, 0, len(serverEnv)+len(additions))
	index := map[string]int{}
	for _, entry := range serverEnv {
		key, _, found := cutEntry(entry)
		if !found {
			merged = append(merged, entry)
			continue
		}
		if at, ok := index[key]; ok {
			merged[at] = entry
			continue
		}
		index[key] = len(merged)
		merged = append(merged, entry)
	}
	for key, value := range additions {
		entry := key + "=" + value
		if at, ok := index[key]; ok {
			merged[at] = entry
			continue
		}
		index[key] = len(merged)
		merged = append(merged, entry)
	}
	return merged
}

func cutEntry(entry string) (string, string, bool) {
	for i := 0; i < len(entry); i++ {
		if entry[i] == '=' {
			return entry[:i], entry[i+1:], true
		}
	}
	return entry, "", false
}
