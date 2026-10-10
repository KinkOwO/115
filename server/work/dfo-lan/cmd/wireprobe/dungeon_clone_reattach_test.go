package main

import (
	"bytes"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/inventory"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDungeonCloneReattachRestoresOrdinaryGearLast(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equipment.json")
	data, err := json.Marshal(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "fixture"},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "clear avatar"}},
			}},
			{ID: 112500000, Path: "default.equ", SHA256: strings.Repeat("b", 64)},
			{ID: 101000013, Path: "weapon.equ", SHA256: strings.Repeat("c", 64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	role := database.Character{
		ID: 5, WireID: 503, Profession: 11,
		State: json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":517500000},{"slot":12,"template":101000013}]}}`),
	}
	w := &worldSession{
		role: role, activeDungeon: &dungeon.Session{},
		characters: &character.Service{DetailedWornCandidate: true, Equipment: catalog},
	}
	plan, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	var actual []string
	for _, packet := range plan {
		if packet.Name == "dungeon_worn_visuals_restored" || strings.HasPrefix(packet.Name, "dungeon_clone_") || packet.Name == "dungeon_nonavatar_worn_restored" {
			actual = append(actual, packet.Name)
		}
	}
	want := []string{"dungeon_worn_visuals_restored", "dungeon_clone_detached", "dungeon_clone_reattached", "dungeon_nonavatar_worn_restored"}
	if len(actual) != len(want) {
		t.Fatalf("unexpected Clone loading sequence: %v", actual)
	}
	for i := range want {
		if actual[i] != want[i] {
			t.Fatalf("Clone loading order %v, want %v", actual, want)
		}
	}
	if plan[len(plan)-2].ID != 14 || plan[len(plan)-1].ID != 1361 {
		t.Fatal("ordinary equipment must follow both mode-1 packets, then buff registration must bind the new actor")
	}
	// Same no-death run returns straight to town. The candidate must restore
	// the same native row layout and keep omitted ordinary gear after mode1.
	w.state = database.WorldState{Position: database.WorldPosition{Town: 38, Area: 2, X: 150, Y: 249}}
	for _, withSource := range []bool{false, true} {
		if withSource {
			bag, err := inventory.ReadBag(w.role.State)
			if err != nil {
				t.Fatal(err)
			}
			bag.Worn[0].CloneSource = &inventory.CloneAvatarSource{Slot: 7, Template: 112500000}
			bag.Special = map[byte][]inventory.BagEquipment{1: {{Slot: 7, Template: 112500000}}}
			w.role.State, err = inventory.SaveBag(w.role.State, bag)
			if err != nil {
				t.Fatal(err)
			}
		}
		stateBefore := append([]byte(nil), w.role.State...)
		returned, err := w.leaveDungeon()
		if err != nil {
			t.Fatal(err)
		}
		indices := map[string]int{}
		for i, packet := range returned {
			indices[packet.Name] = i
		}
		for _, name := range []string{"clone_avatar_sources_replaced", "town_clone_detached", "town_clone_reattached", "town_nonavatar_worn_restored"} {
			if _, ok := indices[name]; !ok {
				t.Fatalf("missing town lifecycle %s", name)
			}
		}
		if indices["clone_avatar_sources_replaced"] >= indices["town_clone_detached"] || indices["town_clone_reattached"] >= indices["town_nonavatar_worn_restored"] {
			t.Fatal("town restoration order")
		}
		sources := returned[indices["clone_avatar_sources_replaced"]].Payload
		if len(sources) != 34 || sources[0] != 11 {
			t.Fatal("town source replacement has the wrong row count")
		}
		for slot := 0; slot < 11; slot++ {
			want := uint16(0xffff)
			if withSource && slot == 3 {
				want = 7 + 12
			}
			p := 1 + slot*3
			if sources[p] != byte(slot) || binary.LittleEndian.Uint16(sources[p+1:]) != want {
				t.Fatalf("town source row %d: %x", slot, sources[p:p+3])
			}
		}
		if !bytes.Equal(w.role.State, stateBefore) {
			t.Fatal("return rewrote physical instances or sources")
		}
	}
}

// TestSeamlessRetryRebuildsCloneWithoutBuffRegistration 钉住 2026-10-09 修的那条边界：
// 「点完再次挑战，Clone 部位立刻裸体」（业主当日复现，会话 _213112_）。
//
// 根因是一处自相矛盾：入口那步按 350-351 的约定跳过了 N14 sent，改由
// finishDungeonLoading 的 dungeon_worn_visuals_restored 补发（对 relay 照样生效）；
// 而 09-30 已实机确认「NOTI14 全量穿戴刷新会重建未变化的 Clone 对象」。
// 只补发 N14 却不重建 Clone，等于把 Clone 的覆盖外观冲掉又不修回来。
//
// 所以 seamlessRetry 时**必须补** Clone 重建；但绝不能顺带把 buff 注册
// （NOTI1361）或随机属性块带回来 —— 那是 next178 §16 明确要跳的
// 「再次挑战自动上 buff / 穿戴效果应用两次」。
func TestSeamlessRetryRebuildsCloneWithoutBuffRegistration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "equipment.json")
	data, err := json.Marshal(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: "fixture"},
		Rows: []inventory.EquipmentDefinition{
			{ID: 517500000, Path: "clone.equ", SHA256: strings.Repeat("a", 64), Fields: map[string][]pvf.Token{
				"[item category]": {{Type: 6, Text: "clear avatar"}},
			}},
			{ID: 112500000, Path: "default.equ", SHA256: strings.Repeat("b", 64)},
			{ID: 101000013, Path: "weapon.equ", SHA256: strings.Repeat("c", 64)},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	catalog, err := inventory.LoadEquipmentCatalog(path, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	role := database.Character{
		ID: 5, WireID: 503, Profession: 11,
		State: json.RawMessage(`{"source_sha256":"fixture","attributes":{"[hp max]":100,"[mp max]":100},"inventory":{"version":"ordinary-bag-v1","worn":[{"slot":3,"template":517500000},{"slot":12,"template":101000013}]}}`),
	}
	w := &worldSession{
		role: role, activeDungeon: &dungeon.Session{}, seamlessRetry: true,
		characters: &character.Service{DetailedWornCandidate: true, Equipment: catalog},
	}
	plan, err := w.finishDungeonLoading(make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	indices := map[string]int{}
	for i, packet := range plan {
		indices[packet.Name] = i
	}
	// 必须补上：N14 补发会重建 Clone 对象，紧跟着就得把覆盖外观修回来。
	for _, name := range []string{"dungeon_worn_visuals_restored", "dungeon_clone_detached", "dungeon_clone_reattached"} {
		if _, ok := indices[name]; !ok {
			t.Fatalf("seamless retry 缺少 %s：N14 冲掉 Clone 覆盖却不修 ⇒ 玩家看到裸体（plan=%v）", name, names(plan))
		}
	}
	if indices["dungeon_clone_detached"] >= indices["dungeon_clone_reattached"] {
		t.Fatal("Clone detach 必须排在 reattach 之前")
	}
	// 必须仍然跳过：这些一旦回来就是「自动上 buff / 穿戴效果应用两次」的老毛病。
	for _, name := range []string{
		"dungeon_buff_enhancement_restored",
		"dungeon_worn_random_options_restored",
		"dungeon_worn_equipment_restored",
	} {
		if _, ok := indices[name]; ok {
			t.Fatalf("seamless retry 不该发 %s：会重上 buff 或把穿戴效果应用两次", name)
		}
	}
	if w.seamlessRetry {
		t.Fatal("seamlessRetry 必须在 finishDungeonLoading 内消费并清零，不能漏到下一次进本")
	}
}

func names(plan []outboundPacket) []string {
	out := make([]string, 0, len(plan))
	for _, packet := range plan {
		out = append(out, packet.Name)
	}
	return out
}
