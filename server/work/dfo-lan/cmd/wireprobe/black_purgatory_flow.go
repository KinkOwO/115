package main

import (
	"context"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"fmt"
	"time"
)

// 当前原版blackpurgatory.etc的小队模式，不开放尚未接入的多人、引导模式。
const blackPurgatorySquadDungeon uint32 = 100000527
const blackPurgatoryEntryFatigue uint16 = 8
const blackPurgatoryDuration = 1500 * time.Second

type blackPurgatoryState struct {
	created       bool
	prepared      bool
	deadline      time.Time
	loaded        bool
	returnToLobby bool
	quotaDay      time.Time
}

func (w *worldSession) blackPurgatoryQuota(ctx context.Context, run, action string, now time.Time) (database.BlackPurgatoryQuota, time.Time, error) {
	if w.characters == nil || w.store == nil || w.fatigue == nil || w.fatigue.Location == nil {
		return database.BlackPurgatoryQuota{}, time.Time{}, fmt.Errorf("黑鸦次数存储或重置时间配置不可用")
	}
	local := now.In(w.fatigue.Location)
	if local.Hour() < w.fatigue.Rules.ResetHour {
		local = local.AddDate(0, 0, -1)
	}
	day := time.Date(local.Year(), local.Month(), local.Day(), w.fatigue.Rules.ResetHour, 0, 0, 0, w.fatigue.Location)
	// dungeonincountinfo.etc的周重置日=2（周二）；时区及小时沿用本服配置。
	week := day.AddDate(0, 0, -(int(day.Weekday())+7-int(time.Tuesday))%7)
	quota, err := w.store.BlackPurgatoryQuota(ctx, w.account, w.role.ID, run, action, day, week)
	if action != "read" {
		w.blackPurgatory.quotaDay = time.Time{}
	}
	return quota, day, err
}

