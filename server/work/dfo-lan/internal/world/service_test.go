package world

import (
	"dfolan/internal/catalog"
	"dfolan/internal/game/protocol"
	"dfolan/internal/storage"
	"errors"
	"testing"
)

func TestTransitionAuthority(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/0": {Town: 38, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}, Portals: []catalog.Portal{{Bounds: [4]int32{500, 200, 80, 80}, Town: 38, Area: 1}, {Town: 38, Area: 3}}},
		"38/1": {Town: 38, Area: 1, MinimumLevel: 1, Walkable: [][4]int32{{400, 100, 250, 300}}, SeriaReturnWarp: true, ReturnWarpBounds: [][4]int32{{470, 320, 150, 40}}},
		"38/3": {Town: 38, Area: 3, MinimumLevel: 14, Walkable: [][4]int32{{0, 0, 1000, 500}}},
	}}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	old := storage.WorldPosition{Town: 38, Area: 0, X: 550, Y: 230}
	req := protocol.AreaChangeRequest{Town: 38, Area: 1, X: 544, Y: 311, PreviousTown: 38, PreviousArea: 0}
	next, e := s.Transition(false, 1, old, req)
	if e != nil || next.Return == nil {
		t.Fatalf("valid entry: %+v %v", next, e)
	}
	req.Area = 3
	if _, e = s.Transition(false, 1, old, req); !errors.Is(e, ErrLevel) {
		t.Fatal("level gate bypass", e)
	}
	req.Area = 1
	req.X = 65535
	if _, e = s.Transition(false, 1, old, req); e == nil {
		t.Fatal("outside coordinates accepted")
	}
	req.X = 544
	req.PreviousTown = 39
	if _, e = s.Transition(false, 1, old, req); e == nil {
		t.Fatal("stale origin accepted")
	}
	req.PreviousTown = 38
	old.X = 10
	if _, e = s.Transition(false, 1, old, req); e == nil {
		t.Fatal("remote portal bypass")
	}
	next.X = 550
	next.Y = 340
	back := protocol.AreaChangeRequest{Town: 38, Area: 0, X: 561, Y: 234, PreviousTown: 38, PreviousArea: 1}
	returned, e := s.Transition(false, 1, next, back)
	if e != nil || returned.Return != nil {
		t.Fatalf("return warp: %+v %v", returned, e)
	}
	back.Area = 3
	if _, e = s.Transition(false, 20, next, back); e == nil {
		t.Fatal("return warp arbitrary destination")
	}
}

