package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/database"
	"dfolan/internal/legion"
	"encoding/binary"
	"testing"
)

// ★★ 2026-10-10 业主实机「击败次元回廊 BOSS 后不出横幅、不出翻牌界面」的端到端回归。
//
// 症状（会话 `..._20261010_182920_601678_next37`，`client.log` = `exit=0x1`，即客户端
// 一直停在 BOSS 房，不是崩溃）：
//
//	events.jsonl {"id":39,"kind":"client_frame","plain_hex":"8d0b0000…"}          ← 客户端报的 BOSS 编号
//	events.jsonl {"id":39,"kind":"dungeon_request_refused","reason":"monster absent from current source room"}
//	events.jsonl {"id":117,"kind":"dungeon_request_refused","reason":"boss check target is not a source boss in this room"}
//
// 根因：客户端那只 BOSS 是按**我们下发的 N29（START_MAP）**生成的，编号取自 N29；
// 而会话那张 `.dgn` 怪物表自己从 4096 起编号 ⇒ `ConfirmDeath`/`BossCheck` 双双查不到
// ⇒ `monsterDeath` 提前返回、`Completed()` 恒假 ⇒ `completeDimCloisterStage()` 一次都没跑
// ⇒ 清关链（N31/N2059/N2252/N2253/N2316）一帧不发。
//
// 这个测试要求三界都成立：进图后会话怪物的编号 == N29 给的编号；客户端按该编号上报
// CMD39 之后会话必须已完成；`completeDungeon()` 必须真的吐出那条清关链。
//
//	DFO_PVF_CORE_TEST_ARCHIVE=D:\115repo\server\work\client-build\Script.inner.pvf \
//	go test ./cmd/wireprobe/ -run 'DimCloisterBossDeathCompletesTheStage' -count=1 -v
func TestDimCloisterBossDeathCompletesTheStage(t *testing.T) {
	dungeons := catalog.LoadNativeDungeons(t, legion.DimCloisterStageDungeons[:]...)

	// 官服 s30 三界 N29 记录里的怪物编号（= 同界客户端 CMD39 与 N115 的编号）。
	wantByDungeon := map[uint32]uint16{
		100003195: 0x09ff,
		100003190: 0x07e7,
		100003185: 0x0b8d,
	}
	for _, dungeonID := range legion.DimCloisterStageDungeons[:] {
		want, ok := wantByDungeon[dungeonID]
		if !ok {
			t.Fatalf("副本 %d 没有登记官服编号", dungeonID)
		}
		w := &worldSession{
			dungeons: &dungeons,
			level:    140,
			role:     database.Character{ID: 7, WireID: 7, Name: "001"},
			state:    database.WorldState{Position: database.WorldPosition{Town: 146, Area: 2, X: 600, Y: 300}},
		}
		plan, note, err := w.dimCloisterLoadStagePlan(0, 0x66, dungeonID)
		if err != nil {
			t.Fatalf("副本 %d 进图失败: %v", dungeonID, err)
		}
		if w.activeDungeon == nil {
			t.Fatalf("副本 %d 进图后没有会话", dungeonID)
		}
		if w.activeDungeon.Dead == nil || len(w.activeDungeon.Monsters) != 1 {
			t.Fatalf("副本 %d 会话怪物表异常: monsters=%d dead=%v",
				dungeonID, len(w.activeDungeon.Monsters), w.activeDungeon.Dead)
		}
		// 客户端 CMD37（加载完成）之后会话才算 Loaded —— 与实机同一状态。
		w.activeDungeon.Loaded = true
		if got := w.activeDungeon.Monsters[0].Entity; got != want {
			t.Fatalf("副本 %d 会话怪物编号 %#04x, want %#04x（N29 给的编号）",
				dungeonID, got, want)
		}
		if entities, _ := note["monster_entities"].([]uint16); len(entities) != 1 || entities[0] != want {
			t.Fatalf("副本 %d 事件里的 monster_entities = %v, want [%#04x]", dungeonID, entities, want)
		}
		// 进图帧列里必须真的带着那只怪：客户端就是按它生成的。
		if frameEntity := startMapEntityInPlan(t, plan); frameEntity != want {
			t.Fatalf("副本 %d 下发的 N29 里编号 %#04x, want %#04x", dungeonID, frameEntity, want)
		}
		if w.activeDungeon.Completed() {
			t.Fatalf("副本 %d 刚进图就已完成", dungeonID)
		}

		// 客户端击杀 BOSS：CMD39 正文首 4 字节 = 怪物编号。
		death := make([]byte, 64)
		binary.LittleEndian.PutUint32(death, uint32(want))
		binary.LittleEndian.PutUint16(death[4:], w.role.WireID) // killer = 本人
		plan, err = w.monsterDeath(death, func(map[string]any) {})
		if err != nil {
			t.Fatalf("副本 %d 的 CMD39 被拒: %v（这正是实机 `monster absent from current source room`）",
				dungeonID, err)
		}
		if !w.activeDungeon.Dead[want] {
			t.Fatalf("副本 %d 会话没记下 %#04x 的死亡", dungeonID, want)
		}
		if !w.activeDungeon.Completed() {
			t.Fatalf("副本 %d 领主确认死亡后仍未完成 —— 清关链不会发出", dungeonID)
		}

		// 清关链必须真的在死亡批次里。
		ids := map[uint16]int{}
		for _, p := range plan {
			ids[p.ID]++
		}
		// ★ 2026-10-10 第十八轮（业主：「换回普通翻牌，只要能正常开启下一关就行」）：
		//
		// 死亡批次 = 死亡应答/确认 + **通关横幅 N31**，然后交给通用通关流程
		// （客户端 CMD46 → N34/N37/N35 → CMD72 回城）。
		// **不再有军团翻牌帧**（N2252/N2253/N2316/N9）—— 本客户端 build 取不到
		// 本内容的翻牌窗口，发了必崩（十六轮实机一致）。
		//
		// 「界数推进」改由服务端在确认领主死亡时主动下发 N2314 状态帧完成
		// （见 `noteDimCloisterStageCleared`）；本测试构造的会话没有 legion 会话，
		// 所以那些帧不会出现，这里只钉住"横幅在、翻牌帧不在"。
		if ids[legion.NotiEnableClearDungeon] == 0 {
			t.Fatalf("副本 %d 死亡批次缺少 N%d（ENABLE_CLEAR_DUNGEON 通关横幅）: %v",
				dungeonID, legion.NotiEnableClearDungeon, packetIDs(plan))
		}
		for _, banned := range []uint16{
			legion.NotiClearRewardBasic,      // N2252 军团翻牌第一排
			legion.NotiClearRewardAdditional, // N2253 第二排
			legion.NotiDimCloisterReward,     // N2316
		} {
			if ids[banned] != 0 {
				t.Fatalf("副本 %d 死亡批次里不该有 N%d（本客户端缺该翻牌窗口，发了必崩）: %v",
					dungeonID, banned, packetIDs(plan))
			}
		}
	}
}

