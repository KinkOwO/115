package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"dfolan/internal/database"
	"dfolan/internal/world"
	"encoding/hex"
	"encoding/json"
	"testing"
)

func TestBoostGraduationCapturedReturnUsesOwnedOrigin(t *testing.T) {
	// Actual C-0949/36 after final C680 in the 2026-09-23 capture:
	// 222/10 -> 38/1. Only the 21-byte business body is used; no actor IDs,
	// credentials, transport checksums or other player's saved state.
	body, e := hex.DecodeString("26000000010000002002370105DE0000000A00000000")
	must115(t, e)
	r, e := protocol.DecodeAreaChangeRequest(body)
	must115(t, e)
	if r.Town != 38 || r.Area != 1 || r.PreviousTown != 222 || r.PreviousArea != 10 {
		t.Fatal(r)
	}
	origin := database.WorldPosition{Town: 38, Area: 1, X: 210, Y: 220}
	originJSON, e := json.Marshal(origin)
	must115(t, e)
	st := boostup.State{Version: 1, Activated: true, Origin: originJSON, Training: boostup.Training{Step: 12, Finished: true}}
	raw, e := boostup.WriteState(json.RawMessage(`{}`), st)
	must115(t, e)
	base := &world.Service{Catalog: boostTestWorldCatalog("test")}
	w := &worldSession{boostup: &boostup.Catalog{Town: 222}, service: base, boostWorldBase: base, level: 115, role: database.Character{State: raw}, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 10}}}
	next, handled, e := w.boostAreaTransition(r)
	must115(t, e)
	if !handled || next != origin {
		t.Fatal("graduation copied captured coordinates or failed to exit", next)
	}
	bad := r
	bad.Town = 39
	if _, _, e = w.boostAreaTransition(bad); e == nil {
		t.Fatal("graduation allows arbitrary destination")
	}
	bad = r
	bad.PreviousArea = 9
	if _, _, e = w.boostAreaTransition(bad); e == nil {
		t.Fatal("stale previous area accepted")
	}
	st.Training.Finished = false
	w.role.State, e = boostup.WriteState(raw, st)
	must115(t, e)
	if _, _, e = w.boostAreaTransition(r); e == nil {
		t.Fatal("unearned graduation accepted")
	}
}

func TestBoostTownAllSourceStepsHaveScopedPermissionAndAuthoredSpawn(t *testing.T) {
	boostFixtureBaselines(t, "../../configs/world.generated.json")
	wc, e := catalog.LoadWorld("../../configs/world.generated.json")
	must115(t, e)
	base := &world.Service{Catalog: wc}
	c := &boostup.Catalog{Town: 222}
	for i := 0; i < 11; i++ {
		c.Steps = append(c.Steps, boostup.Step{Number: byte(i + 1), Area: byte(i)})
	}
	for step := byte(1); step <= 11; step++ {
		st := boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: step}}
		scoped, p, e := boostWorldTarget(base, c, st, 115, true)
		must115(t, e)
		if p.Town != 222 || p.Area != uint32(step-1) {
			t.Fatal("wrong source destination")
		}
		// donor 基线里 base.ValidatePosition 会因 unsupported 事件权限拒绝本 step
		// 区域；本树刻意不这么做（world/service.go：unsupported permission 由客户端
		// 判定，当不可进入会让整片地图打不开）。逐 step 授权的真实门禁在
		// boostWorldTarget 的 [event id]/[condition] 源解析，这里改断言其等价物：
		// 共享 base 目录未被 scoped 克隆改写，且跨 step 授权必须被源解析拒绝。
		if len(base.Catalog.Areas[catalog.AreaKey(222, uint32(step-1))].Pending) == 0 {
			t.Fatal("scoped clone leaked into the shared base catalog")
		}
		if scoped == base {
			t.Fatal("step authorization mutated the shared service")
		}
		if scoped.ValidatePosition(115, false, p) != nil {
			t.Fatal("owned step rejected")
		}
		cross := &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: step, Area: step % 11}}}
		if _, _, e := boostWorldTarget(base, cross, st, 115, true); e == nil {
			t.Fatal("current step granted a foreign source area")
		}
		st.Activated = false
		if _, _, e = boostWorldTarget(base, c, st, 115, true); e == nil {
			t.Fatal("inactive role entered training")
		}
	}
}

