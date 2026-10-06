package inventory

import (
	"dfolan/internal/catalog"
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/game/protocol"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// 教学模式（Starter Boost 训练轨道）内获得的装备应被禁止出售/丢弃/寄邮件/入仓库，
// 出关后自动恢复正常。这里在 inventory 层直接验证：
//   1. AddEquipment 只在训练轨道内给新建装备行盖 TutorialLocked（向前兼容：老存档无该字段=false）；
//   2. 各操作门禁 = 行的来源标记 && 当前仍在训练轨道（ReadBag 从 state 现算 b.tutorialActive）；
//   3. **分解不受封存**：第九关任务本身就是 `[mission][type] disjoint`，要拆的是训练装备。
//
// 判定源用注入的探测谓词模拟（真机由 cmd/wireprobe/main.go 从 boost_up115 注入），避免
// inventory 反向依赖 boostup 包；state 里的 "_tut":"on" 顶层键由 SaveBag 原样保留。

func tutorialCatalog() *EquipmentCatalog {
	return &EquipmentCatalog{Source: pvf.ArchiveSnapshot{Checksum: "tut-test"}, index: map[uint32]EquipmentDefinition{
		70001: {ID: 70001, Fields: map[string][]pvf.Token{
			"[rarity]":         {{Type: 0, Value: 1}},
			"[equipment type]": {{Type: 6, Text: "[jacket]"}},
			"[durability]":     {{Type: 0, Value: 50}},
		}},
	}}
}

func tutorialState(on bool, bagVersion string) json.RawMessage {
	state := `{"inventory":{"version":"` + bagVersion + `"}}`
	if on {
		state = `{"_tut":"on","inventory":{"version":"` + bagVersion + `"}}`
	}
	return json.RawMessage(state)
}

// withTutorial installs a predicate that reports in-tutorial iff the state carries the probe.
func withTutorial(t *testing.T) {
	t.Helper()
	prev := inBoostTraining
	SetInBoostTraining(func(state json.RawMessage) bool { return strings.Contains(string(state), `"_tut":"on"`) })
	t.Cleanup(func() { SetInBoostTraining(prev) })
}

func TestTutorialSealMarksOnlyNewEquipment(t *testing.T) {
	withTutorial(t)
	cat := tutorialCatalog()

	// 训练轨道内建装 → 盖锁。
	on, err := ReadBag(tutorialState(true, "ordinary-bag-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if !on.tutorialActive {
		t.Fatal("ReadBag did not pick up the tutorial predicate from state")
	}
	on, slots, err := on.AddEquipment(cat, [2]uint16{9, 64}, 70001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(on.Equipment) != 1 || !on.Equipment[0].TutorialLocked {
		t.Fatalf("training-reward equipment was not sealed: %+v", on.Equipment)
	}
	// 落库要保留标记，且 SaveBag 会原样保留 state 里的其它顶层键。
	raw, err := SaveBag(tutorialState(true, "ordinary-bag-v1"), on)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"tutorial_locked":true`) {
		t.Fatalf("marker not persisted: %s", raw)
	}
	if !strings.Contains(string(raw), `"_tut":"on"`) {
		t.Fatalf("SaveBag dropped unrelated state keys: %s", raw)
	}
	reread, err := ReadBag(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(reread.Equipment) != 1 || !reread.Equipment[0].TutorialLocked {
		t.Fatalf("marker lost across round-trip: %+v", reread.Equipment)
	}

	// 出关后（无训练轨道）建装 → 不盖锁。
	off, err := ReadBag(tutorialState(false, "ordinary-bag-v1"))
	if err != nil {
		t.Fatal(err)
	}
	if off.tutorialActive {
		t.Fatal("predicate leaked into a non-tutorial state")
	}
	off, offSlots, err := off.AddEquipment(cat, [2]uint16{9, 64}, 70001, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(off.Equipment) != 1 || off.Equipment[0].TutorialLocked {
		t.Fatalf("post-tutorial equipment wrongly sealed: %+v", off.Equipment)
	}
	if len(slots) != 1 || len(offSlots) != 1 {
		t.Fatalf("unexpected landing slots: %v / %v", slots, offSlots)
	}

	// 老存档向前兼容：无 tutorial_locked 字段 → 读出 false。
	legacy := json.RawMessage(`{"inventory":{"version":"ordinary-bag-v1","equipment":[{"slot":20,"template":70001,"durability":50}]}}`)
	lb, err := ReadBag(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if len(lb.Equipment) != 1 || lb.Equipment[0].TutorialLocked {
		t.Fatalf("legacy row should read unlocked: %+v", lb.Equipment)
	}
}

func TestTutorialSealBlocksAndReleasesOperations(t *testing.T) {
	withTutorial(t)
	cat := tutorialCatalog()
	rules := BagRules{
		Source:         "tut-test",
		EquipmentSlots: [2]uint16{9, 64},
		Slots:          map[string][2]uint16{"[material]": {65, 70}},
	}
	loot := catalog.LootCatalog{Source: pvf.ArchiveSnapshot{Checksum: "tut-test"}}

	// 训练轨道内：先落一件锁定的装备到槽 20。
	build := func() Bag {
		b, err := ReadBag(tutorialState(true, "ordinary-bag-v1"))
		if err != nil {
			t.Fatal(err)
		}
		b, _, err = b.AddEquipment(cat, rules.EquipmentSlots, 70001, 1)
		if err != nil {
			t.Fatal(err)
		}
		b.Equipment[0].Slot = 20
		return b
	}

	// 出售被拒（训练轨道内）。
	b := build()
	if _, _, _, err := b.Sell(rules, 0, 20, 1, 10); err == nil || !strings.Contains(err.Error(), "training-reward") {
		t.Fatalf("sell inside tutorial should be refused, got %v", err)
	}
	// 分解**必须放行**（第九关 `[mission][type] disjoint` 要拆的就是训练装备）。
	b = build()
	after, result, err := b.Disjoint(loot, rules, nil, []uint16{20}, 65)
	if err != nil {
		t.Fatalf("disjoint inside tutorial must stay allowed (step 9 mission), got %v", err)
	}
	if len(after.Equipment) != 0 || len(result.DeletedSlots) != 1 || result.DeletedSlots[0] != 20 {
		t.Fatalf("disjoint should remove the training row: bag=%+v result=%+v", after.Equipment, result)
	}
	// 寄邮件被拒（训练轨道内）。
	b = build()
	if _, _, err := b.TakeMailItem(loot, cat, protocol.MailSendItem{List: 0, Slot: 20, Template: 70001, Amount: 1}); !errors.Is(err, ErrMailUntradeable) {
		t.Fatalf("mail-send inside tutorial should be refused, got %v", err)
	}

	// 出关后同一件锁定装备应恢复正常：出售成功（去掉该装备）。
	offState, err := SaveBag(tutorialState(true, "ordinary-bag-v1"), build())
	if err != nil {
		t.Fatal(err)
	}
	offState = json.RawMessage(strings.Replace(string(offState), `,"_tut":"on"`, ``, 1))
	offState = json.RawMessage(strings.Replace(string(offState), `"_tut":"on",`, ``, 1))
	b, err = ReadBag(offState)
	if err != nil {
		t.Fatal(err)
	}
	if b.tutorialActive {
		t.Fatal("expected out-of-tutorial state after exit")
	}
	if len(b.Equipment) != 1 || !b.Equipment[0].TutorialLocked {
		t.Fatalf("post-exit row should still carry its provenance marker: %+v", b.Equipment)
	}
	if _, _, _, err := b.Sell(rules, 0, 20, 1, 10); err != nil {
		t.Fatalf("sell after exiting tutorial should succeed, got %v", err)
	}
}
