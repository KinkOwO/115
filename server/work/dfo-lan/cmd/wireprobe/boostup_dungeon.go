package main

import (
	"context"
	"dfolan/internal/boostup"
	"dfolan/internal/character"
	"dfolan/internal/game/protocol"
	"dfolan/internal/workflow"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"time"
)

// [GAP] 训练房地图箱的「服务端往房间预放物件」没有可用承载帧：已逐字段确认 NOTI29
// （handler 0x1452B7100）在怪表之后只有附加演员/分组/他人演员三段，无 181B 物品记录
// 位置，而客户端砸箱不上行任何帧，服务端观察不到破坏事件。取证与候选方案见
// analysis/tasks/boostup662-mapobject-noti29-forensics-20261004.md；未闭环前不下发
// 物件内容（曾挂怪物死亡 NOTI38，位置语义错误，2026-10-04 回退）。

func (w *worldSession) boostGuideDungeon() (uint32, error) {
	if w.boostup == nil || w.role.ID == 0 {
		return 0, fmt.Errorf("boost guide unavailable")
	}
	event, e := boostup.ReadState(w.role.State)
	if e != nil {
		return 0, e
	}
	if !event.Activated || event.Training.Finished {
		return 0, fmt.Errorf("owned boost training required")
	}
	var role character.State
	if e = json.Unmarshal(w.role.State, &role); e != nil {
		return 0, e
	}
	prof, ok := w.professions.Professions[w.role.Profession]
	if !ok {
		return 0, fmt.Errorf("boost profession source missing")
	}
	return w.boostup.GuideDungeon(event.Training, prof.Job, role.Advancement, event.Variant)
}

// boostGuideSelection 回答「这次选图就是刚被放行的本关引导房吗」。引导房在 PVF 里是
// `[tutorial dungeon]`，普通 dungeon.Select 一律拒绝教程房，入口层要据此挑选择器；
// 判据与 authorizeBoostDungeon 完全同源（本职业本关的引导副本 ID），不另立一套规则。
func (w *worldSession) boostGuideSelection(id uint32) bool {
	if w.boostup == nil || w.state.Position.Town != w.boostup.Town {
		return false
	}
	expected, e := w.boostGuideDungeon()
	return e == nil && expected == id
}
func (w *worldSession) authorizeBoostDungeon(id uint32) error {
	expected, e := w.boostGuideDungeon()
	if e != nil {
		return e
	}
	if id != expected {
		return fmt.Errorf("guide dungeon differs from current profession/step")
	}
	// donor 的 sharedSelection.Generation 代际计数在本树不存在；busy 门禁用
	// activeDungeon/inTutorial/specialWarpPending（与 boostup_challenge.go 同一判据）。
	// selectingDungeon 不在其中：本树 CMD16 boost 分支要求它已置位（先过
	// CMD15 门），donor 测试序列 gate→select 也依赖这一点。
	if w.activeDungeon != nil || w.inTutorial || w.specialWarpPending {
		return fmt.Errorf("boost guide already has an active selection/run")
	}
	// [GAP] donor 基线的 worldSession.currentParty()（队伍子系统）在本树不存在：
	// "boost training is a personal instance" 的队伍人数门禁暂缺，
	// 本树也没有可与个人训练房冲突的共享选房链路；接入队伍时补回。
	st, e := boostup.ReadState(w.role.State)
	if e != nil {
		return e
	}
	if w.state.Position.Town != w.boostup.Town || w.state.Position.Area != uint32(w.boostup.Steps[int(st.Training.Step)-1].Area) {
		return fmt.Errorf("boost guide requires current event area")
	}
	return w.service.ValidatePosition(w.level, w.odyssey, w.state.Position)
}
func (w *worldSession) completeBoostGuide() ([]outboundPacket, error) {
	if w.boostup == nil || w.state.Position.Town != w.boostup.Town {
		return nil, nil
	}
	expected, e := w.boostGuideDungeon()
	if e != nil {
		return nil, e
	}
	if w.loot == nil {
		return nil, fmt.Errorf("boost guide storage missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	next, _, e := (&workflow.LootService{Store: w.store, Loot: w.loot}).CompleteBoostGuide(ctx, w.role, w.boostup, expected, w.activeDungeon)
	if e != nil {
		return nil, e
	}
	w.role = next
	status, e := boostTrainingRestore(w.boostup, next)
	if e != nil {
		return nil, e
	}
	return []outboundPacket{{"boost_guide_clear_progress", 0, 2638, status}}, nil
}
func boostGuideSelectionPayload() []byte {
	p := protocol.EnterDungeonSelection()
	// The four training C15/N27 samples share this1; retain empty character-
	// specific optional collections rather than copying another account's data.
	binary.LittleEndian.PutUint32(p[3:], 1)
	return p
}

func (w *worldSession) isBoostGuideRun() bool {
	if w.boostup == nil || w.activeDungeon == nil || w.state.Position.Town != w.boostup.Town || !w.activeDungeon.Definition.Tutorial {
		return false
	}
	expected, e := w.boostGuideDungeon()
	return e == nil && w.activeDungeon.Definition.ID == expected
}
