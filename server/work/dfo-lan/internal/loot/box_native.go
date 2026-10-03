package loot

import (
	"crypto/sha256"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
	"math"
	"path"
	"regexp"
	"strconv"
	"strings"
)

// The enabled source files and templates are independent of source rewards.
// Slot bands and the absent-stack-limit default are existing server behavior.
type BoxSourcePolicy struct {
	Version           int                  `json:"version"`
	Templates         []uint32             `json:"templates"`
	COSPaths          []string             `json:"cos_paths"`
	MissingStackLimit uint32               `json:"missing_stack_limit"`
	Slots             map[string][2]uint16 `json:"slots"`
}

func ImportBoxes(a *pvf.Archive, index catalog.ItemIndex, policy BoxSourcePolicy) (*BoxCatalog, error) {
	if a == nil || index.Source.Checksum != a.Snapshot().Checksum || policy.Version != 1 || len(policy.Templates) == 0 || len(policy.COSPaths) == 0 || policy.MissingStackLimit == 0 {
		return nil, fmt.Errorf("invalid box source/policy")
	}
	wanted := map[uint32]bool{}
	for _, id := range policy.Templates {
		if id == 0 || wanted[id] {
			return nil, fmt.Errorf("duplicate/zero enabled box")
		}
		wanted[id] = true
	}
	c := BoxCatalog{Source: a.Snapshot().Checksum, Tables: map[string]BoxTable{}, Rewards: map[string]BoxReward{}, Sources: map[string]string{}}
	c.Sources["list/stackable.lst"] = index.IndexHashes["list/stackable.lst"]
	seenPaths := map[string]bool{}
	for _, p := range policy.COSPaths {
		if p != strings.ToLower(p) || path.Clean(p) != p || strings.HasPrefix(p, "../") || path.Ext(p) != ".cos" || seenPaths[p] {
			return nil, fmt.Errorf("invalid/duplicate box COS path")
		}
		seenPaths[p] = true
		text, err := a.ReadText(p)
		if err != nil {
			return nil, err
		}
		id, table, err := parseNativeBoxCOS(text, path.Base(p)+".txt")
		if err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		if !wanted[id] {
			return nil, fmt.Errorf("COS material %d outside enabled box scope", id)
		}
		key := strconv.FormatUint(uint64(id), 10)
		if _, ok := c.Tables[key]; ok {
			return nil, fmt.Errorf("ambiguous native COS material %d", id)
		}
		entry, ok := index.Items[id]
		if !ok || entry.Kind != "stackable" {
			return nil, fmt.Errorf("box %d absent from native item index", id)
		}
		stk, err := catalog.ResolveScript(a, entry.Path)
		if err != nil {
			return nil, err
		}
		action, err := boxScriptText(stk.Cells, "[action type]")
		if err != nil {
			return nil, err
		}
		if action != "[radiant treasure box]" && action != "[enhanced radiant treasure box]" {
			return nil, fmt.Errorf("box material %d lacks a supported native action", id)
		}
		c.Tables[key] = table
		c.Sources[stk.Path] = stk.SHA256
		raw, err := a.ReadRaw(p)
		if err != nil {
			return nil, err
		}
		c.Sources[p] = fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	if len(c.Tables) != len(wanted) {
		return nil, fmt.Errorf("enabled box lacks native material binding")
	}
	rewards := map[uint32]bool{}
	for _, table := range c.Tables {
		for _, rows := range table.Groups {
			for _, row := range rows {
				rewards[row.Template] = true
			}
		}
		for _, stack := range table.PointStacks {
			for _, r := range stack.SectionReward {
				rewards[r.Template] = true
			}
		}
	}
	for id := range rewards {
		entry, ok := index.Items[id]
		if !ok || entry.Kind != "stackable" {
			return nil, fmt.Errorf("box reward %d lacks a native stackable binding", id)
		}
		stk, err := catalog.ResolveScript(a, entry.Path)
		if err != nil {
			return nil, err
		}
		kind, err := boxScriptText(stk.Cells, "[stackable type]")
		if err != nil {
			return nil, err
		}
		if kind != entry.StackableType {
			return nil, fmt.Errorf("box reward %d native index type differs", id)
		}
		limit := entry.StackLimit
		if limit == 0 {
			limit = policy.MissingStackLimit
		}
		var slots *[2]uint16
		if band, ok := policy.Slots[kind]; ok {
			if band[0] > band[1] {
				return nil, fmt.Errorf("invalid box reward band %s", kind)
			}
			slots = &band
		}
		c.Rewards[strconv.FormatUint(uint64(id), 10)] = BoxReward{Path: stk.Path, StackableType: kind, StackLimit: limit, Slots: slots}
		c.Sources[stk.Path] = stk.SHA256
	}
	return NewBoxCatalog(c)
}

func boxScriptText(cells []pvf.Token, tag string) (string, error) {
	var out string
	found := false
	for i, t := range cells {
		if t.Type == 3 && t.Text == tag {
			if found || i+1 >= len(cells) || cells[i+1].Type != 6 {
				return "", fmt.Errorf("invalid duplicate/missing %s", tag)
			}
			out = cells[i+1].Text
			found = true
		}
	}
	if !found || out == "" {
		return "", fmt.Errorf("missing %s", tag)
	}
	return out, nil
}

func boxCOSBlocks(text, tag string) ([]string, error) {
	re := regexp.MustCompile(`(?s)\[` + regexp.QuoteMeta(tag) + `\](.*?)\[/` + regexp.QuoteMeta(tag) + `\]`)
	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) != strings.Count(text, "["+tag+"]") || len(matches) != strings.Count(text, "[/"+tag+"]") {
		return nil, fmt.Errorf("unclosed %s", tag)
	}
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m[1]
	}
	return out, nil
}
func boxCOSNumber(text, tag string) (uint32, error) {
	re := regexp.MustCompile(`\[` + regexp.QuoteMeta(tag) + `\]\s*([0-9]+)`)
	m := re.FindAllStringSubmatch(text, -1)
	if len(m) != 1 {
		return 0, fmt.Errorf("invalid duplicate/missing %s", tag)
	}
	v, err := strconv.ParseUint(m[0][1], 10, 32)
	return uint32(v), err
}
func boxCOSString(text, tag string) (string, error) {
	re := regexp.MustCompile("\\[" + regexp.QuoteMeta(tag) + "\\]\\s*`([^`]+)`")
	m := re.FindAllStringSubmatch(text, -1)
	if len(m) != 1 {
		return "", fmt.Errorf("invalid duplicate/missing %s", tag)
	}
	return m[0][1], nil
}
func boxCOSRows(text string, width int) ([][]uint32, error) {
	f := strings.Fields(strings.Trim(text, " \r\n\t\x00"))
	if len(f) == 0 || len(f)%width != 0 {
		return nil, fmt.Errorf("incomplete COS rows")
	}
	out := make([][]uint32, 0, len(f)/width)
	for i := 0; i < len(f); i += width {
		row := make([]uint32, width)
		for j := range row {
			v, err := strconv.ParseUint(f[i+j], 10, 32)
			if err != nil {
				return nil, err
			}
			row[j] = uint32(v)
		}
		out = append(out, row)
	}
	return out, nil
}

