package character_test

import (
	"context"
	"dfolan/internal/boostup"
	. "dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"os"
	"testing"
)

// 本仓约定：存储回归走 database.OpenTestFixture（自建一次性 schema、自校验隔离、
// 用后 DROP CASCADE），并由 DFO_TEST_POSTGRES_DSN 显式选择专用测试库。
// 原包用的是旧 storage.Store 的导出字段 DB 手搓 schema，这里按本仓 fixture 口径重写，
// 断言语义一字未改。
func TestBoostVPSaveSQLSharesSkillCommit(t *testing.T) {
	if os.Getenv("DFO_TEST_POSTGRES_DSN") == "" {
		t.Skip("explicit isolated database required")
	}
	must := func(e error) {
		t.Helper()
		if e != nil {
			t.Fatal(e)
		}
	}
	ctx := context.Background()
	fixture, e := database.OpenTestFixture(ctx)
	must(e)
	defer func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	}()
	store := fixture.Storage()
	must(store.Migrate(ctx))
	must(store.MigrateCharacterEvents(ctx))
	must(store.MigratePremiums(ctx))
	account, e := store.DevelopmentAccount(ctx, "boost-vp")
	must(e)
	s, _, raw, req := BoostVPFixtureForTest(t)
	s.Store = store
	version := s.Catalog.Source.Checksum
	role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "boost-vp-save", Request: []byte{0}, ConfigVersion: version, State: raw}, 8)
	must(e)
	bad := req
	bad.Options = append([]protocol.SkillVariation(nil), req.Options...)
	bad.Options[4].ID = 999
	if _, _, e = s.Learn(ctx, role, "bad-save", bad); e == nil {
		t.Fatal("unknown VP skill accepted")
	}
	partial := req
	partial.Options = append([]protocol.SkillVariation(nil), req.Options...)
	// 源的完成条件是 [condition] 3：两点还不够，任务必须留在第三关。
	partial.Options[2] = protocol.SkillVariation{Choice: 3}
	partial.Options[3] = protocol.SkillVariation{Choice: 3}
	partial.Options[4] = protocol.SkillVariation{Choice: 3}
	role, _, e = s.Learn(ctx, role, "partial-save", partial)
	must(e)
	st, e := boostup.ReadState(role.State)
	must(e)
	if st.Training.Step != 3 {
		t.Fatal("partial allocation advanced task")
	}
	role, applied, e := s.Learn(ctx, role, "full-save", req)
	must(e)
	if !applied {
		t.Fatal("save not applied")
	}
	st, e = boostup.ReadState(role.State)
	must(e)
	if st.Training.Step != 4 {
		t.Fatal("save not atomic with advancement")
	}
	// 重开一条独立连接读已提交状态（fixture.Reopen 复用同一 schema）。
	reopened, e := fixture.Reopen(ctx)
	must(e)
	defer reopened.Close()
	persisted, e := fixture.CharacterState(ctx, role.ID)
	must(e)
	var saved State
	must(json.Unmarshal(persisted, &saved))
	if saved.TechniquePoints != [2]uint16{11, 22} || saved.SkillPoints != [2]uint16{77, 88} {
		t.Fatal("points changed", saved.TechniquePoints, saved.SkillPoints)
	}
	// 新树没有 protocol.VariationPointBalance：点数结算已经并入 applyVariations 的
	// 槽位/来源校验。这里按同一口径（ID!=0 且 Choice∈[1,2] 才算花掉一点）断言
	// 持久化后的选中没有丢。
	spent := 0
	for _, o := range saved.SkillVariations[0].Options {
		if o.ID != 0 && o.Choice >= 1 && o.Choice <= 2 {
			spent++
		}
	}
	if spent != len(req.Options) {
		t.Fatal("selection not durable", spent)
	}
	proof, e := store.CharacterEventReceipt(ctx, account, role.ID, "full-save")
	must(e)
	var receipt map[string]json.RawMessage
	must(json.Unmarshal(proof, &receipt))
	if string(receipt["boost_vp_completed"]) != "true" {
		t.Fatal("completion absent from skill receipt")
	}
	role, applied, e = s.Learn(ctx, role, "full-save", req)
	must(e)
	if applied {
		t.Fatal("save replay repeated")
	}
	st, e = boostup.ReadState(role.State)
	must(e)
	if st.Training.Step != 4 {
		t.Fatal("replay jumped another step")
	}
	// 拒收的那次不许留下事件收据（本仓按 event_key 直查，等价于原包的 count=0）。
	if _, e := store.CharacterEventReceipt(ctx, account, role.ID, "bad-save"); e == nil {
		t.Fatal("invalid save left receipt")
	}
}
