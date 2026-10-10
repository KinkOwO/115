package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/database"
	"dfolan/internal/game/protocol"
	"dfolan/internal/loot"
	"dfolan/internal/workflow"
	"encoding/binary"
	"fmt"
	"log"
)

// boostChallengeFrameProven 是 NOTI2722 的推包门禁：包体布局没有被客户端证据确认时
// 只提交存档、不发帧。
//
// 关掉它的原因（实机 2026-10-06 15:50:39，会话 …_20261006_154752_229276_next37）：
// 第 11 关领奖成功后我方推的 2722 是 24 B 自造布局（u8 enrolled + u16 marker + u32
// count + 16 B/行），1.7 s 后客户端自己发出 CMD682 退出并落下
// `D:\DNF115\115US\DFO\CrashDNF2.cra`＝直接崩溃。
//
// 置真的依据（同日晚些的 IDA 闭环，产物 analysis/dumps/noti2722-{handler,records,consumer}/）：
// 处理器 sub_140BAE420 固定读取 323 B（sub_146EA0BE0(&buf,323)）再整块拷进 manager+328，
// 长度不足时该 reader 走 `MEMORY[0]=0`（写空地址）——旧 24 B 包体就是这么崩的；
// 槽位、字段与门禁分别由 sub_140BAF000（`manager+331+10*i`，i≤0x1F）、
// sub_140BAF640/6B0/6D0/EFC0/F0F0/F660 和 sub_140BAF450（@2==1）消费。
// 编码见 protocol.BoostChallengeStatus115；官服 16 帧（定长 328 B，末 5 B 处理器不读、
// 含义未闭环）与这里的 323 B 记录块逐字节对得上，故不跟着编那 5 B。
var boostChallengeFrameProven = true

// boostChallengePackets 是 2722 的唯一出口（毕业登记 / 装备解锁 / 665 查询与领奖三处共用）。
func boostChallengePackets(kind string, body []byte) []outboundPacket {
	if !boostChallengeFrameProven {
		return nil
	}
	return []outboundPacket{{kind, 0, 2722, body}}
}

// boostChallengeClearProgress 比较一次通关事务前后的挑战事实，只在确实变化时补一帧 2722。
//
// 665 的通关计数与经验在同一个角色事务里提交（`character.Clear` →
// `applyBoostChallengeClear`），结算链本身不知道它变没变；而客户端的挑战面板**只在收到
// 2722 时**刷新（处理器 sub_140BAE420 拷完记录块才去 refresh 窗口 0xECE）。整场只有毕业
// 登记与 680 领奖两处推过这一帧，所以实机 2026-10-06 16:43 出现「库里 progress=1、
// 面板仍显示 0/10」——计数是对的，缺的是通知。
//
// 与 2638 的 `boostMissionProgress` 同一口径：状态没动就不发（重复下发会让面板重播）。
func boostChallengeClearProgress(c *boostup.Catalog, before, after database.Character) []outboundPacket {
	if c == nil || len(c.Challenges) == 0 || before.ID == 0 || after.ID == 0 || before.ID != after.ID {
		return nil
	}
	a, e := boostup.ReadState(before.State)
	if e != nil {
		return nil
	}
	b, e := boostup.ReadState(after.State)
	if e != nil {
		return nil
	}
	if challengeFactsEqual(a.Challenge, b.Challenge) {
		return nil
	}
	body, e := loot.BoostChallengeSnapshot(c, workflow.LootRole(after))
	if e != nil {
		log.Printf("boost challenge clear snapshot role=%d: %v", after.ID, e)
		return nil
	}
	return boostChallengePackets("boost_challenge_clear_progress", body)
}

