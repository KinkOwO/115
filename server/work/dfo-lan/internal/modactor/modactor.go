// Package modactor 是服务端**场景实体（Actor）**的可编程入口，供服务端层 mod 使用。
//
// # 为什么需要它
//
// 四层 mod 里，server 层是唯一能改服务端玩法行为的层，它的 Go 代码由 modkit 生成
// `mods/zz_mods_gen.go` 后编进服务端二进制（见 `server/work/dfo-lan/mods/README.md`）。
// 但"在城镇里凭空放一个会走路的角色"这类能力，过去只能改 `cmd/wireprobe` 本身——
// 因为实体所在的 `lanHub` / `lanPeer` 在 `package main` 里，而 Go **不允许** import
// `package main`，mod 拿不到它。
//
// 本包是那道墙上的门：它定义一个**宿主接口**（Host），由 `cmd/wireprobe` 在启动时
// 把真实实现装进来（`Install`）；mod 只对着本包编程，不需要认识 main。
// 这正是 `internal/modpolicy` 已经在用的同一个套路（进程内单例 + 宿主注入），
// 差别只是 modpolicy 交出去的是"策略值"，本包交出去的是"对象句柄"。
//
// # 默认全关
//
// 没有 `Install` 时 `Available()` 为假，所有操作一律返回 `ErrNoHost` ——
// 服务端行为与装 mod 之前一致。这跟 modpolicy 的"零值 = 规则全关"是同一条口径：
// 一个没接好的能力必须**明确报错**，不能安静地什么都不做。
//
// # 假人不落库
//
// Actor 是**内存态**的：它只在本次进程活着的时候存在，不写 SQLite、不占角色槽、
// 不进账号。理由有两条：
//   - 存档兼容是最高优先级（根 AGENTS §0.4），往玩家存档库里塞假人角色是拿
//     真实存档冒险换一个装饰效果；
//   - NOTI 24 的区域列表在客户端看来就是"这一屏有哪些人"，服务端多给几个
//     条目并不需要它们真的有档案。
//
// 所以 Despawn 或者进程退出之后，假人就消失了，不会留下任何痕迹。
package modactor

import (
	"errors"
	"fmt"
	"strings"
	"sync"
)

// ErrNoHost 表示宿主还没装（或已卸载）。
//
// 它是**可预期的运行期状态**而不是崩溃：一个 mod 在 server.boot 里就想造实体时
// 宿主确实还没接上（那时 lanHub 都还没建）。mod 应当把它当"时候未到"处理。
var ErrNoHost = errors.New("modactor: 宿主未装载场景实体能力（cmd/wireprobe 尚未 Install）")

// ErrUnknownActor 表示这个实体 id 不是本 mod 造的，或已经被撤掉了。
var ErrUnknownActor = errors.New("modactor: 找不到该场景实体")

// ActorID 是一个场景实体在 wire 上的身份 —— NOTI 2 / 23 / 24 里那 16 位。
type ActorID uint16

// WornSlot 是一个穿戴槽位的外观绑定，对应 protocol 的装备外观块。
//
// 三个数字的分别（照 protocol.Equipment 的既有语义，不要互换）：
//   - ItemTemplate 是**装备模板 ID**（PVF 装备表）。客户端的城镇显示对象按槽位
//     查这张表找模型，所以不能填 0（填 0 的表现是"穿了时装但城镇模型不更新"）。
//   - Model 是**模型索引**，以"长度 + 载荷"的形式跟在模板后面。填 0 表示
//     "这个槽的模型索引不动"，客户端保留它原有的。
//   - UpgradeLevel 是强化/增幅等级 0..31，决定武器光效（打包成 level<<1）。
//
// Slot 沿用客户端穿戴表（12 = 武器，最大 25 = 耳环）。整个 Worn 留空表示
// "什么都不穿"，客户端按默认身体模型画。
type WornSlot struct {
	Slot         byte
	ItemTemplate uint32
	Model        uint32
	UpgradeLevel byte
}

