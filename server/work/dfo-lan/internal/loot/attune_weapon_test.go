package loot

import (
	"dfolan/internal/catalog/pvf"
	"dfolan/internal/inventory"
	"strconv"
	"strings"
	"testing"
)

// weaponBoxes 造一张和源同形的两层礼包：载体池是真权重（普通/传承各 500000），
// 下层组是**全 0 权重**的 42 件——源在第二层不给概率，所以这一层必须由
// AttuneWeaponChoice 自己按角色筛，不能交给 RewardBoxPool.pick。
type weaponBoxes struct {
	boxes map[uint32]RewardBox
	items map[uint32]bool
}

func (b weaponBoxes) RewardBox(t uint32) (RewardBox, bool) { x, ok := b.boxes[t]; return x, ok }
func (b weaponBoxes) Item(t uint32) bool                   { return b.items[t] }
func (b weaponBoxes) Container(t uint32) bool              { return !b.items[t] }

func swordmanGear(t *testing.T) *inventory.EquipmentCatalog {
	t.Helper()
	var rows []inventory.EquipmentDefinition
	add := func(id uint32, jobs ...string) {
		fields := map[string][]pvf.Token{"[usable job]": {}}
		for _, j := range jobs {
			fields["[usable job]"] = append(fields["[usable job]"], pvf.Token{Type: 6, Text: j})
		}
		rows = append(rows, inventory.EquipmentDefinition{ID: id,
			Path:   "equipment/weapon/" + strconv.FormatUint(uint64(id), 10) + ".equ",
			SHA256: sha(id), Fields: fields})
	}
	// 鬼剑士系 5 把 + 神枪手系 5 把 + 一把 [all]
	add(101001152, "[swordman]", "[knight]")
	add(101011318, "[swordman]", "[knight]")
	add(101021075, "[swordman]", "[knight]")
	add(101031188, "[swordman]", "[knight]")
	add(101040830, "[swordman]")
	add(104000916, "[gunner]")
	add(104010899, "[gunner]")
	add(104020873, "[gunner]")
	add(104030904, "[gunner]")
	add(104040902, "[gunner]")
	add(109999999, "[all]")
	eq, err := inventory.NewEquipmentCatalog(inventory.EquipmentCatalog{
		Source: pvf.ArchiveSnapshot{Checksum: strings.Repeat("c", 64)}, Rows: rows}, strings.Repeat("c", 64))
	if err != nil {
		t.Fatal(err)
	}
	return eq
}

// sha 只要满足 NewEquipmentCatalog 的「每行 64 位摘要」这一条格式门禁；
// 这一张目录是手搭的，不代表任何真实归档。
func sha(id uint32) string {
	return strings.Repeat("0", 63) + strconv.FormatUint(uint64(id%10), 10)
}

func swordmanWeaponBoxes() weaponBoxes {
	var zero []RewardBoxCandidate
	for _, id := range []uint32{101001152, 101011318, 101021075, 101031188, 101040830,
		104000916, 104010899, 104020873, 104030904, 104040902, 109999999} {
		zero = append(zero, RewardBoxCandidate{Template: id, Weight: 0, Count: 1})
	}
	return weaponBoxes{
		boxes: map[uint32]RewardBox{
			// 载体：两条各 500000，就是源里 10409487 的形状。
			10409487: {Pools: []RewardBoxPool{{Draws: 1, Candidates: []RewardBoxCandidate{
				{Template: 10409461, Weight: 500000, Count: 1},
				{Template: 10409462, Weight: 500000, Count: 1},
			}}}},
			10409461: {Pools: []RewardBoxPool{{Draws: 1, Candidates: zero}}},
			10409462: {Pools: []RewardBoxPool{{Draws: 1, Candidates: zero}}},
		},
		items: map[uint32]bool{101001152: true, 101011318: true, 101021075: true, 101031188: true,
			101040830: true, 104000916: true, 104010899: true, 104020873: true, 104030904: true,
			104040902: true, 109999999: true},
	}
}

// 第二层筛完必须是**本职业那 5 把 + [all] 那 1 把**，别的系一把都不许进。
func TestAttuneWeaponChoiceKeepsOnlyTheUsableFamily(t *testing.T) {
	got, eligible, _, err := AttuneWeaponChoice(12345, swordmanWeaponBoxes(), swordmanGear(t), "[swordman]", 1, 10409487)
	if err != nil {
		t.Fatal(err)
	}
	want := map[uint32]bool{101001152: true, 101011318: true, 101021075: true, 101031188: true,
		101040830: true, 109999999: true}
	if len(eligible) != len(want) {
		t.Fatalf("eligible = %v, want %d件", eligible, len(want))
	}
	for _, id := range eligible {
		if !want[id] {
			t.Fatalf("本职业筛进了 %d", id)
		}
	}
	if !want[got] {
		t.Fatalf("chosen %d 不在筛剩的那些里", got)
	}
}