func (w *worldSession) refreshBlackPurgatoryQuota(now time.Time) ([]outboundPacket, error) {
	if w.channelType != 73 || w.role.ID == 0 {
		return nil, nil
	}
	if !w.blackPurgatory.quotaDay.IsZero() && now.Before(w.blackPurgatory.quotaDay.AddDate(0, 0, 1)) {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	quota, day, err := w.blackPurgatoryQuota(ctx, "", "read", now)
	if err != nil {
		return nil, err
	}
	w.blackPurgatory.quotaDay = day
	return []outboundPacket{{"黑鸦共享入场次数同步（attempt 1/3）", 0, 537, protocol.BlackPurgatoryRemaining(quota.Daily, quota.Weekly)}}, nil
}

func (w *worldSession) finishBlackPurgatoryQuota(action string) error {
	if w.channelType != 73 || w.activeDungeon == nil || w.activeDungeon.Definition.ID != blackPurgatorySquadDungeon {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, _, err := w.blackPurgatoryQuota(ctx, w.activeDungeon.RunID, action, time.Now())
	return err
}

func (w *worldSession) blackPurgatoryHandle(ctx context.Context, id uint16, p []byte) (bool, []outboundPacket, error) {
	if w.channelType != 73 || w.role.ID == 0 || (id != 12 && id != 13) {
		return false, nil, nil
	}
	fail := func(err error) (bool, []outboundPacket, error) { return true, nil, err }
	if w.characters == nil || w.service == nil || w.role.AccountID != w.account {
		return fail(fmt.Errorf("黑鸦角色或城镇服务不可用"))
	}
	pos := w.state.Position
	switch id {
	case 12:
		name, err := protocol.DecodeBlackPurgatoryParty(p)
		if err != nil {
			return fail(err)
		}
		if pos.Town != 85 || pos.Area != 1 && pos.Area != 2 || w.activeDungeon != nil ||
			w.selectingDungeon || w.blackPurgatory.prepared || w.inTutorial || w.specialWarpPending {
			return fail(fmt.Errorf("请在黑鸦大厅创建小队，挑战中不能更改队伍"))
		}
		if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, pos); err != nil {
			return fail(err)
		}
		quota, _, err := w.blackPurgatoryQuota(ctx, "", "read", time.Now())
		if err != nil {
			return fail(err)
		}
		if quota.Daily == 0 || quota.Weekly == 0 {
			return fail(fmt.Errorf("黑鸦之境剩余入场次数不足"))
		}
		party, err := protocol.BlackPurgatorySoloParty(w.role.WireID, w.characters.ChannelContext, name)
		if err != nil {
			return fail(err)
		}
		basic, err := w.characters.EntryBasicProbe(w.role, w.characters.ChannelContext)
		if err != nil {
			return fail(err)
		}
		detail, err := w.characters.EntryAddition(w.role)
		if err != nil {
			return fail(err)
		}
		w.blackPurgatory = blackPurgatoryState{created: true}
		w.soloPartyReady = true
		// NOTI9恢复真正的队伍，后续仍走原生85/1→85/2区域请求。
		return true, []outboundPacket{
			{"黑鸦队长资料", 0, 2, basic}, {"黑鸦队长详细资料", 0, 2, detail},
			{"黑鸦小队创建（attempt 1/3）", 0, 9, party},
			{"黑鸦准备状态", 0, 1994, protocol.BlackPurgatoryEntryInfo(0, 0)},
		}, nil
	case 13:
		if len(p) > 8 {
			return fail(fmt.Errorf("黑鸦离队请求长度无效"))
		}
		for _, b := range p {
			if b != 0 {
				return fail(fmt.Errorf("不支持的黑鸦离队选项"))
			}
		}
		if !w.blackPurgatory.created || w.activeDungeon != nil || w.blackPurgatory.prepared {
			return fail(fmt.Errorf("请先结束当前黑鸦挑战再离队"))
		}
		w.blackPurgatory = blackPurgatoryState{returnToLobby: pos.Town == 85 && pos.Area == 2}
		w.soloPartyReady = false
		return true, []outboundPacket{
			{"黑鸦离队状态", 0, 1994, protocol.BlackPurgatoryEntryInfo(0, 0)},
			{"黑鸦小队解散", 0, 9, protocol.BlackPurgatoryPartyGone(w.characters.ChannelContext)},
		}, nil
	}
	return false, nil, nil
}

// 开场只初始化黑鸦管理器，不会自动调用选图发送器142A29390。
// CMD1852必须接上NOTI27的场景初始化、NOTI28副本资料和NOTI29首图加载；
// 会话由主循环在全部通知写出后提交，首次CMD37再扣次数、疲劳并开始计时。
func (w *worldSession) startBlackPurgatory(ctx context.Context, p []byte) (*dungeon.Session, []outboundPacket, error) {
	if w.channelType != 73 || w.role.ID == 0 || w.characters == nil || w.service == nil || w.role.AccountID != w.account {
		return nil, nil, fmt.Errorf("黑鸦角色、频道或城镇服务不可用")
	}
	pos := w.state.Position
	// 142A2D230仅发送命令头；14525C4C0的成功分支不负责加载地图。
	if len(p) != 0 || !w.blackPurgatory.created || pos.Town != 85 || pos.Area != 2 ||
		w.inTutorial || w.specialWarpPending || w.pendingTownArrival != nil {
		return nil, nil, fmt.Errorf("请先在黑鸦之境等待区创建小队再出发")
	}
	if w.blackPurgatory.prepared && w.activeDungeon != nil && w.activeDungeon.Definition.ID == blackPurgatorySquadDungeon {
		return nil, []outboundPacket{{"黑鸦重复出发确认", 1, 1852, []byte{1}}}, nil
	}
	if w.activeDungeon != nil || w.selectingDungeon || w.dungeons == nil || w.fatigue == nil {
		return nil, nil, fmt.Errorf("黑鸦副本或疲劳服务不可用，或当前已有副本会话")
	}
	if w.loot == nil || w.loot.BlackPurgatory == nil || w.loot.CardPolicy == nil {
		return nil, nil, fmt.Errorf("黑鸦奖励目录未完整加载，不能开始消耗次数的挑战")
	}
	if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, pos); err != nil {
		return nil, nil, err
	}
	quota, _, err := w.blackPurgatoryQuota(ctx, "", "read", time.Now())
	if err != nil {
		return nil, nil, err
	}
	if quota.Daily == 0 || quota.Weekly == 0 {
		return nil, nil, fmt.Errorf("黑鸦之境剩余入场次数不足")
	}
	definition, ok := w.dungeons.Dungeons[blackPurgatorySquadDungeon]
	if !ok || definition.Script.Path != "dungeon/kcontents2/black_purgatory_guide/black_purgatory_squard.dgn" || !definition.NoFatigue {
		return nil, nil, fmt.Errorf("黑鸦小队源副本配置不匹配")
	}
	fp, err := w.fatigue.State(ctx, w.account, w.role.ID, time.Now())
	if err != nil {
		return nil, nil, err
	}
	if fp.Used >= fp.Limit || fp.Limit-fp.Used < blackPurgatoryEntryFatigue {
		return nil, nil, fmt.Errorf("黑鸦之境入场需要8点疲劳")
	}
	next, entry, err := w.prepareDungeonEntry(protocol.DungeonSelection{ID: blackPurgatorySquadDungeon, Difficulty: 2, Party: 65535})
	if err != nil {
		return nil, nil, fmt.Errorf("黑鸦小队地图无法加载：%w", err)
	}
	// 开场前读取当前账号精锐的真实装备和技能，不能把矿区轮换队伍塞入小队。
	elite, err := w.loadAdventureElite(ctx, []byte{2, 0, 0, 0, 0, 0, 0, 0})
	if err != nil {
		return nil, nil, err
	}
	plan := append(elite,
		outboundPacket{"黑鸦出发确认", 1, 1852, []byte{1}},
		outboundPacket{"黑鸦开场（attempt 2/3）", 0, 1994, protocol.BlackPurgatoryEntryInfo(1, 0)},
		// 145303260经146D4B530初始化加载上下文，1452AC840/1452B7100消费后续地图。
		outboundPacket{"黑鸦入场场景初始化", 0, 27, protocol.EnterDungeonSelection()},
	)
	// 没有CMD16请求，不发送公共入口生成的ACK16；保留其余真实角色和地图通知。
	plan = append(plan, entry[1:]...)
	w.blackPurgatory.prepared = true
	return next, plan, nil
}

