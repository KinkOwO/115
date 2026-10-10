package main

import (
	"dfolan/internal/game/protocol"
	"dfolan/internal/modactor"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// 本文件是 internal/modactor 在服务端这侧的**宿主实现**：把 lanHub 包成
// modactor.Host，让服务端层 mod 能在城镇里凭空放一个会走路的角色。
//
// # 为什么这样接
//
// lanHub / lanPeer 在 `package main` 里，Go 不允许 mod import 它，所以门开在
// internal/modactor：mod 只认那个包的接口，main 在这里把真实实现装进去
// （`installActorHost`）。这是 internal/modpolicy 已经在用的同一个套路。
//
// # 假人是什么
//
// 一个假人就是一个 `send == nil` 的 lanPeer：
//
//   - 它在 hub 的花名册里，所以 `visible()` 认它 —— 真人进同一场景时，
//     既有的 `introducePeers` / `rosterLocked` 会把它当成一个普通 actor 介绍出去，
//     NOTI 24 的区域列表里也会带上它，玩家看到的就是"这里站了几个人"；
//   - 它没有 `send`，所以所有"发给它"的推送都会被既有代码跳过
//     （`if o.send == nil { continue }`）—— 它不会收到任何东西，也不需要；
//   - 它**自己不会动**：真实玩家每走一步都有 CMD 35 推着走，假人没有报文源，
//     只有 mod 在 server.tick 里调 Move 才动。
//
// 对照 ACT12 整合包的做法：那边是 `virtual_peer`（每个假人一个 bot 账号 +
// 一个真实 UDP 端口 + 完整 wire 握手，即"服务端内置无头客户端"）。我们**不照搬**：
// dfolan 是纯 TCP 且战斗不在服务端，照搬要重建整套客户端握手却换不来战斗能力；
// 而 lanPeer 本来就已经是"别的连接眼里的一个 actor"，缺的只是"无连接"这一种形态。
//
// # 不落库
//
// 假人只在内存里：不写 SQLite、不占角色槽、不进账号。进程退出即消失。
// 存档兼容是最高优先级（根 AGENTS §0.4），不拿真实存档换装饰效果。

// actorIDBase 是 mod 造出来的实体用的 actor id 起点。
//
// 真实玩家的 actor id 是数据库分配的角色 WireID（`internal/database/store.go`
// 的 allocation.NextWireID），是从小往大递增的小整数。假人从 0x8000 起往大走，
// 跟那段错开；真要撞上（意味着库里已有三万多个角色）由 `nextActorID` 跳过。
//
// 0 与 0xffff 是保留值：`protocol` 的入口编码会把它们当非法身份拒绝。
const actorIDBase = 0x8000

// lanActorHost 把 lanHub 包成 modactor.Host。
//
// 它自己持一把锁管"id → peer"的映射；hub 内部的 peers 集合由 hub 自己的锁管，
// 两把锁不嵌套使用（先算、再调 hub），避免锁序问题。
type lanActorHost struct {
	hub *lanHub

	mu     sync.Mutex
	next   uint16
	actors map[modactor.ActorID]*lanPeer
	names  map[modactor.ActorID]string
	srcs   map[modactor.ActorID]string
}

// newLanActorHost 造一个宿主。hub 为 nil 时所有操作都会退化成"什么都放不下"，
// 与 lanHub 自身的 nil 容忍一致（见 lan_hub.go 各方法开头的 `if h == nil`）。
func newLanActorHost(hub *lanHub) *lanActorHost {
	return &lanActorHost{
		hub:    hub,
		next:   actorIDBase - 1,
		actors: map[modactor.ActorID]*lanPeer{},
		names:  map[modactor.ActorID]string{},
		srcs:   map[modactor.ActorID]string{},
	}
}

// installActorHost 把宿主装进 internal/modactor，供 main 在 hub 建好之后调用。
//
// 已经装过就直接返回 nil：runGateway 在同一进程里可能被反复起停（测试就是），
// 第二次进来 hub 是新的，但接口实现是同一个套路，没必要因此让启动失败。
// 调用方负责在退出时 uninstallActorHost。
func installActorHost(hub *lanHub) error {
	if modactor.Available() {
		return nil
	}
	return modactor.Install(newLanActorHost(hub), "cmd/wireprobe.lanActorHost")
}

// uninstallActorHost 撤掉宿主（main 退出时调用，与 install 成对）。
func uninstallActorHost() { modactor.Uninstall() }

// nextActorID 分配一个没被占用的 actor id。调用方持有 h.mu。
func (h *lanActorHost) nextActorID() (modactor.ActorID, bool) {
	for i := 0; i < 0x7ff0; i++ {
		h.next++
		if h.next == 0 || h.next == 0xffff {
			h.next = actorIDBase
			continue
		}
		id := modactor.ActorID(h.next)
		if _, taken := h.actors[id]; taken {
			continue
		}
		if h.hub != nil && h.hub.actorTaken(uint16(id)) {
			continue
		}
		return id, true
	}
	return 0, false
}

// Spawn 造一个无连接实体并放进落点。
//
// 要么完整建成（mode0 + mode1 都编得出），要么一个都不建：半残的假人最难受 ——
// 它会出现在别人的区域列表里却画不出来，排查时看不出是"没建成"还是"客户端不认"。
func (h *lanActorHost) Spawn(spec modactor.Spec) (modactor.ActorID, error) {
	rows, err := actorAppearanceRows(spec)
	if err != nil {
		return 0, err
	}
	h.mu.Lock()
	id, ok := h.nextActorID()
	if !ok {
		h.mu.Unlock()
		return 0, errors.New("lanActorHost: actor id 已耗尽")
	}
	info, err := actorBasicInfo(uint16(id), spec, rows)
	addition, addErr := actorAddition(uint16(id), spec)
	if err != nil || addErr != nil {
		h.mu.Unlock()
		if err != nil {
			return 0, err
		}
		return 0, fmt.Errorf("假人 %s 的 mode1 载荷编码失败：%w", spec.Name, addErr)
	}
	p := &lanPeer{
		actorID:  uint16(id),
		channel:  spec.Channel,
		town:     spec.Town,
		area:     spec.Area,
		x:        spec.X,
		y:        spec.Y,
		motion:   spec.Motion,
		speed:    spec.Speed,
		info:     info,
		addition: addition,
		send:     nil, // 无连接：所有发给它的推送都会被既有代码跳过
	}
	h.actors[id] = p
	h.names[id] = spec.Name
	h.srcs[id] = spec.Source
	h.mu.Unlock()

	if h.hub != nil {
		h.hub.add(p)
		joined := h.hub.publish(p, spec.Channel, spec.Town, spec.Area, spec.X, spec.Y, false)
		h.introduceTo(joined, p)
	}
	return id, nil
}

// Move 在同一场景内移动并广播位置。
func (h *lanActorHost) Move(id modactor.ActorID, x, y uint16, motion byte, speed uint16) error {
	h.mu.Lock()
	p, ok := h.actors[id]
	h.mu.Unlock()
	if !ok {
		return modactor.ErrUnknownActor
	}
	if h.hub == nil {
		return nil
	}
	others := h.hub.move(p, x, y, motion, speed)
	if len(others) == 0 {
		return nil
	}
	// NOTI 22 是客户端已经消费的"某人走到哪儿了"（既有 broadcastMove 用的同一条）。
	payload, err := protocol.UserPosition(p.actorID, x, y, motion, speed)
	if err != nil {
		return err
	}
	for _, o := range others {
		if o.send == nil {
			continue
		}
		o.send(0, 22, payload)
	}
	return nil
}

// Relocate 换频道/城镇/区域。
//
// 旧场景的"它走了"由 hub.publish 内部处理（NOTI 23 带新场景号 —— 客户端据此把
// actor 从旧场景摘掉，见 lan_hub.go 的 movedLocked 注释）。新场景的人不认识它，
// 所以要补一次 NOTI 2 介绍 + NOTI 23 落位，否则他们那边只有区域列表里多一行、
// 却画不出人。
func (h *lanActorHost) Relocate(id modactor.ActorID, channel, town, area uint32, x, y uint16) error {
	h.mu.Lock()
	p, ok := h.actors[id]
	h.mu.Unlock()
	if !ok {
		return modactor.ErrUnknownActor
	}
	if h.hub == nil {
		return nil
	}
	joined := h.hub.publish(p, channel, town, area, x, y, false)
	h.introduceTo(joined, p)
	return nil
}

// Despawn 撤走这个实体：向它所在场景广播 NOTI 6（删除 actor）。
func (h *lanActorHost) Despawn(id modactor.ActorID) error {
	h.mu.Lock()
	p, ok := h.actors[id]
	if ok {
		delete(h.actors, id)
		delete(h.names, id)
		delete(h.srcs, id)
	}
	h.mu.Unlock()
	if !ok {
		return modactor.ErrUnknownActor
	}
	if h.hub != nil {
		h.hub.depart(p)
	}
	return nil
}

// Pose 读当前姿态。
func (h *lanActorHost) Pose(id modactor.ActorID) (modactor.Pose, bool) {
	h.mu.Lock()
	p, ok := h.actors[id]
	h.mu.Unlock()
	if !ok {
		return modactor.Pose{}, false
	}
	if h.hub == nil {
		return modactor.Pose{}, false
	}
	h.hub.mu.Lock()
	out := modactor.Pose{Channel: p.channel, Town: p.town, Area: p.area, X: p.x, Y: p.y, Motion: p.motion, Speed: p.speed, Placed: p.placed}
	h.hub.mu.Unlock()
	return out, true
}

// List 列出全部假人，按 actor id 稳定排序。
func (h *lanActorHost) List() []modactor.Snapshot {
	h.mu.Lock()
	ids := make([]modactor.ActorID, 0, len(h.actors))
	for id := range h.actors {
		ids = append(ids, id)
	}
	h.mu.Unlock()
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	out := make([]modactor.Snapshot, 0, len(ids))
	for _, id := range ids {
		pose, ok := h.Pose(id)
		if !ok {
			continue
		}
		h.mu.Lock()
		out = append(out, modactor.Snapshot{ID: id, Name: h.names[id], Pose: pose, Source: h.srcs[id]})
		h.mu.Unlock()
	}
	return out
}

// Scenes 列出此刻有真实连接的场景（假人不算）。
//
// 判定"真连接"用 `send != nil`：假人是 send 为 nil 的那个形态（见本文件顶部注释）。
// 私人实例（private）也算有人，但坐标是玩家自己的副本位；假人不该被扔进副本，
// 所以这里按 visible 的口径过滤掉 —— 与 hub 自身的可见性判断保持一致。
func (h *lanActorHost) Scenes() []modactor.Scene {
	if h.hub == nil {
		return nil
	}
	h.hub.mu.Lock()
	type key struct {
		channel, town, area uint32
	}
	counts := map[key]int{}
	for p := range h.hub.peers {
		if p.send == nil || !p.placed || p.private {
			continue
		}
		counts[key{p.channel, p.town, p.area}]++
	}
	h.hub.mu.Unlock()
	out := make([]modactor.Scene, 0, len(counts))
	for k, n := range counts {
		out = append(out, modactor.Scene{Channel: k.channel, Town: k.town, Area: k.area, Players: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Channel != out[j].Channel {
			return out[i].Channel < out[j].Channel
		}
		if out[i].Town != out[j].Town {
			return out[i].Town < out[j].Town
		}
		return out[i].Area < out[j].Area
	})
	return out
}

// introduceTo 把 p 介绍给这批 peers：先 NOTI 2（mode0 + mode1），再 NOTI 23 落位。
//
// 顺序照既有 announceSelf 的口径：区域列表（NOTI 24）只负责"这一屏有谁"，
// 客户端对没有 user info 的 actor 是排队不画的，所以介绍必须走在前面。
func (h *lanActorHost) introduceTo(peers []*lanPeer, p *lanPeer) {
	if len(peers) == 0 || p == nil {
		return
	}
	placement, placeErr := protocol.UserArea(p.town, p.area, protocol.AreaUser{ActorServerID: p.actorID, X: p.x, Y: p.y, Flags: p.flags})
	for _, o := range peers {
		if o.send == nil || o == p {
			continue
		}
		if len(p.info) > 0 {
			o.send(0, 2, p.info)
		}
		if len(p.addition) > 0 {
			o.send(0, 2, p.addition)
		}
		if placeErr == nil {
			o.send(0, 23, placement)
		}
	}
}

// actorTaken 报告这个 actor id 是不是已经被某个真实连接用了。
func (h *lanHub) actorTaken(actor uint16) bool {
	if h == nil {
		return false
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for p := range h.peers {
		if p.actorID == actor {
			return true
		}
	}
	return false
}

// actorAppearanceRows 把 mod 给的穿戴投影成 protocol 的外观行。
//
// 抽出来单独做，是为了让"这个 spec 编不编得出"能在**分配 actor id 之前**就判掉 ——
// 载荷里带 actor id，只能分配完再编，若那时才发现 spec 有问题就白占一个 id。
func actorAppearanceRows(spec modactor.Spec) ([]protocol.EquippedAppearance, error) {
	rows := make([]protocol.EquippedAppearance, 0, len(spec.Worn))
	seen := map[byte]bool{}
	for _, w := range spec.Worn {
		if w.Slot > protocol.MaxEquippedAppearanceSlot() {
			return nil, fmt.Errorf("假人 %s 的穿戴槽 %d 超出客户端外观表（最大 %d）", spec.Name, w.Slot, protocol.MaxEquippedAppearanceSlot())
		}
		if w.ItemTemplate == 0 {
			// 模板为空时客户端找不到城镇模型（实机症状：穿了但模型不更新），
			// 这里直接拒绝，好过让作者面对一个"穿了没反应"的假人。
			return nil, fmt.Errorf("假人 %s 的穿戴槽 %d 缺装备模板 ID", spec.Name, w.Slot)
		}
		if w.UpgradeLevel > 31 {
			// 强化等级打包成 level<<1 塞进一个字节的高 7 位，>31 会回绕。
			return nil, fmt.Errorf("假人 %s 的穿戴槽 %d 强化等级 %d 超出 0..31", spec.Name, w.Slot, w.UpgradeLevel)
		}
		if seen[w.Slot] {
			return nil, fmt.Errorf("假人 %s 的穿戴槽 %d 重复", spec.Name, w.Slot)
		}
		seen[w.Slot] = true
		rows = append(rows, protocol.EquippedAppearance{Slot: w.Slot, Placeholder: w.ItemTemplate, Model: w.Model, Flags: w.UpgradeLevel << 1})
	}
	return rows, nil
}

// actorBasicInfo 用既有编码器产出一个假人的 NOTI 2 mode 0 载荷。
//
// 刻意**复用** protocol.UserInfoBasicProbe 而不是自己拼字节：那条编码器是
// 已取证的（外观块宽度是逐条指令钉住的，见 entry_userinfo.go 的注释），
// 让每个 mod 自己拼一段载荷等于把协议真源拆成 N 份（根 AGENTS §0.2）。
func actorBasicInfo(actor uint16, spec modactor.Spec, rows []protocol.EquippedAppearance) ([]byte, error) {
	return protocol.UserInfoBasicProbe(protocol.EntryBasicProbe{
		ActorServerID: actor,
		Fame:          spec.Fame,
		// 城镇队伍面板的两条状态条（HP/MP 比例）：满血站着比 0 更像"一个人"。
		BasePercent:   100,
		SkillTreeType: 1,
		Character: protocol.CharacterRow{
			Name:        spec.Name,
			Profession:  spec.Profession,
			Advancement: spec.Advancement,
			Level:       spec.Level,
		},
		Appearance: rows,
	})
}

// actorAddition 产出 NOTI 2 mode 1 载荷（面板数值）。
//
// 这里的数字是**面板数值**，不是协议结构：字段宽度由已取证的
// protocol.PackedEntryStats 决定，填的多少只影响假人头上那两条状态条与
// 属性显示，不参与任何协议判定。缺它客户端仍会画人（`introducePeers` 对
// addition 为空的情况是跳过的），所以编不出来时降级而不失败。
func actorAddition(actor uint16, spec modactor.Spec) ([]byte, error) {
	return protocol.UserInfoAdditionProbe(protocol.EntryAdditionProbe{
		ActorServerID: actor,
		Fame:          spec.Fame,
		SkillTreeType: 1,
		Stats: protocol.PackedEntryStats{
			HP:            100000,
			MP:            100000,
			BasePercent:   100,
			Movement:      100,
			AttackCasting: [2]uint16{100, 100},
		},
	})
}
