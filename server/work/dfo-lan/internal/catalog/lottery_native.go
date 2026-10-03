package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
)

// LotteryPolicy supplies a scope for historical audits and the legacy scoped
// importer. Native runtime discovery does not read this policy. It contains no
// source paths, reward rows, weights or counts.
type LotteryPolicy struct {
	Version   int      `json:"version"`
	Items     []uint32 `json:"item_pools"`
	Equipment []uint32 `json:"equipment_pools"`
}

type LotterySourcePool struct {
	SourceItem         uint32      `json:"source_item"`
	SourceScript       string      `json:"source_script"`
	SourceScriptSHA256 string      `json:"source_script_sha256"`
	Candidates         [][3]uint32 `json:"candidates"`
}

type LotteryPoolCatalog struct {
	SourcePVFSHA256 string              `json:"source_pvf_sha256"`
	Pools           []LotterySourcePool `json:"pools"`
}

// LoadLotteryItemPools reads the historical object-candidate representation.
// The returned source projection has the same shape as native PVF discovery;
// runtime validation and historical content checks belong to the consumer.
func LoadLotteryItemPools(path string) (LotteryPoolCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LotteryPoolCatalog{}, err
	}
	var raw struct {
		SourcePVFSHA256 string `json:"source_pvf_sha256"`
		Pools           []*struct {
			SourceItem         uint32                   `json:"source_item"`
			SourceScript       string                   `json:"source_script"`
			SourceScriptSHA256 string                   `json:"source_script_sha256"`
			Candidates         []BoosterRewardCandidate `json:"candidates"`
		} `json:"pools"`
	}
	if err = json.Unmarshal(data, &raw); err != nil {
		return LotteryPoolCatalog{}, err
	}
	out := LotteryPoolCatalog{SourcePVFSHA256: raw.SourcePVFSHA256}
	for _, row := range raw.Pools {
		if row == nil {
			return LotteryPoolCatalog{}, fmt.Errorf("nil lottery pool")
		}
		pool := LotterySourcePool{SourceItem: row.SourceItem, SourceScript: row.SourceScript, SourceScriptSHA256: row.SourceScriptSHA256}
		for _, candidate := range row.Candidates {
			pool.Candidates = append(pool.Candidates, [3]uint32{candidate.Template, candidate.Weight, candidate.Count})
		}
		out.Pools = append(out.Pools, pool)
	}
	return out, nil
}

func LoadLotteryEquipmentPools(path string) (LotteryPoolCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return LotteryPoolCatalog{}, err
	}
	var source LotteryPoolCatalog
	err = json.Unmarshal(data, &source)
	return source, err
}

type LotteryTables struct{ Items, Equipment LotteryPoolCatalog }

func ReadLotteryPolicy(path string) (LotteryPolicy, error) {
	var p LotteryPolicy
	f, err := os.Open(path)
	if err != nil {
		return p, err
	}
	defer f.Close()
	d := json.NewDecoder(f)
	d.DisallowUnknownFields()
	if err := d.Decode(&p); err != nil {
		return p, err
	}
	if err := d.Decode(new(any)); err != io.EOF {
		return p, fmt.Errorf("lottery policy has trailing data")
	}
	if p.Version != 1 || len(p.Items) == 0 || len(p.Equipment) == 0 {
		return p, fmt.Errorf("invalid or empty lottery policy")
	}
	seen := map[uint32]bool{}
	for _, ids := range [][]uint32{p.Items, p.Equipment} {
		for _, id := range ids {
			if id == 0 || seen[id] {
				return p, fmt.Errorf("invalid or repeated lottery policy pool %d", id)
			}
			seen[id] = true
		}
	}
	return p, nil
}

// ParseLotteryCells is the exporter's exact closed [int data] numeric parser.
// Unsupported types and incomplete triples remain unavailable.
func ParseLotteryCells(tokens []pvf.Token) []int32 {
	var values []int32
	inside := false
	for _, token := range tokens {
		if token.Type == 3 && token.Text == "[int data]" {
			inside = true
			continue
		}
		if token.Type == 3 && token.Text == "[/int data]" {
			break
		}
		if inside {
			if token.Type != 0 {
				return nil
			}
			values = append(values, token.Value)
		}
	}
	if len(values) == 0 || len(values)%3 != 0 {
		return nil
	}
	return values
}

func ImportLotteryTables(a *pvf.Archive, index ItemIndex, p LotteryPolicy) (LotteryTables, error) {
	var out LotteryTables
	if a == nil || a.Snapshot().Checksum != index.Source.Checksum || p.Version != 1 || len(p.Items) == 0 || len(p.Equipment) == 0 {
		return out, fmt.Errorf("lottery source/policy mismatch")
	}
	out.Items.SourcePVFSHA256, out.Equipment.SourcePVFSHA256 = index.Source.Checksum, index.Source.Checksum
	seen := map[uint32]bool{}
	for n, ids := range [][]uint32{p.Items, p.Equipment} {
		ids = append([]uint32(nil), ids...)
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		for _, id := range ids {
			item, ok := index.Items[id]
			if id == 0 || seen[id] || !ok || item.ID != id || item.Kind != "stackable" || item.StackableType != "[upgradable legacy]" {
				return out, fmt.Errorf("lottery pool %d has no unique indexed source", id)
			}
			seen[id] = true
			script, err := ReadScript(a, item.Path)
			if err != nil {
				return out, err
			}
			values := ParseLotteryCells(script.Cells)
			if len(values) == 0 {
				return out, fmt.Errorf("lottery pool %d has unsupported source triples", id)
			}
			pool := LotterySourcePool{SourceItem: id, SourceScript: script.Path, SourceScriptSHA256: script.SHA256}
			for j := 0; j < len(values); j += 3 {
				if values[j] < 0 || values[j+1] <= 0 || values[j+2] <= 0 {
					return out, fmt.Errorf("lottery pool %d has invalid source triple", id)
				}
				pool.Candidates = append(pool.Candidates, [3]uint32{uint32(values[j]), uint32(values[j+1]), uint32(values[j+2])})
			}
			if n == 0 {
				out.Items.Pools = append(out.Items.Pools, pool)
			} else {
				out.Equipment.Pools = append(out.Equipment.Pools, pool)
			}
		}
	}
	return out, nil
}
