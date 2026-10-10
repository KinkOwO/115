package dungeon

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"os"
	"testing"
	"time"
)

// TestMoonSoloBackwardRevisitReusesCachedRoom 守「走回头路到已访问过的格子」这条路径。
//
// 实机 BUG（2026-10-08，会话 20261008_231250，exit=0xC0000005）：
// 一阶段清完两个图后往回走 ⇒ 客户端 30 条
// `CNRDObjectManager::draw, not valid object pointer` ⇒ 闪退。
//
// 根因：往回走时 MoonSoloOwner.StartMap 发的是**普通路径 + 0 活怪**（41 B），
// 而不是 ReuseRoom 形态 ⇒ 客户端既不重建也不复用，缓存里的对象指针已失效。
// 普通副本路径早有这条规则（cmd/wireprobe/dungeon_flow.go：`Visited[next.Room.Map]`
// 命中 ⇒ `state.ReuseRoom = true` / `Monsters = nil`），沉月湖这条线漏了。
//
// 断言用**包形态**而不是字节数猜测：ReuseRoom 走 protocol.StartMap 的
// `append(p, 0, 0, 255)` 分支 ⇒ 尾部三字节恒为 `00 00 ff`，且长度远小于普通路径。
func TestMoonSoloBackwardRevisitReusesCachedRoom(t *testing.T) {
	path := os.Getenv("MOON_TEST_DUNGEONS")
	if path == "" {
		t.Skip("set MOON_TEST_DUNGEONS to a same-client complete export")
	}
	c, e := catalog.LoadDungeons(path)
	if e != nil {
		t.Fatal(e)
	}
	now := time.Unix(1800000000, 0)
	r, e := NewMoonSolo(c, 115, 7, 0, now)
	if e != nil {
		t.Fatal(e)
	}

	clear := func() {
		t.Helper()
		if e := r.Loaded(now); e != nil {
			t.Fatal(e)
		}
		if _, e := r.MoonEnterRoom(r.Stamp(), 7, now); e != nil {
			t.Fatal(e)
		}
		for round := 0; round < 4; round++ {
			killed := 0
			for _, v := range r.Session().LivingMonsters() {
				if v.NonCombat || v.APC {
					continue
				}
				if _, e := r.Session().ConfirmDeath(uint32(v.Entity), 7, 7); e != nil {
					t.Fatal(e)
				}
				if e = r.MoonScoreDeath(r.Stamp(), 7, v.Entity); e != nil {
					t.Fatal(e)
				}
				if _, e = r.MoonAfterDeath(r.Stamp(), 7); e != nil {
					t.Fatal(e)
				}
				killed++
			}
			if killed == 0 {
				return
			}
		}
		t.Fatal("unbounded respawn")
	}

	move := func(pos [2]byte, seed uint32) []byte {
		t.Helper()
		if e := r.Move(c, protocol.DungeonRoomTransition{Position: pos}, seed, now); e != nil {
			t.Fatalf("move to %v: %v", pos, e)
		}
		body, e := r.StartMap()
		if e != nil {
			t.Fatalf("StartMap for %v: %v", pos, e)
		}
		return body
	}
	isReuse := func(b []byte) bool {
		// protocol.StartMap：固定前缀（Position3 + Seed4 + HellPartyMode2 +
		// 0xffffffff4 + 换图记录18 = 31 字节）之后，ReuseRoom 走
		// `return append(p, 0, 0, 255)`（**mode0**，跳过 map/spawn 行）；
		// 普通路径则是 `append(p, 1)` 再跟 map u32 与怪物行。
		// ⇒ 判据取「长度 == 34 且第 32 字节为 0」而不是只看尾三字节
		//（普通包尾部也可能凑巧是 00 00 ff，实测 217 B 那包就是）。
		const prefix = 31
		return len(b) == prefix+3 && b[prefix] == 0 && b[prefix+1] == 0 && b[prefix+2] == 0xff
	}

	// 前进两格：都是**没访问过**的格子 ⇒ 必须是普通路径（带怪物行），不能是 ReuseRoom。
	clear()
	if b := move([2]byte{0, 1}, 1); isReuse(b) {
		t.Fatalf("前进到未访问的 (0,1) 不该发 ReuseRoom（%d B）", len(b))
	} else if r.reuseRoom {
		t.Fatal("前进到未访问的格子时 reuseRoom 应为 false")
	}
	clear()
	if b := move([2]byte{0, 2}, 2); isReuse(b) {
		t.Fatalf("前进到未访问的 (0,2) 不该发 ReuseRoom（%d B）", len(b))
	}
	clear()

	// ★ 往回走到**已经访问过**的 (0,1)：必须发 ReuseRoom。
	body := move([2]byte{0, 1}, 3)
	if !r.reuseRoom {
		t.Fatal("往回走到已访问过的格子时 reuseRoom 应为 true")
	}
	if !isReuse(body) {
		t.Fatalf("往回走应发 ReuseRoom 形态（尾部 00 00 ff），实际 %d B 尾部 % x",
			len(body), body[len(body)-3:])
	}
	// ReuseRoom 形态不得携带怪物行 —— protocol.StartMap 对「ReuseRoom + 非空 Monsters」
	// 会直接报错，所以这条同时证明我们没有把怪又塞回去。
	if len(body) > 40 {
		t.Fatalf("ReuseRoom 形态不应带怪物行，实际 %d B", len(body))
	}
}