// boostChallengeEntryRestore 是进城/重登录时的 2722 恢复帧。只有**真已登记挑战**的角色
// 才出这一帧：其余角色返回 nil，`preparePackets` 跳帧，普通进城序列逐字节不变。
func boostChallengeEntryRestore(c *boostup.Catalog, role database.Character) ([]byte, error) {
	if !boostChallengeFrameProven || c == nil || len(c.Challenges) == 0 {
		return nil, nil
	}
	st, e := boostup.ReadState(role.State)
	if e != nil {
		return nil, e
	}
	if st.Challenge == nil || !st.Challenge.Enrolled {
		return nil, nil
	}
	return loot.BoostChallengeSnapshot(c, workflow.LootRole(role))
}

// boostChallengeEntrySync 是进城/重登录时的 665「对账 + 恢复帧」。
//
// 与 boostChallengeEntryRestore（纯投影）的唯一区别：**先把「已毕业」事实对账进挑战状态**
// （ReconcileBoostChallenge：判据 = activated && Training.Finished && Step>1 && Claimed[Step-1]，
// 幂等、只在变化时写），再按登记状态出帧。
//
// 为什么必须在进城路径对账（实机 2026-10-07 暴露）：登记此前只挂在副作用上 ——
// 穿脱装备（equipment_flow 的 reconcileBoostEquipment）、走训练步、662 领奖复核；
// 于是**正常毕业的角色进城看不到 665 面板，必须去动一次装备才出现**。而客户端全程
// 不发 CMD681（实机会话里 681 = 0 次），面板**只由服务端推的 2722 渲染**
// ⇒「推帧」就是接线本身，不能等副作用。
//
// 未毕业角色在 ReconcileBoostChallenge 里早退（零变化、零写入）⇒ 这里仍返回 nil，
// 进城序列逐字节不变。判据用「已毕业」而不是「满级」：满级但没走 662 的角色
// （例如奥德赛 115 级）不应被登记。
func (c *gameConnection) boostChallengeEntrySync(ctx context.Context, role database.Character) (database.Character, []byte, error) {
	w := c.worldState
	if !boostChallengeFrameProven || w == nil || w.boostup == nil || len(w.boostup.Challenges) == 0 || w.characters == nil {
		return role, nil, nil
	}
	next, _, e := (&workflow.LootService{Store: c.gameStore, Loot: c.lootService}).ReconcileBoostChallenge(ctx, role, w.boostup, w.characters.BoostChallengeFacts)
	if e != nil {
		return role, nil, e
	}
	body, e := boostChallengeEntryRestore(w.boostup, next)
	if e != nil {
		return next, nil, e
	}
	return next, body, nil
}

func challengeFactsEqual(a, b *boostup.ChallengeState) bool {
	if a == nil || b == nil {
		return a == b
	}
	if a.Enrolled != b.Enrolled || len(a.Rows) != len(b.Rows) {
		return false
	}
	for index, p := range a.Rows {
		if b.Rows[index] != p {
			return false
		}
	}
	return true
}

