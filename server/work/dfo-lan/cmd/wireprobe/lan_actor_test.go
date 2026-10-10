package main

import (
	"dfolan/internal/modactor"
	"testing"
)

// actorSpec 造一份最小可用的假人请求，测试里按需改字段。
func actorSpec(name string) modactor.Spec {
	return modactor.Spec{
		Name:       name,
		Profession: 3,
		Level:      60,
		Channel:    1,
		Town:       2,
		Area:       3,
		X:          100,
		Y:          200,
		Source:     "test",
	}
}

// TestActorIsVisibleToRealPlayer 是这条能力的核心断言：
// 造出来的假人必须出现在真人的区域列表里，并且真人收到过它的 NOTI 2 介绍 ——
// 只进花名册不介绍的表现是"列表里多一行但画不出人"。
func TestActorIsVisibleToRealPlayer(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	var got []capture
	player := allPeer(7, 40, &got)
	hub.add(player)
	hub.publish(player, 1, 2, 3, 10, 20, false)

	id, err := host.Spawn(actorSpec("假人甲"))
	if err != nil {
		t.Fatalf("Spawn 失败：%v", err)
	}
	if id < actorIDBase {
		t.Fatalf("假人 id 应落在 %#x 起的保留段，实际 %#x", actorIDBase, id)
	}
	roster := hub.roster(player)
	if len(roster) != 2 {
		t.Fatalf("真人应看到自己 + 假人共 2 个 actor，实际 %d", len(roster))
	}
	found := false
	for _, p := range roster {
		if p.actorID == uint16(id) {
			found = true
			if p.info == nil {
				t.Fatal("假人缺 NOTI2 mode0 载荷")
			}
			if p.addition == nil {
				t.Fatal("假人缺 NOTI2 mode1 载荷")
			}
		}
	}
	if !found {
		t.Fatal("区域列表里没有假人")
	}
	// 进了同一场景就该被介绍：mode0 + mode1 两条 NOTI 2，外加一条 NOTI 23 落位。
	if n := countCapture(got, 2); n < 2 {
		t.Fatalf("真人应收到假人的两条 NOTI 2（mode0/mode1），实际 %d 条", n)
	}
	if findCapture(got, 23) == nil {
		t.Fatal("真人应收到假人的 NOTI 23 落位")
	}
}

// TestActorMoveBroadcastsToScene 断言假人动的时候同场景的人看得到。
// 假人自己没有报文源，这一步只能由 Move 推。
func TestActorMoveBroadcastsToScene(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	var got []capture
	player := allPeer(7, 40, &got)
	hub.add(player)
	hub.publish(player, 1, 2, 3, 10, 20, false)

	id, err := host.Spawn(actorSpec("假人乙"))
	if err != nil {
		t.Fatalf("Spawn 失败：%v", err)
	}
	got = nil
	if err = host.Move(id, 150, 250, 1, 60); err != nil {
		t.Fatalf("Move 失败：%v", err)
	}
	if findCapture(got, 22) == nil {
		t.Fatal("同场景真人应收到 NOTI 22 位置更新")
	}
	pose, ok := host.Pose(id)
	if !ok || pose.X != 150 || pose.Y != 250 || pose.Motion != 1 || pose.Speed != 60 {
		t.Fatalf("Move 之后姿态不对：%+v ok=%v", pose, ok)
	}
}

// TestActorMoveInDifferentAreaIsInvisible 断言不在同一场景的人收不到 ——
// 广播必须走 hub 的可见性判定，不能图省事群发。
func TestActorMoveInDifferentAreaIsInvisible(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	var got []capture
	other := allPeer(8, 41, &got)
	hub.add(other)
	hub.publish(other, 1, 2, 99, 10, 20, false) // 不同区域

	id, err := host.Spawn(actorSpec("假人丙"))
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err = host.Move(id, 150, 250, 1, 60); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("不同区域的玩家不该收到任何推送，实际 %d 条", len(got))
	}
}

