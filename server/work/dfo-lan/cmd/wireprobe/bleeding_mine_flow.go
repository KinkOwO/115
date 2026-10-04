package main

import (
	"context"
	"crypto/sha256"
	"dfolan/internal/adventure"
	"dfolan/internal/database"
	"dfolan/internal/dungeon"
	"dfolan/internal/game/protocol"
	"encoding/binary"
	"fmt"
	"strings"
	"time"
)

// 当前源bleedingmine.ctp的preset 0，按easy/medium/hard及sequence 0..3排列。
// NOTI2702只传preset索引，客户端从同一份CTP读取地图，不能自行选择另一套地图。
var bleedingMineRoutes = [3][4]uint32{
	{100004314, 100004315, 100004316, 100004317},
	{100004321, 100004318, 100004319, 100004320},
	{100004324, 100004325, 100004322, 100004323},
}

var bleedingMineRequiredFame = [3]uint32{33249, 47488, 69300}

type bleedingMineStart struct {
	Stage           uint32
	Week            string
	StageStarted    time.Time
	Carry           time.Duration
	Elapsed         uint32
	StageResult     []byte
	Failed          bool
	Group           uint32
	Members         [4]int64
	Dungeon         uint32
	TeamPacket      []byte
	CharacterPacket []byte
	Deadline        time.Time
	Revives         map[[32]byte]bool
}

// 当前12份源DGN的[dungeon timeout]均为180秒；CTP的[revival use second]为20秒。
const bleedingMineTimeLimit = 180 * time.Second
const bleedingMineReviveCost = 20 * time.Second

func (w *worldSession) bleedingMineTimer(now time.Time) []outboundPacket {
	mine := w.bleedingMineStart
	if mine == nil || w.activeDungeon == nil || w.activeDungeon.Definition.ID != mine.Dungeon {
		return nil
	}
	if mine.Deadline.IsZero() {
		mine.StageStarted = now
		mine.Deadline = now.Add(bleedingMineTimeLimit + mine.Carry)
	}
	remaining := max(mine.Deadline.Sub(now).Milliseconds(), 0)
	return []outboundPacket{{"赤红铁矿剩余时间同步", 0, 1474, protocol.BleedingMineTimer(uint32(remaining))}}
}

func (w *worldSession) reviveBleedingMine(p, frame []byte, now time.Time) ([]outboundPacket, error) {
	// 141416BB0发送空正文；14073CF30成功分支调用140742AF0，直接恢复
	// 当前轮换角色并在客户端扣20秒，不使用普通复活币或CMD40死亡记录。
	if len(p) != 0 || w.channelType != 106 || w.bleedingMineStart == nil ||
		w.activeDungeon == nil || !w.activeDungeon.Loaded || w.resultSent || w.bleedingMineStart.Failed ||
		w.activeDungeon.Definition.ID != w.bleedingMineStart.Dungeon {
		return nil, fmt.Errorf("赤红铁矿复活需要当前已加载的挑战")
	}
	mine := w.bleedingMineStart
	key := sha256.Sum256(frame)
	// 原生成功处理器会再次扣时，重复帧不能重发成功应答。
	if mine.Revives[key] {
		return nil, nil
	}
	if mine.Deadline.IsZero() || mine.Deadline.Sub(now) < 30*time.Second {
		return nil, fmt.Errorf("赤红铁矿剩余时间不足30秒，无法立即复活")
	}
	if mine.Revives == nil {
		mine.Revives = make(map[[32]byte]bool)
	}
	mine.Revives[key] = true
	mine.Deadline = mine.Deadline.Add(-bleedingMineReviveCost)
	if w.pilotDeath != nil && w.pilotDeath.Run == w.activeDungeon.RunID {
		w.pilotDeath.Dead = false
	}
	// 应答先让原生复活并扣时，再以服务端剩余时间校准，避免客户端重复扣20秒。
	return append([]outboundPacket{{"赤红铁矿扣时复活", 1, 2327, []byte{1}}}, w.bleedingMineTimer(now)...), nil
}

func (w *worldSession) bleedingMineTimeout(now time.Time) ([]outboundPacket, error) {
	if w == nil || w.bleedingMineStart == nil || w.activeDungeon == nil ||
		w.bleedingMineStart.Deadline.IsZero() || now.Before(w.bleedingMineStart.Deadline) ||
		w.activeDungeon.Completed() || w.resultSent || w.bleedingMineStart.Failed {
		return nil, nil
	}
	w.bleedingMineStart.Failed = true
	// 保留已通关阶段，由原生失败面板选择结束探索或放弃，不能自动吞掉奖励。
	return []outboundPacket{{"赤红铁矿阶段超时", 0, 2706, protocol.BleedingMineFailed()}}, nil
}

