package catalog

import (
	"dfolan/internal/catalog/pvf"
	"fmt"
	"log"
	"sort"
	"strings"
)

// These streaming consumers keep the standalone importers' path and parser
// rules. The standalone loops remain independent native parity oracles.
type jointSkinImport struct {
	a      *pvf.Archive
	skins  map[uint32]string
	types  map[uint32]skinLabels
	result SkinStorageCatalog
}

func newJointSkinImport(a *pvf.Archive) (*jointSkinImport, error) {
	skins, err := skinList(a)
	if err != nil {
		return nil, err
	}
	return &jointSkinImport{a: a, skins: skins, types: map[uint32]skinLabels{}, result: SkinStorageCatalog{Schema: SkinStorageSchema, Source: a.Snapshot()}}, nil
}

func (b *jointSkinImport) consume(s ItemScript) error {
	id := skinStorageActionParam(s.Cells)
	if id == 0 {
		return nil
	}
	p, ok := b.skins[id]
	if !ok {
		b.result.MissingSkins = append(b.result.MissingSkins, MissingSkin{Template: s.Item.ID, SkinID: id})
		return nil
	}
	kind, ok := b.types[id]
	if !ok {
		cells, err := scriptTokens(b.a, p)
		if err != nil {
			return fmt.Errorf("skin %d (%s): %w", id, p, err)
		}
		kind = skinLabels{Type: skinTypeName(cells), SubType: skinSubTypeName(cells)}
		if kind.Type == "" {
			return fmt.Errorf("skin %d (%s) declares no [type]", id, p)
		}
		b.types[id] = kind
	}
	df, hasDF := damageFontInfo(s.Cells)
	b.result.Entries = append(b.result.Entries, SkinStorageEntry{Template: s.Item.ID, SkinID: id, SkinPath: p, SkinType: kind.Type, SkinSubType: kind.SubType, DamageFontIndex: df, HasDamageFontInfo: hasDF})
	return nil
}

func (b *jointSkinImport) finish() *SkinStorageCatalog {
	sort.Slice(b.result.Entries, func(i, j int) bool { return b.result.Entries[i].Template < b.result.Entries[j].Template })
	// A pointer into b would keep b.a and the full parent directory alive.
	result := b.result
	return &result
}

type jointBoosterImport struct {
	groups                          map[uint32][]BoosterRewardCandidate
	result                          map[uint32]BoosterDefinition
	substituted, sealed, unresolved int
}

func newJointBoosterImport(a *pvf.Archive) (*jointBoosterImport, error) {
	const p = "etc/dungeondroptablebygroup.etc"
	if _, err := a.Tokens(p); err != nil {
		return nil, err
	}
	return &jointBoosterImport{groups: loadSmartDropGroups(a, p), result: map[uint32]BoosterDefinition{}}, nil
}

func (b *jointBoosterImport) consume(s ItemScript) error {
	item := s.Item
	if !strings.HasPrefix(item.Path, "stackable/") {
		return nil
	}
	if !s.Exact {
		return fmt.Errorf("booster scan %d: %w: %s", item.ID, pvf.ErrFileNotFound, item.Path)
	}
	hasInfo := declaresSection(s.Cells, "[booster info]")
	isBooster := strings.Contains(item.StackableType, "booster")
	isPkg := strings.Contains(item.StackableType, "package")
	if hasInfo || isBooster || isPkg {
		pools := parseBoosterInfo(s.Cells)
		if len(pools) == 0 && isPkg {
			pools = parsePackageData(s.Cells)
		}
		if len(pools) > 0 {
			pools = resolveSmartDrop(pools, sectionNumber(s.Cells, "[smart drop group id]"), b.groups, &b.substituted)
			b.result[item.ID] = BoosterDefinition{Template: item.ID, Type: item.StackableType, Pools: pools}
		} else if hasInfo && !isBooster {
			b.result[item.ID] = BoosterDefinition{Template: item.ID, Type: item.StackableType}
			b.sealed++
		} else if hasInfo {
			b.unresolved++
		}
	}
	return nil
}

func (b *jointBoosterImport) finish() map[uint32]BoosterDefinition {
	log.Printf("PVF booster projection: %d definitions, %d smart substitutions, %d sealed markers, %d unparsed booster bodies", len(b.result), b.substituted, b.sealed, b.unresolved)
	return b.result
}