// TestActorRelocateMovesItToAnotherArea 断言换区之后：旧场景不再看到它、
// 新场景能看到它。
func TestActorRelocateMovesItToAnotherArea(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	var oldGot, newGot []capture
	old := allPeer(7, 40, &oldGot)
	hub.add(old)
	hub.publish(old, 1, 2, 3, 10, 20, false)
	id, err := host.Spawn(actorSpec("假人丁"))
	if err != nil {
		t.Fatal(err)
	}

	fresh := allPeer(9, 42, &newGot)
	hub.add(fresh)
	hub.publish(fresh, 1, 2, 77, 10, 20, false)
	oldGot, newGot = nil, nil

	if err = host.Relocate(id, 1, 2, 77, 30, 40); err != nil {
		t.Fatalf("Relocate 失败：%v", err)
	}
	if n := countCapture(newGot, 2); n < 2 {
		t.Fatalf("新场景应收到假人的两条 NOTI 2，实际 %d", n)
	}
	// 旧场景收到的是 NOTI 23（带新场景号的落位更新）：客户端据此把 actor 摘掉。
	if findCapture(oldGot, 23) == nil {
		t.Fatal("旧场景应收到 NOTI 23")
	}
	if hub.roster(old) != nil && len(hub.roster(old)) != 1 {
		t.Fatalf("旧场景不应再看到假人，实际 %+v", hub.roster(old))
	}
}

// TestActorDespawnRemovesIt 断言撤走之后别人收到 NOTI 6，且它不在花名册里了。
func TestActorDespawnRemovesIt(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	var got []capture
	player := allPeer(7, 40, &got)
	hub.add(player)
	hub.publish(player, 1, 2, 3, 10, 20, false)
	id, err := host.Spawn(actorSpec("假人戊"))
	if err != nil {
		t.Fatal(err)
	}
	got = nil
	if err = host.Despawn(id); err != nil {
		t.Fatalf("Despawn 失败：%v", err)
	}
	if findCapture(got, 6) == nil {
		t.Fatal("撤走时应广播 NOTI 6")
	}
	if _, ok := host.Pose(id); ok {
		t.Fatal("Despawn 之后 Pose 应查不到")
	}
	if len(host.List()) != 0 {
		t.Fatal("Despawn 之后 List 应为空")
	}
	if err = host.Despawn(id); err == nil {
		t.Fatal("重复 Despawn 应报未知实体")
	}
}

// TestActorIDSkipsTakenIDs 断言 id 分配既避开真实连接已用的，也避开已分配给假人的。
func TestActorIDSkipsTakenIDs(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)
	// 占住保留段的头两个：一个真人、一个假人。
	taken := allPeer(7, actorIDBase, &[]capture{})
	hub.add(taken)
	hub.publish(taken, 1, 2, 3, 0, 0, false)

	first, err := host.Spawn(actorSpec("甲"))
	if err != nil {
		t.Fatal(err)
	}
	if uint16(first) == actorIDBase {
		t.Fatalf("id 分配应跳过真实连接已占用的 %#x", actorIDBase)
	}
	second, err := host.Spawn(actorSpec("乙"))
	if err != nil {
		t.Fatal(err)
	}
	if second == first {
		t.Fatal("两个假人不应拿到同一个 id")
	}
}

// TestActorSpawnRejectsBadWorn 断言穿戴写错时**整个不建**，而不是建一个画不出来的。
func TestActorSpawnRejectsBadWorn(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	noTemplate := actorSpec("缺模板")
	noTemplate.Worn = []modactor.WornSlot{{Slot: 12, ItemTemplate: 0, Model: 5}}
	if _, err := host.Spawn(noTemplate); err == nil {
		t.Fatal("缺装备模板应被拒绝（客户端会找不到城镇模型）")
	}

	badSlot := actorSpec("槽越界")
	badSlot.Worn = []modactor.WornSlot{{Slot: 99, ItemTemplate: 1}}
	if _, err := host.Spawn(badSlot); err == nil {
		t.Fatal("超出客户端外观表的槽位应被拒绝")
	}

	badLevel := actorSpec("强化越界")
	badLevel.Worn = []modactor.WornSlot{{Slot: 12, ItemTemplate: 1, UpgradeLevel: 99}}
	if _, err := host.Spawn(badLevel); err == nil {
		t.Fatal("强化等级 >31 应被拒绝（打包字段会回绕）")
	}

	dup := actorSpec("槽重复")
	dup.Worn = []modactor.WornSlot{{Slot: 12, ItemTemplate: 1}, {Slot: 12, ItemTemplate: 2}}
	if _, err := host.Spawn(dup); err == nil {
		t.Fatal("同一槽位重复应被拒绝")
	}
	if len(host.List()) != 0 {
		t.Fatal("被拒绝的 spec 不该留下任何实体")
	}
}