func (w *worldSession) endBleedingMine() ([]outboundPacket, error) {
	if w.bleedingMineStart != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		_, err := w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
			g := w.bleedingMineStart.Group
			if s.Completed[g] || s.Week != w.bleedingMineStart.Week {
				return nil, nil
			}
			for i, c := range s.Cards {
				if c.Group == g {
					s.Cards[i] = bleedingMineRewardCard{}
				}
			}
			s.Stages[g] = [4]bool{}
			s.Members[g] = [4]int64{}
			return nil, nil
		})
		cancel()
		if err != nil {
			return nil, err
		}
	}
	plan, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	// 先关闭矿区复活/战斗窗口，再返回准备区；服务端主动退出不伪造CMD42应答。
	return append([]outboundPacket{{"赤红铁矿挑战结束", 0, 2706, protocol.BleedingMineFailed()}}, plan[1:]...), nil
}

func (w *worldSession) validateBleedingMineDungeon(id uint32) error {
	if w == nil || w.bleedingMineStart == nil || w.channelType != 106 ||
		!w.bleedingMineCreated || !w.bleedingMineReady || w.activeDungeon != nil ||
		w.state.Position.Town != 218 || w.state.Position.Area != 2 ||
		id != w.bleedingMineStart.Dungeon {
		return fmt.Errorf("赤红铁矿入场请求与已批准的编队或源地图不一致")
	}
	return w.service.ValidateRestoredPosition(w.level, w.odyssey, w.state.Position)
}

func (w *worldSession) giveUpBleedingMine(p []byte) ([]outboundPacket, error) {
	group, err := protocol.DecodeBleedingMineGiveUp(p)
	if err != nil {
		return nil, err
	}
	if w.channelType != 106 || w.bleedingMineStart == nil || group != w.bleedingMineStart.Group {
		return nil, fmt.Errorf("赤红铁矿放弃请求不属于当前开战会话")
	}
	// 矿区没有注册2319应答reader；使用已经验证的回城通知恢复当前玩家，
	// 不伪造42应答。准备状态通知同时用作写出成功后的会话清理标记。
	return w.endBleedingMine()
}

