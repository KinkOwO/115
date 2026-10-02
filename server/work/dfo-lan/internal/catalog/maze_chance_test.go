package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// mazeChanceScript 造一份只有 [maze info] / [maze chance rate] 的最小脚本。
// 传 -1 表示「这张 maze 不声明 [maze chance rate]」。
func mazeChanceScript(rates ...int32) ScriptRecord {
	var cells []pvf.Token
	for _, r := range rates {
		cells = append(cells,
			pvf.Token{Type: 3, Text: "[maze info]"},
			pvf.Token{Type: 3, Text: "[size]"},
			pvf.Token{Type: 0, Value: 1},
			pvf.Token{Type: 0, Value: 1},
		)
		if r >= 0 {
			cells = append(cells,
				pvf.Token{Type: 3, Text: "[maze chance rate]"},
				pvf.Token{Type: 0, Value: r},
			)
		}
	}
	return ScriptRecord{Path: "dungeon/test.dgn", SHA256: strings.Repeat("a", 64), Cells: cells}
}

func TestReadMazeChanceRatesReadsEveryMaze(t *testing.T) {
	got, ok := ReadMazeChanceRates(mazeChanceScript(992857, 7143))
	if !ok {
		t.Fatal("expected the table to be considered complete")
	}
	want := []uint32{992857, 7143}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

// 半张表不能用来归一化：缺一张就等于静默改变其余各张的相对概率。
func TestReadMazeChanceRatesRejectsPartialTable(t *testing.T) {
	if _, ok := ReadMazeChanceRates(mazeChanceScript(992857, -1)); ok {
		t.Fatal("a maze without [maze chance rate] must make the whole table unusable")
	}
	if _, ok := ReadMazeChanceRates(mazeChanceScript()); ok {
		t.Fatal("a script without any [maze info] must be rejected")
	}
}

func chanceCatalog(sha string, mazes int) DungeonCatalog {
	d := DungeonDefinition{ID: 100005014, Script: ScriptRecord{SHA256: sha}}
	for i := 0; i < mazes; i++ {
		d.Mazes = append(d.Mazes, DungeonMaze{Index: byte(i), Quest: 0, Size: [2]byte{1, 1}})
	}
	return DungeonCatalog{
		Source:   pvf.ArchiveSnapshot{Checksum: "source-checksum"},
		Dungeons: map[uint32]DungeonDefinition{100005014: d},
	}
}

func writeOverlay(t *testing.T, overlay MazeChanceOverlay) string {
	t.Helper()
	b, err := json.Marshal(overlay)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "overlay.json")
	if err := os.WriteFile(path, b, 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAttachMazeChanceRatesInstallsWeights(t *testing.T) {
	sha := strings.Repeat("a", 64)
	c := chanceCatalog(sha, 2)
	path := writeOverlay(t, MazeChanceOverlay{
		SourceChecksum: c.Source.Checksum,
		Dungeons: []MazeChanceDungeon{{
			ID:            100005014,
			DungeonSHA256: sha,
			SourceRates:   []uint32{992857, 7143},
			Rates:         []uint32{980000, 20000},
		}},
	})
	if err := AttachMazeChanceRates(&c, path); err != nil {
		t.Fatal(err)
	}
	got := c.Dungeons[100005014].MazeChanceRates
	if len(got) != 2 || got[0] != 980000 || got[1] != 20000 {
		t.Fatalf("got %v, want [980000 20000]", got)
	}
}

// 没写 rates 时退回 source_rates —— 让「只要官方原值」不用复制一遍数字。
func TestAttachMazeChanceRatesFallsBackToSourceRates(t *testing.T) {
	sha := strings.Repeat("a", 64)
	c := chanceCatalog(sha, 2)
	path := writeOverlay(t, MazeChanceOverlay{
		SourceChecksum: c.Source.Checksum,
		Dungeons: []MazeChanceDungeon{{
			ID: 100005014, DungeonSHA256: sha,
			SourceRates: []uint32{992857, 7143},
		}},
	})
	if err := AttachMazeChanceRates(&c, path); err != nil {
		t.Fatal(err)
	}
	got := c.Dungeons[100005014].MazeChanceRates
	if len(got) != 2 || got[0] != 992857 || got[1] != 7143 {
		t.Fatalf("got %v, want the source rates", got)
	}
}

func TestAttachMazeChanceRatesRejectsBadOverlays(t *testing.T) {
	sha := strings.Repeat("a", 64)
	cases := []struct {
		name     string
		sha      string
		rates    []uint32
		checksum string
	}{
		{"source drift", strings.Repeat("b", 64), []uint32{1, 1}, "source-checksum"},
		{"checksum drift", sha, []uint32{1, 1}, "other"},
		{"count mismatch", sha, []uint32{1}, "source-checksum"},
		{"all zero", sha, []uint32{0, 0}, "source-checksum"},
		// 只有一张可选 = 这个副本不需要掷骰，放进白名单只会让人以为「另一张坏了」。
		{"single selectable", sha, []uint32{1000000, 0}, "source-checksum"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := chanceCatalog(sha, 2)
			path := writeOverlay(t, MazeChanceOverlay{
				SourceChecksum: tc.checksum,
				Dungeons: []MazeChanceDungeon{{
					ID: 100005014, DungeonSHA256: tc.sha, Rates: tc.rates,
				}},
			})
			if err := AttachMazeChanceRates(&c, path); err == nil {
				t.Fatal("expected an error")
			}
			if len(c.Dungeons[100005014].MazeChanceRates) != 0 {
				t.Fatal("a rejected overlay must not leave weights behind")
			}
		})
	}
}

// 真实值守一条：小深渊那张表的官方权重必须仍是 992857 : 7143。
// 我们改了服务端实际用的权重，但 source_rates 这条线索不能跟着变，
// 否则「官方原值是多少」就再也查不回来了。
func TestMazeChanceOverlayKeepsSourceRates(t *testing.T) {
	b, err := os.ReadFile("../../configs/dungeons.maze-chance-rates.json")
	if err != nil {
		t.Fatal(err)
	}
	var overlay MazeChanceOverlay
	if err := json.Unmarshal(b, &overlay); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range overlay.Dungeons {
		if entry.ID != 100005014 {
			continue
		}
		found = true
		if len(entry.SourceRates) != 2 || entry.SourceRates[0] != 992857 || entry.SourceRates[1] != 7143 {
			t.Fatalf("source_rates = %v, want [992857 7143]", entry.SourceRates)
		}
		if len(entry.Rates) != 2 || entry.Rates[1] == 0 {
			t.Fatalf("rates = %v, want a non-zero special-maze weight", entry.Rates)
		}
		if entry.DungeonSHA256 == "" {
			t.Fatal("dungeon_sha256 is required to detect source drift")
		}
	}
	if !found {
		t.Fatal("the small abyss must be in the whitelist")
	}
}

// 最值钱的一条：真实 overlay 必须能装到真实目录上。它一次钉住 source_checksum、
// 副本 sha256、权重个数与 maze 数一致 —— 任一项漂移，服务端都会在启动期直接失败，
// 而这条测试让那个失败出现在测试里，而不是实机上。
func TestMazeChanceOverlayFitsTheRealCatalog(t *testing.T) {
	c := LoadNativeFullDungeons(t)
	if err := AttachMazeChanceRates(&c, "../../configs/dungeons.maze-chance-rates.json"); err != nil {
		t.Fatal(err)
	}
	d := c.Dungeons[100005014]
	if len(d.MazeChanceRates) != 2 || d.MazeChanceRates[0] != 980000 || d.MazeChanceRates[1] != 20000 {
		t.Fatalf("weights = %v, want [980000 20000]", d.MazeChanceRates)
	}
	// 白名单只动它自己：另一个调律副本必须原样零值。
	if got := c.Dungeons[100005067].MazeChanceRates; len(got) != 0 {
		t.Fatalf("dungeon 100005067 must stay untouched, got %v", got)
	}
}
