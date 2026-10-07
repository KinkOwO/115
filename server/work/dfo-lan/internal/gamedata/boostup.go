package gamedata

import (
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"fmt"
)

// BoostUpSource adapts the already-open runtime archive to every source
// interface the boostup catalog needs (Load/Bind* reuse one archive; this
// satisfies boostup.TokenSource, ItemSource and the Text source without a
// second full PVF import).
//
// 模板 → 源路径一律走本树的物品原生索引（catalogs 的补给/分类用的是同一张
// ItemIndex），活动线不再读第二遍归档目录，也不自建礼包目录视图：奖励盒的
// 「[booster] 开一层出什么」由通用 `Source.Boosters` + `loot.RewardBoxSource`
// 负责（§0.2 单一规则）。
type BoostUpSource interface {
	Tokens(string) ([]pvf.Token, error)
	TemplateScript(string, uint32) (catalog.ScriptRecord, error)
	ScriptSHA256(string) (string, error)
	Text(string) (string, error)
}

type boostUpSource struct {
	a     *pvf.Archive
	items catalog.ItemIndex
}

func (b *boostUpSource) Tokens(p string) ([]pvf.Token, error) { return b.a.Tokens(p) }

// TemplateScript reads one item script by template id through the native index.
func (b *boostUpSource) TemplateScript(kind string, id uint32) (catalog.ScriptRecord, error) {
	entry, ok := b.items.Items[id]
	if !ok {
		return catalog.ScriptRecord{}, fmt.Errorf("item %d is missing from the native index", id)
	}
	if kind != "" && entry.Kind != kind {
		return catalog.ScriptRecord{}, fmt.Errorf("item %d is %s, not %s", id, entry.Kind, kind)
	}
	return catalog.ReadScript(b.a, entry.Path)
}

func (b *boostUpSource) ScriptSHA256(p string) (string, error) {
	rec, e := catalog.ReadScript(b.a, p)
	return rec.SHA256, e
}
func (b *boostUpSource) Text(p string) (string, error) { return b.a.ReadText(p) }

var _ boostup.TokenSource = &boostUpSource{}

// BoostUp returns the source adapter and the parsed event-662 catalog from
// the live PVF in one pass. Optional bindings (challenges, capsules, groups,
// set points) stay with the caller so launch flags can gate them.
func (s *Source) BoostUp(index catalog.ItemIndex) (BoostUpSource, *boostup.Catalog, error) {
	if s.mode != PVF || s.archive == nil {
		return nil, nil, fmt.Errorf("starter boost import requires PVF")
	}
	src := &boostUpSource{a: s.archive, items: index}
	c, e := boostup.Load(src)
	return src, c, e
}
