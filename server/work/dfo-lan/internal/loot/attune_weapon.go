package loot

import (
	"dfolan/internal/inventory"
	"fmt"
)

// AttuneWeaponChoice 定出「命运的凝械」**武器格**（隐藏表 index 0）该掉的那一件。
//
// 这一格和另外三档（防具/首饰/特殊装备）**不是一套机制**，源给的事实是：
//
//   - 载体 10409487(史诗)/10409488(太初) 是 `[booster]`，`[etc] 1` 下挂两条各 500000
//     ⇒「普通 / 传承」那一层是**真按源权重掷的**；
//   - 掷到的 10409461..64 各自指向组 21377..21380，组里 **42 件权重全是 0**
//     ⇒ 源在第二层没打算用概率决定给哪一件；
//   - `[usable grow type]` 全空、`[usable job]` 只写到**基础职业系**
//     ⇒ 拿职业筛完，同一系里还剩 4~5 件（鬼剑士：短剑/太刀/钝器/巨剑/光剑）。
//
// 第二层因此**没有源依据**，只能是服主口径（业主 2026-10-12：「就按鬼剑士系 掉一把」）：
// 在本角色用得了的那 5 件里**等概率取一件**。两处刻意不做的事：
//
//   - 不照 `RewardBoxPool.pick` 走完这一层。⚠️ 全 0 权重**不等于**掷不出来：
//     `poolWeight` 把 0 当 1 用，于是 42 件变成等概率 —— 37 件是别的职业的武器，
//     每个角色都有约 88% 拿到一把用不了的枪/弓/十字架。要先筛再选。
//   - 不另写职业表：筛这一道复用 inventory.UsableByJob（与佩戴同一个判据，见
//     internal/inventory/wearable.go 那句「戴着这件」和「被发到这件」不能互相不同意）。
//
// 四张组都是同一个形状（实测 21377/21378/21379/21380 各 42 件、权重全 0、号段分
// 9 系，鬼剑士系每系恰好 5 件），所以史诗/太初、普通/传承四条路走同一段代码。
//
// 返回：选中件、筛剩的候选（日志/测试要看它是不是真只剩一个）、下一个种子。
func AttuneWeaponChoice(seed uint32, src RewardBoxSource, gear *inventory.EquipmentCatalog, job string, advancement byte, carrier uint32) (uint32, []uint32, uint32, error) {
	if src == nil || gear == nil {
		return 0, nil, seed, fmt.Errorf("weapon choice needs the box source and the equipment catalog")
	}
	if job == "" {
		return 0, nil, seed, fmt.Errorf("weapon choice needs the character's job text")
	}
	rng := RNG{seed}

	// 第一层：普通 / 传承，照源权重掷。
	box, ok := src.RewardBox(carrier)
	if !ok || len(box.Pools) == 0 {
		return 0, nil, seed, fmt.Errorf("weapon carrier %d is not an openable booster", carrier)
	}
	virtual, ok := box.Pools[0].pick(&rng)
	if !ok {
		return 0, nil, seed, fmt.Errorf("weapon carrier %d pays nothing", carrier)
	}

	// 第二层：那张全 0 权重的组，按本角色筛。
	gearBox, ok := src.RewardBox(virtual.Template)
	if !ok || len(gearBox.Pools) == 0 {
		return 0, nil, rng.Seed, fmt.Errorf("weapon booster %d has no resolved group", virtual.Template)
	}
	var eligible []uint32
	for _, c := range gearBox.Pools[0].Candidates {
		def, err := gear.Definition(c.Template)
		if err != nil {
			// 定义取不到就发不出去（耐久也取不到），当它不可选而不是硬塞。
			continue
		}
		if inventory.UsableByJob(def.Fields, job, advancement) != nil {
			continue
		}
		eligible = append(eligible, c.Template)
	}
	if len(eligible) == 0 {
		return 0, nil, rng.Seed, fmt.Errorf("weapon group of booster %d has nothing usable by %s (adv %d)",
			virtual.Template, job, advancement)
	}
	chosen := eligible[rng.Next(uint32(len(eligible)))]
	return chosen, eligible, rng.Seed, nil
}
