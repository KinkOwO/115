package character

import (
	"dfolan/internal/catalog/pvf"
	"testing"
)

// 源里有一批技能完全没有 [growtype maximum level]，只带 [awakening maximum level]：
//
//	skill/swordman/nachal.skl  [skill class] 4  [maximum level] 50  [required level] 75
//	[awakening maximum level]  0 0 0 0 0 40   0 0 0 0 0 40   0 0 0 0 0 40
//
// 旧实现要求 len(caps) == 3*len(base)，基础行缺失时 len(base)=0，判定恒为 false，
// 这些技能（全目录 36 个，分布在 48~95 级）永远学不了。列数必须能从矩阵本身推出。
func TestAwakeningMatrixWithoutBaseCaps(t *testing.T) {
	l := loadLearningForTest(t)
	d, ok := l.index[0][300]
	if !ok || d.Path != "skill/swordman/nachal.skl" {
		t.Fatalf("swordman skill 300: %+v", d)
	}
	if len(d.Ints("[growtype maximum level]")) != 0 {
		t.Fatal("sample changed: this skill now carries base caps")
	}
	if cols := d.awakeningColumns(); cols != 6 {
		t.Fatalf("columns derived from the matrix: got %d, want 6", cols)
	}
	// adv5（该技能所属转职）在 stage 1/2/3 都有上限。
	if !d.ForAwakening(5, 1) || !d.ForAwakening(5, 3) {
		t.Fatal("an awakening matrix without base caps is not usable")
	}
	// 其它 growtype 仍必须被拒绝（补齐的列是 0）。
	if d.ForAwakening(1, 1) {
		t.Fatal("a growtype without a cap must stay refused")
	}
	cost, e := d.costForLevel(State{Level: 75, Advancement: 5, Awakening: 1}, 75, 1, map[uint16]byte{})
	if e != nil {
		t.Fatalf("skill carrying only an awakening matrix refused: %v", e)
	}
	if cost <= 0 {
		t.Fatalf("cost: %d", cost)
	}
	// 未觉醒时仍然拒绝。
	if _, e := d.costForLevel(State{Level: 75, Advancement: 5, Awakening: 0}, 75, 1, map[uint16]byte{}); e == nil {
		t.Fatal("unawakened character learned an awakening-only skill")
	}
	// 基础行存在但与矩阵长度不一致时，仍然拒绝（不能靠"推导"混进一个畸形块）。
	broken := d
	broken.Fields = map[string][]pvf.Token{
		"[type]":                    d.Fields["[type]"],
		"[growtype maximum level]":  d.Fields["[growtype maximum level]"],
		"[awakening maximum level]": append(append([]pvf.Token(nil), d.Fields["[awakening maximum level]"]...), pvf.Token{Type: 0, Value: 40}),
	}
	if broken.awakeningColumns() != 0 {
		t.Fatal("an inconsistent base/matrix pair must stay unresolved")
	}
}

// 全目录扫描：凡是带 [awakening maximum level] 的技能，列数都必须能解析出来，
// 否则它在任何阶段都不可学（旧实现下的 36 个）。
func TestEveryAwakeningMatrixResolvesColumns(t *testing.T) {
	l := loadLearningForTest(t)
	total, resolved := 0, 0
	for _, job := range l.index {
		for _, d := range job {
			if len(d.Ints("[awakening maximum level]")) == 0 {
				continue
			}
			total++
			if d.awakeningColumns() > 0 {
				resolved++
				continue
			}
			t.Fatalf("skill %s: awakening matrix whose columns cannot be derived", d.Path)
		}
	}
	if total == 0 {
		t.Fatal("no awakening matrices in the catalog")
	}
	if resolved != total {
		t.Fatalf("resolved %d/%d awakening matrices", resolved, total)
	}
}
