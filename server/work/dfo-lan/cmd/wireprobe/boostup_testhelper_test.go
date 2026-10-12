package main

import (
	"context"
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"dfolan/internal/loot"
	"dfolan/internal/world"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"
)

// must115 是本树测试的统一致命错误断言（donor 基线的同名助手不在补丁里）。
func must115(t *testing.T, e error) {
	t.Helper()
	if e != nil {
		t.Fatal(e)
	}
}

// countPacket 统计计划里指定 NOTI/CMD id 的帧数（donor 基线测试助手，本树补回）。
func countPacket(plan []outboundPacket, id uint16) int {
	var n int
	for _, p := range plan {
		if p.ID == id {
			n++
		}
	}
	return n
}

// boostReloadCharacter 从库里重读一个角色的最新存档。donor 基线的
// store.CurrentOwnedCharacter 不在本树 storage 门面里，这里用等价的按账号列表取回，
// 保证断言读的是提交后的存档而不是内存里的 w.role。
func boostReloadCharacter(t *testing.T, ctx context.Context, store *database.Store, account, id int64) database.Character {
	t.Helper()
	roles, e := store.Characters(ctx, account)
	must115(t, e)
	for _, cur := range roles {
		if cur.ID == id {
			return cur
		}
	}
	t.Fatalf("character %d not persisted", id)
	return database.Character{}
}

// Current native source; no retired JSON exports or player database are needed.
func boostFixtureArchive(t *testing.T) *pvf.Archive {
	t.Helper()
	path := os.Getenv("DFO_PVF_CORE_TEST_ARCHIVE")
	if path == "" {
		t.Skip("set DFO_PVF_CORE_TEST_ARCHIVE for native boost fixture")
	}
	a, err := catalog.OpenTestArchiveCached(path, os.Getenv("DFO_PVF_CORE_TEST_SHA256"))
	must115(t, err)
	t.Cleanup(func() { must115(t, a.Close()) })
	return a
}

// deathBody 组装一条最小可解码的怪物死亡请求（与实机捕获同布局，见
// protocol.DecodeMonsterDeath）：u32 entity@0、u16 killer@4、参与人数@30=0，
// 正文补足 64 字节、8 对齐。只用于测试，不带任何真实账号数据。
func deathBody(entity uint32, killer uint16) []byte {
	p := make([]byte, 64)
	binary.LittleEndian.PutUint32(p[0:], entity)
	binary.LittleEndian.PutUint16(p[4:], killer)
	return p
}

// [GAP] writePartyPackets：donor 基线里这是队伍包的落盘诊断助手，其确切语义
// 不在补丁正文；本树没有对应实现可抄。这里提供环境变量门控的诊断落盘，
// 不影响任何断言。
func writePartyPackets(t *testing.T, plan []outboundPacket) {
	t.Helper()
	dir := os.Getenv("US115_TEST_BOOST_PARTY_DUMP")
	if dir == "" {
		return
	}
	for i, p := range plan {
		path := fmt.Sprintf("%s/plan-%02d-%d.bin", dir, i, p.ID)
		must115(t, os.WriteFile(path, p.Payload, 0600))
	}
}

// moonWorlds 构造 n 个「满月湖事件城镇」的完整 worldSession（donor 基线测试夹具，
// 本树按当前 PVF 重建）：角色目录 + 规则 + 副本目录 + 世界目录 + 临时 SQLite。
func moonWorlds(t *testing.T, n int) []*worldSession {
	t.Helper()
	a := boostFixtureArchive(t)
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	fixture, e := database.OpenTestFixture(ctx)
	must115(t, e)
	t.Cleanup(func() {
		if err := fixture.Close(); err != nil {
			t.Error(err)
		}
	})
	store := fixture.Storage()
	must115(t, store.Migrate(ctx))
	must115(t, store.MigrateCharacterEvents(ctx))
	must115(t, store.MigrateWorld(ctx))

	professions, e := catalog.ImportCharacters(a)
	must115(t, e)
	rulesRaw, e := os.ReadFile("../../configs/character-probe.json")
	must115(t, e)
	var charRules character.Rules
	must115(t, json.Unmarshal(rulesRaw, &charRules))
	characters, e := character.New(store, professions, charRules)
	must115(t, e)

	worldCatalog, e := catalog.ImportWorldRuntime(a)
	must115(t, e)
	rulesRaw, e = os.ReadFile("../../configs/world-probe.json")
	must115(t, e)
	var worldRules world.Rules
	must115(t, json.Unmarshal(rulesRaw, &worldRules))
	worldService := &world.Service{Store: store, Catalog: worldCatalog, Rules: worldRules}

	// These tests exercise the existing map engine through the lake guide.
	dungeons, e := catalog.ImportDungeons(a, []uint32{100004136})
	must115(t, e)

	account, e := store.DevelopmentAccount(ctx, "moon-fixture")
	must115(t, e)
	out := make([]*worldSession, 0, n)
	for i := 0; i < n; i++ {
		prof := professions.Professions[0]
		state, e := json.Marshal(character.State{Level: 115, Attributes: prof.InitialAttributes, AllJobsPilot: true, SourcePath: prof.Path, SourceSHA256: prof.RawSHA256, InitialSkills: prof.InitialSkills})
		must115(t, e)
		raw, e := inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1"})
		must115(t, e)
		role, e := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: fmt.Sprintf("MoonFixture%d", i), Profession: 0, Request: []byte{0}, ConfigVersion: professions.Source.SaveIdentity(), State: raw}, 100)
		must115(t, e)
		ls := &loot.Service{Catalog: catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: professions.Source.Checksum}}}
		out = append(out, &worldSession{
			characters: characters, service: worldService, store: store, account: account,
			role: role, level: 115, state: database.WorldState{Position: database.WorldPosition{Town: 222, Area: 0, X: 200, Y: 200}},
			dungeons: &dungeons, tutorialDungeons: &dungeons, professions: characters.Catalog, loot: ls,
		})
	}
	return out
}
