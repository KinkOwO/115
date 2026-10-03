package catalog

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
)

// [MERGE-20260928-DUNGEON-DROPINFO] etc/dungeondropinfo.cos 是「副本 → 掉落组」的
// 官方索引表：它按 [dungeon type] 把每个副本的品级、掉落组和掉落率对应起来。
//
// 它与 dungeon 脚本里的 [difficulty dropitem group list] 是**同一条链的两端**：
//
//	dungeon 脚本      [normal group index] = 2 10900 21030 5 10014 10015 10024 10021
//	dungeondropinfo   [drop group]        = 10900 / 21030 / 10014 / 10015 / 10024
//	                                       ↑ 完全对应（实机 100003295 已核对）
//
// [dungeon type] 只有两种取值（实测 1731 条）：
//
//	dgn_normal  1705 条   常规（奥德赛、逃生、活动都归这里）
//	dgn_hell      26 条   深渊（只有 3 个副本：100003295/6/7 的 [hell dungeon]）
//
// 这正是「深渊按深渊要求掉落、常规按常规掉落」的**权威依据** —— 不需要靠副本名或
// 段标记猜。
//
// 文件形态：DataType=3 的**文本**（不是脚本），UTF-16LE 编码，所以不能走 Tokens()。
// 这里用 ReadRaw + UTF-16LE 解码，再用与 dungeondroptablebygroup 相同的"标签 + 数字"
// 读法解析。

// DungeonDropRateEntry 是 dungeondropinfo 里的一条 [drop rate] 记录。
type DungeonDropRateEntry struct {
	// Grade 是 [type] 的值：`epic` / `stackable` / `unique` / `legendary` / `rare` / `special`。
	Grade string `json:"grade"`
	// DungeonType 是 [dungeon type] 的值：`dgn_normal` 或 `dgn_hell`。
	DungeonType string `json:"dungeon_type"`
	// DropGroup 是 [drop group] 的组号，索引 etc/dungeondroptablebygroup.etc。
	DropGroup uint32 `json:"drop_group"`
	// DropWeaponRate 是 [drop weapon rate]（百万空间）。
	DropWeaponRate uint32 `json:"drop_weapon_rate"`
	// RateList 是 [rate list] 的内容，键是怪物类别（`boss` / `named` / `normal` ...），
	// 值是该类的若干档率。档数不由这里假设，照抄源里的个数。
	RateList map[string][]uint32 `json:"rate_list"`
}

// DungeonDropInfoSource 记录这张表的来源，便于和 PVF 快照对账。
type DungeonDropInfoSource struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

// ParseDungeonDropInfo 解析 etc/dungeondropinfo.cos 的原始字节。
// raw 是未解码的原始内容（UTF-16LE）。
func ParseDungeonDropInfo(raw []byte) (map[uint32][]DungeonDropRateEntry, error) {
	text, err := decodeUTF16LEStrict(raw)
	if err != nil {
		return nil, err
	}
	return parseDungeonDropInfoText(text)
}

// DropInfoByID 取某个副本在 etc/dungeondropinfo.cos 里的全部 [drop rate] 记录。
//
// 这是发放链的入口：调用方拿它得到 (品级, 副本类型, 掉落组) 三元组，再用
// DropGroupByID 把组展开成物品。
func (c LootCatalog) DropInfoByID(dungeonID uint32) ([]DungeonDropRateEntry, bool) {
	e, ok := c.DungeonDropInfo[dungeonID]
	return e, ok
}

// IsAbyssDungeon 报告该副本是否被 dungeondropinfo 标为深渊（`dgn_hell`）。
//
// 这是「深渊按深渊要求掉落」的权威判据 —— 不靠副本名、不靠段标记猜。
func (c LootCatalog) IsAbyssDungeon(dungeonID uint32) bool {
	for _, e := range c.DungeonDropInfo[dungeonID] {
		if e.DungeonType == "dgn_hell" {
			return true
		}
	}
	return false
}

func decodeUTF16LEStrict(raw []byte) (string, error) {
	if len(raw) < 4 {
		return "", fmt.Errorf("dungeondropinfo too short")
	}
	if len(raw)%2 != 0 {
		return "", fmt.Errorf("dungeondropinfo has odd length %d", len(raw))
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		units = append(units, uint16(raw[i])|uint16(raw[i+1])<<8)
	}
	return string(utf16.Decode(units)), nil
}