// boostChallengeRequest 处理 event 665（毕业后的可选挑战）：680 领奖、681 查询。
//
// 领奖幂等键取本树唯一口径（连接随机数 + 传输帧摘要，见 requestKeySession）；
// 没有传输帧（测试/内部重放）时不能用正文当帧，那会让同一正文的两次不同请求
// 撞同一个键。
func (w *worldSession) boostChallengeRequest(ctx context.Context, p []byte, id uint16, raw ...[]byte) ([]outboundPacket, error) {
	refuse := func(e error) ([]outboundPacket, error) {
		return []outboundPacket{{"boost_challenge_refused", 1, id, protocol.EventRefusal115(boostup.ChallengeEventID, 102, id == 681)}}, e
	}
	if w == nil || w.boostup == nil || len(w.boostup.Challenges) == 0 || w.loot == nil || w.characters == nil || w.role.ID == 0 || w.role.AccountID != w.account {
		return refuse(fmt.Errorf("owned challenge services unavailable"))
	}
	var req protocol.BoostChallengeRequest115
	if id == 680 {
		var e error
		req, e = protocol.DecodeBoostChallengeRequest115(p)
		if e != nil {
			return refuse(e)
		}
		if w.activeDungeon != nil || w.inTutorial || w.selectingDungeon || w.specialWarpPending {
			return refuse(fmt.Errorf("challenge reward requires town"))
		}
	} else {
		r, e := protocol.DecodeEventRequest115(p, false)
		if e != nil || r.Event != boostup.ChallengeEventID {
			return refuse(fmt.Errorf("invalid challenge query"))
		}
	}
	next, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).ReconcileBoostChallenge(ctx, w.role, w.boostup, w.characters.BoostChallengeFacts)
	if e != nil {
		return refuse(e)
	}
	w.role = next
	var values [10]uint32
	if id == 680 {
		frame := p
		if len(raw) == 1 {
			frame = raw[0]
		} else if len(raw) > 1 {
			return refuse(fmt.Errorf("ambiguous event transport frame"))
		}
		key, e := w.boostOperations.requestKey(append([]byte("boost-challenge|"), frame...))
		if e != nil {
			return refuse(e)
		}
		var sent bool
		next, sent, e = (&workflow.LootService{Store: w.store, Loot: w.loot}).ClaimBoostChallenge(ctx, w.role, w.boostup, req, key)
		if e != nil {
			return refuse(e)
		}
		w.role = next
		if sent && w.notifyBoostMail != nil {
			w.notifyBoostMail(w.role.ID)
		}
		values[0], values[1] = uint32(req.Index), uint32(req.Action)
	}
	body, e := loot.BoostChallengeSnapshot(w.boostup, workflow.LootRole(w.role))
	if e != nil {
		return refuse(e)
	}
	return append(boostChallengePackets("boost_challenge_status", body),
		outboundPacket{"boost_challenge_ack", 1, id, protocol.EventReply115(boostup.ChallengeEventID, values)}), nil
}

func isBoostChallengeRequest(p []byte) bool {
	return len(p) >= 4 && binary.LittleEndian.Uint32(p) == boostup.ChallengeEventID
}

// boostChallengeGoTeleport 授权 665 面板 Go 按钮的区域移动，判据来自源自身：
// 同一个 `[info]` 行的 `[go contents town area]`（town area x y）。落点取源值而不是
// 请求值，客户端无法用它选坐标。
//
// 没有这一支时的实机故障（会话 …_20261006_195540_767216 12:15:57、
// …_20261006_212930_180742 13:36:52/13:36:59）：CMD36 请求 241/1@143,173
// （`f1000000010000008f00ad000526000000010000…`，flag 5、tail 0），Seria 房的
// 离场判据把它当 departure 改写成存档的 Return 点，玩家被丢回 38/0 new_elvengard
// （业主口径「传送的位置不对」）。12:16:13 那次没有 Return 存档，则直接被普通门控
// 拒成「no authorized source portal to destination」。
func (w *worldSession) boostChallengeGoTeleport(r protocol.AreaChangeRequest, specialWarp bool) (database.WorldPosition, bool, error) {
	old := w.state.Position
	if specialWarp || w.boostup == nil || len(w.boostup.Challenges) == 0 || !w.ownedTownTeleport(r) {
		return old, false, nil
	}
	st, e := boostup.ReadState(w.role.State)
	if e != nil {
		// 读不出活动存档不等于拒绝：ownedTownTeleport 覆盖所有角色的 flag-5 移动，
		// 这里报错会把 NPC 移动/章节回城一起拒掉。交给原有路径判定。
		return old, false, nil
	}
	if st.Challenge == nil || !st.Challenge.Enrolled {
		return old, false, nil
	}
	for _, d := range w.boostup.Challenges {
		if !d.HasGoTarget || d.GoTarget[0] != r.Town || d.GoTarget[1] != r.Area || !st.Challenge.Rows[d.Index].Unlocked {
			continue
		}
		r.X, r.Y = uint16(d.GoTarget[2]), uint16(d.GoTarget[3])
		next, e := w.teleportTransition(old, r)
		return next, true, e
	}
	return old, false, nil
}