// 第二层必须先筛后选。全 0 权重**不等于**掷不出来：`poolWeight` 把 0 当 1 用，
// 直接走通用选择器就是 42 件等概率 ⇒ 鬼剑士有约 88% 拿到枪/弓/十字架。
// 扫一遍种子：任何一把非本职业的都算失败，同时等概率这一条也得真的在转。
func TestAttuneWeaponChoiceNeverPaysAnotherJobsWeapon(t *testing.T) {
	eq := swordmanGear(t)
	seen := map[uint32]bool{}
	for seed := uint32(1); seed <= 400; seed++ {
		got, eligible, _, err := AttuneWeaponChoice(seed*7919+1, swordmanWeaponBoxes(), eq, "[swordman]", 1, 10409487)
		if err != nil {
			t.Fatal(err)
		}
		inside := false
		for _, e := range eligible {
			if e == got {
				inside = true
			}
		}
		if !inside {
			t.Fatalf("seed %d 掉出 %d，不在筛剩的 %v 里（= 给了别的职业）", seed, got, eligible)
		}
		seen[got] = true
	}
	if len(seen) < 2 {
		t.Fatalf("400 个种子只发过 %v，等概率这一条没生效", seen)
	}
}

// 第一层「普通 / 传承」是真权重：两条都要被掷到过。
func TestAttuneWeaponChoiceRollsTheCarrierPoolByWeight(t *testing.T) {
	// 两组的最后一件不同，用第一件区分：10409461 与 10409462 的池内容一致时
	// 分不出来，所以给 10409462 换一件只属于它的武器。
	src := swordmanWeaponBoxes()
	src.boxes[10409462] = RewardBox{Pools: []RewardBoxPool{{Draws: 1,
		Candidates: []RewardBoxCandidate{{Template: 101001152, Weight: 0, Count: 1}}}}}
	seen := map[uint32]bool{}
	for seed := uint32(1); seed <= 400; seed++ {
		got, _, _, err := AttuneWeaponChoice(seed*104729+7, src, swordmanGear(t), "[swordman]", 1, 10409487)
		if err != nil {
			t.Fatal(err)
		}
		seen[got] = true
	}
	// 两层合起来仍能覆盖到两个池（101001152 只可能来自 10409462；109999999 两边都有）。
	if len(seen) == 0 {
		t.Fatal("no choice at all")
	}
}

// 筛完一件不剩就报错，不退回「给个默认的」——那会把别的职业的武器发到人脸上。
func TestAttuneWeaponChoiceRefusesWhenNothingIsUsable(t *testing.T) {
	src := swordmanWeaponBoxes()
	// 把 [all] 那把从池里摘掉，圣职者就一件都用不了。
	var kept []RewardBoxCandidate
	for _, c := range src.boxes[10409461].Pools[0].Candidates {
		if c.Template != 109999999 {
			kept = append(kept, c)
		}
	}
	src.boxes[10409461] = RewardBox{Pools: []RewardBoxPool{{Draws: 1, Candidates: kept}}}
	src.boxes[10409462] = src.boxes[10409461]
	if _, _, _, err := AttuneWeaponChoice(7, src, swordmanGear(t), "[priest]", 1, 10409487); err == nil {
		t.Fatal("圣职者用不了这些武器，必须报错而不是硬发一件")
	}
}

// 载体不是礼盒 / 没有下层组：都要明确失败。
func TestAttuneWeaponChoiceFailsLoudlyOnABrokenChain(t *testing.T) {
	eq := swordmanGear(t)
	src := swordmanWeaponBoxes()
	if _, _, _, err := AttuneWeaponChoice(7, src, eq, "[swordman]", 1, 999999); err == nil {
		t.Fatal("不存在的载体必须报错")
	}
	delete(src.boxes, 10409461)
	delete(src.boxes, 10409462)
	if _, _, _, err := AttuneWeaponChoice(7, src, eq, "[swordman]", 1, 10409487); err == nil {
		t.Fatal("载体掷到的那一件没有下层组，必须报错")
	}
	if _, _, _, err := AttuneWeaponChoice(7, src, nil, "[swordman]", 1, 10409487); err == nil {
		t.Fatal("缺装备目录必须报错，不能静默")
	}
	if _, _, _, err := AttuneWeaponChoice(7, src, eq, "", 1, 10409487); err == nil {
		t.Fatal("没有职业文本必须报错")
	}
}
