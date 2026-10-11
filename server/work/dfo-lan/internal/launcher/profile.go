package launcher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"dfolan/internal/gamedata"
)

// pathKeys are the environment entries the profiles express as file paths. They are
// resolved against the module directory and therefore also become required
// dependencies: a profile whose archive or policy is missing cannot start.
var pathKeys = map[string]bool{
	"DFO_CHARACTER_CATALOG":        true,
	"DFO_CHARACTER_RULES":          true,
	"DFO_LOGIN_RESPONSE":           true,
	"DFO_SKILL_CATALOG":            true,
	"DFO_EQUIPMENT_WEAR_RULES":     true,
	"DFO_ODYSSEY_DUNGEON_CATALOG":  true,
	"DFO_ODYSSEY_WEAPON_BOX":       true,
	"DFO_ODYSSEY_GROWTH":           true,
	"DFO_LOOT_CATALOG":             true,
	"DFO_ODYSSEY_COIN_RULES":       true,
	"DFO_FATIGUE_RULES":            true,
	"DFO_CLEAR_CUBE_SOURCE":        true,
	"DFO_ODYSSEY_CHAPTER_DROP":     true,
	"DFO_PVF_ITEM_SHOP_POLICY":     true,
	"DFO_PVF_BOX_POLICY":           true,
	"DFO_PVF_CHARACTER_POLICY":     true,
	"DFO_PVF_LAYER_REVISIT_POLICY": true,
	"DFO_PVF_SCRIPT_WARP_POLICY":   true,
	"DFO_PVF_ARCHIVE":              true,
	"DFO_PVF_ENHANCEMENT_POLICY":   true,
	"DFO_PVF_VAULT_POLICY":         true,
	"DFO_PVF_DROP_POLICY":          true,
	"DFO_PVF_SCENE_POLICY":         true,
	"DFO_PVF_CONTENT_POLICY":       true,
	"DFO_PVF_SELECTION_POLICY":     true,
	"DFO_PVF_LOTTERY_POLICY":       true,
}

// flagKeys are the boolean entries. Only "0" and "1" are accepted, exactly as the
// Python validated them.
var flagKeys = map[string]bool{
	"DFO_CHANNEL_IDENTITY":          true,
	"DFO_DETAIL_WORN":               true,
	"DFO_SHOP_RELEASE":              true,
	"DFO_VAULT_PURCHASE_RELEASE":    true,
	"DFO_ODYSSEY_REWARDS_RELEASE":   true,
	"DFO_ODYSSEY_TEMPORARY_CREDITS": true,
	"DFO_SHOP_OPEN_ALL":             true,
	"DFO_PVF_VERIFY_BASELINES":      true,
	"DFO_ATTUNEMENT_REBALANCE":      true,
	"DFO_FATIGUE_FREE":              true,
	// 团本入场事件的诊断候选开关；服务端源码里与 "1" 比较。
	"DFO_RAID_OPEN_EVENTS_PROBE": true,
}

// bakalModes 是服务端源码真正认的 DFO_BAKAL_MODE 取值。
//
// 有意只收 "unlimited"：那处比较就是 == "unlimited"。放行其它字面量只会让人
// 以为改了口径而实际走默认分支；要加新口径必须先在服务端源码里落地。
var bakalModes = map[string]bool{
	"unlimited": true,
}

// pvfDomains is the whitelist a profile's DFO_PVF_CATALOGS may select from. It is not a
// second copy of the list: it is the server's own gamedata.SupportedDomains, because the
// profile decides which content source the gateway reads, and a domain the server does
// not serve would otherwise pass here and fail much later with "domain is not enabled",
// after the gateway is already up. scripts/repair_profile.py keeps the same list for the
// same reason; its comment names this constant, and TestPVFDomainsMatchThePythonWhitelist
// is the drift alarm for the two.
var pvfDomains = func() map[string]bool {
	domains := map[string]bool{}
	for _, domain := range strings.Split(gamedata.SupportedDomains, ",") {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			domains[domain] = true
		}
	}
	return domains
}()

