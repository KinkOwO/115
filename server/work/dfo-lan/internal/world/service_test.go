package world

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"encoding/hex"
	"errors"
	"testing"
)

func TestSiroccoCentralTentPhaseNPCPlacement(t *testing.T) {
	cat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	svc := &Service{Catalog: cat}
	at := WorldPosition{Town: 40, Area: 3, X: 515, Y: 160}
	if _, found := svc.NPCPosition(at, 100000374); found {
		t.Fatal("Sirocco target unexpectedly became a base-map NPC")
	}
	if position, found := svc.PhaseNPCPosition(at, 100000374); !found || position != [2]uint16{515, 114} {
		t.Fatalf("Sirocco phase NPC placement changed: %v, %v", position, found)
	}
	if _, found := svc.PhaseNPCPosition(WorldPosition{Town: 40, Area: 2}, 100000374); found {
		t.Fatal("phase NPC leaked into another area")
	}
}

func TestPandemoniumJunctionNativeZeroLanding(t *testing.T) {
	cat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	from := WorldPosition{Town: 35, Area: 0, X: 903, Y: 353}
	portal := cat.Areas["35/0"].Portals
	found := false
	for _, p := range portal {
		if p.Town == 35 && p.Area == 2 && Contains(p.Bounds, from.X, from.Y, 32) {
			found = true
		}
	}
	if !found {
		t.Fatal("captured source position is not near the 35/2 portal")
	}
	dest := cat.Areas["35/2"]
	if len(dest.Walkable) == 0 || dest.Walkable[0] != [4]int32{13, 180, 1100, 250} {
		t.Fatalf("unexpected native destination geometry: %v", dest.Walkable)
	}
	body, err := hex.DecodeString("230000000200000000000000052300000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	req, err := protocol.DecodeAreaChangeRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 32}}
	next, err := s.Transition(87, false, from, req)
	if err != nil || next.Town != 35 || next.Area != 2 || next.X != 563 || next.Y != 305 {
		t.Fatalf("native fallback landing failed: %+v %v", next, err)
	}
	if _, err := s.Transition(84, false, from, req); !errors.Is(err, ErrLevel) {
		t.Fatalf("level gate bypassed: %v", err)
	}
	remote := from
	remote.X, remote.Y = 100, 200
	if _, err := s.Transition(87, false, remote, req); err == nil {
		t.Fatal("remote zero-coordinate portal request accepted")
	}
	req.TailFlags = [2]byte{0, 5}
	if _, err := s.Transition(87, false, from, req); err == nil {
		t.Fatal("other zero-coordinate request accepted")
	}
}

func TestNativeZeroLandingUsesDestinationGeometryAcrossAreas(t *testing.T) {
	from := WorldPosition{Town: 70, Area: 3, X: 240, Y: 210}
	req := protocol.AreaChangeRequest{Town: 91, Area: 4, PreviousTown: 70, PreviousArea: 3, Flag: 5}
	areas := map[string]catalog.WorldArea{
		"70/3": {Town: 70, Area: 3, Walkable: [][4]int32{{100, 100, 300, 300}}, Portals: []catalog.Portal{{Bounds: [4]int32{200, 200, 80, 80}, Town: 91, Area: 4}}},
		"91/4": {Town: 91, Area: 4, Walkable: [][4]int32{{-100, 200, 600, 300}, {1000, 1000, 200, 200}}},
	}
	s := Service{Catalog: catalog.WorldCatalog{Areas: areas}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 16}}
	next, err := s.Transition(1, false, from, req)
	if err != nil || next.X != 200 || next.Y != 350 {
		t.Fatalf("first destination rectangle center: %+v %v", next, err)
	}
	// A matching destination entrance selects the client's other branch,
	// whose offset is not present in the imported portal catalog.
	dest := areas["91/4"]
	dest.Portals = []catalog.Portal{{Bounds: [4]int32{150, 260, 80, 80}, Town: 70, Area: 3}}
	areas["91/4"] = dest
	if _, err := s.Transition(1, false, from, req); err == nil {
		t.Fatal("zero landing with a matching destination entrance was fabricated")
	}
	delete(areas, "91/4")
	areas["91/4"] = catalog.WorldArea{Town: 91, Area: 4, Walkable: [][4]int32{{0, 200, 100, 100}}}
	delete(areas, "70/3")
	areas["70/3"] = catalog.WorldArea{Town: 70, Area: 3, Walkable: [][4]int32{{100, 100, 300, 300}}, Pending: []string{"dynamic portal destination"}}
	if _, err := s.Transition(1, false, from, req); err == nil {
		t.Fatal("dynamic permissive edge was treated as a source portal")
	}
}

