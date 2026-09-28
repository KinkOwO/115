package character

import (
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestOdysseyJournalTeleportCapturedRequest(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	b, _ := hex.DecodeString("280000000400000081032a01052600000001000000000000")
	r, err := protocol.DecodeAreaChangeRequest(b)
	if err != nil || r.Town != 40 || r.Area != 4 || r.X != 897 || r.Y != 298 {
		t.Fatal(r, err)
	}
	if s.OdysseyJournalTeleport(role, r) {
		t.Fatal("locked node accepted")
	}
	for _, id := range []uint32{100004934, 100004935, 100004936} {
		role.State, err = s.saveOdysseyCompletion(role, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	if !s.OdysseyJournalTeleport(role, r) {
		t.Fatal("confirmed prior node rejected")
	}
	// [MERGE-20260928-JOURNAL-LANDING] 原来这里还有一条 r.X++。落点坐标不是站点的
	// 身份：客户端从传送门/地图选择器出发时报的是**它自己的**默认落点 —— 实机
	// 2026-09-28「前往天界」请求报 town=12 area=0 落在 (363,229)，而路由表那一站
	// 记的是 (257,250)，两者都在该区域的 walkable 之内。站点身份是 (town, area)，
	// 落点是否合法交给 world.validatePosition。故只保留篡改站点/标志的用例。
	// [MERGE-20260928-JOURNAL-TAILFLAGS] 这条原为 TailFlags[0] = 1。尾部标志非零本身
	// 不是排除理由（实机 2026-09-28 从魔界回捷尔瓦的请求是 [0,2]）；要排除的是地图
	// 选择器 —— 其标志位是 5。
	for _, mutate := range []func(*protocol.AreaChangeRequest){func(r *protocol.AreaChangeRequest) { r.Town++ }, func(r *protocol.AreaChangeRequest) { r.Flag = 0 }, func(r *protocol.AreaChangeRequest) { r.TailFlags[0] = 5 }} {
		bad := r
		mutate(&bad)
		if s.OdysseyJournalTeleport(role, bad) {
			t.Fatal("altered journal target accepted", bad)
		}
	}
	role.Request = nil
	if s.OdysseyJournalTeleport(role, r) {
		t.Fatal("ordinary role admitted")
	}
}

// [MERGE-20260928-JOURNAL-LANDING] 实机回归：客户端从传送门/地图选择器出发时报的
// 是**它自己的**默认落点，与路由表里记的坐标不同。实机 2026-09-28 从根特 6/1
// (gent2ant1)「前往天界」，请求报 town=12 area=0 落在 (363,229)，而路由表那一站
// 记的是 (257,250)；旧逻辑逐像素比较，于是永远匹配不上，请求掉到 strict 的门户
// 检查被拒（客户端 area_refused 后卡在门上）。站点身份是 (town, area)。
func TestOdysseyJournalTeleportAcceptsClientLanding(t *testing.T) {
	s, role := odysseyGrowthFixture(t)
	// (12,0) 是日志第 7 站，前 6 站（934..943）必须先通关。
	for _, id := range []uint32{
		100004934, 100004935, 100004936,
		100004937, 100004938,
		100004939, 100004940,
		100004941, 100004942, 100004943,
	} {
		var err error
		role.State, err = s.saveOdysseyCompletion(role, id)
		if err != nil {
			t.Fatal(err)
		}
	}
	// 该站自身尚未打完时也放行（站点是「到达后要打的」，不是门槛）。
	for _, landing := range [][2]uint16{{363, 229}, {257, 250}, {400, 240}, {0, 0}} {
		r := protocol.AreaChangeRequest{Town: 12, Area: 0, X: landing[0], Y: landing[1], Flag: 5}
		if !s.OdysseyJournalTeleport(role, r) {
			t.Fatalf("站点 (12,0) 的客户端落点 %v 被拒", landing)
		}
	}
	// 换一个未解锁的站点仍然拒绝。
	next := protocol.AreaChangeRequest{Town: 6, Area: 1, X: 524, Y: 226, Flag: 5}
	if s.OdysseyJournalTeleport(role, next) {
		t.Fatal("未解锁的下一站被放行")
	}
}

func TestOdysseyJournalSourceIDs(t *testing.T) {
	s, _ := odysseyGrowthFixture(t)
	var nodes []odysseyJournalNode
	if err := json.Unmarshal(odysseyJournalRoutes, &nodes); err != nil || len(nodes) != 29 {
		t.Fatal(err)
	}
	seen := map[uint32]bool{}
	for _, node := range nodes {
		for _, id := range node.Dungeons {
			if seen[id] || s.Odyssey.ClearLevels[id] == 0 {
				t.Fatal("journal does not match source progression", id)
			}
			seen[id] = true
		}
	}
	if len(seen) != 50 {
		t.Fatal("missing source journal nodes")
	}
}
