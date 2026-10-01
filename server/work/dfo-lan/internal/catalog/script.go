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
	return ReadScript(a, ResolveScriptPath(a, name))
}

// ResolveScriptPath applies the same native fallback without decoding cells.
func ResolveScriptPath(a *pvf.Archive, name string) string {
	name = strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	if _, ok := a.FindFile(name); ok {
		return name
	}
	for _, ext := range []string{".ani", ".mob", ".obj"} {
		if strings.Contains(name, ext) {
			return name
		}
	}
	return path.Join(path.Dir(name), "(r)"+path.Base(name))
}

type IndexEntry struct {
	ID   uint32 `json:"id"`
	Path string `json:"path"`
}

func ReadScript(a *pvf.Archive, name string) (ScriptRecord, error) {
	name = strings.ToLower(strings.ReplaceAll(name, "\\", "/"))
	s := ScriptRecord{Path: name}
	f, ok := a.FindFile(name)
	if !ok {
		return s, fmt.Errorf("%w: %s", pvf.ErrFileNotFound, name)
	}
	if f.DataType != 1 {
		return s, fmt.Errorf("entry %s is not a script", name)
	}
	raw, e := a.ReadRaw(name)
	if e != nil {
		return s, e
	}
	s.SHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	s.Cells, e = a.TokensFromRaw(raw)
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

// nestedSectionCells 收集目标段落顶层的单元，跳过内嵌子块。
//
// 与 sectionCells 的差别：源地图的 [town movable area] /
// [virtual movable area] 里会内嵌 [quest condition]、[check condition]、
// [in progress]、[move dungeon info] 等条件子块，子块闭合标签之后
// 还有属于父段的矩形行。sectionCells 遇到任意 type-3 标签就失活且
// 只有同名标签才复活，会把子块后面的行整个吞掉——实测
// elvengard_hendon_storm.map（38/3）通 Elvenmere（38/7）的任务门、
// new_hendon_main.map（39/0）[quest condition]10100 之后的可行走矩形
// 都是这样丢的。这里按开/闭标签维护深度：深度 1 表示正处于目标段
// 顶层，只收集该层的单元；条件子块的内容整体跳过。
//
// 条件本身（如 38/3→38/7 需要任务 3156）不在此求值：客户端读同一份
// 地图数据自行判定，与"未实现条件由客户端把关"的既有约定一致。
func nestedSectionCells(cells []pvf.Token, name string) []pvf.Token {
	var out []pvf.Token
	depth := 0
	for _, c := range cells {
		if c.Type != 3 {
			if depth == 1 {
				out = append(out, c)
			}
			continue
		}
		if depth == 0 {
			if c.Text == name {
				depth = 1
			}
			continue
		}
		if strings.HasPrefix(c.Text, "[/") {
			depth--
		} else {
			depth++
		}
	}
	return out
}
