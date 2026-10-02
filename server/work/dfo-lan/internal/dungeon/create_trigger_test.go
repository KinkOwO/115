package dungeon

import (
	"testing"

	"dfolan/internal/catalog"
)

// 小深渊 100005014 的两张地图把 [monster create trigger] 逐行写在 [monster] 段旁边：
// 前 11 行是 1，天平 109019266 与 109019280 是 2。fixedMonsters 按 SourceIndex
// 对齐保留原值，进图包是否带上它由 protocol.StartMapState.EncodeCreateTrigger 决定。
//
// 这条不变量是「服务端投了怪、客户端却把天平和 109019280 当成地面物品拿走」这一现象的
// 判据基础：值必须与行严格对齐，错一格就会把创建时机安到别的怪身上。
func TestCreateTriggerOrdinalsFollowMonsterRows(t *testing.T) {
	c := catalog.LoadNativeFullDungeons(t)
	script, ok := c.Maps[100016614]
	if !ok {
		t.Skip("map 100016614 is not imported")
	}
	monsters, e := fixedMonsters(script, 115)
	if e != nil {
		t.Fatal(e)
	}
	if len(monsters) != 13 {
		t.Fatalf("expected 13 source rows, got %d", len(monsters))
	}
	for i, m := range monsters {
		want := byte(1)
		if i >= 11 {
			want = 2
		}
		if m.CreateTrigger != want {
			t.Errorf("row %d template %d: CreateTrigger=%d want %d", i, m.Template, m.CreateTrigger, want)
		}
	}
	if monsters[11].Template != 109019266 {
		t.Errorf("row 11 template = %d, want the scale_primer 109019266", monsters[11].Template)
	}
	if monsters[12].Template != 109019280 {
		t.Errorf("row 12 template = %d, want 109019280", monsters[12].Template)
	}
	// 同一张地图的另一半（maze 1 / _special）行结构相同，值也必须对齐。
	if other, ok2 := c.Maps[100016615]; ok2 {
		ms, e2 := fixedMonsters(other, 115)
		if e2 != nil {
			t.Fatal(e2)
		}
		if len(ms) != len(monsters) {
			t.Fatalf("map 100016615 rows = %d, want %d", len(ms), len(monsters))
		}
		for i := range ms {
			if ms[i].CreateTrigger != monsters[i].CreateTrigger {
				t.Errorf("map 100016615 row %d CreateTrigger=%d differs from _normal %d", i, ms[i].CreateTrigger, monsters[i].CreateTrigger)
			}
		}
	}
}
