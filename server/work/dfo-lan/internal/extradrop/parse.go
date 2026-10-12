package extradrop

import (
	"fmt"
	"strconv"
	"strings"

	"dfolan/internal/catalog/pvf"
)

// Parse 把已解码的 CTP 表翻成目录。**纯函数**（不碰归档），所以单测可以直接喂手搭的表。
func Parse(t *pvf.CTPTable) (*Catalog, error) {
	if t == nil {
		return nil, fmt.Errorf("extradrop: 空表")
	}
	c := &Catalog{Path: t.Path, SHA256: t.SHA256, Bytes: t.Bytes, Version: t.Version}
	byIndex := make(map[int]*pvf.CTPRecord, len(t.Records))
	for i := range t.Records {
		byIndex[t.Records[i].Index] = &t.Records[i]
	}
	top := func(name string) (*pvf.CTPRecord, error) {
		col := t.Column(name)
		if col == nil || len(col.Records) == 0 {
			return nil, fmt.Errorf("extradrop: 缺少 %s 段", name)
		}
		r := byIndex[int(col.Records[0])]
		if r == nil {
			return nil, fmt.Errorf("extradrop: %s 指向不存在的记录 %d", name, col.Records[0])
		}
		return r, nil
	}
	refInto := func(r *pvf.CTPRecord, name string) ([]*pvf.CTPRecord, error) {
		var out []*pvf.CTPRecord
		for _, ref := range r.Refs {
			if ref.Name != name {
				continue
			}
			for _, v := range ref.Values {
				child := byIndex[int(v)]
				if child == nil {
					return nil, fmt.Errorf("extradrop: %s → %s 指向不存在的记录 %d", r.Name, name, v)
				}
				out = append(out, child)
			}
		}
		if len(out) == 0 {
			return nil, fmt.Errorf("extradrop: %s 里没有 %s 的引用", r.Name, name)
		}
		return out, nil
	}

	// [event period]：两个文本（起止时刻）。
	period, err := top("[event period]")
	if err != nil {
		return nil, err
	}
	dates := texts(period)
	if len(dates) != 2 {
		return nil, fmt.Errorf("extradrop: [event period] 有 %d 个文本，期望 2", len(dates))
	}
	c.PeriodStart, c.PeriodEnd = dates[0], dates[1]

	// [target dungeon type]：一个文本。
	dungeonType, err := top("[target dungeon type]")
	if err != nil {
		return nil, err
	}
	if ts := texts(dungeonType); len(ts) == 1 {
		c.TargetDungeonType = ts[0]
	} else {
		return nil, fmt.Errorf("extradrop: [target dungeon type] 有 %d 个文本，期望 1", len(ts))
	}
	if strings.TrimSpace(c.TargetDungeonType) == "" {
		return nil, fmt.Errorf("extradrop: [target dungeon type] 是空串")
	}

	// [max amount] → [amount] 一个数 + [type] 若干文本。
	maxRec, err := top("[max amount]")
	if err != nil {
		return nil, err
	}
	amountRecs, err := refInto(maxRec, "[amount]")
	if err != nil {
		return nil, err
	}
	if c.MaxAmount, err = number(amountRecs[0]); err != nil {
		return nil, fmt.Errorf("extradrop: [max amount]→[amount]: %w", err)
	}
	typeRecs, err := refInto(maxRec, "[type]")
	if err != nil {
		return nil, err
	}
	c.MaxTypes = texts(typeRecs[0])
	if len(c.MaxTypes) == 0 {
		return nil, fmt.Errorf("extradrop: [max amount]→[type] 没有任何文本")
	}

	// [group table] → N 个 [group]。
	groupTable, err := top("[group table]")
	if err != nil {
		return nil, err
	}
	groupRecs, err := refInto(groupTable, "[group]")
	if err != nil {
		return nil, err
	}
	for gi, gr := range groupRecs {
		var g Group
		tRecs, err := refInto(gr, "[type]")
		if err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组: %w", gi, err)
		}
		if ts := texts(tRecs[0]); len(ts) == 1 {
			g.Type = ts[0]
		} else {
			return nil, fmt.Errorf("extradrop: 第 %d 组的 [type] 有 %d 个文本，期望 1", gi, len(ts))
		}
		lvRecs, err := refInto(gr, "[item level]")
		if err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组: %w", gi, err)
		}
		if g.ItemLevel, err = number(lvRecs[0]); err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组的 [item level]: %w", gi, err)
		}
		itRecs, err := refInto(gr, "[item type]")
		if err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组: %w", gi, err)
		}
		g.ItemTypes = texts(itRecs[0])
		if len(g.ItemTypes) == 0 {
			return nil, fmt.Errorf("extradrop: 第 %d 组的 [item type] 没有任何文本", gi)
		}
		// [rule] 是**可选**的（本机只有第三组有）。
		for _, ref := range gr.Refs {
			if ref.Name != "[rule]" {
				continue
			}
			for _, v := range ref.Values {
				if r := byIndex[int(v)]; r != nil {
					if ts := texts(r); len(ts) > 0 {
						g.Rule = strings.Join(ts, " ")
					}
				}
			}
		}
		rtRecs, err := refInto(gr, "[reward table]")
		if err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组: %w", gi, err)
		}
		if g.Tiers, err = tiers(rtRecs[0]); err != nil {
			return nil, fmt.Errorf("extradrop: 第 %d 组的 [reward table]: %w", gi, err)
		}
		c.Groups = append(c.Groups, g)
	}
	if len(c.Groups) == 0 {
		return nil, fmt.Errorf("extradrop: [group table] 没有任何组")
	}
	return c, nil
}