// 全部校验和编码完成后才发布开战状态；客户端首次请求的活动名单可以为空，
// 名单真源仍是已保存的稳定角色ID，不能把空活动名单写回编队存档。
func (w *worldSession) prepareBleedingMineStart(ctx context.Context, p []byte) (*bleedingMineStart, []outboundPacket, error) {
	if w == nil || w.channelType != 106 || !w.bleedingMineCreated || !w.bleedingMineReady ||
		w.role.ID == 0 || w.role.AccountID != w.account ||
		w.state.Position.Town != 218 || w.state.Position.Area != 2 ||
		w.characters == nil || w.store == nil || w.dungeons == nil ||
		w.activeDungeon != nil || w.selectingDungeon || w.inTutorial || w.specialWarpPending {
		return nil, nil, fmt.Errorf("请在赤红铁矿准备区完成编队后开始")
	}
	if w.bleedingMineRoster == nil {
		return nil, nil, fmt.Errorf("赤红铁矿账号角色名单尚未同步")
	}
	request, err := protocol.DecodeBleedingMineTeam(p)
	if err != nil {
		return nil, nil, err
	}
	if w.bleedingMineStart != nil {
		if request.Group != w.bleedingMineStart.Group {
			return nil, nil, fmt.Errorf("赤红铁矿已有等待入场的编队")
		}
		// 重复确认只应答，不能重建客户端角色、重播淡出或重置会话。
		return w.bleedingMineStart, []outboundPacket{{"赤红铁矿重复开战确认", 1, 2318, []byte{1}}}, nil
	}
	saved, err := w.store.BleedingMineTeams(ctx, w.account)
	if err != nil {
		return nil, nil, err
	}
	ids := saved[request.Group]
	if ids[0] != w.role.ID {
		return nil, nil, fmt.Errorf("赤红铁矿首个出战角色必须为当前登录角色")
	}
	if err := w.validateBleedingMineRewards(ctx, request.Group, ids); err != nil {
		return nil, nil, err
	}
	start := &bleedingMineStart{Group: request.Group, Members: ids, Dungeon: bleedingMineRoutes[request.Group][0], Week: adventure.SeasonWeek(time.Now())}
	definition, ok := w.dungeons.Dungeons[start.Dungeon]
	if !ok || !strings.HasPrefix(strings.ToLower(definition.Script.Path), "contents/2025/bleedingmine/dungeon/") || !definition.NoFatigue {
		return nil, nil, fmt.Errorf("赤红铁矿源副本配置缺失或不匹配")
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return nil, nil, err
	}
	owned := make(map[int64]database.Character, len(roles))
	for _, role := range roles {
		owned[role.ID] = role
	}
	slots := make(map[int64]int32, len(w.bleedingMineRoster))
	for i, id := range w.bleedingMineRoster {
		slots[id] = int32(i)
	}
	var members []protocol.BleedingMineMember
	var companions []protocol.TagCharacter
	seen := map[int64]bool{}
	empty := false
	for _, id := range ids {
		if id == 0 {
			empty = true
			continue
		}
		role, exists := owned[id]
		slot, listed := slots[id]
		if empty || !exists || !listed || role.AccountID != w.account || seen[id] {
			return nil, nil, fmt.Errorf("赤红铁矿编队存在空位间隔、失效或重复角色，请重新保存")
		}
		seen[id] = true
		fame, err := w.characters.EquipmentFame(role.State)
		if err != nil {
			return nil, nil, err
		}
		if fame < bleedingMineRequiredFame[request.Group] {
			return nil, nil, fmt.Errorf("角色%s名望%d低于该矿区要求%d", role.Name, fame, bleedingMineRequiredFame[request.Group])
		}
		members = append(members, protocol.BleedingMineMember{Server: w.characters.ChannelContext[0], Slot: slot})
		if id != w.role.ID {
			snapshot, err := w.characters.TagCharacterSnapshot(role)
			if err != nil {
				return nil, nil, fmt.Errorf("读取出战角色%s：%w", role.Name, err)
			}
			if uint32(snapshot.Level) < definition.MinimumLevel {
				return nil, nil, fmt.Errorf("角色%s等级低于矿区源副本要求", role.Name)
			}
			companions = append(companions, snapshot)
		}
	}
	// 开战淡出前验证实际房间及怪物可加载，避免发送成功之后才发现缺地图。
	if _, err := dungeon.Select(*w.dungeons, protocol.DungeonSelection{ID: start.Dungeon, Party: 65535}, w.level, nil); err != nil {
		return nil, nil, fmt.Errorf("赤红铁矿入场资源不可用：%w", err)
	}
	start.TeamPacket, err = protocol.TagTeamInfo(members)
	if err != nil {
		return nil, nil, err
	}
	start.CharacterPacket, err = protocol.TagCharacterInfo(w.role.WireID, companions)
	if err != nil {
		return nil, nil, err
	}
	// 1381建立索引，1382建立角色；2704=1触发淡出，原生tick随后发送C15。
	room := bleedingMinePreparation()
	binary.LittleEndian.PutUint32(room.Payload, 1)
	room.Name = "赤红铁矿开战入场状态"
	return start, []outboundPacket{
		{"赤红铁矿源地图预设", 0, 2702, make([]byte, 4)},
		{"赤红铁矿活动编队", 0, 1381, start.TeamPacket},
		{"赤红铁矿出战角色资料", 0, 1382, start.CharacterPacket},
		{"赤红铁矿开战确认", 1, 2318, []byte{1}},
		room,
	}, nil
}