// Spec 描述一个由 mod 凭空造出来的场景实体。
//
// 字段刻意是**人能读懂的**（名字/职业/等级/穿戴），而不是一段含义不明的字节：
// 字节该由宿主用 `internal/game/protocol` 的编码器产出，那才是协议的真源
// （根 AGENTS §0.2）——让每个 mod 自己拼一段 NOTI 2 载荷，等于把协议真源拆成 N 份。
type Spec struct {
	// Name 是别人头顶看到的名字。
	Name string
	// Profession / Advancement / Level 是职业、转职与等级。
	Profession  byte
	Advancement byte
	Level       byte
	// Fame 是名望。
	Fame uint32
	// Worn 是穿戴外观；不需要时留 nil。
	Worn []WornSlot

	// Channel / Town / Area / X / Y 是落点。Town 与 Area 必须能在 PVF 的城镇表里
	// 找到，否则其它玩家永远跟它不在同一个场景里（visible 要求两边全等）。
	Channel uint32
	Town    uint32
	Area    uint32
	X       uint16
	Y       uint16
	// Motion 是朝向/动作，Speed 是移速，语义与 CMD 35 上报的一致。
	Motion byte
	Speed  uint16

	// Source 是造它的人（mod id）。必填：出现"不知道谁造的实体"时无从排查，
	// 这条与 modpolicy.Source 是同一个要求。
	Source string
}

// Pose 是一个实体此刻站在哪里、朝哪边。
type Pose struct {
	Channel uint32
	Town    uint32
	Area    uint32
	X       uint16
	Y       uint16
	Motion  byte
	Speed   uint16
	// Placed 为真表示它此刻在场景里（别人看得见）；Relocate 途中为假。
	Placed bool
}

// Snapshot 是宿主回给 mod 的实体视图，供诊断与"我的假人还在吗"这类查询。
type Snapshot struct {
	ID   ActorID
	Name string
	Pose Pose
	// Source 是哪个 mod 造的。
	Source string
}

// Scene 是一个**此刻有真实连接**的场景（假人不算在内）。
//
// 为什么非给 mod 这个查询不可：假人要有用就得站在有人的地方，而城镇/区域的
// 编号是从 PVF 来的、mod 拿不到目录；让 mod 在配置里写死一组坐标等于逼作者
// 先去做一次协议/资源取证（根 AGENTS §0.2、§0.1）。给一份"现在哪些场景有人"，
// mod 就能写出"有人进城就出现在他身边"这种不需要任何取证的行为。
//
// 它是**只读**的，且语义很稳（"哪些场景有真连接"），所以进接口是划算的。
type Scene struct {
	Channel uint32
	Town    uint32
	Area    uint32
	// Players 是这个场景里真实连接的条数。
	Players int
}

// Host 是宿主（cmd/wireprobe）实现的场景实体能力。
//
// 方法数刻意做少：每加一个方法，等于承诺"这个语义长期稳定"。
// 下面是"能在场景里放一个会动的东西"的最小集，加一个只读的场景查询
// （没有它假人只能靠写死坐标，而写死坐标就是猜）。
type Host interface {
	// Spawn 造一个**没有自己连接**的场景实体并放进落点，返回它的 wire 身份。
	// 返回 ErrUnknownActor 之外的 error 时调用方应当记日志并放弃这一个。
	Spawn(spec Spec) (ActorID, error)
	// Move 在同一场景内移动，并把新位置广播给同场景的其它人。
	Move(id ActorID, x, y uint16, motion byte, speed uint16) error
	// Relocate 换频道/城镇/区域（会向旧场景广播"它走了"、向新场景广播"它来了"）。
	Relocate(id ActorID, channel, town, area uint32, x, y uint16) error
	// Despawn 撤走这个实体（向它所在场景广播离开）。
	Despawn(id ActorID) error
	// Pose 读当前姿态；不存在时返回 ok=false。
	Pose(id ActorID) (Pose, bool)
	// List 列出**本进程当前全部**由 mod 造出来的实体（稳定排序）。
	List() []Snapshot
	// Scenes 列出此刻有真实连接的场景（按 channel/town/area 稳定排序）。
	// 一个真人都没有时返回 nil —— 那时"把假人放到人堆里"无从谈起。
	Scenes() []Scene
}

