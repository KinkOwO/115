package catalog

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// TestParseSourceCoinLimit 守「复活币使用上限」的两种源形态。
//
// 背景（next188）：沉月湖用 `[coin limit] <N>`，蔚蓝号用 `[coin info] … [normal] <N> <M> …`；
// 此前服务端**都没读**，沉月湖因此没有上限（客户端显示持有量，业主实机看到 38）。
//
// ⚠️ `[normal]` 是 **type=6 的子标签**（label），不是 type=3 的 section ——
// 按 section 匹配会一个都找不到（实现时踩过）。
func TestParseSourceCoinLimit(t *testing.T) {
	// 最小可解析脚本骨架（必填的等级段 + maze 段）。
	base := func(extra ...pvf.Token) []pvf.Token {
		cells := []pvf.Token{section("[minimum required level]"), number(115)}
		cells = append(cells, extra...)
		return append(cells, section("[maze info]"), section("[size]"), number(1), number(1),
			section("[map specification]"), label("boss"), number(0), number(0), number(100006472), section("[/map specification]"),
			section("[start map]"), number(0), number(0), section("[/start map]"),
			section("[boss map]"), number(0), number(0), section("[/boss map]"))
	}

	for _, tc := range []struct {
		name string
		body []pvf.Token
		want uint32
	}{
		{"形态一 [coin limit] 8", []pvf.Token{section("[coin limit]"), number(8)}, 8},
		{"形态一 值 0", []pvf.Token{section("[coin limit]"), number(0)}, 0},
		{"形态一 多条", []pvf.Token{section("[coin limit]"), number(8), number(8)}, 0},
		{"形态二 蔚蓝号单档", []pvf.Token{
			section("[coin info]"), number(0),
			label("[normal]"), number(8), number(1),
			section("[/coin info]")}, 8},
		{"形态二 同族三档（取 normal）", []pvf.Token{
			section("[coin info]"), number(0),
			label("[normal]"), number(8), number(1),
			label("[expert]"), number(9), number(1),
			label("[master]"), number(10), number(1),
			section("[/coin info]")}, 8},
		// 块内只有 expert（没有 normal）：取不到第一档 ⇒ 0。
		//
		// ⚠️ 这里不能把裸 label 放在两个 type-3 段之间 —— label 是 type=6，
		// sectionCells 遇到它不会停，会把后面的值并进**上一个**段（实测会破坏
		// [minimum required level] 的解析，报 "invalid [minimum required level]"）。
		{"形态二 块内无 normal", []pvf.Token{
			section("[coin info]"), number(0),
			label("[expert]"), number(9), number(1),
			section("[/coin info]")}, 0},
		{"无声明", nil, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, err := ParseDungeon(100004131, ScriptRecord{Cells: base(tc.body...)})
			if err != nil || d.CoinLimit != tc.want {
				t.Fatalf("CoinLimit = %d，期望 %d，err=%v", d.CoinLimit, tc.want, err)
			}
		})
	}
}
