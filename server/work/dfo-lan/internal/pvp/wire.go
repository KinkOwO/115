// Package pvp 实现决斗场（PKC）的自由练习房间。
//
// 协议对照（来源：115 客户端 analysis/dumps/opcodes.tsv；90US 官方源码 pvp/wire.go）：
//
//	C→S（cmd 表）                S→C（noti 表）
//	50 MAKE_PVP_ROOM             41 PVP_ROOM_INFO    45 START_PVP
//	51 ENTER_PVP_ROOM            42 PVP_ROOM_STATE   46 DIE_PVP_CHARACTER
//	52 SET_PVP_SEAT_STATE        43 PVP_SEAT_STATE   47 END_PVP
//	53 SET_PVP_READY_STATE       44 PVP_READY_STATE  48 PVP_RECORD
//	54 SET_PVP_TEAM_MODE         49 REQ_PVP_RANK
//	59 SET_PVP_MAP_INDEX
//
// 应答走**同号（cmd 表）**，广播/推送走 **noti 表**——两者共用 16 位 ID 空间，
// 所以数字会撞车但语义独立（例如 43 既是 cmd GET_ITEM 又是 noti PVP_SEAT_STATE）。
//
// 实测真实 CMD50（2026-10-08 客户端实机，32 字节明文）：
//
//	00 | 11 00 00 00 | "In Arena Training" | 00 00 | 00 | 01 | 00 | 00 00 00 00 00
//	NameType  NameLen      Name(17 字节)      Map    Pwd  Special Flag  尾部 5 字节
//
// ⇒ 与 90US 的 ParseMake **同形**，但尾部多 5 字节。所以解析器按字段顺序消费，
// **不要求"读完即尽"**（90US 的 ParseMake 也没有尾部检查）。
package pvp

import (
	"encoding/binary"
	"errors"
)

var ErrRoom = errors.New("invalid pvp room operation")

// MakeRequest 是 CMD50 MAKE_PVP_ROOM 的请求体。
//
// SpecialMode：1 = 普通练习房间，3 = 街机（单人打 APC）。
// 实机 2026-10-08 客户端"创建房间"发的是 SpecialMode=1。
type MakeRequest struct {
	NameType    byte
	Name        []byte
	Map         uint16
	Password    []byte
	SpecialMode byte
	Flag        byte
}

type reader struct {
	b   []byte
	pos int
	bad bool
}