// GroupByType 按 `[type]` 找一个组（找不到 ok=false）。
func (c *Catalog) GroupByType(kind string) (Group, bool) {
	if c == nil {
		return Group{}, false
	}
	for _, g := range c.Groups {
		if g.Type == kind {
			return g, true
		}
	}
	return Group{}, false
}

// texts 取出一个记录里所有文本型单元格（`name` / `name_indexed`）。
func texts(r *pvf.CTPRecord) []string {
	var out []string
	for _, cell := range r.Cells {
		switch cell.Kind {
		case "name", "name_indexed":
			out = append(out, cell.Name)
		}
	}
	return out
}

// number 把一个记录当成"只有一个数"来读：先试 float 单元格，再退回文本解析。
//
// 为什么要两条路：`.ctp` 里的数值单元格**有时是 float 标签、有时是带引号的文本**
// （本机 `[amount]` / `[item level]` 两种形态都出现过），只认一种会漏读。
func number(r *pvf.CTPRecord) (uint32, error) {
	for _, cell := range r.Cells {
		switch cell.Kind {
		case "float":
			if cell.Float < 0 {
				return 0, fmt.Errorf("负数 %g", cell.Float)
			}
			return uint32(cell.Float + 0.5), nil
		case "name", "name_indexed":
			n, err := strconv.ParseUint(strings.TrimSpace(cell.Name), 10, 32)
			if err != nil {
				return 0, fmt.Errorf("文本 %q 不是十进制数", cell.Name)
			}
			return uint32(n), nil
		}
	}
	return 0, fmt.Errorf("记录里没有可读的数")
}

// tiers 解析 `[reward table]`：**三元组**（稀有度名, 物品号, 数量）。
//
// 单元格是混排的（第一个是文本、后两个是数），所以按顺序拍平再按 3 切；
// 个数不是 3 的整数倍就拒绝（宁可拒绝也不要按错位发奖）。
func tiers(r *pvf.CTPRecord) ([]RewardTier, error) {
	type flat struct {
		text string
		num  uint32
		isTx bool
	}
	var cells []flat
	for _, cell := range r.Cells {
		switch cell.Kind {
		case "name", "name_indexed":
			cells = append(cells, flat{text: cell.Name, isTx: true})
		case "float":
			if cell.Float < 0 {
				return nil, fmt.Errorf("负数 %g", cell.Float)
			}
			cells = append(cells, flat{num: uint32(cell.Float + 0.5)})
		}
	}
	if len(cells) == 0 || len(cells)%3 != 0 {
		return nil, fmt.Errorf("有 %d 个可读单元格，不是 3 的整数倍", len(cells))
	}
	out := make([]RewardTier, 0, len(cells)/3)
	for i := 0; i < len(cells); i += 3 {
		a, b, c := cells[i], cells[i+1], cells[i+2]
		if !a.isTx || a.text == "" {
			return nil, fmt.Errorf("第 %d 行的稀有度不是文本", i/3)
		}
		if b.isTx || c.isTx {
			return nil, fmt.Errorf("第 %d 行的物品/数量不是数", i/3)
		}
		if b.num == 0 {
			return nil, fmt.Errorf("第 %d 行的物品号是 0", i/3)
		}
		out = append(out, RewardTier{Rarity: a.text, Item: b.num, Amount: c.num})
	}
	return out, nil
}
