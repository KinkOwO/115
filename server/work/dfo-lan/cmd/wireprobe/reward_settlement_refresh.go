package main

import (
	"bytes"
	"context"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
)

// rewardDelta 记「库里那份与内存那份在哪些投影上不一样」。
//
// 比的是**编码后的帧体**而不是 state 的原始字节：Characters 走的是
// CharactersWithAdventure，会把账号迷雾阶段投影进每个角色的 state
// （见 adventure_elite_eligibility_test.go:64），整段比字节会场场误报。
type rewardDelta struct {
	// bag：普通背包（物品行 + 金币那一行 + 扩容位）变了。
	bag bool
	// pet：宠物用品 / 特殊容器变了。这两块不在 Rows() 里，必须单独判。
	pet bool
	// addition：USERINFO1 投影变了（扩展装备槽挂锁、背包/时装档位、复活币这类角色待遇）。
	addition bool
}

// rewardStateAfterCommit 在结算帧编码之前，把「已经落库、但内存里还没有」的那批奖励读回来。
//
// 为什么需要它：Lua 脚本在 level_up / quest_complete 里 grant_item、发角色待遇，是在业务事务
// **提交之后**才写库的 —— reward.Notifier 三个方法没有返回值、错误只记日志
// （internal/reward/reward.go 的 flush）。所以提交返回给网关的那份角色里根本没有这批东西，
// 而结算帧又全部由它编码：物品进了库、帧里没有、内存里也没有，玩家要等下一次全量背包
// （多半是重登）才看得到 —— 发奖白给。
//
// 返回的角色是库里那份（没变过就原样返回），调用方据此编码；变化按投影分类交回调用方，
// 由既有的帧形补齐 —— 本文件不发明任何未实机验证过的包。
func (w *worldSession) rewardStateAfterCommit(ctx context.Context, encoded database.Character) (database.Character, rewardDelta, error) {
	var none rewardDelta
	if w == nil || w.store == nil || encoded.ID == 0 {
		return encoded, none, nil
	}
	roles, e := w.store.Characters(ctx, w.account)
	if e != nil {
		return encoded, none, e
	}
	committed, found := encoded, false
	for _, role := range roles {
		if role.ID == encoded.ID {
			committed, found = role, true
			break
		}
	}
	if !found {
		return encoded, none, nil
	}
	committed.WireID = encoded.WireID
	beforeBag, e := inventory.ReadBag(encoded.State)
	if e != nil {
		return encoded, none, e
	}
	afterBag, e := inventory.ReadBag(committed.State)
	if e != nil {
		return encoded, none, e
	}
	var d rewardDelta
	// 金币余额就挂在 slot0/template0 那一行上，所以只比 Rows()+Expansion
	// 已经能覆盖「脚本只给钱」这种情形（与 quest_flow.go 那段同一口径）。
	before, e := bagRestoreBody(beforeBag)
	if e != nil {
		return encoded, none, e
	}
	after, e := bagRestoreBody(afterBag)
	if e != nil {
		return encoded, none, e
	}
	d.bag = !bytes.Equal(before, after)
	if d.pet, e = petContainerDelta(beforeBag, afterBag); e != nil {
		return encoded, none, e
	}
	// 待遇投影编不出来就当作没变：宁可不补这一帧，也不能把已完成的任务/通关卡在编码错误里。
	if w.characters != nil {
		beforeAddition, e := w.characters.EntryAddition(encoded)
		if e == nil {
			if afterAddition, e := w.characters.EntryAddition(committed); e == nil {
				d.addition = !bytes.Equal(beforeAddition, afterAddition)
			}
		}
	}
	w.deferActorAdditionInsideDungeon(&d)
	return committed, d, nil
}

// deferActorAdditionInsideDungeon 把副本内的待遇变化转成回城补发。
//
// 挂锁/扩容这类只能由 USERINFO1（EntryAddition）投影，而客户端只在登录/选角/进本构造装备栏
// 行对象 —— 副本内补发它会把装备栏显示清空（dungeon_flow.go:1786、next50 取证文档）。
// 所以副本内一律不发，改置 slotUnlockDirty，由 leaveDungeon 趁回城重建一次。
func (w *worldSession) deferActorAdditionInsideDungeon(d *rewardDelta) {
	if d == nil || !d.addition {
		return
	}
	if w.activeDungeon != nil {
		w.slotUnlockDirty = true
		d.addition = false
	}
}

// rewardCatchUpPackets 把提交之后才入库的那批奖励带进本轮结算帧。
//
// 与 quest_flow.go 的 finishQuest 用同一批已验证帧形：13 全量背包（odyssey 里程碑、
// 任务结算都发它）、13 宠物容器、城镇里的 USERINFO1 三件套。副本内的待遇变化
// 在回读那一步就已经被 deferActorAdditionInsideDungeon 转成回城补发，不会走到这里。
func (w *worldSession) rewardCatchUpPackets(role database.Character, d rewardDelta) ([]outboundPacket, error) {
	if w == nil || (!d.bag && !d.pet && !d.addition) {
		return nil, nil
	}
	var plan []outboundPacket
	if d.bag || d.pet {
		bag, e := inventory.ReadBag(role.State)
		if e != nil {
			return nil, e
		}
		if d.bag {
			body, e := bagRestoreBody(bag)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"reward_event_inventory", 0, 13, body})
		}
		if d.pet && len(bag.PetItems)+len(bag.Special[7]) > 0 {
			body, e := inventory.PetContainerBody(bag, true)
			if e != nil {
				return nil, e
			}
			plan = append(plan, outboundPacket{"reward_event_pet_container", 0, 13, body})
		}
	}
	// 走到这里的 addition 一定在城镇里：副本内那一份在回读时就被转成了回城补发。
	if d.addition {
		refresh, e := w.unlockRefresh(role)
		if e != nil {
			return nil, e
		}
		plan = append(plan, refresh...)
	}
	return plan, nil
}

func bagRestoreBody(bag inventory.Bag) ([]byte, error) {
	return protocol.InventoryRestore(bag.Rows(), bag.Expansion)
}

func petContainerDelta(before, after inventory.Bag) (bool, error) {
	if len(after.PetItems)+len(after.Special[7]) == 0 {
		return false, nil
	}
	afterBody, e := inventory.PetContainerBody(after, true)
	if e != nil {
		return false, e
	}
	if len(before.PetItems)+len(before.Special[7]) == 0 {
		return true, nil
	}
	beforeBody, e := inventory.PetContainerBody(before, true)
	if e != nil {
		return false, e
	}
	return !bytes.Equal(beforeBody, afterBody), nil
}