func (w *worldSession) validateBlackPurgatoryArea(r protocol.AreaChangeRequest) error {
	if r.Town == 85 && (w.channelType != 73 || r.Area > 2) {
		return fmt.Errorf("黑鸦区域仅在黑鸦频道开放")
	}
	if w.channelType != 73 {
		return nil
	}
	if w.blackPurgatory.prepared && (r.Town != w.state.Position.Town || r.Area != w.state.Position.Area) {
		return fmt.Errorf("黑鸦正在入场，请先结束当前挑战")
	}
	if r.Town == 85 && r.Area == 2 && !w.blackPurgatory.created {
		return fmt.Errorf("请先创建黑鸦小队再进入等待区")
	}
	return nil
}

func (w *worldSession) validateBlackPurgatoryDungeon(id uint32) error {
	if w.channelType != 73 || !w.blackPurgatory.created || !w.blackPurgatory.prepared ||
		w.activeDungeon != nil || w.state.Position.Town != 85 || w.state.Position.Area != 2 || id != blackPurgatorySquadDungeon {
		return fmt.Errorf("黑鸦入场缺少当前小队的出发许可")
	}
	return w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position)
}

func (w *worldSession) blackPurgatoryLoaded(ctx context.Context) ([]outboundPacket, error) {
	if w.channelType != 73 || !w.blackPurgatory.prepared || w.activeDungeon == nil ||
		w.activeDungeon.Definition.ID != blackPurgatorySquadDungeon || w.blackPurgatory.loaded {
		return nil, nil
	}
	if w.fatigue == nil {
		return nil, fmt.Errorf("黑鸦入场疲劳服务不可用")
	}
	now := time.Now()
	currentFatigue, err := w.fatigue.State(ctx, w.account, w.role.ID, now)
	if err != nil {
		return nil, err
	}
	d := w.activeDungeon
	if _, _, err := w.blackPurgatoryQuota(ctx, d.RunID, "enter", now); err != nil {
		return nil, err
	}
	// 复用存储层RunID+地图幂等回执，首次加载扣8点；重发加载及以后过图不再扣。
	fp, _, err := w.fatigue.Store.ConsumeRoomFatigue(ctx, w.account, w.role.ID, w.fatigue.Day(now), currentFatigue.Limit,
		d.RunID, d.Room.Map, blackPurgatoryEntryFatigue)
	if err != nil {
		return nil, err
	}
	payload, err := protocol.Fatigue(min(fp.Used, fp.Limit), fp.Limit, fp.UsedMax)
	if err != nil {
		return nil, err
	}
	w.blackPurgatory.loaded = true
	w.blackPurgatory.deadline = now.Add(blackPurgatoryDuration)
	return []outboundPacket{
		{"黑鸦入场疲劳同步", 0, 36, payload},
		{"黑鸦挑战开始", 0, 1994, protocol.BlackPurgatoryEntryInfo(2, uint32(blackPurgatoryDuration/time.Second))},
	}, nil
}

func (w *worldSession) blackPurgatoryTimeout(now time.Time) ([]outboundPacket, error) {
	if w.channelType != 73 || !w.blackPurgatory.loaded || w.blackPurgatory.deadline.IsZero() ||
		now.Before(w.blackPurgatory.deadline) || w.activeDungeon == nil || w.activeDungeon.Completed() {
		return nil, nil
	}
	packets, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	// 主动超时使用城镇通知，不伪造玩家的CMD42请求。
	w.activeDungeon = nil
	w.selectingDungeon = false
	w.approvedDungeonGate = 0
	w.drops = nil
	w.deathSent = nil
	w.pilotDeath = nil
	w.completionSent = false
	w.completionErr = nil
	w.resultSent = false
	w.resetCards()
	return packets[1:], nil
}

// 黑鸦位置仅在连接内使用，不写入玩家普通频道城镇落点。
func blackPurgatoryEntry() database.WorldPosition {
	return database.WorldPosition{Town: 85, Area: 0, X: 565, Y: 234}
}