func parseDungeonDropInfoText(text string) (map[uint32][]DungeonDropRateEntry, error) {
	toks := dropInfoTokens(text)
	out := map[uint32][]DungeonDropRateEntry{}

	curDungeon := -1
	var cur *DungeonDropRateEntry
	inRateList := false

	flush := func() {
		if cur != nil && curDungeon >= 0 {
			e := *cur
			if e.RateList == nil {
				e.RateList = map[string][]uint32{}
			}
			out[uint32(curDungeon)] = append(out[uint32(curDungeon)], e)
		}
		cur = nil
	}

	for i := 0; i < len(toks); i++ {
		t := toks[i]
		switch {
		case t.kind == tokText && t.s == "[index]":
			// 下一个数字是副本 ID
			if i+1 < len(toks) && toks[i+1].kind == tokNum {
				curDungeon = int(toks[i+1].n)
				i++
			}
		case t.kind == tokText && t.s == "[drop rate]":
			flush()
			cur = &DungeonDropRateEntry{RateList: map[string][]uint32{}}
		case t.kind == tokText && t.s == "[/drop rate]":
			flush()
		case t.kind == tokText && t.s == "[type]":
			if cur != nil && i+1 < len(toks) && toks[i+1].kind == tokBacktick {
				cur.Grade = toks[i+1].s
				i++
			}
		case t.kind == tokText && t.s == "[dungeon type]":
			if cur != nil && i+1 < len(toks) && toks[i+1].kind == tokBacktick {
				cur.DungeonType = toks[i+1].s
				i++
			}
		case t.kind == tokText && t.s == "[drop group]":
			if cur != nil && i+1 < len(toks) && toks[i+1].kind == tokNum {
				cur.DropGroup = uint32(toks[i+1].n)
				i++
			}
		case t.kind == tokText && t.s == "[drop weapon rate]":
			if cur != nil && i+1 < len(toks) && toks[i+1].kind == tokNum {
				cur.DropWeaponRate = uint32(toks[i+1].n)
				i++
			}
		case t.kind == tokText && t.s == "[rate list]":
			inRateList = true
		case t.kind == tokText && t.s == "[/rate list]":
			inRateList = false
		case t.kind == tokBacktick && inRateList && cur != nil:
			name := t.s
			var vals []uint32
			j := i + 1
			for j < len(toks) && toks[j].kind == tokNum {
				vals = append(vals, uint32(toks[j].n))
				j++
			}
			cur.RateList[name] = vals
			i = j - 1
		}
	}
	flush()
	if len(out) == 0 {
		return nil, fmt.Errorf("dungeondropinfo yielded no entries")
	}
	return out, nil
}

type dropInfoTokKind int

const (
	tokNum dropInfoTokKind = iota
	tokText
	tokBacktick
)

type dropInfoTok struct {
	kind dropInfoTokKind
	s    string
	n    int64
}

// dropInfoTokens 把文本切成三类：数字、[标签]、`反引号词`。
func dropInfoTokens(text string) []dropInfoTok {
	var out []dropInfoTok
	i := 0
	n := len(text)
	for i < n {
		c := text[i]
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			i++
		case c == '[':
			j := strings.IndexByte(text[i:], ']')
			if j < 0 {
				return out
			}
			out = append(out, dropInfoTok{kind: tokText, s: text[i : i+j+1]})
			i += j + 1
		case c == '`':
			j := strings.IndexByte(text[i+1:], '`')
			if j < 0 {
				return out
			}
			out = append(out, dropInfoTok{kind: tokBacktick, s: text[i+1 : i+1+j]})
			i += j + 2
		case c >= '0' && c <= '9':
			j := i
			for j < n && text[j] >= '0' && text[j] <= '9' {
				j++
			}
			v, e := strconv.ParseInt(text[i:j], 10, 64)
			if e != nil {
				i = j
				continue
			}
			out = append(out, dropInfoTok{kind: tokNum, n: v})
			i = j
		default:
			i++
		}
	}
	return out
}