// TestActorHostInstallRoundTrip 断言 modactor 侧的代理真能走到本宿主 ——
// 这是"mod 调 modactor.Spawn → 实体真的出现在 hub 里"这条链的端到端检查。
func TestActorHostInstallRoundTrip(t *testing.T) {
	modactor.Uninstall()
	defer modactor.Uninstall()
	hub := newLanHub()
	if err := modactor.Install(newLanActorHost(hub), "test"); err != nil {
		t.Fatalf("Install 失败：%v", err)
	}
	id, err := modactor.Spawn(actorSpec("端到端"))
	if err != nil {
		t.Fatalf("modactor.Spawn 失败：%v", err)
	}
	snaps := modactor.List()
	if len(snaps) != 1 || snaps[0].ID != id {
		t.Fatalf("List 应回这一个假人，实际 %+v", snaps)
	}
	if snaps[0].Pose.Town != 2 || snaps[0].Pose.X != 100 {
		t.Fatalf("落点不对：%+v", snaps[0].Pose)
	}
	if _, ok := modactor.PoseOf(id); !ok {
		t.Fatal("PoseOf 应查得到")
	}
	if err = modactor.Despawn(id); err != nil {
		t.Fatalf("modactor.Despawn 失败：%v", err)
	}
	if len(modactor.List()) != 0 {
		t.Fatal("撤走后应为空")
	}
}

// TestInstallActorHostIsIdempotent 断言重复装载不报错：runGateway 在同一进程里
// 会被反复起停，第二次进来不能因为这个失败。
func TestInstallActorHostIsIdempotent(t *testing.T) {
	modactor.Uninstall()
	defer modactor.Uninstall()
	if err := installActorHost(newLanHub()); err != nil {
		t.Fatalf("首次装载失败：%v", err)
	}
	if err := installActorHost(newLanHub()); err != nil {
		t.Fatalf("重复装载不该报错：%v", err)
	}
	uninstallActorHost()
	if modactor.Available() {
		t.Fatal("uninstallActorHost 之后应不可用了")
	}
}

// TestActorHostScenesReportsWherePlayersAre 断言场景查询只报**有真人**的场景，
// 且把假人排除在外 —— 假人要是能被自己"撑"出一个场景，就会出现没人时假人
// 互相作伴的怪相。
func TestActorHostScenesReportsWherePlayersAre(t *testing.T) {
	hub := newLanHub()
	host := newLanActorHost(hub)

	if got := host.Scenes(); len(got) != 0 {
		t.Fatalf("一个人都没有时应返回空，实际 %+v", got)
	}

	player := allPeer(7, 40, &[]capture{})
	hub.add(player)
	hub.publish(player, 1, 2, 3, 10, 20, false)
	// 私人实例（副本内）不算：假人不该被扔进别人的副本。
	private := allPeer(8, 41, &[]capture{})
	hub.add(private)
	hub.publish(private, 1, 2, 55, 0, 0, true)

	if _, err := host.Spawn(actorSpec("假人")); err != nil {
		t.Fatal(err)
	}
	got := host.Scenes()
	if len(got) != 1 {
		t.Fatalf("应只报有真人的那一个场景，实际 %+v", got)
	}
	if got[0].Channel != 1 || got[0].Town != 2 || got[0].Area != 3 || got[0].Players != 1 {
		t.Fatalf("场景内容不对：%+v", got[0])
	}
}

// TestActorNilHubDegradesQuietly 断言 hub 缺失时不会 panic —— 与 lanHub 各方法
// 自身的 nil 容忍口径一致。
func TestActorNilHubDegradesQuietly(t *testing.T) {
	host := newLanActorHost(nil)
	id, err := host.Spawn(actorSpec("无hub"))
	if err != nil {
		t.Fatalf("hub 为 nil 时 Spawn 也不该失败（只是放不进场景）：%v", err)
	}
	if err = host.Move(id, 1, 1, 0, 0); err != nil {
		t.Fatalf("hub 为 nil 时 Move 应空转：%v", err)
	}
	if err = host.Relocate(id, 1, 1, 1, 0, 0); err != nil {
		t.Fatalf("hub 为 nil 时 Relocate 应空转：%v", err)
	}
	if err = host.Despawn(id); err != nil {
		t.Fatalf("hub 为 nil 时 Despawn 应空转：%v", err)
	}
}