// CMD2316由140743BD0无正文发送。实机20260929_001609会话在218/1
// 两次发送同样的创建请求；14073C380成功分支不读额外正文，而是进入
// BleedingMine.ctp的[party waiting area]：218/2、600/200。
func (w *worldSession) createBleedingMine(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if w == nil || w.channelType != 106 || w.role.ID == 0 || w.role.AccountID != w.account {
		return nil, fmt.Errorf("赤红铁矿创建请求缺少本账号的频道会话")
	}
	if len(p) != 0 {
		return nil, fmt.Errorf("赤红铁矿创建请求不应包含正文")
	}
	pos := w.state.Position
	if pos.Town != 218 || pos.Area != 1 && !(pos.Area == 2 && w.bleedingMineCreated) {
		return nil, fmt.Errorf("请在赤红铁矿大厅创建队伍")
	}
	if w.activeDungeon != nil || w.selectingDungeon || w.bleedingMineStart != nil || w.inTutorial || w.specialWarpPending {
		return nil, fmt.Errorf("当前角色正在进行其他场景操作")
	}
	if err := w.service.ValidateRestoredPosition(w.level, w.odyssey, pos); err != nil {
		return nil, err
	}
	// 创建的是本连接的准备会话；尚未开战，不扣入场次数或发放奖励。
	// 重复请求保留同一会话，不重置后续准备状态。
	var packets []outboundPacket
	if !w.bleedingMineCreated {
		if w.characters == nil || w.store == nil {
			return nil, fmt.Errorf("赤红铁矿编队存储不可用")
		}
		roster := w.bleedingMineRoster
		if roster == nil {
			// 重登后直接创建队伍不一定发送CMD1462。恢复编队前主动复用
			// 同一条账号名单同步链，角色详情、编队索引及开战校验共用快照。
			payload, ids, err := w.characters.AllServerRoster(ctx, w.account, w.fatigue, time.Now())
			if err != nil {
				return nil, err
			}
			roster = ids
			count := len(roster)
			counts := []byte{1, w.characters.ChannelContext[0], byte(count), byte(count >> 8)}
			packets = append(packets,
				outboundPacket{"赤红铁矿账号角色数量同步", 0, 1396, counts},
				outboundPacket{"赤红铁矿账号角色资料同步", 0, 2, payload},
			)
		}
		profile, err := w.bleedingMineProfile(ctx, roster)
		if err != nil {
			return nil, err
		}
		packets = append(packets, profile)
		// 所有读取和编码都成功后才发布缓存；错误重试仍会补发完整名单。
		// 调用方遇到发送失败会关闭连接，不保留未发送成功的会话。
		w.bleedingMineRoster = roster
	}
	w.bleedingMineCreated = true
	return append(packets, outboundPacket{"赤红铁矿准备会话已创建", 1, 2316, []byte{1}}), nil
}

// 按这次实际发给客户端的账号列表投影；删除的角色恢复为空槽，不挪动其他成员。
func (w *worldSession) bleedingMineProfile(ctx context.Context, roster []int64) (outboundPacket, error) {
	if w.characters == nil || w.store == nil {
		return outboundPacket{}, fmt.Errorf("赤红铁矿编队存储不可用")
	}
	if roster == nil {
		return outboundPacket{}, fmt.Errorf("恢复赤红铁矿编队需要已同步的账号角色名单")
	}
	roles, err := w.store.Characters(ctx, w.account)
	if err != nil {
		return outboundPacket{}, err
	}
	active := make(map[int64]bool, len(roles))
	for _, role := range roles {
		active[role.ID] = true
	}
	slots := make(map[int64]int32, len(roster))
	for i, id := range roster {
		if active[id] {
			slots[id] = int32(i)
		}
	}
	saved, err := w.store.BleedingMineTeams(ctx, w.account)
	if err != nil {
		return outboundPacket{}, err
	}
	var teams [3]protocol.BleedingMineTeam
	for group, ids := range saved {
		teams[group].Group = uint32(group)
		for i, id := range ids {
			member := protocol.BleedingMineMember{Slot: -1}
			if slot, ok := slots[id]; ok {
				member.Server, member.Slot = w.characters.ChannelContext[0], slot
			}
			teams[group].Members[i] = member
		}
	}
	p := protocol.BleedingMineProfile(teams)
	if w.loot != nil && w.loot.BleedingMine != nil {
		s, err := w.updateBleedingMineRewards(ctx, func(*bleedingMineRewardState) ([]database.MailAsset, error) { return nil, nil })
		if err != nil {
			return outboundPacket{}, err
		}
		bleedingMineRewardProfile(p, s)
	}
	return outboundPacket{"赤红铁矿已保存编队恢复", 0, 2703, p}, nil
}