func parseNativeBoxCOS(text, tableName string) (uint32, BoxTable, error) {
	text = regexp.MustCompile(`(?m)//[^\r\n]*`).ReplaceAllString(text, "")
	t := BoxTable{Table: tableName, Groups: map[string][]BoxEntry{}}
	material := regexp.MustCompile(`\[material\]\s*([0-9]+)\s+([0-9]+)`).FindAllStringSubmatch(text, -1)
	if len(material) != 1 {
		return 0, t, fmt.Errorf("COS has no unique material binding")
	}
	id, err := strconv.ParseUint(material[0][1], 10, 32)
	if err != nil || id == 0 {
		return 0, t, fmt.Errorf("invalid COS material")
	}
	amount, err := strconv.ParseUint(material[0][2], 10, 32)
	if err != nil || amount == 0 {
		return 0, t, fmt.Errorf("invalid COS material count")
	}
	t.MaterialCount = uint32(amount)
	for tag, dst := range map[string]*uint32{"rate": &t.Rate, "change box stack": &t.ChangeStack} {
		v, e := boxCOSNumber(text, tag)
		if e != nil {
			return 0, t, e
		}
		*dst = v
	}
	main, e := boxCOSNumber(text, "main lot group id")
	if e != nil || main > math.MaxInt32 {
		return 0, t, fmt.Errorf("invalid main group")
	}
	t.MainGroup = int32(main)
	special, e := boxCOSNumber(text, "special lot group id")
	if e != nil || special > math.MaxInt32 {
		return 0, t, fmt.Errorf("invalid special group")
	}
	t.SpecialGroup = int32(special)
	groups, err := boxCOSBlocks(text, "lot group")
	if err != nil || len(groups) == 0 {
		return 0, t, fmt.Errorf("invalid COS lot groups")
	}
	for _, g := range groups {
		group, e := boxCOSNumber(g, "id")
		if e != nil || group > math.MaxInt32 {
			return 0, t, fmt.Errorf("invalid group id")
		}
		key := strconv.FormatUint(uint64(group), 10)
		if _, ok := t.Groups[key]; ok {
			return 0, t, fmt.Errorf("duplicate lot group")
		}
		items, e := boxCOSBlocks(g, "item")
		if e != nil || len(items) != 1 {
			return 0, t, fmt.Errorf("invalid lot item block")
		}
		rows, e := boxCOSRows(items[0], 4)
		if e != nil {
			return 0, t, e
		}
		var sum uint64
		for _, r := range rows {
			if r[0] > math.MaxInt32 || r[1] == 0 || r[2] == 0 || r[3] == 0 {
				return 0, t, fmt.Errorf("invalid lot row")
			}
			sum += uint64(r[3])
			t.Groups[key] = append(t.Groups[key], BoxEntry{Group: int32(r[0]), Template: r[1], Count: r[2], Weight: r[3]})
		}
		if sum > math.MaxUint32 {
			return 0, t, fmt.Errorf("lot weights overflow")
		}
	}
	stacks, err := boxCOSBlocks(text, "point stack")
	if err != nil {
		return 0, t, err
	}
	for _, s := range stacks {
		var p BoxPointStack
		header := s
		if at := strings.Index(header, "[reward]"); at >= 0 {
			header = header[:at]
		}
		if at := strings.Index(header, "[section reward]"); at >= 0 {
			header = header[:at]
		}
		p.Type, err = boxCOSString(header, "type")
		if err != nil {
			return 0, t, err
		}
		p.Gain, err = boxCOSNumber(header, "gain")
		if err != nil {
			return 0, t, err
		}
		p.Max, err = boxCOSNumber(header, "max")
		if err != nil || p.Max == 0 {
			return 0, t, fmt.Errorf("invalid point stack max")
		}
		reward, e := boxCOSBlocks(s, "reward")
		if e != nil {
			return 0, t, e
		}
		section, e := boxCOSBlocks(s, "section reward")
		if e != nil {
			return 0, t, e
		}
		switch p.Type {
		case "bonus":
			if len(reward) != 1 || len(section) != 0 {
				return 0, t, fmt.Errorf("invalid bonus stack")
			}
			p.RewardType, err = boxCOSString(reward[0], "type")
			if err != nil {
				return 0, t, err
			}
			param, e := boxCOSNumber(reward[0], "param")
			if e != nil || param > math.MaxInt32 {
				return 0, t, fmt.Errorf("invalid bonus parameter")
			}
			p.RewardParam = int32(param)
		case "section":
			if len(section) != 1 || len(reward) != 0 {
				return 0, t, fmt.Errorf("invalid section stack")
			}
			rows, e := boxCOSRows(section[0], 4)
			if e != nil {
				return 0, t, e
			}
			for _, r := range rows {
				if r[0] > math.MaxInt32 || r[1] == 0 || r[1] > p.Max || r[2] == 0 || r[3] == 0 {
					return 0, t, fmt.Errorf("invalid section reward")
				}
				p.SectionReward = append(p.SectionReward, BoxSectionReward{Group: int32(r[0]), Threshold: r[1], Template: r[2], Count: r[3]})
			}
		default:
			return 0, t, fmt.Errorf("unsupported native point stack")
		}
		t.PointStacks = append(t.PointStacks, p)
	}
	postal, err := boxCOSBlocks(text, "postal tag")
	if err != nil || len(postal) != 1 {
		return 0, t, fmt.Errorf("invalid postal tag")
	}
	for _, m := range regexp.MustCompile("`([^`]+)`").FindAllStringSubmatch(postal[0], -1) {
		t.PostalTag = append(t.PostalTag, m[1])
	}
	if len(t.PostalTag) != 2 {
		return 0, t, fmt.Errorf("invalid postal tag pair")
	}
	return uint32(id), t, nil
}
