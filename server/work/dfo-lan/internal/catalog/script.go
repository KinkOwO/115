package catalog

import (
	"crypto/sha256"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"path"
	"strings"
)

// ScriptRecord keeps every typed cell, including untranslated localization
// references and conditions not yet supported by the rules engine.
type ScriptRecord struct {
	Path   string      `json:"path"`
	SHA256 string      `json:"sha256"`
	Cells  []pvf.Token `json:"cells"`
}

// ResolveScript follows current runtime 147c49900: try the exact path, then
// prepend (R) to the filename (147c4bbb0), except ANI/MOB/OBJ (147c4b610).
func ResolveScript(a *pvf.Archive, name string) (ScriptRecord, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	if _, ok := a.FindFile(name); ok {
		return ReadScript(a, name)
	}
	for _, ext := range []string{".ani", ".mob", ".obj"} {
		if strings.Contains(name, ext) {
			return ReadScript(a, name)
		}
	}
	return ReadScript(a, path.Join(path.Dir(name), "(r)"+path.Base(name)))
}

type IndexEntry struct {
	ID   uint32 `json:"id"`
	Path string `json:"path"`
}

func ReadScript(a *pvf.Archive, name string) (ScriptRecord, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	s := ScriptRecord{Path: name}
	raw, e := a.ReadRaw(name)
	if e != nil {
		return s, e
	}
	s.SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	s.Cells, e = a.Tokens(name)
	return s, e
}

func ParseIndex(cells []pvf.Token) ([]IndexEntry, error) {
	if len(cells)%2 != 0 {
		return nil, fmt.Errorf("unpaired source index")
	}
	rows := make([]IndexEntry, 0, len(cells)/2)
	seen := map[uint32]bool{}
	for i := 0; i < len(cells); i += 2 {
		if cells[i].Type != 0 || cells[i].Value < 0 || cells[i+1].Type != 6 || cells[i+1].Text == "" {
			return nil, fmt.Errorf("invalid source index at cell %d", i)
		}
		id := uint32(cells[i].Value)
		if seen[id] {
			return nil, fmt.Errorf("duplicate source index %d", id)
		}
		seen[id] = true
		rows = append(rows, IndexEntry{id, strings.ToLower(strings.ReplaceAll(cells[i+1].Text, "\\", "/"))})
	}
	return rows, nil
}

func sectionCells(cells []pvf.Token, name string) []pvf.Token {
	var out []pvf.Token
	active := false
	for _, c := range cells {
		if c.Type == 3 {
			active = c.Text == name
			continue
		}
		if active {
			out = append(out, c)
		}
	}
	return out
}
