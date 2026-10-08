package pvp

import (
	"bytes"
	"sort"
	"sync"
)

// 座位与房间状态常量（90US 同值）。
const (
	SeatCount  = 8
	Waiting    byte = 1
	Fighting   byte = 2
	EmptySeat  byte = 255
	ClosedSeat byte = 254
)

// 房间模式（Room.Mode）。
//
//	2 = 普通 PvP 房间；6 = 练习房间（PVPRoomInfo+0x88 == 6）；10 = 街机单人。
const (
	NormalMode   byte = 2
	PracticeMode byte = 6
	ArcadeMode   byte = 10
)

// Identity 标记一个会话。Generation 用来防止"旧连接迟到的包"操作到新连接上。
type Identity struct {
	UserID     uint16
	Generation uint64
}

type Seat struct {
	Owner   Identity
	State   byte
	Ready   bool
	Alive   bool
	Loaded  bool
	EndAck  bool
	TimedOut bool
}

type Room struct {
	ID       uint16
	Channel  int
	NameType byte
	Name     []byte
	Password []byte
	Map      uint16
	Mode     byte
	State    byte
	Manager  byte
	Seats    [SeatCount]Seat

	Finishing bool
	Draw      bool
}

func (r Room) SeatOf(who Identity) int {
	for i, s := range r.Seats {
		if s.Owner.UserID == who.UserID && who.UserID != 0 {
			return i
		}
	}
	return -1
}

func (r Room) Members() []Identity {
	out := []Identity{}
	for _, s := range r.Seats {
		if s.Owner.UserID != 0 {
			out = append(out, s.Owner)
		}
	}
	return out
}

func (r Room) clone() Room {
	c := r
	c.Name = bytes.Clone(r.Name)
	c.Password = bytes.Clone(r.Password)
	return c
}

// Manager 是同一个游戏进程里所有决斗场房间的持有者。零值可用。
type Manager struct {
	mu    sync.Mutex
	rooms map[uint16]Room
	next  uint16
}

// Snapshot 返回某频道下的全部房间（按 ID 升序），用于 noti41 房间列表。
func (m *Manager) Snapshot(channel int) []Room {
	m.mu.Lock()
	defer m.mu.Unlock()
	v := []Room{}
	for _, r := range m.rooms {
		if r.Channel == channel {
			v = append(v, r.clone())
		}
	}
	sort.Slice(v, func(i, j int) bool { return v[i].ID < v[j].ID })
	return v
}

// Find 返回 who 所在的房间。
func (m *Manager) Find(who Identity) (Room, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.findLocked(who)
}

func (m *Manager) findLocked(who Identity) (Room, bool) {
	for _, r := range m.rooms {
		if r.SeatOf(who) >= 0 {
			return r.clone(), true
		}
	}
	return Room{}, false
}

// Create 建一个普通房间（SpecialMode 必须已被归一为 0）。
func (m *Manager) Create(who Identity, channel int, req MakeRequest) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.createLocked(who, channel, req)
}

func (m *Manager) createLocked(who Identity, channel int, req MakeRequest) (Room, error) {
	if who.UserID == 0 || who.Generation == 0 || channel <= 0 || req.SpecialMode != 0 || req.Flag != 0 {
		return Room{}, ErrRoom
	}
	for _, r := range m.rooms {
		for _, s := range r.Seats {
			if s.Owner.UserID == who.UserID {
				return Room{}, ErrRoom
			}
		}
	}
	if m.rooms == nil {
		m.rooms = make(map[uint16]Room)
	}
	for tries := 0; tries < 65535; tries++ {
		m.next++
		if m.next == 0 {
			m.next++
		}
		if _, ok := m.rooms[m.next]; ok {
			continue
		}
		r := Room{
			ID:       m.next,
			Channel:  channel,
			NameType: req.NameType,
			Name:     bytes.Clone(req.Name),
			Password: bytes.Clone(req.Password),
			Map:      req.Map,
			Mode:     NormalMode,
			State:    Waiting,
		}
		for i := range r.Seats {
			r.Seats[i].State = EmptySeat
		}
		r.Seats[0] = Seat{Owner: who, State: Waiting}
		m.rooms[r.ID] = r
		return r.clone(), nil
	}
	return Room{}, ErrRoom
}

// CreatePractice 建练习房间：房主坐 0 号位，1..7 号位对客户端显示为"关闭"。
// 客户端自己的 APC 是**本地**的，不是第二个认证玩家，所以不占座位。
func (m *Manager) CreatePractice(who Identity, channel int, req MakeRequest) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if req.SpecialMode != 1 || req.Flag != 0 || len(req.Password) != 0 {
		return Room{}, ErrRoom
	}
	normal := req
	normal.SpecialMode = 0
	r, err := m.createLocked(who, channel, normal)
	if err != nil {
		return Room{}, err
	}
	r.Mode = PracticeMode
	for i := 1; i < SeatCount; i++ {
		r.Seats[i].State = ClosedSeat
	}
	m.rooms[r.ID] = r
	return r.clone(), nil
}

