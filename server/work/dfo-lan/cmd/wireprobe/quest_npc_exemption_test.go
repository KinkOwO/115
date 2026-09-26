package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/quest"
	"dfolan/internal/storage"
	"dfolan/internal/world"
	"encoding/binary"
	"testing"
)

func sectionCells(d catalog.QuestDefinition, name string) []pvf.Token {
	var out []pvf.Token
	on := false
	for _, c := range d.Script.Cells {
		if c.Type == 3 {
			on = c.Text == name
			continue
		}
		if on {
			out = append(out, c)
		}
	}
	return out
}

// 三个扩展装备槽任务（649/650/2636）的 [alternative npc index] 目标 100001447
// 只在 town139，原始目标 NPC 28 在 town40/area2。它们都是 [sub type] 1 的显式
// 对话任务，客户端据此发出 CMD33，所以服务端不能再按"目标 NPC 在本图"拦。
//
// 本测试不依赖数据库：被豁免的任务会继续走到 MeetNPC（需要连接库），因此这里只
// 断言"地图检查"这一层的判据本身，以及豁免没有放宽到普通对话任务。
func TestQuestNPCCheckExemptsOnlyExplicitDialogueQuests(t *testing.T) {
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	wcat, e := catalog.LoadWorld("../../configs/world.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	qcat, e := catalog.LoadQuests("../../configs/quests.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	world40 := storage.WorldPosition{Town: 40, Area: 2}
	svc := &world.Service{Catalog: wcat}

	// 判据前置事实：原始目标在城镇 40/区域 2 内，替代目标不在。
	earring, ok := qcat.Quests[2636]
	if !ok {
		t.Fatal("source quest 2636 missing")
	}
	alt := sectionCells(earring, "[alternative npc index]")
	if len(alt) < 2 || alt[0].Value != 28 || alt[1].Value != 100001447 {
		t.Fatalf("quest 2636 alternative NPC rule changed: %+v", alt)
	}
	if !quest.AllowsRemoteNPCInteraction(earring) {
		t.Fatal("quest 2636 is no longer exempt from the positional check")
	}
	if !svc.HasNPC(world40, uint32(alt[0].Value)) {
		t.Fatal("the raw objective NPC is no longer in town40/area2: the old check passed for another reason")
	}
	if svc.HasNPC(world40, uint32(alt[1].Value)) {
		t.Fatal("the alternative NPC now stands in town40/area2: the exemption is no longer load-bearing")
	}
	if !svc.HasNPC(storage.WorldPosition{Town: 139, Area: 0}, uint32(alt[1].Value)) &&
		!svc.HasNPC(storage.WorldPosition{Town: 139, Area: 1}, uint32(alt[1].Value)) {
		t.Fatal("the alternative NPC is not in town139 either")
	}

	// 普通对话任务（无可豁免形态）的目标不在本图时，仍然必须被拒。
	refused := 0
	for _, d := range qcat.Quests {
		if d.ID > 65535 || d.Kind != "[meet npc]" || len(d.ObjectiveCells) != 1 || len(d.Pending) != 0 {
			continue
		}
		if quest.AllowsRemoteNPCInteraction(d) {
			continue
		}
		npc := uint32(d.ObjectiveCells[0].Value)
		if npc == 0 || npc == 0xffffffff || svc.HasNPC(world40, npc) {
			continue
		}
		w := &worldSession{
			service: svc,
			quests:  &quest.Service{Catalog: qcat},
			role:    storage.Character{ID: 1, WireID: 3},
			state:   storage.WorldState{Position: world40},
		}
		payload := make([]byte, 16)
		binary.LittleEndian.PutUint16(payload, 33)
		binary.LittleEndian.PutUint16(payload[2:], uint16(d.ID))
		if _, e := w.questInteraction(payload); e == nil {
			t.Fatalf("quest %d: dialogue with NPC %d outside the area was accepted", d.ID, npc)
		}
		refused++
		if refused >= 3 {
			break
		}
	}
	if refused == 0 {
		t.Fatal("no ordinary dialogue quest could be probed: the map check is untested")
	}
}

func TestPreyQuestVisibleNPCUsesObservedBlackMarketInteraction(t *testing.T) {
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	wcat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := qcat.Quests[6200]
	at := storage.WorldPosition{Town: 54, Area: 1}
	if (&world.Service{Catalog: wcat}).HasNPC(at, 8000) {
		t.Fatal("NPC 8000 unexpectedly exists in the static Black Market map")
	}
	if !allowsQuestVisibleNPCInteraction(6200, 8000, at, d, qcat) {
		t.Fatal("observed Prey_01 CMD33 should reach the owned quest check")
	}
	for _, tc := range []struct {
		id  uint16
		npc uint32
		at  storage.WorldPosition
	}{
		{6200, 8000, storage.WorldPosition{Town: 54, Area: 0}},
		{6200, 8000, storage.WorldPosition{Town: 35, Area: 2}},
		{6200, 607, at},
		{6201, 8000, at},
	} {
		if allowsQuestVisibleNPCInteraction(tc.id, tc.npc, tc.at, d, qcat) {
			t.Fatalf("unobserved quest interaction was exempted: %+v", tc)
		}
	}
}

func TestQuestVisibleNPCRelaxSwitch(t *testing.T) {
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	farAway := storage.WorldPosition{Town: 1, Area: 0}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	for _, id := range []uint16{6200, 12411, 12952} {
		d := qcat.Quests[uint32(id)]
		npc := uint32(d.ObjectiveCells[0].Value)
		if !questShowsObjectiveNPCOnAccept(d, npc) {
			t.Fatalf("quest %d no longer has an accept/show objective NPC", id)
		}
		if allowsQuestVisibleNPCInteraction(id, npc, farAway, d, qcat) {
			t.Fatalf("quest %d was relaxed while the switch was off", id)
		}
	}
	if questShowsObjectiveNPCOnAccept(qcat.Quests[12167], 100000319) {
		t.Fatal("hide-on-clear quest was classified as show-on-accept")
	}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "1")
	for _, id := range []uint16{6200, 12411, 12952} {
		d := qcat.Quests[uint32(id)]
		npc := uint32(d.ObjectiveCells[0].Value)
		if !allowsQuestVisibleNPCInteraction(id, npc, farAway, d, qcat) {
			t.Fatalf("quest %d was not relaxed with the switch on", id)
		}
	}
	if allowsQuestVisibleNPCInteraction(12167, 100000319, farAway, qcat.Quests[12167], qcat) {
		t.Fatal("hide-on-clear quest was relaxed")
	}
	if allowsQuestVisibleNPCInteraction(6200, 607, farAway, qcat.Quests[6200], qcat) {
		t.Fatal("wrong objective NPC was relaxed")
	}
}

