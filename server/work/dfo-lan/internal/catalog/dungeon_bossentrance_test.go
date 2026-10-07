package catalog

import (
	"os"
	"path/filepath"
	"testing"

	"dfolan/internal/catalog/pvf"
)

// 塞洛可（100004961）的Boss门红柱只认 NOTI312；服务端是否发送取决于这里从源
// [boss room entrance condition] 采到的模板。源形状（2f_100004961.dgn）：
//
//	[boss room entrance condition] [hunt monster] 1 109019125 0 1
//
// 静态取证 2026-10-04：客户端 handler 1452FF500 读 <u32 conditionID><u8 completed>
// 后置 dungeon+8056 标志，门 tick 14614E440 依赖该标志播放开门动画。
func TestSiroccoBossEntranceConditionFromSource(t *testing.T) {
	p := os.Getenv("DFO_LOOT_PVF")
	if p == "" {
		p = filepath.Join("..", "..", "..", "..", "..", "client-build", "Script.inner.pvf")
	}
	if _, err := os.Stat(p); err != nil {
		t.Skip("current PVF absent")
	}
	a, err := pvf.OpenReadOnly(pvf.Options{Path: p, MaxBytes: 1024 * 1024 * 1024}, "")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	imported, err := ImportDungeons(a, []uint32{100004961, 37, 2005})
	if err != nil {
		t.Fatal(err)
	}
	d := imported.Dungeons[100004961]
	if !d.Odyssey {
		t.Fatalf("100004961 should be Odyssey")
	}
	if len(d.BossEntranceConditionIDs) != 1 || d.BossEntranceConditionIDs[0] != 109019125 {
		t.Fatalf("BossEntranceConditionIDs = %v, want [109019125]", d.BossEntranceConditionIDs)
	}
	// 37（shadowmaze `1 50097 1 1`）与 2005（gentinfiltrate 双目标 + [time condition]）
	// 是非 Odyssey 旧副本：入场门由客户端本地判定，服务端不采、不发 NOTI312。
	for _, id := range []uint32{37, 2005} {
		if got := imported.Dungeons[id].BossEntranceConditionIDs; len(got) != 0 {
			t.Fatalf("dungeon %d BossEntranceConditionIDs = %v, want empty (非 Odyssey 不采纳)", id, got)
		}
	}
}