// Join 进一个已有的普通房间。
func (m *Manager) Join(who Identity, channel int, id uint16, password []byte) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if who.UserID == 0 || who.Generation == 0 {
		return Room{}, ErrRoom
	}
	for _, r := range m.rooms {
		for _, s := range r.Seats {
			if s.Owner.UserID == who.UserID {
				return Room{}, ErrRoom
			}
		}
	}
	r, ok := m.rooms[id]
	if !ok || r.Mode == PracticeMode || r.Mode == ArcadeMode || r.Channel != channel ||
		r.State != Waiting || !bytes.Equal(password, r.Password) {
		return Room{}, ErrRoom
	}
	for i, s := range r.Seats {
		if s.State == EmptySeat {
			r.Seats[i] = Seat{Owner: who, State: Waiting}
			m.rooms[id] = r
			return r.clone(), nil
		}
	}
	return Room{}, ErrRoom
}

// Leave 让 who 离开房间。返回（房间快照, 是否真的离开了）。
// 房间空了就删除，房主离开则顺移给下一个非空座位。
func (m *Manager) Leave(who Identity) (Room, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.leaveLocked(who)
}

func (m *Manager) leaveLocked(who Identity) (Room, bool) {
	r, ok := m.findLocked(who)
	if !ok {
		return Room{}, false
	}
	seat := r.SeatOf(who)
	r.Seats[seat] = Seat{State: EmptySeat}
	if len(r.Members()) == 0 {
		delete(m.rooms, r.ID)
		r.State = 0
		return r, true
	}
	if int(r.Manager) == seat {
		for i, s := range r.Seats {
			if s.Owner.UserID != 0 {
				r.Manager = byte(i)
				break
			}
		}
	}
	if r.State == Fighting {
		r.Finishing = true
		r.Draw = true
	}
	m.rooms[r.ID] = r
	return r.clone(), true
}

// SetSeat 设置座位状态（CMD52）。
func (m *Manager) SetSeat(who Identity, index, state byte) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.findLocked(who)
	if !ok || r.Mode == ArcadeMode || index >= SeatCount {
		return Room{}, ErrRoom
	}
	actor := r.SeatOf(who)
	target := r.Seats[index]
	if actor != int(r.Manager) && actor != int(index) {
		return Room{}, ErrRoom
	}
	if r.State != Waiting && !(int(index) == actor && state == EmptySeat) {
		return Room{}, ErrRoom
	}
	if target.Owner.UserID != 0 && state == EmptySeat {
		next, _ := m.leaveLocked(target.Owner)
		return next, nil
	}
	if target.Owner.UserID == 0 {
		if state != EmptySeat && state != ClosedSeat {
			return Room{}, ErrRoom
		}
	} else if state > 4 {
		return Room{}, ErrRoom
	}
	r.Seats[index].State = state
	for i := range r.Seats {
		r.Seats[i].Ready = false
	}
	m.rooms[r.ID] = r
	return r.clone(), nil
}

// SetMode 切队伍/对战模式（CMD54，仅房主、仅等待中）。
//
// 允许 1..4：115 客户端实机发过 1 和 3（90US 只认 1/2，那是 90 客户端的取值范围）。
// 注意别和 PracticeMode(6)/ArcadeMode(10) 撞车 —— 那两个是服务端内部模式，不由 CMD54 设置。
func (m *Manager) SetMode(who Identity, mode byte) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.findLocked(who)
	if !ok || r.Mode == ArcadeMode || r.Mode == PracticeMode || r.State != Waiting ||
		r.SeatOf(who) != int(r.Manager) || mode < 1 || mode > 4 {
		return Room{}, ErrRoom
	}
	r.Mode = mode
	for i := range r.Seats {
		r.Seats[i].Ready = false
	}
	m.rooms[r.ID] = r
	return r.clone(), nil
}

// SetMap 选地图（CMD59，仅房主、仅等待中）。
func (m *Manager) SetMap(who Identity, index uint16) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.findLocked(who)
	if !ok || r.Mode == ArcadeMode || r.State != Waiting || r.SeatOf(who) != int(r.Manager) {
		return Room{}, ErrRoom
	}
	r.Map = index
	for i := range r.Seats {
		r.Seats[i].Ready = false
	}
	m.rooms[r.ID] = r
	return r.clone(), nil
}

// Ready 设置准备状态（CMD53）。返回（房间, 是否因此开打, 错误）。
// 开打条件：所有非空座位都已准备。
func (m *Manager) Ready(who Identity, value bool) (Room, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.findLocked(who)
	if !ok || r.Mode == ArcadeMode || r.State != Waiting {
		return Room{}, false, ErrRoom
	}
	seat := r.SeatOf(who)
	if r.Seats[seat].State >= 3 {
		return Room{}, false, ErrRoom
	}
	r.Seats[seat].Ready = value
	started := false
	if value {
		started = true
		occupied := 0
		for _, s := range r.Seats {
			if s.Owner.UserID == 0 {
				continue
			}
			occupied++
			if !s.Ready {
				started = false
			}
		}
		if occupied == 0 {
			started = false
		}
	}
	if started {
		r.State = Fighting
		for i := range r.Seats {
			if r.Seats[i].Owner.UserID != 0 {
				r.Seats[i].Alive = true
			}
		}
	}
	m.rooms[r.ID] = r
	return r.clone(), started, nil
}

// Loaded 标记客户端完成加载（CMD298）。
func (m *Manager) Loaded(who Identity) (Room, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.findLocked(who)
	if !ok {
		return Room{}, ErrRoom
	}
	seat := r.SeatOf(who)
	r.Seats[seat].Loaded = true
	m.rooms[r.ID] = r
	return r.clone(), nil
}