func TestTransitionAuthority(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/0": {Town: 38, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}, Portals: []catalog.Portal{{Bounds: [4]int32{500, 200, 80, 80}, Town: 38, Area: 1}, {Town: 38, Area: 3}}},
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"38/3": {Town: 38, Area: 3, MinimumLevel: 14, Walkable: [][4]int32{{0, 0, 1000, 500}}},
	}}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	old := WorldPosition{Town: 38, Area: 0, X: 550, Y: 230}
	req := protocol.AreaChangeRequest{Town: 38, Area: 1, X: 544, Y: 311, PreviousTown: 38, PreviousArea: 0}
	next, e := s.Transition(1, false, old, req)
	if e != nil || next.Return == nil {
		t.Fatalf("valid entry: %+v %v", next, e)
	}
	req.Area = 3
	if _, e = s.Transition(1, false, old, req); !errors.Is(e, ErrLevel) {
		t.Fatal("level gate bypass", e)
	}
	req.Area = 1
	req.X = 65535
	if _, e = s.Transition(1, false, old, req); e == nil {
		t.Fatal("outside coordinates accepted")
	}
	req.X = 544
	req.PreviousTown = 39
	if _, e = s.Transition(1, false, old, req); e == nil {
		t.Fatal("stale origin accepted")
	}
	req.PreviousTown = 38
	old.X = 10
	if _, e = s.Transition(1, false, old, req); e == nil {
		t.Fatal("remote portal bypass")
	}
	next.X = 550
	next.Y = 340
	back := protocol.AreaChangeRequest{Town: 38, Area: 0, X: 561, Y: 234, PreviousTown: 38, PreviousArea: 1}
	returned, e := s.Transition(1, false, next, back)
	if e != nil || returned.Return != nil {
		t.Fatalf("return warp: %+v %v", returned, e)
	}
	// 出门的目的地永远是戳，不是请求：客户端报同镇另一个区域（38/3）也一样。
	// 旧守卫「请求目的地 == Return」在这里不进分支，因为赛丽亚房没有门户边且
	// SeriaReturnWarp=true，permissive 恒 false ⇒ 玩家被困在房间里。
	back.Area = 3
	redirected, e := s.Transition(20, false, next, back)
	if e != nil || redirected.Town != 38 || redirected.Area != 0 || redirected.X != 550 || redirected.Y != 230 || redirected.Return != nil {
		t.Fatalf("return warp did not ignore client destination: %+v %v", redirected, e)
	}
}

func TestSeriaLeaveIgnoresClientDestination(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"41/1": {Town: 41, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{350, 150, 100, 100}}},
	}}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	stamp := &WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}
	inside := WorldPosition{Town: 38, Area: 1, X: 550, Y: 340, Return: stamp}
	for _, requested := range []protocol.AreaChangeRequest{
		{Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1},
		{Town: 999, Area: 999, X: 1, Y: 1, PreviousTown: 38, PreviousArea: 1},
	} {
		out, err := s.Transition(1, false, inside, requested)
		if err != nil || out.Town != stamp.Town || out.Area != stamp.Area || out.X != stamp.X || out.Y != stamp.Y || out.Return != nil {
			t.Fatalf("requested %+v returned %+v: %v", requested, out, err)
		}
	}
	inside.X, inside.Y = 410, 200
	if _, err := s.Transition(1, false, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, PreviousTown: 38, PreviousArea: 1}); err == nil {
		t.Fatal("return outside exit bounds accepted")
	}
	inside.X, inside.Y = 550, 340
	inside.Return.X = 65535
	if _, err := s.Transition(1, false, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, PreviousTown: 38, PreviousArea: 1}); err == nil {
		t.Fatal("invalid stamped destination accepted")
	}
}