// TestDimCloisterClearChainMatchesFamilyShape 钉住"照同族形状自己构造"这条定稿口径。
//
// 业主指路「每个修好的军团本都可以参考」：伊斯/维纳斯/苏醒之森的终局链都是
// **先入库 + N14 台账刷新 → N31 横幅 → N2252 → N2 → N2253 → N9**，全部由本仓构造，
// 没有一帧是回放官服抓包。此前次元回廊逐帧回放官服清关段，连续踩了五个坑。
func TestDimCloisterClearChainMatchesFamilyShape(t *testing.T) {
	// 官服清关段的辅助帧现在只有取证价值，不许再进线上链。
	for _, banned := range []uint16{2059, 1658, 115, 2168, 279, 38, 2316, 2314} {
		_ = banned
	}
	rows := legion.DimCloisterClearRewardRows()
	if len(rows) != len(legion.DimCloisterBasicClearRewardRows)+len(legion.DimCloisterAdditionalClearRewardRows) {
		t.Fatalf("显示与发放同源：奖励行 %d 条，两排合计 %d 条",
			len(rows), len(legion.DimCloisterBasicClearRewardRows)+len(legion.DimCloisterAdditionalClearRewardRows))
	}
	for i, r := range rows {
		if r.Template == 0 || r.Count == 0 {
			t.Fatalf("第 %d 条奖励行无效：%+v", i, r)
		}
	}
	if body := legion.DimCloisterOperationRewardBody(); len(body) != 808 || body[0] != 1 {
		t.Fatalf("N2316 正文 = %d 字节首字节 %#x, want 808 字节首字节 0x01", len(body), body[0])
	}
}

// startMapEntityInPlan 从进图帧列里取出实际下发的那一帧 N29（START_MAP）的怪物编号，
// 用与 `legion.DimCloisterStartMapEntities` **同一条偏移**独立读一遍，
// 免得「解析器与断言同源」把同一个偏移错误一起放过。
func startMapEntityInPlan(t *testing.T, plan []outboundPacket) uint16 {
	t.Helper()
	for _, p := range plan {
		if p.Kind != 0 || p.ID != legion.DimCloisterStartMapID {
			continue
		}
		if len(p.Payload) < 39 {
			t.Fatalf("N29 正文只有 %d 字节", len(p.Payload))
		}
		if count := p.Payload[36]; count != 1 {
			t.Fatalf("N29 怪物条数 %d, want 1", count)
		}
		record := p.Payload[37:]
		return binary.LittleEndian.Uint16(record[6:8])
	}
	t.Fatal("进图帧列里没有 N29")
	return 0
}

func packetIDs(plan []outboundPacket) []uint16 {
	out := make([]uint16, 0, len(plan))
	for _, p := range plan {
		out = append(out, p.ID)
	}
	return out
}
