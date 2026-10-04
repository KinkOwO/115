package launcher

import (
	"os"
	"strings"
)

// LegacyGameplaySwitches are removed when a profile does not declare DFO_PVF_CATALOGS,
// because such a profile is not a native PVF profile and these switches belong to the
// scenario/Odyssey modes it must not silently inherit. Mirrors launch_environment.
var LegacyGameplaySwitches = []string{"DFO_SKILL_RELEASE", "DFO_ODYSSEY_REWARDS_PILOT"}

// BuildServerEnv computes the environment the server runs with, mirroring
// launch_local.py's launch_environment:
//
//  1. start from the caller's environment,
//  2. in JSON mode drop every DFO_PVF_* variable, so the server cannot pick up PVF
//     domains from an earlier shell,
//  3. if the profile does not declare DFO_PVF_CATALOGS, drop the legacy gameplay
//     switches, because a non-native profile must not inherit them,
//  4. overlay the profile's own values last, so the profile always wins.
//
// It is pure on purpose: the environment the server inherits decides which content
// source it reads, and getting that wrong is invisible until gameplay breaks.
func BuildServerEnv(base []string, profileEnv map[string]string, jsonMode bool) []string {
	// Preserve order for readability while keeping last-wins semantics.
	merged := make([]string, 0, len(base)+len(profileEnv))
	index := map[string]int{}
	set := func(key, value string) {
		entry := key + "=" + value
		if at, ok := index[key]; ok {
			merged[at] = entry
			return
		}
		index[key] = len(merged)
		merged = append(merged, entry)
	}

	for _, entry := range base {
		key, value, found := strings.Cut(entry, "=")
		if !found {
			// Not a KEY=VALUE entry; carry it through untouched rather than dropping it.
			merged = append(merged, entry)
			continue
		}
		if jsonMode && strings.HasPrefix(key, "DFO_PVF_") {
			continue
		}
		set(key, value)
	}

	if len(profileEnv) > 0 {
		if _, declaresCatalogs := profileEnv["DFO_PVF_CATALOGS"]; !declaresCatalogs {
			for _, key := range LegacyGameplaySwitches {
				if at, ok := index[key]; ok {
					merged = append(merged[:at], merged[at+1:]...)
					delete(index, key)
					for k, v := range index {
						if v > at {
							index[k] = v - 1
						}
					}
				}
			}
		}
		for key, value := range profileEnv {
			set(key, value)
		}
	}
	return merged
}

// CurrentEnv is the launcher's starting environment, split out so tests can inject one.
func CurrentEnv() []string { return os.Environ() }
