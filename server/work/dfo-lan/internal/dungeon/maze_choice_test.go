package dungeon

import (
	"dfolan/internal/catalog"
	"testing"
)

func chanceMaze(index byte, quest uint16) catalog.DungeonMaze {
	return catalog.DungeonMaze{Index: index, Quest: quest, Size: [2]byte{1, 1}}
}

// 边界值必须落在对的那一张上 —— 差一位就是 0.02 的概率分布错了位置。
func TestPickWeightedMazeHitsTheBoundary(t *testing.T) {
	candidates := []catalog.DungeonMaze{chanceMaze(0, 0), chanceMaze(1, 0)}
	rates := []uint32{980000, 20000}
	for _, tc := range []struct {
		roll uint64
		want byte
	}{{0, 0}, {979999, 0}, {980000, 1}, {999999, 1}} {
		if got := pickWeightedMaze(candidates, rates, tc.roll); got.Index != tc.want {
			t.Errorf("roll %d picked maze %d, want %d", tc.roll, got.Index, tc.want)
		}
	}
}

// pickWeightedMaze 只管分段；权重为 0 的那一段不可达，靠 roll 永远进不去。
func TestPickWeightedMazeSkipsZeroWeight(t *testing.T) {
	candidates := []catalog.DungeonMaze{chanceMaze(0, 0), chanceMaze(1, 0)}
	for roll := uint64(0); roll < 1000000; roll += 9973 {
		if got := pickWeightedMaze(candidates, []uint32{1000000, 0}, roll); got.Index != 0 {
			t.Fatalf("roll %d reached a zero-weight maze", roll)
		}
		if got := pickWeightedMaze(candidates, []uint32{0, 1000000}, roll); got.Index != 1 {
			t.Fatalf("roll %d did not reach the only weighted maze", roll)
		}
	}
}

// 没声明权重的副本必须完全保持原行为（288 个多 maze 副本靠这条不变）。
func TestChooseMazeKeepsLowestIndexWithoutWeights(t *testing.T) {
	d := catalog.DungeonDefinition{Mazes: []catalog.DungeonMaze{chanceMaze(1, 0), chanceMaze(0, 0)}}
	got, err := chooseMaze(d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 0 {
		t.Fatalf("picked maze %d, want the lowest index", got.Index)
	}
}

// 权重长度与 maze 数不符 = 表没对齐，宁可退回确定性规则也不能按错位的数据掷骰。
func TestChooseMazeIgnoresMismatchedWeights(t *testing.T) {
	d := catalog.DungeonDefinition{
		Mazes:           []catalog.DungeonMaze{chanceMaze(0, 0), chanceMaze(1, 0)},
		MazeChanceRates: []uint32{0, 1000000, 0},
	}
	got, err := chooseMaze(d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 0 {
		t.Fatalf("picked maze %d, want the lowest index fallback", got.Index)
	}
}

// 全零权重（overlay 理论上会被 Attach 拒掉，但直接构造的目录可能绕过）也要退回
// 确定性规则，而不是抛错让玩家点不动图。
func TestChooseMazeFallsBackWhenEveryWeightIsZero(t *testing.T) {
	d := catalog.DungeonDefinition{
		Mazes:           []catalog.DungeonMaze{chanceMaze(0, 0), chanceMaze(1, 0)},
		MazeChanceRates: []uint32{0, 0},
	}
	got, err := chooseMaze(d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 0 {
		t.Fatalf("picked maze %d, want the lowest index fallback", got.Index)
	}
}

// 权重真的分流：把权重全压到某一张上，结果必须确定。
func TestChooseMazeCarriesTheWeightedPick(t *testing.T) {
	for _, index := range []byte{0, 1} {
		rates := []uint32{0, 0}
		rates[index] = 1
		d := catalog.DungeonDefinition{
			Mazes:           []catalog.DungeonMaze{chanceMaze(0, 0), chanceMaze(1, 0)},
			MazeChanceRates: rates,
		}
		for i := 0; i < 32; i++ {
			got, err := chooseMaze(d, 0)
			if err != nil {
				t.Fatal(err)
			}
			if got.Index != index {
				t.Fatalf("weight on maze %d but picked %d", index, got.Index)
			}
		}
	}
}

// 候选只按 quest 过滤：不匹配的 maze 既不参与掷骰，也不影响归一化分母。
func TestChooseMazeOnlyConsidersTheRequestedQuest(t *testing.T) {
	d := catalog.DungeonDefinition{
		Mazes: []catalog.DungeonMaze{
			chanceMaze(0, 23108),
			chanceMaze(1, 0),
			chanceMaze(2, 0),
		},
		MazeChanceRates: []uint32{0, 1, 0},
	}
	got, err := chooseMaze(d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 1 {
		t.Fatalf("picked maze %d, want maze 1", got.Index)
	}
}

// 解析不完整的 maze 不进候选 —— 进了就会把一张加载不了的地图选出来。
func TestChooseMazeSkipsPendingMazes(t *testing.T) {
	pending := chanceMaze(0, 0)
	pending.Pending = []string{"unsupported room specification"}
	d := catalog.DungeonDefinition{
		Mazes:           []catalog.DungeonMaze{pending, chanceMaze(1, 0)},
		MazeChanceRates: []uint32{1000000, 1},
	}
	got, err := chooseMaze(d, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got.Index != 1 {
		t.Fatalf("picked maze %d, want the only resolvable one", got.Index)
	}
}