func TestSourceSeriaRoundTrip(t *testing.T) {
	cat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	s := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	start := storage.WorldPosition{Town: 38, Area: 0, X: 622, Y: 196}
	inside, e := s.Transition(false, 1, start, protocol.AreaChangeRequest{Town: 38, Area: 1, X: 544, Y: 311, PreviousTown: 38, PreviousArea: 0})
	if e != nil {
		t.Fatal(e)
	}
	inside.Y = 340
	// Captured map-selector request supplies a non-walkable generic landing
	// point. Only an authorized Seria return resolves to the saved origin.
	out, e := s.Transition(false, 1, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, X: 746, Y: 157, PreviousTown: 38, PreviousArea: 1})
	if e != nil || out.Return != nil || out.Town != start.Town || out.Area != start.Area {
		t.Fatalf("source return warp failed: %+v %v", out, e)
	}
	if out.X != start.X || out.Y != start.Y {
		t.Fatal("saved return coordinates lost", out)
	}
	inside.Return.X = 65535
	if _, e = s.Transition(false, 1, inside, protocol.AreaChangeRequest{Town: 38, Area: 0, X: 561, Y: 234, PreviousTown: 38, PreviousArea: 1}); e == nil {
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
	at := storage.WorldPosition{Town: 38, Area: 3, X: 60, Y: 260}
	req := protocol.AreaChangeRequest{Town: 38, Area: 7, X: 100, Y: 260, PreviousTown: 38, PreviousArea: 3}
	next, e := s.Transition(false, 20, at, req)
	if e != nil || next.Town != 38 || next.Area != 7 {
		t.Fatalf("quest-gated Elvenmere portal refused: %+v %v", next, e)
	}
	if _, e = s.Transition(false, 20, storage.WorldPosition{Town: 38, Area: 3, X: 900, Y: 300}, req); e == nil {
		t.Fatal("remote portal bypass")
	}
	if _, e = s.Transition(false, 16, at, req); e == nil {
		t.Fatal("minimum level 17 not enforced")
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
	at := storage.WorldPosition{Town: 89, Area: 0, X: 985, Y: 416}
	req := protocol.AreaChangeRequest{
		Town: 89, Area: 2, X: 65534, Y: 309, // X = -2
		PreviousTown: 89, PreviousArea: 0,
	}
	next, e := s.Transition(false, 20, at, req)
	if e != nil || next.Town != 89 || next.Area != 2 || next.X != 65534 || next.Y != 309 {
		t.Fatalf("transition to 89/2 with negative X failed: %+v %v", next, e)
	}

	// 验证从 89/2 返回 89/0：89/2 的门户矩形为 [-22, 249, 40, 120]（负 X 边界）
	at2 := storage.WorldPosition{Town: 89, Area: 2, X: 65534, Y: 300} // X = -2
	reqReturn := protocol.AreaChangeRequest{
		Town: 89, Area: 0, X: 950, Y: 400,
		PreviousTown: 89, PreviousArea: 2,
	}
	sProximity := Service{Catalog: cat, Rules: Rules{RequirePortalProximity: true, PortalMargin: 32}}
	next0, e := sProximity.Transition(false, 20, at2, reqReturn)
	if e != nil || next0.Town != 89 || next0.Area != 0 {
		t.Fatalf("return to 89/0 from negative portal bounds failed: %+v %v", next0, e)
	}

	// 超出负边界的坐标必须被拒绝
	oobX := int16(-500)
	reqOOB := protocol.AreaChangeRequest{
		Town: 89, Area: 2, X: uint16(oobX), Y: 309,
		PreviousTown: 89, PreviousArea: 0,
	}
	if _, e = s.Transition(false, 20, at, reqOOB); e == nil {
		t.Fatal("out-of-bounds negative X accepted")
	}
}

// TestTransitionOdysseyLevelGate 覆盖 [need level] 与 [odyssey enter level]
// 并存的双标签区域（风云径 43/*：need 50 / odyssey 45）的模式门禁：
// 奥德赛角色走 odyssey 门槛、剧情角色走 need 门槛；未标注 odyssey 标签的
// 区域两种模式同用 need。
func TestTransitionOdysseyLevelGate(t *testing.T) {
	s := Service{Catalog: catalog.WorldCatalog{Areas: map[string]catalog.WorldArea{
		"38/0": {Town: 38, Area: 0, MinimumLevel: 1, Walkable: [][4]int32{{0, 0, 1000, 500}}, Portals: []catalog.Portal{{Bounds: [4]int32{500, 200, 80, 80}, Town: 43, Area: 1}, {Bounds: [4]int32{500, 200, 80, 80}, Town: 38, Area: 2}}},
		"43/1": {Town: 43, Area: 1, MinimumLevel: 50, OdysseyEnterLevel: 45, Walkable: [][4]int32{{0, 0, 1000, 500}}},
		"38/2": {Town: 38, Area: 2, MinimumLevel: 17, Walkable: [][4]int32{{0, 0, 1000, 500}}},
	}}, Rules: Rules{RequirePortalProximity: true, PortalMargin: 10}}
	at := storage.WorldPosition{Town: 38, Area: 0, X: 550, Y: 230}
	storm := protocol.AreaChangeRequest{Town: 43, Area: 1, X: 100, Y: 100, PreviousTown: 38, PreviousArea: 0}
	if _, e := s.Transition(true, 44, at, storm); !errors.Is(e, ErrLevel) {
		t.Fatal("odyssey 44 into dual-tag (need 50 / odyssey 45):", e)
	}
	if _, e := s.Transition(true, 45, at, storm); e != nil {
		t.Fatal("odyssey 45 into dual-tag must pass:", e)
	}
	if _, e := s.Transition(false, 45, at, storm); !errors.Is(e, ErrLevel) {
		t.Fatal("non-odyssey 45 into dual-tag must use need 50:", e)
	}
	if _, e := s.Transition(false, 50, at, storm); e != nil {
		t.Fatal("non-odyssey 50 into dual-tag must pass:", e)
	}
	plain := protocol.AreaChangeRequest{Town: 38, Area: 2, X: 100, Y: 100, PreviousTown: 38, PreviousArea: 0}
	if _, e := s.Transition(true, 16, at, plain); !errors.Is(e, ErrLevel) {
		t.Fatal("odyssey 16 into untagged 17-level area:", e)
	}
	if _, e := s.Transition(true, 17, at, plain); e != nil {
		t.Fatal("odyssey 17 into untagged 17-level area must pass:", e)
	}
}