func TestZasuraRevealedByPrecedingQuest(t *testing.T) {
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := qcat.Quests[6357]
	const npc = 100000175
	if !questShownByPrerequisiteOnClear(d, npc, qcat) {
		t.Fatal("quest 6356 should reveal the objective NPC for 6357 on clear")
	}
	if questShowsObjectiveNPCOnAccept(d, npc) {
		t.Fatal("quest 6357 itself has no accept/show visibility rule")
	}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	market := storage.WorldPosition{Town: 54, Area: 1}
	if !allowsQuestVisibleNPCInteraction(6357, npc, market, d, qcat) {
		t.Fatal("observed Black Market CMD33 remains blocked")
	}
	farAway := storage.WorldPosition{Town: 1, Area: 0}
	if allowsQuestVisibleNPCInteraction(6357, npc, farAway, d, qcat) {
		t.Fatal("quest 6357 was allowed outside the observed area with switch off")
	}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "1")
	if !allowsQuestVisibleNPCInteraction(6357, npc, farAway, d, qcat) {
		t.Fatal("the relaxed predecessor visibility form was not recognized")
	}
	if allowsQuestVisibleNPCInteraction(6357, 100000181, farAway, d, qcat) ||
		allowsQuestVisibleNPCInteraction(6358, npc, farAway, qcat.Quests[6358], qcat) {
		t.Fatal("unrelated NPC or quest was allowed")
	}
}

