package catalog

import (
	"testing"

	"dfolan/internal/catalog/pvf"
)

// DungeonType 读的是副本脚本自己的 [dungeon type]：玩法分流的判据取它，
// 而不是按副本号硬编名单（源里加了副本自动跟上）。
func TestDungeonTypeReadsTheSourceDeclaration(t *testing.T) {
	cases := []struct {
		name  string
		cells []pvf.Token
		want  string
	}{
		{
			"引用形式（当前资源的写法）",
			[]pvf.Token{{Type: 3, Text: "[dungeon type]"}, {Type: 8, Reference: "boundary of attunement"}},
			"boundary of attunement",
		},
		{
			"字面量形式",
			[]pvf.Token{{Type: 3, Text: "[dungeon type]"}, {Type: 6, Text: "endkeeper of order"}},
			"endkeeper of order",
		},
		{
			"没有声明",
			[]pvf.Token{{Type: 3, Text: "[maze info]"}, {Type: 0, Value: 1}},
			"",
		},
		{
			"声明后面紧接另一个段头",
			[]pvf.Token{{Type: 3, Text: "[dungeon type]"}, {Type: 3, Text: "[difficulty]"}},
			"",
		},
		{
			"空脚本",
			nil,
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := DungeonType(DungeonDefinition{Script: ScriptRecord{Cells: c.cells}})
			if got != c.want {
				t.Fatalf("DungeonType = %q, want %q", got, c.want)
			}
		})
	}
}
