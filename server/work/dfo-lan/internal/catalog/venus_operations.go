package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
)

const VenusOperationsPath = "contents/2025/venus/etc/venus.cos"

// Venus uses a text COS, whereas Corridor and Dusky use binary ETC scripts.
func importVenusOperations(a *pvf.Archive, contents map[string]LegionContent) error {
	c, ok := contents["Venus Legion"]
	if !ok {
		return nil
	}
	if _, ok := a.FindFile(VenusOperationsPath); !ok {
		return nil
	}
	raw, err := a.ReadRaw(VenusOperationsPath)
	if err != nil {
		return err
	}
	text, err := a.ReadText(VenusOperationsPath)
	if err != nil {
		return err
	}
	ops, err := parseVenusOperations(text, raw)
	if err != nil {
		return err
	}
	c.NativeOperations = ops
	contents[c.Name] = c
	return nil
}

func parseVenusOperations(text string, raw []byte) (ops *NativeLegionOperations, err error) {
	// [type] is a scalar in operations but a block in relic definitions.
	// Scope the existing COS reader to each complete operation block.
	root := &legionBlock{}
	for rest := text; ; {
		start := strings.Index(rest, "[operation data set]")
		if start < 0 {
			break
		}
		rest = rest[start:]
		end := strings.Index(rest, "[/operation data set]")
		if end < 0 {
			return nil, fmt.Errorf("unclosed Venus operation")
		}
		end += len("[/operation data set]")
		block, e := parseLegionBlocks(strings.Split(strings.ReplaceAll(rest[:end], "\r\n", "\n"), "\n"), "root")
		if e != nil {
			return nil, e
		}
		root.children = append(root.children, block.children...)
		rest = rest[end:]
	}
	sum := sha256.Sum256(raw)
	ops = &NativeLegionOperations{Path: VenusOperationsPath, SHA256: hex.EncodeToString(sum[:]), Definitions: map[uint16]NativeLegionOperation{}}
	for _, block := range root.children {
		if block.name != "operation data set" {
			continue
		}
		index, err := block.intField("index")
		if err != nil || index <= 0 || index > math.MaxInt16 {
			return nil, fmt.Errorf("invalid Venus operation index")
		}
		if _, dup := ops.Definitions[uint16(index)]; dup {
			return nil, fmt.Errorf("duplicate Venus operation")
		}
		kind, err := block.intField("type")
		if err != nil || kind <= 0 || kind > math.MaxUint32 {
			return nil, fmt.Errorf("invalid Venus operation type")
		}
		d := NativeLegionOperation{Type: uint32(kind)}
		d.MemberLimit, _ = block.stringField("member limit")
		reward := block.child("reward data")
		if reward == nil {
			return nil, fmt.Errorf("Venus operation reward missing")
		}
		for _, line := range reward.lines {
			fields := strings.Fields(line)
			if len(fields) != 6 {
				return nil, fmt.Errorf("invalid Venus reward row")
			}
			v := [5]uint32{}
			for i := range v {
				v[i], err = parseU32Zero(fields[i+1], "Venus reward")
				if err != nil {
					return nil, err
				}
			}
			if v[0] == 0 || v[1] == 0 || v[2] == 0 {
				return nil, fmt.Errorf("invalid Venus reward identity/weight")
			}
			d.Rewards = append(d.Rewards, NativeLegionReward{Difficulty: strings.Trim(fields[0], "`"), Template: v[0], Amount: v[1], Weight: v[2], Tier: v[3], Account: v[4]})
		}
		ops.Definitions[uint16(index)] = d
	}
	if len(ops.Definitions) == 0 {
		return nil, fmt.Errorf("Venus source operations empty")
	}
	return ops, nil
}
