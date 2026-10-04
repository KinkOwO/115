package launcher

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// pathKeys are the environment entries the profiles express as file paths. They are
// resolved against the module directory and therefore also become required
// dependencies: a profile whose archive or policy is missing cannot start.
var pathKeys = map[string]bool{
	"DFO_CHARACTER_CATALOG":         true,
	"DFO_CHARACTER_RULES":           true,
	"DFO_LOGIN_RESPONSE":            true,
	"DFO_SKILL_CATALOG":             true,
	"DFO_EQUIPMENT_WEAR_RULES":      true,
	"DFO_ODYSSEY_DUNGEON_CATALOG":   true,
	"DFO_ODYSSEY_WEAPON_BOX":        true,
	"DFO_ODYSSEY_GROWTH":            true,
	"DFO_LOOT_CATALOG":              true,
	"DFO_ODYSSEY_COIN_RULES":        true,
	"DFO_FATIGUE_RULES":             true,
	"DFO_CLEAR_CUBE_SOURCE":         true,
	"DFO_ODYSSEY_CHAPTER_DROP":      true,
	"DFO_PVF_ITEM_SHOP_POLICY":      true,
	"DFO_PVF_BOX_POLICY":            true,
	"DFO_PVF_CHARACTER_POLICY":      true,
	"DFO_PVF_LAYER_REVISIT_POLICY":  true,
	"DFO_PVF_SCRIPT_WARP_POLICY":    true,
	"DFO_PVF_ARCHIVE":               true,
	"DFO_PVF_ENHANCEMENT_POLICY":    true,
	"DFO_PVF_VAULT_POLICY":          true,
	"DFO_PVF_DROP_POLICY":           true,
	"DFO_PVF_SCENE_POLICY":          true,
	"DFO_PVF_CONTENT_POLICY":        true,
	"DFO_PVF_SELECTION_POLICY":      true,
	"DFO_PVF_LOTTERY_POLICY":        true,
}

// flagKeys are the boolean entries. Only "0" and "1" are accepted, exactly as the
// Python validated them.
var flagKeys = map[string]bool{
	"DFO_CHANNEL_IDENTITY":             true,
	"DFO_DETAIL_WORN":                  true,
	"DFO_SHOP_RELEASE":                 true,
	"DFO_VAULT_PURCHASE_RELEASE":       true,
	"DFO_ODYSSEY_REWARDS_RELEASE":      true,
	"DFO_ODYSSEY_TEMPORARY_CREDITS":    true,
	"DFO_SHOP_OPEN_ALL":                true,
	"DFO_PVF_VERIFY_BASELINES":         true,
	"DFO_ATTUNEMENT_REBALANCE":         true,
	"DFO_FATIGUE_FREE":                 true,
}

// Profile is a loaded launcher profile: which binary to run, which files it needs, and
// the environment it runs with.
type Profile struct {
	Path     string
	Binary   string
	Required []string
	Env      map[string]string
}

// profileFile is the on-disk shape. Requiring exactly these two fields is what the
// Python enforced, and it is what catches a profile edited into something else.
type profileFile struct {
	Binary      string            `json:"binary"`
	Environment map[string]string `json:"environment"`
}

// LoadProfile reads a launcher profile and resolves everything path-like against the
// module directory, mirroring repair_profile.load_profile.
//
// Not yet ported: the per-key value validation (the numeric, checksum, omen, oath and
// i-spins rules). Those keys are forwarded verbatim, and the server validates its own
// environment at startup, so a bad value still fails loudly - just one step later. The
// binary, the required-file list and the path resolution - which is what the launcher
// acts on - are ported exactly.
func LoadProfile(path, module string) (Profile, error) {
	profile := Profile{Path: path, Env: map[string]string{}}
	data, err := readFileAllowingBOM(path)
	if err != nil {
		return profile, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return profile, fmt.Errorf("profile %s: %w", path, err)
	}
	binaryRaw, hasBinary := raw["binary"]
	environmentRaw, hasEnvironment := raw["environment"]
	if len(raw) != 2 || !hasBinary || !hasEnvironment {
		return profile, fmt.Errorf("profile %s: expected exactly binary and environment fields", path)
	}

	resolve := func(value string) (string, error) {
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("profile %s: expected a nonempty file path", path)
		}
		if filepath.IsAbs(value) {
			return filepath.Clean(value), nil
		}
		return filepath.Clean(filepath.Join(module, value)), nil
	}

	var binary string
	if err := json.Unmarshal(binaryRaw, &binary); err != nil {
		return profile, fmt.Errorf("profile %s: binary must be a path string", path)
	}
	binary, err = resolve(binary)
	if err != nil {
		return profile, err
	}
	profile.Binary = binary
	profile.Required = []string{binary}

	var environment map[string]any
	if err := json.Unmarshal(environmentRaw, &environment); err != nil {
		return profile, fmt.Errorf("profile %s: environment must be an object", path)
	}
	for key, value := range environment {
		text := stringifyValue(value)
		if pathKeys[key] {
			resolved, err := resolve(text)
			if err != nil {
				return profile, fmt.Errorf("profile %s: %s: %w", path, key, err)
			}
			profile.Required = append(profile.Required, resolved)
			profile.Env[key] = resolved
			continue
		}
		if key == "DFO_EQUIPMENT_FULL_CATALOG" {
			resolved, err := resolve(text)
			if err != nil {
				return profile, fmt.Errorf("profile %s: %s: %w", path, key, err)
			}
			profile.Required = append(profile.Required, resolved+".data", resolved+".index.json")
			profile.Env[key] = resolved
			continue
		}
		if flagKeys[key] && (text == "0" || text == "1") {
			profile.Env[key] = text
			continue
		}
		profile.Env[key] = text
	}
	return profile, nil
}

// stringifyValue renders a JSON scalar the way the profiles write them: booleans become
// 0/1 so the environment receives what the server expects.
func stringifyValue(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		if typed {
			return "1"
		}
		return "0"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", typed), "0"), ".")
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(encoded)
	}
}

// readFileAllowingBOM reads a file and tolerates the byte-order mark the existing
// configuration files carry.
func readFileAllowingBOM(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return stripBOM(data), nil
}
