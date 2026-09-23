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
