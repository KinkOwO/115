// Package adventureelite defines the owner's opt-in compatibility policy.
// It contains no gameplay content or dungeon allowlist.
package adventureelite

import "os"

const EnvKey = "DFO_ADVENTURE_ELITE"

// EnabledValue deliberately accepts only 1. An unset/invalid value keeps native rules.
func EnabledValue(value string) bool { return value == "1" }

func Enabled() bool { return EnabledValue(os.Getenv(EnvKey)) }
