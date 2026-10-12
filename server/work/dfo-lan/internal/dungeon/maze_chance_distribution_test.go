package dungeon

import (
	"dfolan/internal/catalog"
	"math"
	"testing"
)

// 选图掷骰的**分发**测试。
//
// maze_choice_test.go 钉的是边界与回退（哪一段落在哪一张、权重不齐时退回最小
// index），一次都没有量过「实际掷出来的比例对不对」。这件事必须测：大深渊
// 「命运的凝械」的**入口就是这一掷** —— `[hidden drop table]` 只挂在 epic.ctp 的
// maze 1/2 上，而源把这两张的权重写成 9143/857（百万分位）。掷偏了不是「概率略差」，
// 是整场事件按错误频率出现，或者根本不出现。
//
// 判据沿用掉落那一套（见 [[feedback-seed-sweep-probability-tests]]）：按**位次**聚合、
// 与源权重摆表、并单独列出「零命中分支」。这里不能只看总偏差 —— 0.0857% 那一档
// 在 1e6 次里期望只有 857 次，绝对偏差看着小，相对误差却是 ±3.4%（1σ）。

const mazeRolls = 1000000

// mazeChanceCase 是一张按权重掷图的副本：权重原样取**源声明**的值，不做四舍五入。
type mazeChanceCase struct {
	name  string
	rates []uint32
	mazes int
}

func TestChooseMazeDistributionMatchesSourceRates(t *testing.T) {
	cases := []mazeChanceCase{
		// 100005068 / 100004768 的 epic.ctp：源 [990000 9143 857]，policy 未改动。
		{"epic 砝码大深渊 100005068", []uint32{990000, 9143, 857}, 3},
		// 100005014 小深渊：源是 [992857 7143]，业主把异空间那一档提到 2%。
		{"小深渊 100005014（服主改过权重）", []uint32{980000, 20000}, 2},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := catalogDefinition(c.rates, c.mazes)
			var total uint64
			for _, w := range c.rates {
				total += uint64(w)
			}

			hits := make([]uint64, c.mazes)
			for i := 0; i < mazeRolls; i++ {
				got, err := chooseMaze(d, 0)
				if err != nil {
					t.Fatalf("roll %d: %v", i, err)
				}
				if got.Index >= byte(c.mazes) {
					t.Fatalf("roll %d picked maze %d, outside the %d maze(s)", i, got.Index, c.mazes)
				}
				hits[got.Index]++
			}

			t.Logf("%s: %d rolls over %d maze(s), weight space %d", c.name, mazeRolls, c.mazes, total)
			t.Logf("  %-6s %-9s %9s %9s %9s %7s", "maze", "source%", "measured%", "dev pp", "1sigma", "sigmas")
			var unreachable []int
			var worstPP, worstSigma float64
			for i, w := range c.rates {
				want := float64(w) / float64(total)
				got := float64(hits[i]) / float64(mazeRolls)
				devPP := (got - want) * 100
				sigma := math.Sqrt(want*(1-want)/float64(mazeRolls)) * 100
				ratio := 0.0
				if sigma > 0 {
					ratio = devPP / sigma
				}
				t.Logf("  %-6d %-9.4f %9.4f %9.4f %7.4f %7.2f", i, want*100, got*100, devPP, sigma, ratio)
				if hits[i] == 0 && w > 0 {
					unreachable = append(unreachable, i)
				}
				if math.Abs(devPP) > worstPP {
					worstPP = math.Abs(devPP)
				}
				if math.Abs(ratio) > math.Abs(worstSigma) {
					worstSigma = ratio
				}
			}
			// 零命中 = 这一档在实机上永远进不去，等于事件没接。
			if len(unreachable) > 0 {
				t.Fatalf("mazes %v carry weight but were never picked in %d rolls", unreachable, mazeRolls)
			}
			// 5σ 之外才是分布错，不是抽样噪声（1e6 次对 0.0857% 那档的分辨率就到这里）。
			if math.Abs(worstSigma) > 5 {
				t.Fatalf("worst deviation is %.2f sigma (%.4f pp) - the roll does not follow the source weights", worstSigma, worstPP)
			}
			// 大档另加一道绝对界：0.1 pp 内。太初那 0.0857% 走上面的 σ 判据。
			if worstPP > 0.5 {
				t.Fatalf("worst absolute deviation %.4f pp is too large even for a weighted pick", worstPP)
			}
		})
	}
}

func catalogDefinition(rates []uint32, mazes int) catalog.DungeonDefinition {
	d := catalog.DungeonDefinition{MazeChanceRates: append([]uint32(nil), rates...)}
	for i := 0; i < mazes; i++ {
		d.Mazes = append(d.Mazes, chanceMaze(byte(i), 0))
	}
	return d
}