func TestBoostC36UsesCurrentStepNotArbitraryDestination(t *testing.T) {
	wc := boostTestWorldCatalog("test")
	base := &world.Service{Catalog: wc}
	c := &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1, Area: 0}, {Number: 2, Area: 1}}}
	raw, e := boostup.WriteState(json.RawMessage(`{}`), boostup.State{Version: 1, Activated: true, Training: boostup.Training{Step: 2}})
	must115(t, e)
	w := &worldSession{service: base, boostup: c, level: 115, role: database.Character{State: raw}, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 0, X: 200, Y: 200}}}
	r := protocol.AreaChangeRequest{Town: 222, Area: 1, X: 200, Y: 200, PreviousTown: 222, PreviousArea: 0, Flag: 5}
	next, e := w.areaTransition(r)
	must115(t, e)
	// 同上：本树 ValidatePosition 不把事件权限当全局门禁；C36 的逐步授权
	// 由 boostAreaTransition 的 current-step target 校验保证。这里断言共享
	// base 目录未被克隆改写。
	if next.Area != 1 || len(base.Catalog.Areas[catalog.AreaKey(222, 1)].Pending) == 0 {
		t.Fatal("step route leaked into the shared base catalog")
	}
	r.Area = 0
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("old/unowned target entered")
	}
	r.Area = 1
	r.PreviousArea = 3
	if _, e = w.areaTransition(r); e == nil {
		t.Fatal("stale transition entered")
	}
}

func boostTestWorldCatalog(source string) catalog.WorldCatalog {
	wc := catalog.WorldCatalog{Source: pvf.ArchiveSnapshot{Checksum: source}, Areas: map[string]catalog.WorldArea{
		"38/1": {Town: 38, Area: 1, Kind: "[normal]", Walkable: [][4]int32{{100, 100, 1000, 1000}}},
	}}
	for i := 0; i < 2; i++ {
		area := catalog.WorldArea{Town: 222, Area: uint32(i), Kind: "[gate]", Walkable: [][4]int32{{100, 100, 1000, 1000}},
			Pending:    []string{"unsupported permission [check event condition]", "unsupported permission [event id]", "unsupported permission [condition]", "unsupported permission [/check event condition]"},
			Definition: []pvf.Token{{Type: 3, Text: "[check event condition]"}, {Type: 3, Text: "[event id]"}, {Type: 0, Value: 662}, {Type: 3, Text: "[condition]"}, {Type: 0, Value: int32(i + 1)}, {Type: 3, Text: "[/check event condition]"}, {Type: 6, Text: "[gate]"}, {Type: 0, Value: 200}, {Type: 0, Value: 200}}}
		wc.Areas[catalog.AreaKey(222, uint32(i))] = area
	}
	return wc
}

// Called within the existing random-schema transaction fixture.
func assertBoostWorldSQL(t *testing.T, ctx context.Context, store *database.Store, role database.Character) {
	t.Helper()
	must115(t, store.MigrateWorld(ctx))
	base := &world.Service{Store: store, Catalog: boostTestWorldCatalog(role.ConfigVersion)}
	c := &boostup.Catalog{Town: 222, Steps: []boostup.Step{{Number: 1, Area: 0}, {Number: 2, Area: 1}}}
	origin := database.WorldPosition{Town: 38, Area: 1, X: 200, Y: 200}
	w := &worldSession{service: base, account: role.AccountID, boostup: c}
	must115(t, w.enter(role, origin))
	if w.state.Position.Town != 222 || w.state.Position.Area != 0 {
		t.Fatal("activation did not enter source event area")
	}
	oldRevision := w.state.Revision
	again := &worldSession{service: base, account: role.AccountID, boostup: c}
	must115(t, again.enter(role, origin))
	if again.state.Revision != oldRevision {
		t.Fatal("relogin rewrote identical event position")
	}
	// Simulate a completed owned step committed by the training domain; only
	// this role's scoped permission changes, never the shared base catalog.
	event, e := boostup.ReadState(role.State)
	must115(t, e)
	event.Training.Step = 2
	raw, e := boostup.WriteState(role.State, event)
	must115(t, e)
	role, _, e = store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "fixture:step2", "test", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	must115(t, e)
	must115(t, again.enter(role, origin))
	// 本树 ValidatePosition 对 unsupported 事件权限放行（见上方两处同因适配）；
	// SQL 夹具改断言 step 推进正确 + 共享 base 目录的事件权限记录未被改写。
	if again.state.Position.Area != 1 || len(base.Catalog.Areas[catalog.AreaKey(222, 1)].Pending) == 0 {
		t.Fatal("step advance wrong or base catalog mutated")
	}
	// Graduation returns once; future ordinary moves are not reset to origin.
	event.Training.Finished = true
	event.Training.Step = 3
	raw, e = boostup.WriteState(role.State, event)
	must115(t, e)
	role, _, e = store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion, "fixture:graduate", "test", func(database.Character) (json.RawMessage, json.RawMessage, error) {
		return raw, json.RawMessage(`{}`), nil
	})
	must115(t, e)
	saved, handled, e := again.enterBoostWorld(ctx, role, 115, origin)
	must115(t, e)
	if !handled || saved.Position.Town != 38 {
		t.Fatal("graduation return missing")
	}
	moved := origin
	moved.X = 400
	_, e = store.SaveWorld(ctx, role.AccountID, role.ID, 0, saved, moved)
	must115(t, e)
	_, handled, e = again.enterBoostWorld(ctx, role, 115, origin)
	must115(t, e)
	if handled {
		t.Fatal("finished event stole ordinary world entry")
	}
}