func TestSameAreaRepositionKeepsReturnStamp(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
	}}, Rules: Rules{RequirePortalProximity: true}}
	stamp := &WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}
	inside := WorldPosition{Town: 38, Area: 1, X: 520, Y: 200, Return: stamp}
	out, err := s.Transition(1, false, inside, protocol.AreaChangeRequest{Town: 38, Area: 1, X: 540, Y: 210, PreviousTown: 38, PreviousArea: 1})
	if err != nil || out.Town != 38 || out.Area != 1 || out.X != 540 || out.Y != 210 || out.Return != stamp {
		t.Fatalf("in-room reposition changed return stamp: %+v %v", out, err)
	}
}

func TestSourceSeriaRoundTrip(t *testing.T) {
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	start := WorldPosition{Town: 38, Area: 0, X: 622, Y: 196}
	inside, e := s.Transition(1, false, start, protocol.AreaChangeRequest{Town: 38, Area: 1, X: 544, Y: 311, PreviousTown: 38, PreviousArea: 0})
	if e != nil {
		t.Fatal(e)
	}
	inside.Y = 340
	// Captured map-selector request supplies a non-walkable generic landing
	// point. Only an authorized Seria return resolves to the saved origin.
	out, e := s.Transition(1, false, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1})
	if e != nil || out.Return != nil || out.Town != start.Town || out.Area != start.Area {
		t.Fatalf("source return warp failed: %+v %v", out, e)
	}
	if out.X != start.X || out.Y != start.Y {
		t.Fatal("saved return coordinates lost", out)
	}
	inside.Return.X = 65535
	if _, e = s.Transition(1, false, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, X: 561, Y: 234, PreviousTown: 38, PreviousArea: 1}); e == nil {
		t.Fatal("invalid saved origin accepted")
	}
}

func TestQuestGatedPortalToElvenmere(t *testing.T) {
	// 实源：elvengard_hendon_storm.map（38/3）的 [town movable area] 内嵌
	// [quest condition] 3156 [/quest condition]，其后是通往 Elvenmere（38/7）
	// 的门户行。旧导入器在子块处失活吞掉该行，实机进入时报
	// "no authorized source portal to destination"。任务条件由客户端
	// 依据同一份地图数据自行判定，服务端只须授权这条边。
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	at := WorldPosition{Town: 38, Area: 3, X: 60, Y: 260}
	req := protocol.AreaChangeRequest{Town: 38, Area: 7, X: 100, Y: 260, PreviousTown: 38, PreviousArea: 3}
	next, e := s.Transition(20, false, at, req)
	if e != nil || next.Town != 38 || next.Area != 7 {
		t.Fatalf("quest-gated Elvenmere portal refused: %+v %v", next, e)
	}
	if _, e = s.Transition(20, false, WorldPosition{Town: 38, Area: 3, X: 900, Y: 300}, req); e == nil {
		t.Fatal("remote portal bypass")
	}
	if _, e = s.Transition(16, false, at, req); e == nil {
		t.Fatal("minimum level 17 not enforced")
	}
}

func TestWestCoastTownOriginSync(t *testing.T) {
	cat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	old := WorldPosition{Town: 40, Area: 0, X: 412, Y: 181}
	actual, err := hex.DecodeString("28000000000000009c01b500002800000000000000000000")
	if err != nil {
		t.Fatal(err)
	}
	r, err := protocol.DecodeAreaChangeRequest(actual)
	if err != nil {
		t.Fatal(err)
	}
	next, err := s.Transition(96, false, old, r)
	if err != nil || next != old {
		t.Fatalf("native town origin sync rejected: next=%+v err=%v", next, err)
	}
	r.X++
	if _, err := s.Transition(96, false, old, r); err == nil {
		t.Fatal("different destination should still require an authorized portal")
	}
}

