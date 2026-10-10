package main

import (
	"context"
	"dfolan/internal/character"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"dfolan/internal/inventory"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// 发奖白给：脚本在 level_up / quest_complete 里发的东西，是业务事务提交**之后**才落库的，
// 提交返回给网关的角色里没有它们，而结算帧全由那份旧状态编码 —— 物品进了库、帧里没有。
// rewardStateAfterCommit 就是编码前的那次回读；这组测试钉住它的两个方向：
// 变了必须认出来（否则还是白给），没变必须一个字节都不发（否则变成周期重发）。

const rewardReloadSource = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func rewardReloadWorld(t *testing.T) (*worldSession, database.Character) {
	t.Helper()
	ctx := context.Background()
	store, err := database.Open(ctx, database.Config{Driver: database.DriverSQLite, SQLitePath: filepath.Join(t.TempDir(), "reward-reload.sqlite3")})
	require.NoError(t, err)
	t.Cleanup(store.Close)
	require.NoError(t, store.Migrate(ctx))
	require.NoError(t, store.MigrateCharacterEvents(ctx))
	account, err := store.DevelopmentAccount(ctx, "reward-reload")
	require.NoError(t, err)
	creation := make([]byte, 12)
	creation[6] = 255
	state, err := json.Marshal(character.State{Level: 15, Advancement: 1, Awakening: 1,
		CreationOptions: creation, SourceSHA256: rewardReloadSource})
	require.NoError(t, err)
	state, err = inventory.SaveBag(state, inventory.Bag{Version: "ordinary-bag-v1"})
	require.NoError(t, err)
	role, err := store.CreateCharacter(ctx, database.Character{AccountID: account, Name: "ReloadRole",
		Profession: 0, ConfigVersion: rewardReloadSource, Request: creation, State: state}, 24)
	require.NoError(t, err)
	return &worldSession{store: store, account: account, role: role}, role
}

// grantAfterCommit 复刻奖励脚本的落库动作：一笔独立的角色事件事务，键与 model 同
// reward-item-v1 那条路径（cmd/wireprobe/reward_flow.go:290）。
func grantAfterCommit(t *testing.T, store *database.Store, role database.Character, item inventory.BagItem, gold uint32) {
	t.Helper()
	ctx := context.Background()
	_, _, err := store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion,
		"reward:level_up:level.lua:15:item", "reward-item-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			if item.Template != 0 {
				bag.Items = append(bag.Items, item)
			}
			if gold != 0 {
				bag.Gold += gold
			}
			next, e := inventory.SaveBag(current.State, bag)
			return next, json.RawMessage(`{"granted":1}`), e
		})
	require.NoError(t, err)
}

func TestRewardStateAfterCommitIsSilentWhenNothingChanged(t *testing.T) {
	w, role := rewardReloadWorld(t)
	ctx := context.Background()
	committed, d, err := w.rewardStateAfterCommit(ctx, role)
	require.NoError(t, err)
	require.Equal(t, role.State, committed.State)
	require.Equal(t, rewardDelta{}, d, "没人动过存档就不许报出任何变化")

	packets, err := w.rewardCatchUpPackets(committed, d)
	require.NoError(t, err)
	require.Empty(t, packets, "没变过就不许补帧，哪怕一帧")
}

func TestRewardStateAfterCommitCarriesPostCommitItemGrant(t *testing.T) {
	w, role := rewardReloadWorld(t)
	ctx := context.Background()
	grantAfterCommit(t, w.store, role, inventory.BagItem{Slot: 2, Template: 10300001, Amount: 5}, 0)

	committed, d, err := w.rewardStateAfterCommit(ctx, role)
	require.NoError(t, err)
	require.True(t, d.bag, "提交之后进包的东西必须被认出来")
	require.False(t, d.pet)
	require.False(t, d.addition)

	bag, err := inventory.ReadBag(committed.State)
	require.NoError(t, err)
	require.Len(t, bag.Items, 1)

	packets, err := w.rewardCatchUpPackets(committed, d)
	require.NoError(t, err)
	require.Len(t, packets, 1)
	require.Equal(t, uint16(13), packets[0].ID)
	require.Equal(t, byte(0), packets[0].Kind)
	// 帧体就是「库里那份」的全量背包投影，与 finishQuest 用的同一构造。
	body, err := protocol.InventoryRestore(bag.Rows(), bag.Expansion)
	require.NoError(t, err)
	require.Equal(t, body, packets[0].Payload)
}

func TestRewardStateAfterCommitCarriesGoldOnlyGrant(t *testing.T) {
	w, role := rewardReloadWorld(t)
	ctx := context.Background()
	grantAfterCommit(t, w.store, role, inventory.BagItem{}, 4300)

	committed, d, err := w.rewardStateAfterCommit(ctx, role)
	require.NoError(t, err)
	require.True(t, d.bag, "只给金币也是奖励：金币行就在背包投影里，不补帧钱包就不动")

	packets, err := w.rewardCatchUpPackets(committed, d)
	require.NoError(t, err)
	require.Len(t, packets, 1)
}

// 装备栏挂锁/扩容这类待遇只能由 USERINFO1 投影，而客户端只在登录/选角/进本构造装备栏
// 行对象：副本内补发会把装备栏显示清空（dungeon_flow.go:1786）。所以副本内一律转成回城补发门。
func TestRewardActorAdditionDefersInsideDungeon(t *testing.T) {
	w, _ := rewardReloadWorld(t)
	d := rewardDelta{addition: true}
	w.deferActorAdditionInsideDungeon(&d)
	require.True(t, d.addition, "在城里就照发，不该被转走")
	require.False(t, w.slotUnlockDirty)

	w.activeDungeon = &dungeon.Session{}
	w.deferActorAdditionInsideDungeon(&d)
	require.False(t, d.addition, "副本内不许发 USERINFO1")
	require.True(t, w.slotUnlockDirty, "改成回城那一次补发，玩家不必重登")

	// 补帧器自己绝不发明包：没有角色目录服务时 USERINFO1 三件套编不出来，就只能零帧。
	packets, err := w.rewardCatchUpPackets(w.role, rewardDelta{addition: true})
	require.NoError(t, err)
	require.Empty(t, packets)
}

func TestRewardCatchUpSendsPetContainerOnlyWithPets(t *testing.T) {
	w, role := rewardReloadWorld(t)
	ctx := context.Background()
	_, _, err := w.store.CommitCharacterEvent(ctx, role.AccountID, role.ID, role.ConfigVersion,
		"reward:level_up:level.lua:15:state", "reward-entitlement-v1",
		func(current database.Character) (json.RawMessage, json.RawMessage, error) {
			bag, e := inventory.ReadBag(current.State)
			if e != nil {
				return nil, nil, e
			}
			bag.PetItems = append(bag.PetItems, inventory.BagItem{Slot: 376, Template: 10500001, Amount: 1})
			next, e := inventory.SaveBag(current.State, bag)
			return next, json.RawMessage(`{"granted":1}`), e
		})
	require.NoError(t, err)

	committed, d, err := w.rewardStateAfterCommit(ctx, role)
	require.NoError(t, err)
	require.False(t, d.bag, "宠物容器不在 Rows() 里，普通背包应当判定为没变")
	require.True(t, d.pet)

	packets, err := w.rewardCatchUpPackets(committed, d)
	require.NoError(t, err)
	require.Len(t, packets, 1)
	require.Equal(t, uint16(13), packets[0].ID)
}
