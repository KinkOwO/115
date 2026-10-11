package cashshop

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
)

const PremiumListPath = "etc/premiumlist_new.etc"

type ContractRules struct {
	Source     string
	SHA256     string
	Items      map[uint32]Contract
	Restricted map[uint32]bool
}

var installedContracts atomic.Pointer[ContractRules]
var premiumTypes = regexp.MustCompile(`(?m)\[type\]\s+(\d+)`)
var premiumItems = regexp.MustCompile(`(?s)\[item\](.*?)\[/item\]`)
var premiumTerms = regexp.MustCompile(`\[term\]\s+(\d+)\s+\[/term\]`)
var premiumMinutes = regexp.MustCompile(`\[is term unit minute\]\s+([01])\s+\[/is term unit minute\]`)

// ParseContractRules reads aliases and actual terms, never translated names or
// target-item icons. Restricted aliases are retained as refusal evidence.
func ParseContractRules(source, digest, text string) (*ContractRules, error) {
	out := &ContractRules{Source: source, SHA256: digest, Items: map[uint32]Contract{}, Restricted: map[uint32]bool{}}
	matches := premiumTypes.FindAllStringSubmatchIndex(text, -1)
	for n, m := range matches {
		typ, _ := strconv.ParseUint(text[m[2]:m[3]], 10, 8)
		if typ == 0 {
			return nil, fmt.Errorf("invalid premium type")
		}
		end := len(text)
		if n+1 < len(matches) {
			end = matches[n+1][0]
		}
		for _, item := range premiumItems.FindAllStringSubmatch(text[m[1]:end], -1) {
			fields := strings.Fields(item[1])
			if len(fields) == 0 {
				return nil, fmt.Errorf("empty premium alias")
			}
			id, err := strconv.ParseUint(fields[0], 10, 32)
			if err != nil || id == 0 {
				return nil, fmt.Errorf("invalid premium alias")
			}
			if strings.Contains(item[1], "[restrict]") {
				out.Restricted[uint32(id)] = true
				continue
			}
			terms := premiumTerms.FindAllStringSubmatch(item[1], -1)
			if len(terms) != 1 {
				return nil, fmt.Errorf("invalid term for premium alias %d", id)
			}
			term, err := strconv.ParseInt(terms[0][1], 10, 64)
			if err != nil || term <= 0 || term > (1<<31-1)/86400 {
				return nil, fmt.Errorf("invalid premium duration")
			}
			multiplier := int64(86400)
			if strings.Contains(item[1], "[is term unit minute]") {
				units := premiumMinutes.FindAllStringSubmatch(item[1], -1)
				if len(units) != 1 {
					return nil, fmt.Errorf("invalid premium time unit")
				}
				if units[0][1] == "1" {
					multiplier = 60
				}
			}
			value := Contract{Type: uint8(typ), DurationSecond: term * multiplier}
			if old, exists := out.Items[uint32(id)]; exists && old != value {
				return nil, fmt.Errorf("conflicting premium alias %d", id)
			}
			out.Items[uint32(id)] = value
		}
	}
	if len(out.Items) == 0 {
		return nil, fmt.Errorf("premium list has no consumable aliases")
	}
	for id := range out.Restricted {
		delete(out.Items, id)
	}
	return out, nil
}

func ImportContractRules(a *pvf.Archive, index *catalog.ItemIndex) (*ContractRules, error) {
	if a == nil {
		return nil, fmt.Errorf("missing contract archive")
	}
	raw, err := a.ReadRaw(PremiumListPath)
	if err != nil {
		return nil, err
	}
	text, err := a.ReadText(PremiumListPath)
	if err != nil {
		return nil, err
	}
	out, err := ParseContractRules(a.Snapshot().Checksum, fmt.Sprintf("%x", sha256.Sum256(raw)), text)
	if err != nil || index == nil {
		return out, err
	}
	if index.Source.Checksum != out.Source {
		return nil, fmt.Errorf("contract item index source mismatch")
	}
	// Single-child usable packages inherit their child's source contract, not an
	// independently maintained duration. Multi-reward boxes keep their own flow.
	pending := map[uint32]uint32{}
	for id, item := range index.Items {
		if item.StackableType != "[usable cera package]" {
			continue
		}
		script, err := catalog.ResolveScript(a, item.Path)
		if err != nil {
			return nil, err
		}
		rows, ok := PackageItems(script)
		if ok && len(rows) == 1 && rows[0].Count == 1 {
			pending[id] = rows[0].Template
		}
	}
	for changed := true; changed; {
		changed = false
		for id, child := range pending {
			value, ok := out.Items[child]
			if !ok {
				continue
			}
			if old, exists := out.Items[id]; exists && old != value {
				return nil, fmt.Errorf("conflicting contract package %d", id)
			}
			out.Items[id] = value
			delete(pending, id)
			changed = true
		}
	}
	return out, nil
}

func InstallContractRules(r *ContractRules) (func(), error) {
	if r == nil || !digestValid(r.Source) || !digestValid(r.SHA256) || len(r.Items) == 0 {
		return nil, fmt.Errorf("invalid native contract rules")
	}
	copy := &ContractRules{Source: r.Source, SHA256: r.SHA256, Items: map[uint32]Contract{}, Restricted: map[uint32]bool{}}
	for id, c := range r.Items {
		if id == 0 || c.Type == 0 || c.DurationSecond <= 0 || r.Restricted[id] {
			return nil, fmt.Errorf("invalid native contract alias %d", id)
		}
		copy.Items[id] = c
	}
	for id, v := range r.Restricted {
		copy.Restricted[id] = v
	}
	old := installedContracts.Swap(copy)
	return func() { installedContracts.Store(old) }, nil
}

func IsRestrictedContract(template uint32) bool {
	rules := installedContracts.Load()
	return rules != nil && rules.Restricted[template]
}
