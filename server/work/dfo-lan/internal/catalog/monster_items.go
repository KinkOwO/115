package catalog

import (
	"fmt"
	"strings"

	"dfolan/internal/catalog/pvf"
)

// MonsterItemPair retains the native MOB [item] two-int record. Value is
// deliberately unnamed: the reader establishes storage, not an award rate or
// denominator. Reference servers use it as a selection weight.
type MonsterItemPair struct {
	Template int32
	Value    int32
}

type MonsterItemTable struct {
	Path, SHA256     string
	Declared         bool
	ExcludeWorldDrop bool
	Items            []MonsterItemPair
}

// Current LIST entries include archive-root contents/... paths as well as
// legacy paths relative to monster/. Preserve an existing exact binding before
// supplying the legacy directory; never replace it with another MOB's pool.
func monsterItemScriptPath(a *pvf.Archive, p string) string {
	if _, found := a.FindFile(p); found || strings.HasPrefix(p, "monster/") {
		return p
	}
	return "monster/" + p
}

// ParseMonsterItemTable follows native 147467A99: every [item] resets the
// vector before appending raw pairs. Repeated sections replace, not merge.
// This projection does not enable drops or reinterpret creation-rate zero.
func ParseMonsterItemTable(s ScriptRecord) (MonsterItemTable, error) {
	t := MonsterItemTable{Path: s.Path, SHA256: s.SHA256}
	if !strings.HasSuffix(s.Path, ".mob") || len(s.SHA256) != 64 {
		return t, fmt.Errorf("monster item table without MOB source")
	}
	for i := 0; i < len(s.Cells); i++ {
		// Native case11379 at1474541BC sets MOB+1694 bit8 without
		// consuming a value: this is a presence marker, not a numeric rate.
		if s.Cells[i].Type == 3 && s.Cells[i].Text == "[exclude world drop]" {
			t.ExcludeWorldDrop = true
		}
		if s.Cells[i].Type != 3 || s.Cells[i].Text != "[item]" {
			continue
		}
		t.Declared, t.Items = true, nil
		i++
		for {
			if i >= len(s.Cells) {
				return t, fmt.Errorf("unterminated monster item table: %s", s.Path)
			}
			if s.Cells[i].Type == 3 && s.Cells[i].Text == "[/item]" {
				break
			}
			if i+1 >= len(s.Cells) || s.Cells[i].Type != 0 || s.Cells[i+1].Type != 0 {
				return t, fmt.Errorf("invalid monster item pair: %s cell %d", s.Path, i)
			}
			t.Items = append(t.Items, MonsterItemPair{s.Cells[i].Value, s.Cells[i+1].Value})
			i += 2
		}
	}
	return t, nil
}

// ImportMonsterItemTables resolves only requested templates through the current
// LIST. Unknown templates and unreadable scripts never acquire a fallback pool.
func ImportMonsterItemTables(a *pvf.Archive, ids []uint32) (map[uint32]MonsterItemTable, error) {
	if a == nil {
		return nil, fmt.Errorf("monster item source unavailable")
	}
	list, err := ResolveScript(a, "list/monster.lst")
	if err != nil {
		return nil, err
	}
	rows, err := ParseIndex(list.Cells)
	if err != nil {
		return nil, err
	}
	paths := make(map[uint32]string, len(rows))
	for _, r := range rows {
		paths[r.ID] = r.Path
	}
	out := make(map[uint32]MonsterItemTable, len(ids))
	for _, id := range ids {
		if _, exists := out[id]; exists {
			continue
		}
		p, exists := paths[id]
		if !exists {
			return nil, fmt.Errorf("monster %d absent from native LIST", id)
		}
		p = monsterItemScriptPath(a, p)
		s, err := ResolveScript(a, p)
		if err != nil {
			return nil, err
		}
		t, err := ParseMonsterItemTable(s)
		if err != nil {
			return nil, err
		}
		out[id] = t
	}
	return out, nil
}

// The bounded native view remains usable after the import archive is closed.
// Missing scripts are recorded without substituting another monster's pool.
func (c *LootCatalog) EnableMonsterItemDetails(a *pvf.Archive) error {
	if c.monsterItems != nil {
		return nil
	}
	if a == nil || c.Source.Checksum != a.Snapshot().Checksum {
		return fmt.Errorf("monster item source mismatch")
	}
	list, err := ResolveScript(a, "list/monster.lst")
	if err != nil {
		return err
	}
	rows, err := ParseIndex(list.Cells)
	if err != nil {
		return err
	}
	refs := map[uint32]ScriptRecord{}
	c.monsterItemUnavailable = map[uint32]string{}
	for _, r := range rows {
		p := monsterItemScriptPath(a, r.Path)
		if _, found := a.FindFile(p); !found {
			c.monsterItemUnavailable[r.ID] = p
			continue
		}
		refs[r.ID] = ScriptRecord{Path: p}
	}
	c.monsterItems, err = NewScriptDetails(a, refs, func(_ uint32, s ScriptRecord) ScriptRecord { return s }, ScriptBytes)
	return err
}

func (c LootCatalog) HasMonsterItemDetails() bool { return c.monsterItems != nil }

func (c LootCatalog) MonsterItemTable(id uint32) (MonsterItemTable, bool, error) {
	if c.monsterItems == nil {
		return MonsterItemTable{}, false, nil
	}
	if p, missing := c.monsterItemUnavailable[id]; missing {
		return MonsterItemTable{}, true, fmt.Errorf("monster %d source absent: %s", id, p)
	}
	if _, known := c.monsterItems.refs[id]; !known {
		return MonsterItemTable{}, false, nil
	}
	s, err := c.monsterItems.Get(id)
	if err != nil {
		return MonsterItemTable{}, true, err
	}
	t, err := ParseMonsterItemTable(s)
	return t, true, err
}