func (r *reader) take(n int) []byte {
	if n < 0 || n > len(r.b)-r.pos {
		r.bad = true
		return make([]byte, max(n, 0))
	}
	b := r.b[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *reader) u8() byte    { return r.take(1)[0] }
func (r *reader) u16() uint16 { return binary.LittleEndian.Uint16(r.take(2)) }

func (r *reader) str(limit int) []byte {
	n := binary.LittleEndian.Uint32(r.take(4))
	if n > uint32(limit) || n > uint32(len(r.b)-r.pos) {
		r.bad = true
		return nil
	}
	return append([]byte(nil), r.take(int(n))...)
}

func (r *reader) password() []byte {
	v := r.u8()
	if v > 1 {
		r.bad = true
	}
	if v == 1 {
		return r.str(8)
	}
	return nil
}

// ParseMake 解析 CMD50。尾部多出的字节按 115 客户端的实际形态忽略。
func ParseMake(b []byte) (MakeRequest, error) {
	r := reader{b: b}
	q := MakeRequest{NameType: r.u8()}
	if q.NameType > 16 {
		r.bad = true
	}
	if q.NameType == 0 {
		q.Name = r.str(127)
	}
	q.Map = r.u16()
	q.Password = r.password()
	q.SpecialMode = r.u8()
	q.Flag = r.u8()
	if r.bad {
		return q, ErrRoom
	}
	return q, nil
}

// ParseEnter 解析 CMD51 ENTER_PVP_ROOM：u16 房间号 + 密码字段，要求读完即尽。
func ParseEnter(b []byte) (uint16, []byte, error) {
	r := reader{b: b}
	id := r.u16()
	pw := r.password()
	if r.bad || r.pos != len(b) || id == 0 {
		return 0, nil, ErrRoom
	}
	return id, pw, nil
}

type writer []byte

func (w *writer) u8(v byte)    { *w = append(*w, v) }
func (w *writer) u16(v uint16) { *w = binary.LittleEndian.AppendUint16(*w, v) }
func (w *writer) u32(v uint32) { *w = binary.LittleEndian.AppendUint32(*w, v) }
func (w *writer) str(v []byte) { w.u32(uint32(len(v))); *w = append(*w, v...) }

func flag(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func seatUserID(s Seat) uint16 {
	if s.Owner.UserID == 0 {
		return uint16(EmptySeat)
	}
	return s.Owner.UserID
}

// RoomList 构造 noti41 PVP_ROOM_INFO。
//
// ⚠️ 2026-10-08 两次实机对照（结论：**旧结构可用，官方"空列表"形态不可用**）：
//
//  1. 官方抓包 `official_20261002`：noti41 有 10 次推送、**0 次 c2s 41**（⇒ 服务器主动推送），
//     明文恒为 **8 字节**：`00 00 a51f257d3f00`（跨 52 分钟完全一致 ⇒ 后缀不是时间戳）。
//  2. 但把本函数改成"恒发这 8 字节空列表"后，**业主侧建房直接失败**（"不能创建房间了"）。
//
// ⇒ **不能**用"空列表"形态去编码"有房间"的列表。旧结构（90 级形态）虽然未经官方样本证实，
// **但实测客户端接受它、建房/进房/座位/准备全流程都跑得通**，所以保留。
//
// 待闭环：官方那 8 字节的 6 字节后缀含义（疑似服务器会话/校验值）未知；
// 「有房间」的官方样本仍缺。**在此之前不要动这个结构** —— 动了就建房失败。
func RoomList(rooms []Room) []byte {
	return roomListWithRooms(rooms)
}

// roomListWithRooms 是当前生效的结构（源自 90 级，实测可用但未经官方样本证实）。
func roomListWithRooms(rooms []Room) []byte {
	w := writer{}
	w.u16(uint16(len(rooms)))
	for _, r := range rooms {
		w.u16(r.ID)
		w.u8(r.NameType)
		if r.NameType == 0 {
			w.str(r.Name)
		}
		w.u8(r.State)
		w.u8(r.Manager)
		w.u16(r.Map)
		w.u8(r.Mode)
		for _, s := range r.Seats {
			w.u8(s.State)
			w.u16(seatUserID(s))
		}
		w.u8(flag(len(r.Password) > 0))
		w.u32(0)
	}
	return w
}

// RoomState 构造 noti42 PVP_ROOM_STATE。
func RoomState(r Room) []byte {
	w := writer{}
	w.u16(r.ID)
	w.u8(r.State)
	w.u8(r.Manager)
	w.u16(r.Map)
	w.u8(r.Mode)
	w.u32(0)
	return w
}

// Seats 构造 noti43 PVP_SEAT_STATE。
func Seats(r Room) []byte {
	w := writer{}
	w.u16(r.ID)
	w.u8(r.Mode)
	w.u8(SeatCount)
	for i, s := range r.Seats {
		w.u8(byte(i))
		w.u8(s.State)
		w.u16(seatUserID(s))
	}
	return w
}

// EnterSuccess 构造 CMD51 的应答体：success 字节(1) + 8 个准备标志。
// 115 的应答统一走 send(kind=1, 同号, body)，首字节 1 = 成功（对照官方抓包
// s2c 44 的 `01` 前缀）。
func EnterSuccess(r Room) []byte {
	w := writer{1}
	for _, s := range r.Seats {
		w.u8(flag(s.Ready))
	}
	return w
}

// UserState 构造 noti3 用户状态（进房/离房时同步）。
func UserState(ids []Identity, state byte) []byte {
	w := writer{byte(len(ids))}
	for _, id := range ids {
		w.u16(id.UserID)
		w.u8(state)
	}
	return w
}

// RefusalBody 与 115 现有的 protocol.Refusal 同形：0 + u16 错误码。
// 错误码 19 来自 90US 的 pvp 拒绝路径（sendGameUpperRawClassCodec(..., {0,19})）。
func RefusalBody(code uint16) []byte {
	return binary.LittleEndian.AppendUint16([]byte{0}, code)
}
