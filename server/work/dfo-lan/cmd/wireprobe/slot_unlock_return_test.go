package main

import (
	"dfolan/internal/catalog"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/inventory"
	"encoding/json"
	"strings"
	"testing"
)

func slotPlanNames(plan []outboundPacket) []string {
	out := make([]string, 0, len(plan))
	for _, p := range plan {
		out = append(out, p.Name)
	}
	return out
}

// 扩展装备槽解锁位只能由 EntryAddition（USERINFO1）投影到装备栏挂锁，而客户端只在
// 登录 / 选角 / 进副本那种时机构造装备栏行对象；副本内补发它会把装备栏显示清空
// （2026-09-22 实测，见 analysis/tasks/next50-odyssey-expanded-equip-slot.md）。
//
// 因此副本内解锁只置脏标记，改在**回城**补发一次。本用例钉住三件事：
//  1. dirty=true 时回城必须补发 addition，且**紧跟 appearance**（与登录/进副本的
//     basic → addition 配对顺序一致），载荷带着解锁字节；
//  2. dirty=false 时不得多发（不影响其它回城路径）；
//  3. 补发成功后脏标记要清掉，避免之后每次回城都重放。
func TestLeaveDungeonReplaysUnlockedSlotsOnlyWhenDirty(t *testing.T) {
	professions, e := catalog.LoadCharacters("../../configs/characters.generated.json")
	if e != nil {
		t.Fatal(e)
	}
	state, e := inventory.SaveBag(
		json.RawMessage(`{"source_sha256":"`+strings.Repeat("a", 64)+
			`","level":84,"attributes":{"[hp max]":100,"[mp max]":50}}`),
		inventory.Bag{
			Version:          "ordinary-bag-v1",
			ExpandEquipFlags: inventory.ExpandSupport | inventory.ExpandMagicStone | inventory.ExpandEarring,
		})
	if e != nil {
		t.Fatal(e)
	}
	session := func(dirty bool) *worldSession {
		return &worldSession{
			role:            database.Character{ID: 1, WireID: 3, Name: "SlotProbe", State: state},
			characters:      &character.Service{Catalog: professions},
			slotUnlockDirty: dirty,
		}
	}
	index := func(plan []outboundPacket, name string) int {
		for i, p := range plan {
			if p.Name == name {
				return i
			}
		}
		return -1
	}

	unlocked := inventory.ExpandSupport | inventory.ExpandMagicStone | inventory.ExpandEarring

	// dirty：必须补发，紧跟 appearance，且把解锁字节带上。
	dirty := session(true)
	plan, err := dirty.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	appearance, addition := index(plan, "town_actor_appearance_restored"), index(plan, "town_actor_addition_restored")
	if appearance < 0 || addition < 0 {
		t.Fatalf("dirty 回城缺帧: appearance=%d addition=%d plan=%v", appearance, addition, slotPlanNames(plan))
	}
	if addition != appearance+1 {
		t.Fatalf("addition 必须紧跟 appearance（与登录/进副本一致）: addition=%d appearance=%d",
			addition, appearance)
	}
	if plan[addition].ID != 2 || plan[addition].Kind != 0 {
		t.Fatalf("addition 帧形态不对: %+v", plan[addition])
	}
	if len(plan[addition].Payload) <= 360 || plan[addition].Payload[360] != unlocked {
		t.Fatalf("解锁字节没进载荷 offset 360: len=%d", len(plan[addition].Payload))
	}
	if dirty.slotUnlockDirty {
		t.Fatal("补发成功后脏标记应清掉")
	}

	// 非 dirty：不得多发，但 appearance 照常。
	clean := session(false)
	cleanPlan, err := clean.leaveDungeon()
	if err != nil {
		t.Fatal(err)
	}
	if index(cleanPlan, "town_actor_addition_restored") >= 0 {
		t.Fatalf("非 dirty 不应补发 addition: %v", slotPlanNames(cleanPlan))
	}
	if index(cleanPlan, "town_actor_appearance_restored") < 0 {
		t.Fatalf("appearance 应照常下发: %v", slotPlanNames(cleanPlan))
	}
}
