package catalog

import (
	"dfolan/internal/catalog/pvf"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
)

const SkinStorageSchema = "skin-storage-items-v2"

// SkinStorageEntry is one `[add skin storage]` stackable: the template that sends
// CMD507 action 169, plus the PVF facts that name the skin it registers. The
// `[action type]` parameter is already the `list/skin.lst` ID the client keys its
// cargo pages with, so it is also the cargo key; the client confirms this itself by
// reading the same parameter back out of the item record (0x1444EC1B0 requires the
// action word to be 169 and takes the skin ID from the following parameter vector).
// The resolved path and the skin's own `[type]` label are kept because the family a
// skin belongs to is decided by that label, not by the item: `[damage font info]
// [index]` is a different namespace (it selects a `.dfk` in
// `event/damagefontskin/damagefontskin.lst`) and only 73 of the 1860 templates
// declare it while 354 register a `damage font` skin, so it is recorded for
// diagnostics and never used as a key.
type SkinStorageEntry struct {
	Template          uint32 `json:"template"`
	SkinID            uint32 `json:"skin_id"`
	SkinPath          string `json:"skin_path"`
	SkinType          string `json:"skin_type"`
	DamageFontIndex   uint32 `json:"damage_font_index"`
	HasDamageFontInfo bool   `json:"has_damage_font_info"`
}

// SkinKey is the ID NOTI1545/1546 carry for this skin.
func (e SkinStorageEntry) SkinKey() uint32 { return e.SkinID }

// IsDamageFont reports whether the registered skin is a damage font, according to
// the `.skn` itself rather than according to the item.
func (e SkinStorageEntry) IsDamageFont() bool {
	return strings.Contains(strings.ToLower(e.SkinType), "damage font")
}

// MissingSkin records an `[add skin storage]` template whose `[action type]`
// parameter names an ID that `list/skin.lst` does not define (127 of the 1860
// templates, e.g. the 30001..30040 skill-cutscene band). The client's owned-page map
// would accept the ID but the skin script cannot be read, so the entry is exported
// for audit and never registered as cargo.
type MissingSkin struct {
	Template uint32 `json:"template"`
	SkinID   uint32 `json:"skin_id"`
}

type SkinStorageCatalog struct {
	Schema       string              `json:"schema"`
	Source       pvf.ArchiveSnapshot `json:"source"`
	Entries      []SkinStorageEntry  `json:"entries"`
	MissingSkins []MissingSkin       `json:"missing_skins"`
}

// AddSkinStorageLabel is the PVF `[action type]` label of a skin-register
// consumable; its single parameter is the `list/skin.lst` ID.
const AddSkinStorageLabel = "[add skin storage]"

// skinStorageActionParam returns the `[add skin storage]` parameter, or 0 when the
// script is not a skin-register consumable.
func skinStorageActionParam(cells []pvf.Token) uint32 {
	for _, cell := range sectionCells(cells, "[action type]") {
		switch cell.Type {
		case 6:
			if cell.Text != AddSkinStorageLabel {
				return 0
			}
		case 0:
			return uint32(cell.Value)
		}
	}
	return 0
}

// damageFontInfo reads `[damage font info] [index]`, a nested block that
// sectionCells cannot walk because of its inner labels.
func damageFontInfo(cells []pvf.Token) (uint32, bool) {
	for i, cell := range cells {
		if cell.Type != 3 || cell.Text != "[damage font info]" {
			continue
		}
		wantIndex := false
		for _, inner := range cells[i+1:] {
			if inner.Type == 3 {
				if inner.Text == "[/damage font info]" {
					break
				}
				wantIndex = inner.Text == "[index]"
				continue
			}
			if wantIndex && inner.Type == 0 {
				return uint32(inner.Value), true
			}
		}
	}
	return 0, false
}

// skinTypeName returns the `[type]` label a `.skn` declares, e.g. `damage font`.
func skinTypeName(cells []pvf.Token) string {
	for _, cell := range sectionCells(cells, "[type]") {
		if cell.Type == 6 {
			return cell.Text
		}
	}
	return ""
}