var (
	mu         sync.RWMutex
	host       Host
	hostSource string
)

// Install 由 cmd/wireprobe 在 lanHub 建好之后调用，把真实实现装进来。
//
// 只能装一次：重复 Install 返回 error 而不覆盖 —— 静默换掉宿主会让"谁的实体
// 在谁手里"变成一笔糊涂账。测试里想换实现先调 Uninstall。
func Install(h Host, source string) error {
	if h == nil {
		return errors.New("modactor: Install 收到 nil 宿主")
	}
	source = strings.TrimSpace(source)
	if source == "" {
		return errors.New("modactor: Install 必须写明宿主来源")
	}
	mu.Lock()
	defer mu.Unlock()
	if host != nil {
		return fmt.Errorf("modactor: 宿主已装载（来源 %s），不能重复 Install", hostSource)
	}
	host, hostSource = h, source
	return nil
}

// Uninstall 撤掉宿主（测试用；生产路径只走 Install）。
func Uninstall() {
	mu.Lock()
	host, hostSource = nil, ""
	mu.Unlock()
}

// Available 报告宿主是否已装载。
func Available() bool {
	mu.RLock()
	defer mu.RUnlock()
	return host != nil
}

// Source 返回宿主来源（给日志用）。
func Source() string {
	mu.RLock()
	defer mu.RUnlock()
	return hostSource
}

func current() (Host, error) {
	mu.RLock()
	defer mu.RUnlock()
	if host == nil {
		return nil, ErrNoHost
	}
	return host, nil
}

// Spawn 造一个场景实体。宿主未装载时返回 ErrNoHost。
func Spawn(spec Spec) (ActorID, error) {
	spec.Source = strings.TrimSpace(spec.Source)
	if spec.Source == "" {
		return 0, errors.New("modactor: Spawn 必须写明 Source（mod id）")
	}
	h, err := current()
	if err != nil {
		return 0, err
	}
	return h.Spawn(spec)
}

// Move 移动一个场景实体并广播。
func Move(id ActorID, x, y uint16, motion byte, speed uint16) error {
	h, err := current()
	if err != nil {
		return err
	}
	return h.Move(id, x, y, motion, speed)
}

// Relocate 换场景。
func Relocate(id ActorID, channel, town, area uint32, x, y uint16) error {
	h, err := current()
	if err != nil {
		return err
	}
	return h.Relocate(id, channel, town, area, x, y)
}

// Despawn 撤走一个场景实体。
func Despawn(id ActorID) error {
	h, err := current()
	if err != nil {
		return err
	}
	return h.Despawn(id)
}

// PoseOf 读一个场景实体的姿态。
//
// 名字带 Of 是因为 `Pose` 已经被姿态结构用掉了（Go 不允许函数与类型同名），
// 顺带也把"读一个"和"这个东西"区分开了。
func PoseOf(id ActorID) (Pose, bool) {
	h, err := current()
	if err != nil {
		return Pose{}, false
	}
	return h.Pose(id)
}

// List 列出全部由 mod 造出来的场景实体。宿主未装载时返回 nil（与"一个都没有"
// 同形，调用方不必区分这两种）。
func List() []Snapshot {
	h, err := current()
	if err != nil {
		return nil
	}
	return h.List()
}

// Scenes 列出此刻有真实连接的场景。宿主未装载、或一个真人都没有时返回 nil。
func Scenes() []Scene {
	h, err := current()
	if err != nil {
		return nil
	}
	return h.Scenes()
}