// The per-key value patterns, transcribed from repair_profile.load_profile. Go's regexp
// has no fullmatch, so every pattern is anchored instead.
var (
	dropPercentValue = regexp.MustCompile(`^[0-9]{1,5}$`)
	sha256Value      = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)
	omenInfoValue    = regexp.MustCompile(`^[0-9a-fA-Fx,; \-]+$`)
	oathGradesValue  = regexp.MustCompile(`^\d{1,3}(,\d{1,3})?$`)
	// 调律（attunement）的两个倍率：正整数，上限见 validMultiplier。
	// 键名与取值口径来自服务端源码里真正读它们的地方（见 applyEnvironment 的注释）。
	multiplierValue = regexp.MustCompile(`^[0-9]{1,5}$`)
)

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
// module directory, mirroring repair_profile.load_profile key by key: the same accepted
// keys, the same value rules (flags, numeric diagnostics, checksums, the PVF domain
// whitelist) and the same refusal of everything else.
//
// The validation is as strict as the Python's for a reason. A profile is hand-edited
// configuration, and a value the Python rejected must not reach the gateway, where the
// same mistake would surface later and far less clearly.
func LoadProfile(path, module string) (Profile, error) {
	profile := Profile{Path: path, Env: map[string]string{}}
	data, err := readFileAllowingBOM(path)
	if err != nil {
		return profile, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		// A valid top level that is not an object is the shape error, not a parse error;
		// the Python's `set(data) != {...}` reported it the same way.
		if trimmed := bytes.TrimSpace(data); len(trimmed) > 0 && trimmed[0] != '{' {
			return profile, fmt.Errorf("profile %s: Expected binary and environment fields", path)
		}
		return profile, fmt.Errorf("profile %s: %w", path, err)
	}
	binaryRaw, hasBinary := raw["binary"]
	environmentRaw, hasEnvironment := raw["environment"]
	if len(raw) != 2 || !hasBinary || !hasEnvironment {
		return profile, fmt.Errorf("profile %s: Expected binary and environment fields", path)
	}

	binary, err := resolveProfilePath(module, binaryRaw)
	if err != nil {
		return profile, fmt.Errorf("profile %s: binary: %w", path, err)
	}
	profile.Binary = binary
	profile.Required = []string{binary}

	var environment profileEnvironment
	if err := json.Unmarshal(environmentRaw, &environment); err != nil {
		return profile, fmt.Errorf("profile %s: environment must be an object: %w", path, err)
	}
	// Member order, not map order: the Python reported the first bad entry it met, and a
	// message that changes between runs is not worth the saved lines.
	for _, key := range environment.keys {
		if err := profile.applyEnvironment(module, key, environment.values[key]); err != nil {
			return profile, fmt.Errorf("profile %s: %w", path, err)
		}
	}
	return profile, nil
}

// profileEnvironment is the profile's environment object kept in JSON member order.
type profileEnvironment struct {
	keys   []string
	values map[string]json.RawMessage
}

// UnmarshalJSON reads the object member by member, preserving the order json.loads gave
// the Python (which iterated the dict in file order). A duplicate key keeps its first
// position and its last value, exactly as json.loads did.
func (e *profileEnvironment) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if delimiter, ok := token.(json.Delim); !ok || delimiter != '{' {
		return fmt.Errorf("environment must be an object")
	}
	e.values = map[string]json.RawMessage{}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("environment keys must be strings")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		if _, seen := e.values[key]; !seen {
			e.keys = append(e.keys, key)
		}
		e.values[key] = value
	}
	_, err = decoder.Token()
	return err
}