func (w *worldSession) saveBleedingMineTeam(ctx context.Context, p []byte) ([]outboundPacket, error) {
	if w.channelType != 106 || !w.bleedingMineCreated || !w.bleedingMineReady ||
		w.state.Position.Town != 218 || w.state.Position.Area != 2 ||
		w.activeDungeon != nil || w.selectingDungeon || w.bleedingMineStart != nil || w.characters == nil || w.bleedingMineRoster == nil {
		return nil, fmt.Errorf("请在赤红铁矿准备区加载角色列表后保存编队")
	}
	team, err := protocol.DecodeBleedingMineTeam(p)
	if err != nil {
		return nil, err
	}
	var ids [4]int64
	for i, member := range team.Members {
		if member.Slot == -1 {
			continue
		}
		if member.Server != w.characters.ChannelContext[0] || int64(member.Slot) >= int64(len(w.bleedingMineRoster)) {
			return nil, fmt.Errorf("赤红铁矿成员不属于当前服务器的账号角色列表")
		}
		ids[i] = w.bleedingMineRoster[member.Slot]
	}
	if err = w.store.SaveBleedingMineTeam(ctx, w.account, w.role.ID, team.Group, ids); err != nil {
		return nil, err
	}
	// 14073D1E0成功分支不更新名单；必须先由1444FB350恢复队员，再由
	// 14073E380调用141437810刷新原生编队按钮，不能只返回成功ACK。
	return []outboundPacket{
		{"赤红铁矿编队同步", 0, 2705, protocol.BleedingMineTeamInfo(team)},
		{"赤红铁矿编队已保存", 1, 2317, []byte{1}},
	}, nil
}

// NOTI2704的14073E310固定读取12字节，并把首u32映射到矿区公共状态。
// 141430FD6调用142AB25E0读取该状态，只有0才进入142AC3320的开始流程。
// 1会经140743BF0进入140741D80并保持状态1，导致按钮点击静默返回。
// 创建成功只代表进入准备区，必须保持未开战状态0；不能提前发布启动状态。
// 其余字段保留reader原生初值，不借用为玩家编号。
func bleedingMinePreparation() outboundPacket {
	p := make([]byte, 12)
	binary.LittleEndian.PutUint32(p[4:], ^uint32(0))
	return outboundPacket{"赤红铁矿准备区状态已同步", 0, 2704, p}
}

// 在准备区第一个位置上报后发布状态，避免区域尚在加载时创建编队界面。
func (w *worldSession) syncBleedingMinePreparation(send func(byte, uint16, []byte) error, event func(map[string]any)) error {
	if w.channelType != 106 || !w.bleedingMineCreated || w.bleedingMineReady ||
		w.state.Position.Town != 218 || w.state.Position.Area != 2 {
		return nil
	}
	p := bleedingMinePreparation()
	if err := send(p.Kind, p.ID, p.Payload); err != nil {
		return err
	}
	w.bleedingMineReady = true
	event(map[string]any{"kind": p.Name, "id": p.ID, "character_id": w.role.ID,
		"room_state": 0, "plain_hex": fmt.Sprintf("%x", p.Payload),
		"attempt": "2/3，准备状态按原生开始按钮的零值门禁修正"})
	return nil
}

// 使用现有区域拒绝格式；未创建的会话不能靠走门进入独立准备区。
func (w *worldSession) validateBleedingMineArea(r protocol.AreaChangeRequest) error {
	if w.bleedingMineStart != nil && (r.Town != w.state.Position.Town || r.Area != w.state.Position.Area) {
		return fmt.Errorf("赤红铁矿已开始入场，请先放弃当前探索再离开")
	}
	if w.channelType == 106 && r.Town == 218 && r.Area == 2 && !w.bleedingMineCreated {
		return fmt.Errorf("赤红铁矿准备区需要先创建队伍")
	}
	return nil
}