func TestNegativeCoordinateAreaTransition(t *testing.T) {
	// 实源：lemidia_right.map（89/2，雷米迪亚大圣堂右侧区域）：
	// 可行走矩形为 [-13, 256, 800, 140]，客户端生成的落点 X=-2（补码 uint16 为 65534）。
	// 旧的 Contains 将 x/y 无符号提升为 int64(x)，把 65534 当作正大数，
	// 导致判定在矩形外（"position outside source walkable rectangles"）。
	// 改为 int64(int16(x)) 后应正常放行负坐标。
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: false, PortalMargin: 32}}
	at := WorldPosition{Town: 89, Area: 0, X: 985, Y: 416}
	req := protocol.AreaChangeRequest{
		Town: 89, Area: 2, X: 65534, Y: 309, // X = -2
		PreviousTown: 89, PreviousArea: 0,
	}
	next, e := s.Transition(20, false, at, req)
	if e != nil || next.Town != 89 || next.Area != 2 || next.X != 65534 || next.Y != 309 {
		t.Fatalf("transition to 89/2 with negative X failed: %+v %v", next, e)
	}

	// 验证从 89/2 返回 89/0：89/2 的门户矩形为 [-22, 249, 40, 120]（负 X 边界）
	at2 := WorldPosition{Town: 89, Area: 2, X: 65534, Y: 300} // X = -2
	reqReturn := protocol.AreaChangeRequest{
		Town: 89, Area: 0, X: 950, Y: 400,
		PreviousTown: 89, PreviousArea: 2,
	}
	sProximity := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 32}}
	next0, e := sProximity.Transition(20, false, at2, reqReturn)
	if e != nil || next0.Town != 89 || next0.Area != 0 {
		t.Fatalf("return to 89/0 from negative portal bounds failed: %+v %v", next0, e)
	}

	// 超出负边界的坐标必须被拒绝
	oobX := int16(-500)
	reqOOB := protocol.AreaChangeRequest{
		Town: 89, Area: 2, X: uint16(oobX), Y: 309,
		PreviousTown: 89, PreviousArea: 0,
	}
	if _, e = s.Transition(20, false, at, reqOOB); e == nil {
		t.Fatal("out-of-bounds negative X accepted")
	}
}

func TestOdysseyEnterLevelGateAtStormPass(t *testing.T) {
	// 实源 map/cataclysm/town/stormpass/stormpass.map（43/0）与
	// ..._deathvalley_storm.map（43/1、43/2）的 [permission] 块同时给出
	// [need level] 50 与 [odyssey enter level] 45。
	// 客户端在奥德赛模式下按后者判定（拒绝提示 DSTR 535 填 45），服务端此前只读
	// [need level]，于是 45 级奥德赛角色传送进城 43/1 被回 code 8 卡住剧情。
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	for _, key := range []string{"43/0", "43/1", "43/2"} {
		a, ok := cat.Areas[key]
		if !ok {
			t.Fatalf("missing source area %s", key)
		}
		if a.MinimumLevel != 50 || a.OdysseyMinimumLevel != 45 {
			t.Fatalf("%s gates: need %d odyssey %d", key, a.MinimumLevel, a.OdysseyMinimumLevel)
		}
		if got := RequiredLevel(a, true); got != 45 {
			t.Fatalf("%s odyssey gate %d", key, got)
		}
		if got := RequiredLevel(a, false); got != 50 {
			t.Fatalf("%s ordinary gate %d", key, got)
		}
	}
	s := Service{Catalog: cat}
	p := WorldPosition{Town: 43, Area: 1, X: 396, Y: 403} // Odyssey journal node 6
	if e := s.ValidatePosition(44, true, p); !errors.Is(e, ErrLevel) {
		t.Fatalf("odyssey level 44 admitted: %v", e)
	}
	if e := s.ValidatePosition(45, false, p); !errors.Is(e, ErrLevel) {
		t.Fatalf("ordinary level 45 admitted: %v", e)
	}
	if e := s.ValidatePosition(45, true, p); e != nil {
		t.Fatalf("odyssey level 45 refused: %v", e)
	}
	// 重登恢复使用宽松门槛：两个源门槛中较小的那个生效，角色不会因为升级后
	// 服务端改用奥德赛门槛而卡在已保存的位置上。
	if e := s.ValidateRestoredPosition(45, true, p); e != nil {
		t.Fatalf("restore refused: %v", e)
	}
	if got := RestorationLevel(cat.Areas["43/1"], true); got != 45 {
		t.Fatalf("restoration gate %d", got)
	}
}