// scriptTokens reads a script by its indexed path, retrying the `(r)` spelling the
// source lists use for relocated files.
func scriptTokens(a *pvf.Archive, name string) ([]pvf.Token, error) {
	cells, err := a.Tokens(name)
	if errors.Is(err, pvf.ErrFileNotFound) {
		cells, err = a.Tokens(path.Join(path.Dir(name), "(r)"+path.Base(name)))
	}
	return cells, err
}

// ImportSkinStorage scans every stackable template for `[add skin storage]` and
// resolves each skin ID through `list/skin.lst` down to the skin's own `[type]`.
func ImportSkinStorage(a *pvf.Archive) (SkinStorageCatalog, error) {
	result := SkinStorageCatalog{Schema: SkinStorageSchema, Source: a.Snapshot()}
	skins, e := skinList(a)
	if e != nil {
		return result, e
	}
	index, e := ResolveScript(a, "list/stackable.lst")
	if e != nil {
		return result, e
	}
	rows, e := ParseIndex(index.Cells)
	if e != nil {
		return result, e
	}
	types := map[uint32]string{}
	for _, row := range rows {
		name := row.Path
		if !strings.HasPrefix(name, "stackable/") {
			name = path.Join("stackable", name)
		}
		cells, e := scriptTokens(a, name)
		if e != nil {
			return SkinStorageCatalog{}, fmt.Errorf("template %d (%s): %w", row.ID, name, e)
		}
		skinID := skinStorageActionParam(cells)
		if skinID == 0 {
			continue
		}
		skinPath, ok := skins[skinID]
		if !ok {
			// The item names a skin this PVF build does not define; keep the fact
			// in the export instead of failing the whole catalog.
			result.MissingSkins = append(result.MissingSkins, MissingSkin{Template: row.ID, SkinID: skinID})
			continue
		}
		kind, ok := types[skinID]
		if !ok {
			skinCells, e := scriptTokens(a, skinPath)
			if e != nil {
				return SkinStorageCatalog{}, fmt.Errorf("skin %d (%s): %w", skinID, skinPath, e)
			}
			kind = skinTypeName(skinCells)
			if kind == "" {
				return SkinStorageCatalog{}, fmt.Errorf("skin %d (%s) declares no [type]", skinID, skinPath)
			}
			types[skinID] = kind
		}
		dfIndex, hasDF := damageFontInfo(cells)
		result.Entries = append(result.Entries, SkinStorageEntry{
			Template: row.ID, SkinID: skinID, SkinPath: skinPath, SkinType: kind,
			DamageFontIndex: dfIndex, HasDamageFontInfo: hasDF,
		})
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].Template < result.Entries[j].Template })
	return result, nil
}

func skinList(a *pvf.Archive) (map[uint32]string, error) {
	index, e := ResolveScript(a, "list/skin.lst")
	if e != nil {
		return nil, e
	}
	rows, e := ParseIndex(index.Cells)
	if e != nil {
		return nil, e
	}
	out := make(map[uint32]string, len(rows))
	for _, row := range rows {
		out[row.ID] = row.Path
	}
	return out, nil
}

// LoadSkinStorage reads the exported `[add skin storage]` table and refuses it
// unless it was imported from the same PVF build the running catalogs use.
func LoadSkinStorage(file, expectedSource string) (map[uint32]SkinStorageEntry, error) {
	b, e := os.ReadFile(file)
	if e != nil {
		return nil, e
	}
	var data SkinStorageCatalog
	if e = json.Unmarshal(b, &data); e != nil {
		return nil, e
	}
	if data.Schema != SkinStorageSchema || len(expectedSource) != 64 || data.Source.Checksum != expectedSource || len(data.Entries) == 0 {
		return nil, fmt.Errorf("skin storage catalog source/schema mismatch or empty")
	}
	out := make(map[uint32]SkinStorageEntry, len(data.Entries))
	var previous uint32
	for _, entry := range data.Entries {
		if entry.Template == 0 || entry.Template <= previous {
			return nil, fmt.Errorf("skin storage catalog IDs must be positive and increasing")
		}
		previous = entry.Template
		if entry.SkinKey() == 0 || entry.SkinPath == "" || entry.SkinType == "" {
			return nil, fmt.Errorf("skin storage template %d resolves to no skin", entry.Template)
		}
		out[entry.Template] = entry
	}
	return out, nil
}