func (w *worldSession) completeBleedingMineStage() ([]outboundPacket, error) {
	mine := w.bleedingMineStart
	if mine == nil || w.activeDungeon == nil || w.activeDungeon.Definition.ID != mine.Dungeon || !w.activeDungeon.Completed() {
		return nil, fmt.Errorf("矿区领主尚未确认死亡")
	}
	if len(mine.StageResult) == 0 {
		now := time.Now()
		if mine.Deadline.IsZero() || now.After(mine.Deadline) {
			mine.Failed = true
			return []outboundPacket{{"赤红铁矿领主通关超时", 0, 2706, protocol.BleedingMineFailed()}}, nil
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := w.freezeBleedingMineStage(ctx); err != nil {
			return nil, err
		}
		mine.Carry = mine.Deadline.Sub(now)
		elapsed := uint32(max(now.Sub(mine.StageStarted).Milliseconds(), 0))
		mine.Elapsed += elapsed
		finished := mine.Stage == 3 || mine.Members[mine.Stage+1] == 0
		mine.StageResult = protocol.BleedingMineStageResult(mine.Stage, uint32(mine.Carry.Milliseconds()), elapsed, mine.Elapsed, finished)
	}
	// 14073DD40接收空NOTI2707后停止计时并发送CMD2320。
	var plan []outboundPacket
	if w.activeDungeon.CompletionNeedsBossCheck() {
		body, err := protocol.BossCheckConfirmed(w.activeDungeon.CompletionTarget())
		if err != nil {
			return nil, err
		}
		plan = append(plan, outboundPacket{"boss_check_confirmed", 0, 115, body})
	}
	return append(plan, outboundPacket{"赤红铁矿领主通关确认", 0, 2707, nil}), nil
}

// 141435420在死亡等待结束后发送当前活动角色索引；1444F9630只消费成功字节。
func (w *worldSession) bleedingMineDeath(p []byte) ([]outboundPacket, error) {
	mine := w.bleedingMineStart
	if mine == nil || w.channelType != 106 || w.activeDungeon == nil || !w.activeDungeon.Loaded || w.activeDungeon.Completed() || len(p) != 8 || uint32(p[0]) != mine.Stage {
		return nil, fmt.Errorf("矿区死亡请求与当前活动角色不一致")
	}
	mine.Failed = true
	return []outboundPacket{{"赤红铁矿死亡确认", 1, 1461, []byte{1}}, {"赤红铁矿死亡结算选项", 0, 2706, protocol.BleedingMineFailed()}}, nil
}

func (w *worldSession) settleBleedingMineStage(p []byte) ([]outboundPacket, error) {
	group, _, err := protocol.DecodeBleedingMineStage(p)
	if err != nil {
		return nil, err
	}
	mine := w.bleedingMineStart
	if mine == nil || mine.Group != group || len(mine.StageResult) == 0 || w.activeDungeon == nil || !w.activeDungeon.Completed() {
		return nil, fmt.Errorf("矿区结算缺少服务端通关回执")
	}
	return []outboundPacket{{"赤红铁矿阶段结算确认", 1, 2320, []byte{1}}, {"赤红铁矿阶段结算", 0, 2706, mine.StageResult}}, nil
}

func (w *worldSession) advanceBleedingMine(r protocol.DungeonDirectMove) (*dungeon.Session, []outboundPacket, error) {
	mine := w.bleedingMineStart
	if mine == nil || w.activeDungeon == nil || !w.activeDungeon.Completed() || len(mine.StageResult) == 0 || mine.Stage >= 3 || mine.Members[mine.Stage+1] == 0 || r.ID != bleedingMineRoutes[mine.Group][mine.Stage+1] || uint32(r.Difficulty) != mine.Stage+1 {
		return nil, nil, fmt.Errorf("矿区后续关卡与已通关阶段不一致")
	}
	// 1407417A0把阶段索引放入2062第二个u32；它不是副本难度。
	next, route, err := w.prepareDungeonEntry(protocol.DungeonSelection{ID: r.ID, Difficulty: 0, Party: 65535})
	if err != nil {
		return nil, nil, err
	}
	plan := []outboundPacket{{"dungeon_gate_ack", 1, 15, []byte{1}}, {"dungeon_direct_move_selection_initialized", 0, 27, protocol.EnterDungeonSelection()}, {"dungeon_direct_move_ready", 0, 2281, protocol.BleedingMineNextNotice(r)}}
	mine.Stage++
	mine.Dungeon = r.ID
	mine.StageResult = nil
	mine.Deadline = time.Time{}
	mine.Revives = nil
	return next, append(plan, route...), nil
}

func (w *worldSession) finishBleedingMine(p []byte) ([]outboundPacket, error) {
	group, _, err := protocol.DecodeBleedingMineEnd(p)
	if err != nil {
		return nil, err
	}
	mine := w.bleedingMineStart
	if mine == nil || group != mine.Group {
		return nil, fmt.Errorf("矿区结束请求缺少当前探索")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err = w.updateBleedingMineRewards(ctx, func(s *bleedingMineRewardState) ([]database.MailAsset, error) {
		if s.Week != mine.Week || s.Claimed {
			return nil, fmt.Errorf("矿区奖励周期已改变")
		}
		if s.Completed[group] {
			return nil, nil
		}
		s.Completed[group] = true
		s.Members[group] = mine.Members
		if s.Stages[group][3] {
			s.CombineEarned = min(uint32(5), s.CombineEarned+w.loot.BleedingMine.Combine.Chances[group])
		}
		return nil, nil
	})
	if err != nil {
		return nil, err
	}
	plan, err := w.leaveDungeon()
	if err != nil {
		return nil, err
	}
	return append([]outboundPacket{{"赤红铁矿探索结算完成", 1, 2321, []byte{1}}}, plan[1:]...), nil
}