func TestOdysseyGateNeverRaisesEntry(t *testing.T) {
	// 实机缺陷（2026-09-22，角色 gugugaga，20 级奥德赛）：从赫顿玛尔（39/0）
	// 传送西海岸（40/0）被拒 code 8，客户端提示 DSTR 30069「You must be Level 15
	//  to go to West Coast」。40/0 的 [permission] 是 [need level] 15 /
	// [odyssey enter level] 35（16 个 odyssey>need 区之一）。客户端在 20 级就放行
	// 请求且提示数字是 15，说明高出的 odyssey 值不是入门门槛（它约束 [phase]
	// 变体）；服务端取 min 与客户端一致。
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	a, ok := cat.Areas["40/0"]
	if !ok {
		t.Fatal("missing source area 40/0")
	}
	if a.MinimumLevel != 15 || a.OdysseyMinimumLevel != 35 {
		t.Fatalf("40/0 gates: need %d odyssey %d", a.MinimumLevel, a.OdysseyMinimumLevel)
	}
	if got := RequiredLevel(a, true); got != 15 {
		t.Fatalf("odyssey gate raised the entry bar: %d", got)
	}
	if got := RequiredLevel(a, false); got != 15 {
		t.Fatalf("ordinary gate %d", got)
	}
	s := Service{Catalog: cat}
	p := WorldPosition{Town: 40, Area: 0, X: 388, Y: 180}
	if e := s.ValidatePosition(20, true, p); e != nil {
		t.Fatalf("odyssey level 20 refused West Coast: %v", e)
	}
	if e := s.ValidatePosition(14, true, p); !errors.Is(e, ErrLevel) {
		t.Fatalf("odyssey level 14 admitted: %v", e)
	}
	if e := s.ValidatePosition(14, false, p); !errors.Is(e, ErrLevel) {
		t.Fatalf("ordinary level 14 admitted: %v", e)
	}
}

func TestRequiredLevelKeepsOrdinaryGateWithoutSourceOdysseyLevel(t *testing.T) {
	// 源里没有 [odyssey enter level] 的区域，奥德赛角色仍按 [need level] 判定。
	a := catalog.WorldArea{Town: 39, Area: 5, MinimumLevel: 41}
	if got := RequiredLevel(a, true); got != 41 {
		t.Fatalf("gate %d", got)
	}
	if got := RestorationLevel(a, true); got != 41 {
		t.Fatalf("restoration gate %d", got)
	}
}