func TestWoonRevealedByGroupedVisibilityInGhentAfterwar(t *testing.T) {
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	wcat, err := catalog.LoadWorld("../../configs/world.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	const npc = 100000304
	ghent := storage.WorldPosition{Town: 6, Area: 3}
	d := qcat.Quests[13588]
	if (&world.Service{Catalog: wcat}).HasNPC(ghent, npc) {
		t.Fatal("Woon unexpectedly exists in the base Ghent dock map")
	}
	if shown, found := questVisibilityOnClear(qcat.Quests[13587].Script.Cells, npc); !found || !shown {
		t.Fatal("skywar_30 grouped visibility should show its second NPC on clear")
	}
	if !questShownByPrerequisiteOnClear(d, npc, qcat) || !allowsQuestVisibleNPCInteraction(13588, npc, ghent, d, qcat) {
		t.Fatal("the observed skywar_31 interaction should pass the area check")
	}
	for _, at := range []storage.WorldPosition{{Town: 6, Area: 2}, {Town: 40, Area: 2}} {
		if allowsQuestVisibleNPCInteraction(13588, npc, at, d, qcat) {
			t.Fatalf("Woon interaction allowed outside Ghent dock at %+v", at)
		}
	}
	if allowsQuestVisibleNPCInteraction(13588, 100000299, ghent, d, qcat) ||
		allowsQuestVisibleNPCInteraction(13590, npc, ghent, qcat.Quests[13590], qcat) {
		t.Fatal("unrelated quest or NPC was allowed")
	}
}

func TestQuestLineageGuidesRevealedNPC(t *testing.T) {
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := qcat.Quests[13565]
	const npc = 100001497
	street := storage.WorldPosition{Town: 14, Area: 1}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "0")
	if !questLineageShowsGuidedNPC(d, npc, street, qcat, true) ||
		!allowsQuestVisibleNPCInteraction(13565, npc, street, d, qcat) {
		t.Fatal("the 13562 clear/show and 13563 go guide should authorize 13565 in Slaugh 14/1")
	}
	for _, at := range []storage.WorldPosition{{Town: 14, Area: 0}, {Town: 54, Area: 1}} {
		if allowsQuestVisibleNPCInteraction(13565, npc, at, d, qcat) {
			t.Fatalf("quest 13565 was allowed outside its source guide at %+v", at)
		}
	}
	if allowsQuestVisibleNPCInteraction(13565, 100001498, street, d, qcat) {
		t.Fatal("wrong NPC was allowed")
	}
	if shown, found := questVisibilityOnClear(qcat.Quests[13562].Script.Cells, npc); !found || !shown {
		t.Fatal("quest 13562 should show Erje on clear")
	}
	if shown, found := questVisibilityOnClear(qcat.Quests[13574].Script.Cells, npc); !found || shown {
		t.Fatal("quest 13574 should hide Erje on clear")
	}
	t.Setenv("DFO_QUEST_VISIBLE_NPC_RELAX", "1")
	if !allowsQuestVisibleNPCInteraction(13565, npc, storage.WorldPosition{Town: 1, Area: 0}, d, qcat) {
		t.Fatal("opt-in source visibility path should cover deeper prerequisite chains")
	}
}

func TestQuestReachTemporaryNPCFromNativeTrigger(t *testing.T) {
	qcat, err := catalog.LoadQuests("../../configs/quests.generated.json")
	if err != nil {
		t.Fatal(err)
	}
	d := qcat.Quests[13574]
	const npc = 100001497
	street := storage.WorldPosition{Town: 14, Area: 1}
	if !questLineageShowsReachNPC(d, npc, street, qcat) {
		t.Fatal("the source should authorize the 13574 NPC reach trigger in Slaugh 14/1")
	}
	if questLineageShowsReachNPC(d, npc, storage.WorldPosition{Town: 14, Area: 0}, qcat) ||
		questLineageShowsReachNPC(d, 100001494, street, qcat) {
		t.Fatal("range trigger accepted an unlinked area or NPC")
	}
	if shown, found := questVisibilityOnClear(d.Script.Cells, npc); !found || shown {
		t.Fatal("the NPC should hide only when quest 13574 is cleared")
	}
}

// 客户端 CMD33 的形态（u16 33 / u16 quest / 其余 12 字节 0）之外的一律拒绝。
func TestQuestInteractionRejectsNonNativeCMDForm(t *testing.T) {
	w := &worldSession{
		service: &world.Service{Catalog: catalog.WorldCatalog{}},
		quests:  &quest.Service{Catalog: catalog.QuestCatalog{}},
		role:    storage.Character{ID: 1, WireID: 3},
		state:   storage.WorldState{Position: storage.WorldPosition{Town: 40, Area: 2}},
	}
	if _, e := w.questInteraction(make([]byte, 12)); e == nil {
		t.Fatal("short quest check request accepted")
	}
	bad := make([]byte, 16)
	binary.LittleEndian.PutUint16(bad, 33)
	bad[15] = 1
	if _, e := w.questInteraction(bad); e == nil {
		t.Fatal("quest check request with a non-zero tail accepted")
	}
}
