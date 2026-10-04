package loot

import (
	"strings"
	"testing"

	"dfolan/internal/catalog"
)

// [AZURE-CLEAR-RESULT] 蔚蓝号与沉月湖的副本脚本，[dungeon clear result] 一段的**取值**对照。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=... go test ./internal/loot/ -run ClearResultRegion -count=1 -v
func TestClearResultRegion(t *testing.T) {
	a := catalog.OpenNativeArchive(t)
	cases := []struct {
		path     string
		from, to int
	}{
		{"contents/2025/azuremain/dungeon/azuremain.dgn", 143, 153},
		{"contents/2025/moonlake/dungeon/2f/2f_100004137.dgn", 75, 85},
		{"contents/2025/moonlake/dungeon/1f/1f_100004136.dgn", 128, 138},
	}
	for _, c := range cases {
		txt, err := a.ReadText(c.path)
		if err != nil {
			t.Logf("%s: %v", c.path, err)
			continue
		}
		lines := strings.Split(txt, "\n")
		t.Logf("=== %s (共 %d 行)", c.path, len(lines))
		for i := c.from; i < c.to && i < len(lines); i++ {
			t.Logf("  %4d| %s", i+1, strings.TrimRight(lines[i], "\r"))
		}
	}
}
