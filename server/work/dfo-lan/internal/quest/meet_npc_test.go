package quest

import (
	"dfolan/internal/catalog"
	"dfolan/internal/testfixture"
	"testing"
)

// [sub type] 1 是"显式对话"形态：客户端拿任务自己解析出的目标 NPC 覆盖传入的
// 对话对象，再走目标推进与 CMD33 发送链，全程不要求该 NPC 站在当前地图里。
//
// 这三个扩展装备槽任务正好是这个形态，而且它们的 [alternative npc index] 目标
// （100001447）只在 town139，原始目标 NPC 28 在 town40/area2 —— 所以"先解析目标
// 再做地图检查"必然把客户端已经发出的请求拒掉。豁免对它们不是顺带，而是必需。
func TestExplicitDialogueQuestsMayResolveTheirOwnTarget(t *testing.T) {
	c, e := catalog.LoadQuests(testfixture.CatalogPath(t, "quests"))
	if e != nil {
		t.Fatal(e)
	}
	for _, id := range []uint32{649, 650, 2636} {
		d, ok := c.Quests[id]
		if !ok {
			t.Fatalf("source quest %d missing", id)
		}
		if !AllowsRemoteNPCInteraction(d) {
			t.Fatalf("quest %d must accept an explicit dialogue request", id)
		}
		alt := cells(d.Script.Cells, "[alternative npc index]")
		if len(alt) < 2 || alt[0].Type != 0 || alt[1].Type != 0 ||
			uint32(alt[0].Value) == uint32(alt[1].Value) {
			t.Fatalf("quest %d no longer carries a distinct alternative NPC: %+v", id, alt)
		}
		if uint32(alt[1].Value) == uint32(d.ObjectiveCells[0].Value) {
			t.Fatalf("quest %d alternative NPC no longer differs from the objective", id)
		}
	}

	// 全目录量化：subtype1 的 [meet npc] 任务共 53 条，其中 13 条目标 NPC 为 -1
	// （真目标要靠未建模的条件解析，不能只凭 subtype1 放行）。
	sub1, negative, allowed := 0, 0, 0
	for _, d := range c.Quests {
		if d.Kind != "[meet npc]" {
			continue
		}
		st := cells(d.Script.Cells, "[sub type]")
		if len(st) != 1 || st[0].Type != 0 || st[0].Value != 1 {
			continue
		}
		sub1++
		if len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Value == -1 {
			negative++
		}
		if AllowsRemoteNPCInteraction(d) {
			allowed++
		}
	}
	if sub1 != 53 {
		t.Fatalf("subtype1 [meet npc] quests: %d", sub1)
	}
	if negative != 13 {
		t.Fatalf("subtype1 quests whose objective NPC is -1: %d", negative)
	}
	if allowed != sub1-negative {
		t.Fatalf("exempted %d of %d positive-NPC subtype1 quests", allowed, sub1-negative)
	}
	if allowed == 0 {
		t.Fatal("no quest is exempt; the positional check still blocks every explicit dialogue")
	}

	// 没有 [sub type] 的普通对话任务仍受地图检查约束。
	plain := 0
	for _, d := range c.Quests {
		if d.Kind != "[meet npc]" || len(cells(d.Script.Cells, "[sub type]")) != 0 {
			continue
		}
		plain++
		if AllowsRemoteNPCInteraction(d) {
			t.Fatalf("quest %d without [sub type] was exempted from the map check", d.ID)
		}
	}
	if plain == 0 {
		t.Fatal("no plain [meet npc] quest in the catalog: the filter cannot be distinguished")
	}

	// 任何目标 NPC 为 -1 或数据未解析的任务一律不放行。
	for _, d := range c.Quests {
		if len(d.ObjectiveCells) == 1 && d.ObjectiveCells[0].Value == -1 && AllowsRemoteNPCInteraction(d) {
			t.Fatalf("quest %d with an unresolved objective NPC was exempted", d.ID)
		}
		if len(d.Pending) != 0 && AllowsRemoteNPCInteraction(d) {
			t.Fatalf("quest %d with unresolved source data was exempted", d.ID)
		}
	}
}