// applyEnvironment validates and stores one environment entry, following the branch order
// of repair_profile.load_profile. An entry that matches no branch is an error rather than
// a value to forward: that is what catches a typo in a key name.
func (p *Profile) applyEnvironment(module, key string, raw json.RawMessage) error {
	text, isString := profileString(raw)
	switch {
	case pathKeys[key]:
		resolved, err := resolveProfilePath(module, raw)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		p.Required = append(p.Required, resolved)
		p.Env[key] = resolved
	case key == "DFO_EQUIPMENT_FULL_CATALOG":
		resolved, err := resolveProfilePath(module, raw)
		if err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
		// The catalog is read as a pair, so both halves are as required as the path.
		p.Required = append(p.Required, resolved+".data", resolved+".index.json")
		p.Env[key] = resolved
	case key == "DFO_PVF_CATALOGS":
		if !isString {
			return invalidProfileValue(key)
		}
		domains := strings.Split(text, ",")
		seen := map[string]bool{}
		for index := range domains {
			domains[index] = strings.TrimSpace(domains[index])
			if !pvfDomains[domains[index]] || seen[domains[index]] {
				return fmt.Errorf("Invalid PVF candidate domains: %s", text)
			}
			seen[domains[index]] = true
		}
		p.Env[key] = strings.Join(domains, ",")
	case key == "DFO_HELL_PARTY_DROP_PERCENT" && isString && validDropPercent(text):
		// Independent Hell numerical multiplier; 100 = 1x, which is the Go default. The
		// value is normalised the way str(int(value)) did ("0100" -> "100").
		percent, err := strconv.Atoi(text)
		if err != nil {
			return invalidProfileValue(key)
		}
		p.Env[key] = strconv.Itoa(percent)
	case key == "DFO_PVF_SHA256" && isString && (text == "" || sha256Value.MatchString(text)):
		// Empty means "derive it from the inner archive"; a value pins the version and is
		// lower-cased so the comparison cannot depend on how it was typed.
		p.Env[key] = strings.ToLower(text)
	case key == "DFO_OMEN_INFO" && isString && (text == "" || omenInfoValue.MatchString(text)):
		// Diagnostic payload for noti 2836; empty is the normal path.
		p.Env[key] = text
	case key == "DFO_OATH_GRADES" && isString && (text == "" || oathGradesValue.MatchString(text)):
		// Diagnostic: fixed oath grades, "45" or "45,45"; empty is the normal path.
		p.Env[key] = text
	case key == "DFO_ISPINS_MODE" && isString && (text == "unlimited" || text == "weekly"):
		p.Env[key] = text
	case key == "DFO_ATTUNEMENT_QUANTITY_MULTIPLIER" && isString && validMultiplier(text):
		// 调律数量倍率。消费点：服务端调律（attunement）奖励换算。1 = 官方数值。
		// 上限与 DFO_HELL_PARTY_DROP_PERCENT 同口径（10000）。
		p.Env[key] = normalizeMultiplier(text)
	case key == "DFO_ATTUNEMENT_RARITY_WEIGHT_MULTIPLIER" && isString && validMultiplier(text):
		// 调律品质权重倍率，与上面同一消费族，取值口径一致。
		p.Env[key] = normalizeMultiplier(text)
	case key == "DFO_BAKAL_MODE" && isString && bakalModes[text]:
		// 巴卡尔团本次数口径。服务端源码里的比较是
		//   os.Getenv("DFO_BAKAL_MODE") == "unlimited"
		// 所以这里**只放行源码真正认的字面量**：放行别的值，等于让玩家以为改了口径、
		// 实际服务端仍走默认分支。要加新口径必须先在服务端源码里落地。
		p.Env[key] = text
	case flagKeys[key] && (text == "0" || text == "1"):
		// Only the two literal strings: a JSON number or boolean is not a flag value, and
		// the Python rejected those too.
		p.Env[key] = text
	default:
		return invalidProfileValue(key)
	}
	return nil
}

// validDropPercent mirrors the "[0-9]{1,5}" test plus the "at most 10000" cap.
func validDropPercent(text string) bool {
	if !dropPercentValue.MatchString(text) {
		return false
	}
	percent, err := strconv.Atoi(text)
	return err == nil && percent <= 10000
}

// resolveProfilePath resolves one profile path entry against the module directory,
// mirroring the Python's resolve(): an absolute path is used as written, everything else
// is joined to the module. A non-string or empty value is rejected, which is what the
// Python's "Expected nonempty file path" did.
func resolveProfilePath(module string, raw json.RawMessage) (string, error) {
	value, isString := profileString(raw)
	if !isString || value == "" {
		return "", fmt.Errorf("Expected nonempty file path")
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value), nil
	}
	return filepath.Clean(filepath.Join(module, value)), nil
}

// profileString decodes a JSON string and reports false for every other type. JSON null
// has to be rejected explicitly: unmarshalling it into a string succeeds and leaves it
// empty, which would silently accept a value the Python refused.
func profileString(raw json.RawMessage) (string, bool) {
	if string(bytes.TrimSpace(raw)) == "null" {
		return "", false
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return "", false
	}
	return text, true
}

// invalidProfileValue is the message every unknown key and invalid value produced in the
// Python, kept verbatim so a rejected profile reads the same either way.
func invalidProfileValue(key string) error {
	return fmt.Errorf("Unknown profile key or invalid value: %s", key)
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

// validMultiplier 与 validDropPercent 同口径：1..5 位数字，且不超过 10000。
//
// 有意不设下限 1：0 是"关掉加成"的合理写法，服务端按默认处理；
// 拒绝 0 会把一个本可启动的配置挡下来。
func validMultiplier(text string) bool {
	if !multiplierValue.MatchString(text) {
		return false
	}
	n, err := strconv.Atoi(text)
	return err == nil && n <= 10000
}

// normalizeMultiplier 去掉前导零（"0005" -> "5"），与 dropPercent 的处理一致。
func normalizeMultiplier(text string) string {
	n, err := strconv.Atoi(text)
	if err != nil {
		return text
	}
	return strconv.Itoa(n)
}