// 离开赛丽亚房间（38/1、142/1，源里 [is seria room warp] 仅此两处）的目的地永远是
// 本次进房时存下的 Return 戳，与客户端在请求里报的坐标 / 城镇无关。
//
// 上游旧守卫是「请求目的地 == Return」，于是两种真实请求都落空：
//   - 通用动画落点（实机 live17 的 746,157，属于本镇另一区域）；
//   - 地图选择器报的另一个镇。
//
// 落空之后赛丽亚房没有门户边且 SeriaReturnWarp=true（permissive 恒 false）⇒
// "no authorized source portal to destination"，玩家被困在房间里出不去。
func TestSeriaLeaveReturnsToStampedOrigin(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/0": {Town: 38, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}},
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"41/1": {Town: 41, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{300, 100, 400, 300}}},
		"40/0": {Town: 40, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}},
	}}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}

	// 从阿法利亚（41/1）进来，戳 = 41/1 (404,197)；客户端报的是同镇通用落点 38/0
	// (746,157)。旧的「请求目的地 == 戳」守卫在这里不进分支。
	inside := WorldPosition{Town: 38, Area: 1, X: 550, Y: 340, Return: &WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}}
	out, e := s.Transition(30, false, inside, protocol.AreaChangeRequest{
		Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1,
	})
	if e != nil {
		t.Fatal(e)
	}
	if out.Town != 41 || out.Area != 1 || out.X != 404 || out.Y != 197 {
		t.Fatalf("leave did not land on the stamped origin: %+v", out)
	}
	if out.Return != nil {
		t.Fatal("return stamp survived the leave")
	}

	// 客户端报另一个镇也一样：目的地仍然是戳，不是请求里的镇。
	inside = WorldPosition{Town: 38, Area: 1, X: 550, Y: 340, Return: &WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}}
	out, e = s.Transition(30, false, inside, protocol.AreaChangeRequest{
		Town: 40, Area: 0, X: 388, Y: 180, PreviousTown: 38, PreviousArea: 1,
	})
	if e != nil {
		t.Fatal(e)
	}
	if out.Town != 41 || out.Area != 1 || out.X != 404 || out.Y != 197 {
		t.Fatalf("leave followed the client's town instead of the stamp: %+v", out)
	}

	// 站在出门光圈之外仍然不放行：ReturnWarpBounds 的站位判定没有被放宽。
	far := WorldPosition{Town: 38, Area: 1, X: 410, Y: 200, Return: &WorldReturn{Town: 41, Area: 1, X: 404, Y: 197}}
	if _, e = s.Transition(30, false, far, protocol.AreaChangeRequest{
		Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1,
	}); e == nil {
		t.Fatal("leave outside the return gate was admitted")
	}
}

// Return 戳机制不区分来源镇：西海岸 / 阿法利亚 / 亨顿 / 艾尔文各来一例，出门都回
// 各自来图；而且改写的必须是 Town/Area/X/Y 四元组，不能只改坐标。
func TestSeriaReturnIsSourceAgnostic(t *testing.T) {
	origins := []WorldReturn{
		{Town: 40, Area: 0, X: 400, Y: 200},
		{Town: 41, Area: 1, X: 404, Y: 197},
		{Town: 38, Area: 3, X: 500, Y: 250},
		{Town: 39, Area: 2, X: 320, Y: 306},
	}
	areas := map[string]catalog.WorldArea{
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"38/0": {Town: 38, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}},
	}
	for _, o := range origins {
		if o.Town == 38 && o.Area == 1 {
			continue
		}
		areas[catalog.AreaKey(o.Town, o.Area)] = catalog.WorldArea{
			Town: o.Town, Area: o.Area, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 2000, 800}},
		}
	}
	s := Service{Catalog: catalog.WorldCatalog{Areas: areas}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	for _, o := range origins {
		stamp := o
		inside := WorldPosition{Town: 38, Area: 1, X: 550, Y: 340, Return: &stamp}
		// 客户端报的永远是"本镇通用落点"，与戳无关。
		out, e := s.Transition(30, false, inside, protocol.AreaChangeRequest{
			Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1,
		})
		if e != nil {
			t.Fatalf("origin %d/%d: %v", o.Town, o.Area, e)
		}
		if out.Town != o.Town || out.Area != o.Area || out.X != o.X || out.Y != o.Y {
			t.Fatalf("origin %d/%d not restored: %+v", o.Town, o.Area, out)
		}
	}
}
